# FlyPrint Cloud Agent Guide

本文件是 `fly-print-cloud` 子项目的 Agent 入口说明，也供上级 `FlyPrint` 工作区的统筹 Agent 阅读。先读本文件，再根据任务进入 `api/`、`admin/`、`nginx/` 或 `docker-compose.yml`。项目的用户可见启动与部署说明详见 [README.md](README.md)；跨仓开发计划、进度和非技术使用说明位于上级工作区，详见 README 末尾列出的文件。

## 1. 项目定位

FlyPrint Cloud 是 FlyPrint 的云端控制面和文件服务，不直接访问现场打印机。它负责：

- 用户/第三方/Edge 的 OAuth2 认证与权限校验；
- Edge 节点注册、启用/禁用、在线状态和系统信息管理；
- 打印机注册、能力/状态同步和管理；
- 文件上传、预检、下载、内容哈希和短期访问凭证；
- 打印任务创建、能力校验、WebSocket 下发、ACK/状态回传、取消和有限重试；
- 管理端 Dashboard、Edge Nodes、Printers、Print Jobs、OAuth2 Clients、Business Settings 和公共上传页。

现场打印机发现、文档预览和实际打印由 FlyPrint Edge 完成。Cloud 与 Edge 的边界协议以 Go 路由、请求模型和 `api/internal/websocket/message.go` 为权威；Swagger 当前没有完整覆盖所有新接口，不能仅凭 Swagger 判断完整契约。

当前交付状态是“核心业务闭环已实现，工程化仍需补齐”：Users/Settings 前端仍为占位；Dashboard 失败时可能显示模拟趋势；没有持续集成；Cloud/Edge 真实端到端、断线恢复和升级兼容测试尚未成为发布门禁；数据库仍由启动时建表/补列；Compose 中 MinIO 使用 `latest`。

## 2. 技术栈与运行形态

- 后端：Go 1.25、Gin、`database/sql` + PostgreSQL、Gorilla WebSocket、Viper、Zap、Swagger。
- 认证：`builtin` 内置账号/JWT/OAuth2，或 `keycloak` 外部 OAuth2/OIDC。
- 存储：`local` 本地文件系统或 `minio`；下载模式支持 `proxy`/`presigned`，当前建议 `proxy`。
- 前端：Node 18 基线、React 18、TypeScript、React Router 6、Ant Design、ECharts、Create React App。
- 部署：Docker Compose 编排 PostgreSQL、MinIO、API、前端构建器和 Nginx；默认统一入口 `http://127.0.0.1:8012`。

### 已确认的生产部署边界

- Cloud 部署在受控的 Linux 服务器上，负责统一认证、任务、文件、对象存储和云边通信；它不是现场打印或终端维护入口。
- 每套 Edge 现场设备是“一台终端工控 PC + 一台直连打印机”的一体机。Cloud 应将 Edge 视为独立终端，并以独立、可验证的设备身份绑定 `node_id`；不得依赖 MAC、共享 OAuth Client 或请求参数来推断终端身份。
- Edge 的本地用户页在正常运行时由 kiosk 锁定；运维维护需要打开一体机带锁后门并在 PC 上接入键鼠，钥匙由运维保管。因此 Edge 本地管理页属于受物理访问控制保护的维护面，而不是面向局域网/公网的管理服务。
- 任何修改只要会让 Edge 改为非回环监听、经反向代理暴露、允许远程维护或使非运维人员能进入本地管理页，就改变了上述安全边界；必须先在对话中说明并获得确认，同时补充相应网络/应用层鉴权方案。

主要目录：

```text
api/
  cmd/server/          API 入口、路由、后台定时任务
  cmd/migrate-files/   local -> MinIO 文件迁移工具
  internal/auth/       内置认证服务
  internal/business/   动态业务设置
  internal/config/     Viper 配置加载与校验
  internal/database/   PostgreSQL 连接、启动时 schema、repository
  internal/handlers/   HTTP handler
  internal/middleware/ OAuth2、CORS、安全头、Edge 启用检查
  internal/models/     API/数据库模型
  internal/security/   一次性文件凭证和 token 使用记录
  internal/storage/    local/MinIO 抽象和实现
  internal/websocket/  Cloud <-> Edge 长连接与消息分发
  tests/               运行中环境的 smoke/performance 脚本
admin/                 React 管理端和公共上传页
nginx/                 统一入口、静态文件、API/Auth/WS 反代
docker-compose.yml     完整容器编排
```

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

