# Fly Print Cloud API — 开发交接说明

本文档面向接手本 API 的下一任开发，汇总运行方式、目录结构、配置、核心流程与注意事项。

---

## 1. 项目与职责

- **定位**：云打印系统的后端 API 服务，为管理控制台、边缘节点与第三方提供 REST 与 WebSocket 能力。
- **主要能力**：
  - 管理端：用户、边缘节点、打印机、打印任务、OAuth2 客户端（builtin 模式）的 CRUD 与看板数据。
  - 边缘节点：注册、WebSocket 长连接、心跳、任务接收与状态上报、上传/下载凭证下发。
  - 认证：OAuth2 内置模式（用户名密码 + JWT）或 Keycloak 外接。
  - 文件：上传/下载、一次性 upload/download token、预检与页数校验（PDF/DOCX）。

---

## 2. 技术栈与依赖

- **Go**：1.25（见 `go.mod`）。
- **主要依赖**：
  - `github.com/gin-gonic/gin`：HTTP 路由与中间件。
  - `github.com/spf13/viper`：配置（YAML + 环境变量）。
  - `go.uber.org/zap`：结构化日志（`internal/logger`）。
  - `github.com/lib/pq`：PostgreSQL 驱动。
  - `github.com/gorilla/websocket`：边缘节点 WebSocket。
  - `github.com/golang-jwt/jwt/v5`、`golang.org/x/oauth2`：JWT 与 OAuth2。
  - `github.com/swaggo/swag` + `gin-swagger`：Swagger 文档（`/swagger/*`）。
  - `github.com/ulule/limiter/v3`：全局限流（如 10 req/s）。
  - `github.com/pdfcpu/pdfcpu`：PDF 页数校验（`internal/utils`）。

---

## 3. 目录结构

```
api/
├── cmd/server/
│   └── main.go              # 入口：配置加载、DB/WS/Handler 装配、路由、后台任务、优雅退出
├── internal/
│   ├── config/              # 配置结构体与 Load/Validate，Viper 读 config.yaml + FLY_PRINT_* 环境变量
│   ├── database/            # DB 连接、InitTables、CreateDefaultAdmin、CreateDefaultOAuth2Client、各 Repository
│   ├── models/              # 领域模型（models.go, file.go, oauth2_client.go）
│   ├── handlers/             # HTTP 处理器（含 common/errors/response 与各 *_handler.go）
│   ├── middleware/          # CORS、OAuth2 资源服务器、EdgeNode/Printer 启用检查
│   ├── auth/                # 内置认证 BuiltinAuthService（builtin 模式）
│   ├── security/            # TokenManager（上传/下载一次性凭证）、密码强度校验
│   ├── websocket/           # WebSocket 升级、ConnectionManager、Connection、消息类型与 ACK
│   ├── logger/              # Zap 封装，全局 Info/Error/Warn/Debug/Fatal
│   └── utils/               # 文档校验（PDF/DOCX 页数等）
├── docs/                    # Swag 生成的 Swagger（docs.go）
├── config.example.yaml      # 配置模板，独立部署时复制为 config.yaml
├── Dockerfile               # 多阶段构建，最终镜像无 config.yaml
├── .dockerignore            # 排除 config.yaml、uploads、*.log，保证 compose 不用 api 本地配置
├── go.mod / go.sum
└── README.md                # 本文档
```

---

## 4. 配置与运行方式

### 4.1 两种部署方式

| 场景 | 配置来源 | 说明 |
|------|----------|------|
| **独立部署**（在 api 目录下 build/run） | `api/config.yaml` | 将 `config.example.yaml` 复制为 `config.yaml` 并修改。Viper 读取路径：当前工作目录、`./configs`、`/etc/fly-print-cloud`。 |
| **Docker Compose**（在 cloud 根目录） | **不使用** api 下 config | 镜像通过 `.dockerignore` 不包含 `config.yaml`，完全由 **cloud 根目录 `.env`** 与 compose 的 `environment` 注入 `FLY_PRINT_*` 等变量。 |

### 4.2 环境变量覆盖

- 所有配置项均可通过环境变量覆盖，前缀为 `FLY_PRINT`，键名与 config 层级对应（点号改为下划线），例如：
  - `FLY_PRINT_DATABASE_HOST`
  - `FLY_PRINT_OAUTH2_JWT_SIGNING_SECRET`
  - `FLY_PRINT_SECURITY_FILE_ACCESS_SECRET`
