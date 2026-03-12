# Nginx 配置说明

本目录为 fly-print-cloud 的 Nginx 配置，负责：反向代理 API / 认证 / WebSocket，以及托管 Admin 前端静态资源。

## 与 docker-compose / 环境变量

- **端口**：对外暴露的 HTTP 端口由 `fly-print-cloud/.env` 中的 `HTTP_PORT` 控制（默认 8012），无需改本目录配置。
- **挂载**：`docker-compose.yml` 将本目录的 `nginx.conf` 与 `conf.d/` 挂载到容器的 `/etc/nginx/`，修改本目录下 conf 后重启 nginx 或重新 `docker compose up` 即可生效。

## 文件结构

| 文件/目录 | 说明 |
|-----------|------|
| `nginx.conf` | 主配置：worker、mime、上传大小、WebSocket 升级映射、日志、限流 zone |
| `conf.d/admin.conf` | 虚拟主机：各 `location` 与后端/静态的对应关系 |
| `ssl/` | 证书生成脚本（`generate_certs.sh`、`generate_certs.ps1`），启用 HTTPS 时使用；启用后可在 `admin.conf` 中取消 HSTS 注释并配置 `listen 443 ssl` |

## Location 与后端对应关系

| 路径 | 后端 | 说明 |
|------|------|------|
| `/api/` | `http://api:8080/api/` | REST API（文件、打印任务、打印机、边缘节点、用户等），限流 10r/s、burst 20 |
| `/auth/` | `http://api:8080/auth/` | OAuth2 认证回调等，限流 10r/s、burst 5 |
| `/api/v1/edge/ws` | `http://api:8080/api/v1/edge/ws` | 边缘节点 WebSocket，需 `Upgrade`/`Connection` 与长超时 |
| `/` | 本地静态 | Admin 前端（SPA），`root` + `try_files` 回退到 `index.html` |

**说明**：当前配置不做「剥前缀」，即对外路径与后端路径一致（如 `/api/v1/...` 直接转到 API 的 `/api/v1/...`）。

## 主配置要点（nginx.conf）

- **上传大小**：`client_max_body_size 20M`，与打印上传需求一致。
- **WebSocket**：通过 `map $http_upgrade $connection_upgrade` 支持升级；长连接超时在 `admin.conf` 的 `proxy_read_timeout` / `proxy_send_timeout` 中设置（当前 86400s）。
- **限流**：`limit_req_zone` 定义 `api_limit`（10r/s），在 `admin.conf` 的 `/api/`、`/auth/`、`/api/v1/edge/ws` 中按需使用。

## 子路径部署（剥前缀）

若希望整站挂在一个子路径下（例如 `https://example.com/fly-print/`），需要：

1. **Nginx 剥前缀**：把 `/fly-print/api/` 转成后端的 `/api/`，同理 `/fly-print/auth/`、`/fly-print/api/v1/edge/ws`、`/fly-print/` 静态。
2. **Admin 前端**：构建时设置 `base` / `publicPath` 为 `/fly-print/`，保证静态与 API 请求都带该前缀。

Nginx 示例（仅示意，需按实际 upstream 与 server_name 调整）：

```nginx
# 子路径前缀
location /fly-print/api/ {
    limit_req zone=api_limit burst=20 nodelay;
    rewrite ^/fly-print/api/(.*)$ /api/$1 break;
    proxy_pass http://api:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Cookie $http_cookie;
}

location /fly-print/auth/ {
    limit_req zone=api_limit burst=5 nodelay;
    rewrite ^/fly-print/auth/(.*)$ /auth/$1 break;
    proxy_pass http://api:8080;
    # ... 同上 header
}

location /fly-print/api/v1/edge/ws {
    limit_req zone=api_limit burst=10 nodelay;
    proxy_pass http://api:8080/api/v1/edge/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    # ... 其余与当前 admin.conf 中 WebSocket 一致
}

location /fly-print/ {
    alias /usr/share/nginx/html/;
    try_files $uri $uri/ /fly-print/index.html;
}
```

根路径部署（当前 `conf.d/admin.conf`）无需上述 rewrite，直接使用现有配置即可。

## HTTPS 与证书

- 当前默认仅 HTTP（`listen 80`）。启用 HTTPS 时：在 `ssl/` 下使用脚本生成证书，在 `admin.conf` 中增加 `listen 443 ssl` 及 `ssl_certificate` / `ssl_certificate_key`，并取消 HSTS 头注释。
- 证书与密钥路径可挂载到容器（参见 `docker-compose.yml` 中注释掉的 ssl volume）。

## 安全头

当前在 `admin.conf` 的 `server` 中已设置：

- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`

启用 HTTPS 时建议在 Nginx 或上游再增加 `Strict-Transport-Security`。

## 与 API / Admin 的约定

- API 期望收到的请求路径为 **无前缀** 的 `/api/`、`/auth/`、`/api/v1/edge/ws` 等；若使用子路径，必须在 Nginx 剥掉前缀再转发。
- Admin 前端请求 API 时使用**相对路径**（如 `/api/v1/...`），因此若部署在子路径，需配置前端的 base/publicPath，使浏览器请求带上前缀，再由 Nginx 剥前缀转发到后端。

更多 API 路由与 WebSocket 说明见仓库内 `api/README.md`。
