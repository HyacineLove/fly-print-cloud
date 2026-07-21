# FlyPrint Cloud 运行、接口与验证说明

## 3. 后端启动与依赖注入

入口为 `api/cmd/server/main.go`。启动顺序是：加载配置并校验 -> 连接 PostgreSQL -> `DB.InitTables()` -> 可选创建默认管理员 -> 创建 repositories/services -> 启动 token、文件、过期任务和 pending 重试任务 -> 创建 WebSocket manager -> 创建 Gin 路由并监听。

配置加载规则：Viper 查找当前目录、`./configs`、`/etc/fly-print-cloud` 下的 `config.yaml`；所有 `FLY_PRINT_` 环境变量覆盖 YAML（点号转下划线）。Compose 不把 `config.yaml` 放进镜像，主要从根目录 `.env` 注入环境变量。独立运行时应复制 `api/config.example.yaml` 为 `api/config.yaml`。

数据库没有独立版本迁移目录，`api/internal/database/database.go` 的 `InitTables` 会创建/补齐：`users`、`edge_nodes`、`printers`、`print_jobs`、`token_usage_records`、`system_settings`、`files`、`oauth2_clients` 及索引/更新时间触发器。修改 schema 时必须考虑既有实例、幂等性、数据兼容和回滚；不要把临时 SQL 直接依赖在某次启动顺序上。

后台任务：

- 每小时清理过期 token 使用记录；
- 每小时清理创建超过 24 小时且满足条件的文件及物理对象；正式部署前复核业务留存要求；
- 每 30 分钟把超过 30 分钟未更新的“打印中”任务标记为失败；
- 每 5 分钟检查创建超过 3 分钟的 pending 任务，在节点在线、打印机启用且未超过 `MaxRetries` 时重发。

## 4. HTTP 路由与权限

路由权威在 `setupRoutes`（`api/cmd/server/main.go`）。所有 `/api/v1` 业务接口都经全局限流（内存 store，10 req/s）、日志、Recovery、CORS 和安全头中间件；Nginx 也对 API/Auth 做限流。

公开/认证入口：

- `GET|HEAD /health`：基础健康检查；`GET|HEAD /api/v1/health`：数据库/WebSocket 等详细健康检查。
- `/swagger/*any`：Swagger UI。
- `/auth/mode`、`/auth/token`、`/auth/login`、`/auth/callback`、`/auth/userinfo`、`/auth/me`、`/auth/verify`、`/auth/logout`：认证相关入口；`token` 主要用于 builtin，Keycloak 走登录/回调。

业务路由：

- `/api/v1/admin/dashboard/trends`：`fly-print-admin` 或 `fly-print-operator`。
- `/api/v1/admin/users/*`：仅 `fly-print-admin`；当前前端 Users 页面占位，但 API 已实现。
- `/api/v1/admin/profile`：任意已认证用户。
- `/api/v1/admin/business-settings`：仅 `fly-print-admin`。
- `/api/v1/admin/edge-nodes/*`、`/admin/printers/*`、`/admin/print-jobs/*`：admin/operator；后台不创建打印任务，只查询、更新、取消、删除。
- `/api/v1/admin/oauth2-clients/*`：仅 builtin 模式注册，且仅 admin。
- `POST|GET /api/v1/print-jobs`：第三方 `print:submit`；创建任务会校验打印机/节点状态和打印能力。
- `GET /api/v1/printers`：`print:submit`。
- `POST /api/v1/edge/register`：`edge:register`；节点心跳不再走 HTTP。
- `/api/v1/edge/:node_id/printers*`：`edge:printer`；注册/删除受节点启用检查，批量状态上报允许禁用打印机，但仍要求节点身份。
- `GET /api/v1/edge/ws?node_id=...`：Edge WebSocket，需 OAuth2 认证并绑定 node ID。
- `/api/v1/files/*`：上传/下载凭证或 OAuth2 可选认证；包括 `upload-policy`、`verify-upload-token`、`preflight`、上传和按 ID 下载。

OAuth2 scope 校验在 `api/internal/middleware/oauth2.go`：Bearer token 可解析 JWT 或回退调用 UserInfo；角色来源包含 groups、roles、Keycloak realm/client roles 和 scope；`admin` 角色拥有全部 scope；传入多个 scope 是 AND 逻辑。改认证时务必审查令牌签名/issuer/audience 校验、角色映射和 builtin/Keycloak 两种模式。

## 5. 文件、上传和打印链路

文件记录保存原名、MIME、大小、SHA-256、上传者以及 storage provider/bucket/object key；物理内容由 storage service 管理。公共上传通常由 Edge 请求一次性 upload token，前端 `/upload?token=...&node_id=...&printer_id=...` 先获取策略并验证会话，再执行 preflight 和上传。下载凭证同样短期、签名、按资源绑定，并在 `token_usage_records` 中支持预登记、撤销和一次性消费。

打印任务典型链路：第三方创建任务 -> Cloud 检查打印机/Edge 启用、WebSocket 在线和 color/duplex/paper 能力 -> 写入 pending/dispatched 状态并下发 `print_job` -> Edge ACK/回传 `job_update` -> Cloud 更新状态；网络暂不可用或 ACK 未完成时由后台重试，超出次数失败。取消/节点禁用/打印机禁用时要注意 token 撤销和任务状态的一致性。

存储后端切换使用 `api/cmd/migrate-files`，该工具按数据库中的 local 文件记录读本地文件、写 MinIO、更新 metadata；生产执行前必须备份 PostgreSQL 和文件/对象存储，并在隔离环境验证。只备份数据库或只备份对象存储都不能完整恢复文件关系。

## 8. 开发、测试、部署命令

```powershell
# 完整 Compose
Copy-Item .env.example .env
docker compose up --build -d
docker compose ps
docker compose logs -f api nginx
docker compose down

# API
Set-Location api
go run ./cmd/server
go test ./...

# Admin（PowerShell 中优先使用 npm.cmd，避免 npm.ps1 执行策略问题）
Set-Location admin
npm.cmd ci
npm.cmd test -- --watchAll=false --runInBand
npm.cmd run build

# 已启动 Cloud/Edge 后的运行中检查
python api/tests/cloud_smoke_test.py
python api/tests/cloud_api_perf.py
```

Smoke/performance 脚本不是离线单元测试，依赖运行中的 Cloud，并可能读取同级工作区 `fly-print-edge/config.json`。发布前至少做健康检查、登录、扫码/公共上传、预览、打印、状态回传、Edge 重连、重复消息和重复文件验证。

## 10. 当前验证记录

在本次生成文档前完成源码阅读并执行：

- `admin`: `npm.cmd test -- --watchAll=false --runInBand`，2 个测试套件、7 个测试通过；存在 React act/React Router 警告。
- `admin`: `npm.cmd run build` 通过；存在未使用变量、Hook 依赖、无效 anchor 和 bundle 体积警告。
- `api`: 各主要 package 测试实际通过，但 `go test ./...` 最终因 Windows Go build cache 文件 `Access is denied` 在 setup 阶段退出非零；重试仍复现，属于当前本机测试缓存/权限问题，不能据此断言业务测试失败。

若本文件与源码冲突，以当前源码为准，并在修复后同步更新本文件；若说明过长需要拆文档，必须在此处增加明确的“详见 `相对路径`”链接，方便上级统筹 Agent 按需加载。