- 独立部署时：可只配 `config.yaml`，或只设环境变量，或混用（环境变量优先）。

### 4.3 配置校验

- `config.Validate()` 在启动时执行：必填项、端口范围、OAuth2 模式、生产环境下默认密钥检查、JWT/文件密钥最小长度（如 32 字符）等。

---

## 5. 启动与入口逻辑（main.go）

1. **加载配置**：`config.Load()` → `Validate()`。
2. **日志**：`logger.Init(cfg.App.Debug)`，defer `logger.Sync()`。
3. **数据库**：`database.New` → `InitTables()` → 可选 `CreateDefaultAdmin()`。
4. **builtin 模式**：若为 builtin，则创建 `OAuth2ClientRepository`、`BuiltinAuthService`，并执行 `CreateDefaultOAuth2Client()`。
5. **依赖组装**：各 Repository、`TokenManager`、`ConnectionManager`、`WebSocketHandler`、各 HTTP Handler。
6. **后台任务**（goroutine）：
   - 文件清理：每小时删除超过 1 天的文件记录与物理文件。
   - 过期任务清理：每 30 分钟将超时未更新的“打印中”任务标为失败。
   - Token 使用记录清理：每小时清理过期 token 记录。
   - Pending 任务重试：每 5 分钟对创建超过 3 分钟的 pending 任务尝试重新下发。
7. **路由**：`setupRoutes` 注册所有路由；启动 HTTP 服务；监听 SIGINT/SIGTERM 做优雅退出。

---

## 6. 路由与分层

- **分层**：Handler → Repository，无单独 Service 层；Handler 内可直接用多个 Repo、WebSocket Manager、TokenManager。
- **路由分组**（详见 `cmd/server/main.go` 的 `setupRoutes`）：
  - `/swagger/*`：Swagger UI。
  - `/health`、`/api/v1/health`：健康检查。
  - `/auth/*`：OAuth2（mode、token、userinfo、login、callback、me、verify、logout）。
  - `/api/v1/admin/*`：管理端（dashboard、users、profile、edge-nodes、printers、print-jobs、oauth2-clients），需对应 scope。
  - `/api/v1/print-jobs`（POST/GET）、`/api/v1/printers`（GET）：第三方打印 API，需 `print:submit`。
  - `/api/v1/edge/*`：边缘节点注册、打印机注册/删除、状态上报、WebSocket `/ws`，需 `edge:*` scope。
  - `/api/v1/files/*`：文件验证、预检、上传、下载，支持 token 或 OAuth2。

**中间件**：全局限流、Logger、Recovery、CORS、SecurityHeaders；部分路由使用 `OAuth2ResourceServer(scope...)`、`EdgeNodeEnabledCheck`、`OptionalOAuth2ResourceServer`。

---

## 7. 认证与权限

- **OAuth2 模式**：
  - **builtin**：内置登录页跳转、`/auth/token`（用户名密码等）、`/auth/userinfo`（JWT 解析），用户与 OAuth2 客户端存在 DB。
  - **keycloak**：使用外部 Keycloak 的 auth/token/userinfo 端点，API 仅做代理与 session/cookie 处理。
- **Scope/角色**（与路由对应）：
  - `fly-print-admin`：管理员（用户、OAuth2 客户端、全部管理接口）。
  - `fly-print-operator`：运营（边缘节点、打印机、任务、看板，不可管用户与 OAuth2 客户端）。
  - `edge:register`、`edge:printer`、`edge:heartbeat`：边缘节点注册、打印机管理、心跳/状态。
  - `print:submit`：第三方提交打印任务、拉取打印机列表。
  - `file:read`：文件读取权限（与 OAuth2 用户下载文件配合）。

---

## 8. WebSocket（边缘节点）

- **用途**：边缘节点长连接、心跳、打印任务与预览指令下发、任务状态上报、上传凭证请求与下发。
- **包**：`internal/websocket`（handler 处理 HTTP 升级与鉴权，manager 管理连接与下发，connection 处理单连接读写与业务消息，message 定义类型与 payload）。
- **认证**：请求需带 `Authorization: Bearer <token>`，并满足 `edge:heartbeat` scope；`node_id` 可从 query 或 token 的 sub 取。
- **协议**：上行心跳、任务状态更新、提交打印参数、请求上传凭证、ACK；下行打印任务、预览文件、上传凭证、节点状态、错误等。部分指令带 ACK 超时与重试（如任务下发失败回滚为 pending）。

