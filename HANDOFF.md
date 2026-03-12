# FlyPrint Cloud 全项目对接文档

面向接手开发的**关键信息清单**：本文档与各子模块文档一起，提供以下内容，便于快速接手与二次开发。

| 类别 | 关键信息 |
|------|----------|
| **定位与范围** | 云打印管理平台：Admin 前端 + Go API + Nginx 统一入口 + Postgres；支持边缘节点 WebSocket、OAuth2(builtin/Keycloak)、文件上传/下载与一次性凭证。 |
| **配置与入口** | 一键部署由 **根目录 `.env`** 驱动；API 无 config.yaml，由 `FLY_PRINT_*` 环境变量注入；Admin 构建时由 `REACT_APP_*` 注入路径；Nginx 对外端口 `HTTP_PORT`。 |
| **路径与网关** | 对外路径即后端路径：`/api/`、`/auth/`、`/api/v1/edge/ws`；子路径部署时需 Nginx 剥前缀 + Admin base 配置。 |
| **认证与权限** | builtin（用户名密码+JWT）或 keycloak；Scope：fly-print-admin / fly-print-operator / edge:* / print:submit / file:read。 |
| **数据与存储** | Postgres 由 compose 提供，表结构在 API `InitTables()`；文件落盘 API 容器 `upload_dir`，上传/下载靠一次性 token。 |
| **模块文档索引** | API：`api/README.md`；Admin：`admin/DEV_HANDOFF.md`；Nginx：`nginx/README.md`。 |

---

## 1. 项目定位与范围

- **FlyPrint Cloud**：云端打印管理后台，统一管理多边缘节点、多打印机与打印任务。
- **核心能力**：管理端 CRUD（用户、边缘节点、打印机、任务、OAuth2 客户端）、边缘节点 WebSocket 长连接与任务下发、第三方打印 API、文件上传/下载与预检、OAuth2 双模式认证。
- **交付范围**：本仓库为 **Cloud 端**（Admin + API + Nginx + Compose）；边缘端（Edge）为独立项目，通过 WebSocket 与 REST 与本 API 对接。

---

## 2. 整体架构

### 2.1 服务组成（Docker Compose）

| 服务 | 说明 | 依赖 |
|------|------|------|
| **postgres** | PostgreSQL 15，数据持久化在 volume `postgres_data` | - |
| **api** | Go 后端，端口 8080（内部），配置全部来自环境变量 | postgres 健康后启动 |
| **admin-console-builder** | 构建 Admin 前端，产物写入 volume `admin_build` | - |
| **nginx** | 反向代理 + 静态托管，对外端口由 `HTTP_PORT` 决定 | api、admin-console-builder |

- 数据库由 compose 的 Postgres 提供，通过 `.env` 中 `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` 配置；API 连接 `postgres:5432`。
- 对外唯一入口：`http://<host>:${HTTP_PORT}`（默认 8012，README 中示例常写 8180，以实际 `.env` 为准）。

### 2.2 请求路径与职责

- **Nginx**：`/api/` → api:8080；`/auth/` → api:8080；`/api/v1/edge/ws` → WebSocket；`/` → Admin 静态（SPA）。
- **Admin**：所有请求通过 `buildApiUrl` / `buildAuthUrl` 拼路径，默认 `/api/v1`、`/auth`，与 Nginx 一致。
- **API**：期望收到**无子路径前缀**的路径；若网关做子路径（如 `/fly-print/api/`），需在 Nginx 剥前缀后转发。

详见 `nginx/README.md` 的 Location 表与子路径部署说明。

---

## 3. 配置总览

### 3.1 唯一配置入口（Compose 部署）

- **修改处**：`fly-print-cloud/.env`（可复制 `.env.example` 为 `.env`）。
- **API**：镜像内无 `config.yaml`，所有配置由 compose 的 `environment` 注入，键为 `FLY_PRINT_*`（对应 config 层级，如 `FLY_PRINT_DATABASE_HOST`、`FLY_PRINT_OAUTH2_JWT_SIGNING_SECRET`）。
- **Admin**：构建时从 compose 传入 `REACT_APP_API_BASE_PATH`、`REACT_APP_AUTH_BASE_PATH`（及可选 `REACT_APP_API_URL`）；**admin 目录下的 `.env` 在 Docker 构建时被排除**，不生效。
- **Nginx**：仅端口依赖 `HTTP_PORT`；conf 来自 `nginx/nginx.conf` 与 `nginx/conf.d/`，如需改 server_name/限流/子路径，直接改 conf 后重启。

### 3.2 关键环境变量速查

