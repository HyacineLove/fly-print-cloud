# FlyPrint Cloud 架构、技术栈与协议说明

本文件是 `fly-print-cloud` 子项目的 Agent 入口说明，也供上级 `FlyPrint` 工作区的统筹 Agent 阅读。先读本文件，再根据任务进入 `api/`、`admin/`、`nginx/` 或 `docker-compose.yml`。项目的用户可见启动与部署说明详见 [README.md](README.md)；跨仓开发计划、进度和非技术使用说明位于上级工作区，详见 README 末尾列出的文件。

## Edge terminal-result protocol (2026-07-21)

- `print_job` identifies a physical target by `job_id`, `printer_id`, `file_url`, and `content_hash`. `printer_name` is not part of the wire contract and must never be used as a routing fallback.
- Cloud keeps the existing three-attempt delivery ACK loop. That ACK means Edge durably recorded the incoming job; it does not mean printing finished.
- Edge sends `job_update` for real-time `processing` state only on a best-effort basis. Terminal states (`completed`, `failed`, `canceled`, `unconfirmed`) require a stable UUID `event_id`.
- Cloud persists the terminal job update and `edge_job_update_receipts` record in one transaction, after verifying that the authenticated WebSocket node owns the target printer. It replies with `job_update_ack`: `accepted` permits Edge to remove its local outbox item; `rejected` is a protocol error and Edge must retain it as a visible local communication fault without retrying.
- An identical terminal `event_id` is idempotently accepted; reuse with different node, job, status, or payload hash is rejected. Terminal status is monotonic except that `unconfirmed/dispatch_ack_timeout` can later be replaced by a real terminal result.

## Node deletion and interactive preview repair (2026-07-21)

- Admin node deletion is a historical-preserving operation: the node and its printers are soft-deleted, while historical print jobs, tickets, integration requests, and callback records remain queryable. Active tickets and non-terminal integration requests are cancelled in the same transaction; ephemeral upload-session mappings and alerts are removed before commit, then the node WebSocket is disconnected.
- `terminal_tickets.printer_id` is intentionally retained as a foreign-key reference to the soft-deleted printer. Hard-deleting printers during node removal is prohibited.
- A `waiting_terminal` integration preview may bind to the current Cloud/Edge session when `node_id` and `terminal_session_id` match and the stored Edge ticket hash is still NULL. Once Edge binds the first preview, a non-empty ticket hash is required to match exactly on later events.
- Edge preview binding must report one `terminal_session_state` after binding the ticket and integration request. It must not create a standard `print_job` before the user confirms print parameters.

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

### 二维码公开地址

- 用户页只使用 Edge 的 `/api/qr_code`。Cloud 通过原 WebSocket 凭证响应返回相对 `/entry?token=...` 路径，Edge 使用自身 `cloud.base_url` 拼接，并将其中的 `localhost`/`127.0.0.1` 改写为 Edge 本机局域网 IP。
- `/entry` 校验原上传凭证后签发独立终端票据，展示官方与已启用第三方入口；选择官方后再生成新的上传凭证并进入 `/upload`，选择第三方时只传终端票据，绝不传上传凭证。
- 该二维码链路不依赖 `EXTERNAL_API_URL` 生成绝对地址，因此 Compose 默认 `EXTERNAL_API_URL=http://localhost:8012` 不会导致局域网手机二维码失效。不得再增加第二个 Edge 二维码接口。

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

## 6. WebSocket 云边协议

Cloud 到 Edge 的主要 command：`print_job`、`preview_file`、`upload_token`、`node_state`、`config_update`、`report_status`、`error`；Edge 到 Cloud 的 message：`edge_heartbeat`、`job_update`、`submit_print_params`、`request_upload_token`、`ack`。消息/命令包含 type、node、时间戳及 data，command 另有 `command_id` 和通信层 `msg_id`；ACK 需要回填对应 ID。

涉及文件的 payload 带 `content_hash` 和短期 `file_access_token`。当前 Edge 将 hash 当作缓存键并校验格式，但尚未对下载字节重新计算 SHA-256，因此不要把它宣传为完整端到端完整性校验。修改消息结构或状态时要同步检查 `message.go`、`connection.go`、`manager.go`、Cloud handler，以及同级工作区 Edge consumer/provider 测试。

## 7. 前端结构

`admin/src/App.tsx` 负责认证状态、菜单和路由：`/` Dashboard、`/edge-nodes`、`/printers`、`/print-jobs`、`/users`、`/oauth2-clients`、`/business-settings`、`/settings`、`/upload`、`/login`。API 请求集中在 `admin/src/services/api.ts`，路径拼接在 `admin/src/config.ts`，公共上传辅助在 `admin/src/services/upload.ts`。

前端通过 `/auth/me` 判断登录；builtin 登录得到 token 后写入 `access_token` cookie，API 请求以 Bearer 方式发送。Compose 同域部署默认 `REACT_APP_API_BASE_PATH=/api/v1`、`REACT_APP_AUTH_BASE_PATH=/auth`；修改反代前缀时必须同时检查 `.env.example`、Compose build args、`config.ts` 和 `nginx/conf.d/admin.conf`。

## 11. 通用第三方交互式打印与 Demo（2026-07-21）

- 第三方文件接管完成后不会直接创建 `print_job`。`integration/terminal_dispatcher.go` 先下发标准 `preview_file`，其中可选携带 `terminal_session_id`、`terminal_ticket_hash`、`integration_request_id` 和第三方建议的 `print_options`。
- Edge 用户最终确认后通过 `submit_print_params` 原样回传三项集成上下文；Cloud 在同一事务校验请求、节点、打印机、文件、票据及活动会话，并且只允许为一个集成请求创建一个标准任务。官方上传仍走原有分支，不要求集成字段。
- `integration_providers.allow_private_file_hosts` 默认关闭。开启后也只允许 `allowed_file_hosts` 精确列出的主机，并继续拒绝环回、链路本地、未指定、多播和保留地址；禁止将此开关作为全局私网放行。
- `integration-demo/` 是独立测试服务，provider code 固定为 `livacloud-demo`，Compose 服务名为 `integration-demo`，经 Nginx `/integration-demo/` 暴露。它模拟 SSO、PDF 提交、HMAC 请求、状态查询和幂等 callback，不允许在核心打印链路增加 provider 专属分支。
- Demo 双向密钥由 Cloud 创建/轮换时一次性生成，再粘贴到 `/integration-demo/setup`；设置页保存后不回显，日志不得输出密钥、签名、终端票据或文件 URL。`livacloud-demo` 默认配置允许访问精确 Docker 主机 `integration-demo`，正式 provider 不继承此策略。
- 本机启动方式仍为 `docker compose up -d --build`。2026-07-21 已完成 API、Admin、Demo 镜像重建，`livacloud-demo` 已配置并启用，原占位 `livacloud` 已禁用；未推送外部镜像仓库。
- 本轮验证：API `go test ./...` 全部通过；Admin 4 个套件、10 项测试通过且生产构建成功；Demo `go test ./...` 通过。现场完整扫码和实际 Edge 打印由安装包部署后验收。
