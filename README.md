# FlyPrint Cloud

云端打印管理平台：统一管理多边缘节点与打印机，提供管理后台、REST/WebSocket API、OAuth2 认证与文件上传/下载。

## 功能

- **管理后台**：看板、边缘节点、打印机、打印任务、用户、OAuth2 客户端（builtin）、系统设置、公共上传页
- **API**：管理 CRUD、第三方打印、边缘节点注册与状态、文件预检/上传/下载、一次性凭证
- **边缘连接**：WebSocket 长连接（心跳、任务下发、状态上报）
- **认证**：内置 JWT（builtin）或 Keycloak；Scope：admin / operator / edge:* / print:submit

## 结构

```
fly-print-cloud/
├── api/          # Go 后端（Gin, GORM, Gorilla WebSocket）
├── admin/        # React 前端（TypeScript, Ant Design）
├── nginx/        # 反向代理 + 静态托管
├── docker-compose.yml
├── .env.example
├── README.md
└── HANDOFF.md    # 全项目对接文档（交付必读）
```

## 亮点

- **单源配置**：Compose 部署仅改根目录 `.env`，API 无 config 文件，由 `FLY_PRINT_*` 注入
- **路径解耦**：Admin 通过 `REACT_APP_API_BASE_PATH` / `REACT_APP_AUTH_BASE_PATH` 适配同域或子路径
- **安全**：限流、安全头、一次性文件凭证、生产密钥校验

## 快速部署

1. **依赖**：Docker + Docker Compose；可选 Python 3.8+（安装脚本）
2. **配置**：`cp .env.example .env`，按需修改（数据库、`HTTP_PORT`、认证、`DEFAULT_ADMIN_PASSWORD` 等）
3. **启动**：`docker compose build && docker compose up -d`

访问：**管理后台** `http://localhost:${HTTP_PORT}`（默认 8012），**健康检查** `http://localhost:${HTTP_PORT}/api/v1/health`。默认管理员用户名 `admin`，密码见 `.env` 中 `DEFAULT_ADMIN_PASSWORD`。

**交付与二次开发**：请阅读 **[HANDOFF.md](./HANDOFF.md)**，内含配置总览、模块文档索引、路径与认证约定、部署与注意事项。子模块详见 `api/README.md`、`admin/DEV_HANDOFF.md`、`nginx/README.md`。

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.21+, Gin, GORM, Gorilla WebSocket |
| 前端 | React 18, TypeScript, Ant Design |
| 数据库 | PostgreSQL 15（compose 服务，`.env` 配置） |
| 网关 | Nginx Alpine |

## 更多

- **认证模式**：builtin（默认）/ keycloak，见 `.env.example` 中 `OAUTH2_*`
- **生产**：改默认密码与 JWT/文件密钥、HTTPS、`COOKIE_SECURE=true`
- **故障排查**：端口占用、`/auth/mode`、WebSocket 与 Token

详见 [HANDOFF.md](./HANDOFF.md)。