## 6. WebSocket 云边协议

Cloud 到 Edge 的主要 command：`print_job`、`preview_file`、`upload_token`、`node_state`、`config_update`、`report_status`、`error`；Edge 到 Cloud 的 message：`edge_heartbeat`、`job_update`、`submit_print_params`、`request_upload_token`、`ack`。消息/命令包含 type、node、时间戳及 data，command 另有 `command_id` 和通信层 `msg_id`；ACK 需要回填对应 ID。

涉及文件的 payload 带 `content_hash` 和短期 `file_access_token`。当前 Edge 将 hash 当作缓存键并校验格式，但尚未对下载字节重新计算 SHA-256，因此不要把它宣传为完整端到端完整性校验。修改消息结构或状态时要同步检查 `message.go`、`connection.go`、`manager.go`、Cloud handler，以及同级工作区 Edge consumer/provider 测试。

## 7. 前端结构

`admin/src/App.tsx` 负责认证状态、菜单和路由：`/` Dashboard、`/edge-nodes`、`/printers`、`/print-jobs`、`/users`、`/oauth2-clients`、`/business-settings`、`/settings`、`/upload`、`/login`。API 请求集中在 `admin/src/services/api.ts`，路径拼接在 `admin/src/config.ts`，公共上传辅助在 `admin/src/services/upload.ts`。

前端通过 `/auth/me` 判断登录；builtin 登录得到 token 后写入 `access_token` cookie，API 请求以 Bearer 方式发送。Compose 同域部署默认 `REACT_APP_API_BASE_PATH=/api/v1`、`REACT_APP_AUTH_BASE_PATH=/auth`；修改反代前缀时必须同时检查 `.env.example`、Compose build args、`config.ts` 和 `nginx/conf.d/admin.conf`。

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

## 9. Agent 改动规则与风险清单

- 先定位路由、请求模型、handler、repository 和前端调用的完整链路，再修改单点；不要只改 Swagger 或前端类型。
- 新增/修改数据库字段必须在 `InitTables` 兼容旧实例，并补 repository、handler、测试和数据清理逻辑；长期方案应引入有版本/回滚的迁移工具。
- 新增/修改 Cloud-Edge 协议必须同时更新消息结构、序列化测试、Cloud provider test 和 Edge consumer test。
- 不要把默认密码、JWT 签名密钥、文件访问密钥、MinIO 密钥带入提交或生产环境；`.env.example` 仅是模板。
- 不要在保留数据的环境执行 `docker compose down -v`；它会删除命名卷。
- 生产固定 MinIO 镜像版本，明确 HTTPS 下的 `COOKIE_SECURE`、CORS、外部 URL 和 Keycloak issuer。
- 注意当前 `api/internal/middleware/oauth2.go` 的 JWT 解析/外部 UserInfo 双路径，任何安全相关改动都必须重新核对真实签名验证和 issuer 配置。
- 保留工作区已有改动；本次调查时 Git 已存在 `docs/superpowers` 两个删除项以及 `.claude/`、`README.md` 未跟踪项，除非用户明确要求，不要恢复、删除或重写它们。
- 仅新增文档时不要顺手格式化或重构业务代码；提交前检查 `git status --short` 和 `git diff -- AGENTS.md`。

## 10. 当前验证记录

在本次生成文档前完成源码阅读并执行：

- `admin`: `npm.cmd test -- --watchAll=false --runInBand`，2 个测试套件、7 个测试通过；存在 React act/React Router 警告。
- `admin`: `npm.cmd run build` 通过；存在未使用变量、Hook 依赖、无效 anchor 和 bundle 体积警告。
- `api`: 各主要 package 测试实际通过，但 `go test ./...` 最终因 Windows Go build cache 文件 `Access is denied` 在 setup 阶段退出非零；重试仍复现，属于当前本机测试缓存/权限问题，不能据此断言业务测试失败。

若本文件与源码冲突，以当前源码为准，并在修复后同步更新本文件；若说明过长需要拆文档，必须在此处增加明确的“详见 `相对路径`”链接，方便上级统筹 Agent 按需加载。