---

## 9. 数据库

- **数据库**：PostgreSQL。
- **表结构**：当前无独立迁移工具，在 `internal/database/database.go` 的 `InitTables()` 中建表与触发器，部分 ALTER 在代码中做兼容。
- **主要表**：users、edge_nodes、printers、print_jobs、files、token_usage_records、oauth2_clients。
- **Repository**：每个实体一个 *Repository，接收 `*DB`，提供 CRUD 与业务查询；事务通过 `db.BeginTx()` 得到 `*Tx`，再传给需要事务的 Repository 方法（如 `DeletePrintersByEdgeNodeTx`）。

---

## 10. 文件与安全

- **上传/下载**：文件落盘到 `storage.upload_dir`，元数据入 `files` 表；下载/预览使用一次性 download token，上传可使用一次性 upload token（如 Web 扫码上传、Edge 上报）。
- **TokenManager**：生成/校验 upload、download token，与 `token_usage_records` 配合实现一次性消耗与撤销；预注册、撤销旧 token 等逻辑在 `internal/security/token_manager.go`。
- **安全约定**：生产环境必须更换并满足最小长度（如 32 字符）：`oauth2.jwt_signing_secret`、`security.file_access_secret`；配置校验会检查。

---

## 11. 日志与错误

- **日志**：统一使用 `internal/logger`（Zap），无标准库 `log`。按需使用 `logger.Info/Warn/Error/Debug`，并带 `zap.String`、`zap.Error` 等字段。
- **响应**：成功/分页/错误通过 `internal/handlers/response.go` 统一返回；业务错误码与中文文案在 `handlers/errors.go`（如 `ErrCodePrintJobNotFound`、`GetErrorMessage`）。

---

## 12. 构建、测试与部署

- **本地运行**：在 api 目录下执行 `go build -o main ./cmd/server` 或 `go run ./cmd/server`，确保工作目录为 api（或 config 位于 `./configs` / `/etc/fly-print-cloud`），并已准备好 `config.yaml`（若独立部署）。
- **Docker**：在 api 目录 `docker build -t fly-print-api .`；镜像内无 `config.yaml`，运行时的配置完全由环境变量提供。
- **Compose**：在 **cloud 根目录** 使用 `docker-compose up`，API 服务由 compose 注入 `FLY_PRINT_*` 等变量，无需 api 下 config。
- **测试**：当前仓库内无 `*_test.go`（已移除），后续若恢复单测/集成测，需自行补充 mock 或测试用 DB。

---

## 13. 与外部的关系

- **Admin 前端**：调用 `/api/v1`、`/auth`；需配置 CORS `allowed_origins` 与 `admin.console_url`。
- **边缘节点**：通过 WebSocket 连接、携带 Bearer token 与 `node_id`；协议见 `internal/websocket/message.go` 及对接方文档。
- **Nginx/网关**：API 设计为“相对路径”（如 `/api/v1`、`/auth`），不依赖固定 base path。若网关将 API 挂在子路径（如 `/api/fly-print-api`），需在网关侧做“剥前缀”转发，使到达 API 的路径仍为 `/api/v1`、`/auth` 等。

---

## 14. 注意事项与约定

- **不要提交**：`api/config.yaml` 已加入仓库 .gitignore，仅保留 `config.example.yaml` 作为模板。
- **生产环境**：务必修改 JWT 签名密钥、文件访问密钥等，并满足配置校验中的长度与安全要求。
- **后续若引入迁移**：可将 `InitTables` 及零散 ALTER 抽离为迁移脚本或迁移工具，便于多环境与回滚。
- **Swagger**：注释在 `main.go` 与各 handler；若修改路由或模型，需重新执行 `swag init`（如 `swag init -g cmd/server/main.go`）以更新 `docs/`。

---

## 15. 快速命令参考

```bash
# 独立部署（在 api 目录）
cp config.example.yaml config.yaml
# 编辑 config.yaml 后：
go run ./cmd/server

# 构建二进制
go build -o main ./cmd/server

# 生成 Swagger 文档（若修改了注释）
swag init -g cmd/server/main.go
```

若使用 **Docker Compose**，在 **fly-print-cloud 根目录** 配置 `.env` 并执行 `docker-compose up -d` 即可，无需在 api 下放置 config。