| 用途 | 变量示例 | 说明 |
|------|----------|------|
| 数据库 | `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` | compose Postgres + API 连接 |
| 对外端口 | `HTTP_PORT` | Nginx 暴露端口，默认 8012 |
| 首次管理员 | `CREATE_DEFAULT_ADMIN`, `DEFAULT_ADMIN_PASSWORD` | API 启动时是否创建默认 admin |
| 认证模式 | `OAUTH2_MODE=builtin` 或 `keycloak` | 以及 Keycloak 相关 `OAUTH2_*` |
| 前端路径 | `REACT_APP_API_BASE_PATH`, `REACT_APP_AUTH_BASE_PATH` | 同域一般为 `/api/v1`、`/auth` |
| 安全 | `OAUTH2_JWT_SIGNING_SECRET`, `FILE_ACCESS_SECRET` | 生产必须更换且满足长度（如 32 字符） |

完整列表见 `.env.example` 内注释。

---

## 4. 子模块入口与文档

| 模块 | 入口/结构 | 详细文档 |
|------|-----------|----------|
| **API** | `api/cmd/server/main.go`：配置加载 → DB → 路由 → 后台任务 | `api/README.md`（路由、认证、WebSocket、DB、文件、配置方式） |
| **Admin** | `admin/src/App.tsx`（路由、布局）、`config.ts`（API/Auth 前缀）、`services/api.ts` | `admin/DEV_HANDOFF.md`（运行、环境变量、目录、认证、API 约定） |
| **Nginx** | `nginx/nginx.conf`、`nginx/conf.d/admin.conf` | `nginx/README.md`（Location、限流、子路径、HTTPS） |

独立部署时：
- **API**：在 `api` 目录使用 `config.yaml`（从 `config.example.yaml` 复制），或仍用 `FLY_PRINT_*` 环境变量。
- **Admin**：在 `admin` 目录用 `admin/.env` 配置 `REACT_APP_*`，再 `npm run build` / `npm start`。

---

## 5. 对接要点

### 5.1 认证与 Scope

- **builtin**：`/auth/token`（用户名密码）→ JWT；`/auth/me` 等携带 Cookie/Authorization。
- **keycloak**：跳转 `/auth/login`，回调后由 API 处理 session/cookie。
- **Scope 与路由**：fly-print-admin（全管理）、fly-print-operator（除用户与 OAuth2 客户端）、edge:register/edge:printer/edge:heartbeat、print:submit、file:read。详见 `api/README.md` §7。

### 5.2 WebSocket（边缘节点）

- **路径**：`/api/v1/edge/ws`（可带 `node_id` 等 query）。
- **认证**：`Authorization: Bearer <token>`，需 edge:heartbeat 等权限。
- **协议**：心跳、任务下发、状态上报、上传凭证请求等，见 `api/internal/websocket` 与 `api/README.md` §8。

### 5.3 文件与安全

- 上传/下载支持一次性 **upload token** / **download token**；预检与页数校验见 API 文件相关接口。
- 生产环境必须更换 `OAUTH2_JWT_SIGNING_SECRET`、`FILE_ACCESS_SECRET`，并满足 API 配置校验（长度等）。

### 5.4 数据库

- 无独立迁移工具；表结构在 `api/internal/database` 的 `InitTables()` 中创建。
- 主要表：users、edge_nodes、printers、print_jobs、files、token_usage_records、oauth2_clients。

---

## 6. 部署与扩展

- **一键部署**：根目录 `cp .env.example .env` → 编辑 `.env` → `docker compose build && docker compose up -d`。若使用安装脚本则执行 `python install.py --auto`（若有）。
- **子路径部署**：见 `nginx/README.md`：Nginx 剥前缀 + Admin 的 `REACT_APP_API_BASE_PATH` / `REACT_APP_AUTH_BASE_PATH` 设为带前缀路径。
- **HTTPS**：Nginx 使用 `ssl/` 下脚本生成证书，在 `admin.conf` 中配置 `listen 443 ssl` 及证书路径，并设置 `COOKIE_SECURE=true` 等。

---

## 7. 注意事项

- 勿提交 `api/config.yaml`（已 gitignore）；Compose 部署不依赖该文件。
- 生产务必更换 JWT 与文件访问密钥，并满足 API 启动时的校验。
- 新增 API 或前端页面时：Admin 统一用 `buildApiUrl` / `buildAuthUrl`；若增路由或 Swagger，API 需执行 `swag init -g cmd/server/main.go` 更新文档。

---

**推荐阅读顺序**：本文档 → `api/README.md` → `admin/DEV_HANDOFF.md` → `nginx/README.md`；再按需查看 `main.go`、`App.tsx`、`config.ts`、`docker-compose.yml`。
