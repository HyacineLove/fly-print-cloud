# FlyPrint Cloud - 云端智能打印管理平台

## 系统概述

FlyPrint Cloud 是一个云打印管理后台系统，用于管理和监控分布式的打印资源。系统包含管理后台、API服务、边缘端连接器等核心组件，支持多 Edge Node 和多打印机的统一管理。

## 系统要求

### 必需依赖

| 依赖 | 最低版本 | 说明 |
|------|----------|------|
| Docker | 20.10+ | 容器运行时 |
| Docker Compose | V2 或 1.29+ | 容器编排 |
| Python | 3.8+ | 运行安装脚本 |

### 硬件要求

| 配置 | 最低要求 | 推荐配置 |
|------|----------|----------|
| CPU | 2 核 | 4 核 |
| 内存 | 2 GB | 4 GB |
| 磁盘 | 10 GB | 20 GB |

### 端口占用

| 端口 | 服务 | 说明 |
|------|------|------|
| 8180 | Nginx | 统一入口 (HTTP) |
| 8090 | Keycloak | 可选，仅 keycloak 模式 |

## 快速开始

### 1. 安装 Docker

**Windows / macOS:**
- 下载并安装 [Docker Desktop](https://www.docker.com/products/docker-desktop)

**Linux (Ubuntu/Debian):**
```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

### 2. 运行安装脚本

```bash
# 进入项目目录
cd fly-print-cloud

# 自动安装（使用默认配置）
python install.py --auto

# 或交互式安装（可自定义配置）
python install.py
```

### 3. 访问服务

安装完成后，访问以下地址：

- **Admin Console**: http://localhost:8180
- **API Health**: http://localhost:8180/api/v1/health

**默认管理员账户:**
- 用户名: `admin`
- 密码: 安装时显示（随机生成或自定义）

## 手动安装

如果不想使用安装脚本，可以手动安装：

```bash
# 1. 复制环境变量模板
cp .env.example .env

# 2. 编辑配置（根据实际环境修改）
vim .env

# 3. 构建镜像
docker compose build

# 4. 启动服务
docker compose up -d

# 5. 查看日志
docker compose logs -f
```

## 系统架构

```
fly-print-cloud/
├── api/                    # Go 后端 API
│   ├── cmd/server/         # 主程序入口
│   ├── internal/           # 内部模块
│   │   ├── auth/           # 认证服务
│   │   ├── config/         # 配置管理
│   │   ├── database/       # 数据库访问
│   │   ├── handlers/       # HTTP 处理器
│   │   ├── middleware/     # 中间件
│   │   ├── models/         # 数据模型
│   │   ├── security/       # 安全模块
│   │   └── websocket/      # WebSocket
│   └── Dockerfile
├── admin/                  # React 前端
│   ├── src/
│   │   ├── components/     # 页面组件
│   │   └── App.tsx         # 应用入口
│   └── Dockerfile
├── nginx/                  # Nginx 配置
│   ├── nginx.conf
│   └── conf.d/admin.conf
├── docker-compose.yml      # Docker Compose 配置
├── .env.example            # 环境变量模板
├── install.py              # 安装脚本
└── README.md               # 本文档
```

## 核心功能

### Admin Console (管理后台)
- **Dashboard**: 系统概览、打印任务趋势图
- **Edge Nodes**: 边缘节点管理、在线状态监控
- **Printers**: 打印机管理、状态查看
- **Print Jobs**: 打印任务列表、状态追踪
- **Users**: 用户管理 (builtin 模式)
- **Settings**: 系统设置
- **Public Upload**: 公共文件上传页面

### API 服务

| 路由 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/auth/mode` | GET | 获取认证模式 |
| `/auth/token` | POST | 获取 Token (builtin) |
| `/auth/login` | GET | OAuth2 登录 |
| `/auth/me` | GET | 当前用户信息 |
| `/api/v1/admin/*` | - | 管理 API |
| `/api/v1/edge/*` | - | Edge 节点 API |
| `/api/v1/print-jobs` | POST | 第三方打印 API |
| `/api/v1/files/*` | - | 文件上传/下载 |

### WebSocket 通信

Edge 节点通过 WebSocket 与 Cloud 保持长连接：

- **连接地址**: `ws://localhost:8180/api/v1/edge/ws?node_id={node_id}`
- **认证方式**: Bearer Token (OAuth2)
- **功能**: 心跳、打印任务分发、状态同步

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.21, Gin, GORM |
| 前端 | React 18, Ant Design, TypeScript |
| 数据库 | PostgreSQL 15 |
| 代理 | Nginx Alpine |
| 认证 | JWT, OAuth2 |
| 通信 | WebSocket (Gorilla) |

## 认证模式

### Builtin 模式 (默认)

内置轻量级认证服务，适用于：
- 校园网/局域网部署
- 快速测试环境
- 无需外部依赖

配置：
```env
OAUTH2_MODE=builtin
OAUTH2_JWT_SIGNING_SECRET=your-random-secret
```

### Keycloak 模式

外部 Keycloak 认证，适用于：
- 生产环境部署
- 需要 SSO 集成
- 多租户场景

配置：
```env
OAUTH2_MODE=keycloak
OAUTH2_CLIENT_ID=fly-print-admin-console
OAUTH2_CLIENT_SECRET=your-client-secret
OAUTH2_AUTH_URL=https://keycloak.example.com/realms/fly-print/...
```

### 角色权限
- **fly-print-admin**: 所有权限
- **fly-print-operator**: 查看状态 + 操作任务
- **edge:***: Edge 节点相关权限
- **print:submit**: 第三方打印提交权限

## Edge 节点连接

Edge 端需要配置以下信息连接 Cloud：

```yaml
cloud:
  api_url: http://localhost:8180
  websocket_url: ws://localhost:8180/api/v1/edge/ws
  client_id: edge-node-1
  client_secret: your-edge-secret
```

## 常用命令

```bash
# 查看服务状态
python install.py --status
docker compose ps

# 查看日志
docker compose logs -f
docker compose logs -f api      # 只看 API 日志

# 重启服务
docker compose restart

# 停止服务
python install.py --stop
docker compose down

# 重新构建
python install.py --rebuild
docker compose build --no-cache && docker compose up -d

# 清理数据（危险操作）
docker compose down -v
```

## 生产部署建议

### 安全配置

1. **修改默认密码**
   ```env
   DEFAULT_ADMIN_PASSWORD=your-strong-password
   POSTGRES_PASSWORD=your-db-password
   ```

2. **生成随机密钥**
   ```bash
   openssl rand -hex 32
   ```

3. **启用 HTTPS**: 配置 SSL 证书，设置 `COOKIE_SECURE=true`

## 故障排查

| 问题 | 排查方法 |
|------|----------|
| 服务无法启动 | 检查端口占用: `netstat -tlnp \| grep 8180` |
| 登录失败 | 检查认证模式: `curl http://localhost:8180/auth/mode` |
| WebSocket 连接失败 | 检查防火墙规则，确认 Token 有效性 |

## 许可证

MIT License