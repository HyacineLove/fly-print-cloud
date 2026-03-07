# FlyPrint Cloud 部署手册

## 1. 系统要求

### 硬件要求
- **CPU**: 2核+ (推荐4核)
- **内存**: 4GB+ (推荐8GB)
- **磁盘**: 20GB+ 可用空间
- **网络**: 固定IP或域名（用于 Edge 节点连接）

### 软件依赖
- **Docker**: 20.10+
- **Docker Compose**: V2 或 1.29+
- **操作系统**: Linux (推荐 Ubuntu 20.04/22.04) / Windows Server

### 端口占用
| 端口 | 服务 | 说明 |
|------|------|------|
| 80 | Nginx | HTTP 主入口 |
| 8080 | API | 后端服务（内部） |
| 5432 | PostgreSQL | 数据库（内部） |

## 2. 部署步骤

### 2.1 获取代码
```bash
# 将 fly-print-cloud 文件夹复制到服务器
cd /opt
# 假设代码已上传到 /opt/fly-print-cloud
```

### 2.2 配置环境变量
复制并编辑配置文件：
```bash
cd fly-print-cloud
cp example.env .env
nano .env  # 或使用 vim
```

**关键配置项**：
```bash
# 数据库配置
POSTGRES_USER=flyprint
POSTGRES_PASSWORD=<强密码>  # 必须修改
POSTGRES_DB=flyprint_db

# 管理员账号（首次启动自动创建）
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<强密码>  # 必须修改
ADMIN_EMAIL=admin@example.com

# JWT密钥（用于认证）
JWT_SECRET=<随机64位字符串>  # 必须修改

# 文件访问密钥（用于 Token 签名）
FILE_ACCESS_SECRET=<随机64位字符串>  # 必须修改

# OAuth2 客户端（用于 Edge 节点认证）
OAUTH2_CLIENT_ID=fly-print-edge
OAUTH2_CLIENT_SECRET=<随机32位字符串>  # 必须修改
```

**生成随机密钥**：
```bash
# Linux/macOS
openssl rand -hex 32

# Windows PowerShell
[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

### 2.3 启动服务
```bash
docker-compose up -d
```

### 2.4 查看日志
```bash
# 查看所有服务日志
docker-compose logs -f

# 查看 API 服务日志
docker-compose logs -f api

# 查看数据库日志
docker-compose logs -f postgres
```

### 2.5 验证部署
```bash
# 检查容器状态
docker-compose ps

# 测试 API 健康检查
curl http://localhost:8080/health
# 应返回: {"code":200,"data":{"service":"fly-print-cloud-api","status":"ok"},"message":"success"}

# 测试 Nginx
curl http://localhost/health
```

## 3. 访问管理后台

浏览器访问 `http://<服务器IP>/admin`

- 用户名: 在 `.env` 中配置的 `ADMIN_USERNAME`
- 密码: 在 `.env` 中配置的 `ADMIN_PASSWORD`

## 4. Edge 节点配置

Edge 节点需要配置以下信息才能连接到 Cloud：

```json
{
  "cloud": {
    "enabled": true,
    "base_url": "http://<服务器IP>",
    "auth_url": "http://<服务器IP>/auth/token",
    "client_id": "fly-print-edge",
    "client_secret": "<与 .env 中 OAUTH2_CLIENT_SECRET 一致>"
  }
}
```

## 5. 数据备份

### 5.1 备份数据库
```bash
# 导出数据库
docker exec fly-print-postgres pg_dump -U flyprint -d flyprint_db > backup_$(date +%Y%m%d).sql

# 备份上传文件
tar -czf uploads_backup_$(date +%Y%m%d).tar.gz ./api/cmd/server/uploads
```

### 5.2 恢复数据库
```bash
# 停止服务
docker-compose down

# 恢复数据库
cat backup_20260304.sql | docker exec -i fly-print-postgres psql -U flyprint -d flyprint_db

# 恢复文件
tar -xzf uploads_backup_20260304.tar.gz

# 重启服务
docker-compose up -d
```

## 6. 服务管理

### 启动服务
```bash
docker-compose up -d
```

### 停止服务
```bash
docker-compose down
```

### 重启服务
```bash
docker-compose restart
```

### 更新代码后重新构建
```bash
docker-compose down
docker-compose up --build -d
```

### 查看资源占用
```bash
docker stats
```

## 7. 常见问题

### Q1: 容器无法启动
**检查端口占用**：
```bash
# Linux
sudo netstat -tlnp | grep -E '80|8080|5432'

# Windows
netstat -ano | findstr "80 8080 5432"
```

### Q2: Edge 节点无法连接
1. 检查防火墙是否开放 80 端口
2. 验证 Edge 配置中的 `client_secret` 与服务器 `.env` 一致
3. 查看 API 日志：`docker-compose logs -f api`

### Q3: 管理后台无法登录
1. 检查 `.env` 中的管理员账号密码
2. 查看数据库中用户表：
   ```bash
   docker exec -it fly-print-postgres psql -U flyprint -d flyprint_db -c "SELECT id, username, email, role FROM users;"
   ```

### Q4: 数据库连接失败
```bash
# 检查数据库容器状态
docker-compose ps postgres

# 查看数据库日志
docker-compose logs postgres

# 测试数据库连接
docker exec fly-print-postgres psql -U flyprint -d flyprint_db -c "SELECT version();"
```

## 8. 安全建议

1. **修改所有默认密码**：数据库、管理员、JWT密钥、OAuth2密钥
2. **启用 HTTPS**：配置 Let's Encrypt 或自签名证书
3. **防火墙配置**：仅开放必要端口（80, 443）
4. **定期备份**：设置 cron 定时备份数据库和文件
5. **监控日志**：定期检查异常访问和错误日志

## 9. 性能优化

### 数据库优化
编辑 `docker-compose.yml` 中的 PostgreSQL 配置：
```yaml
environment:
  - POSTGRES_MAX_CONNECTIONS=100
  - POSTGRES_SHARED_BUFFERS=256MB
```

### API 服务扩展
```bash
# 增加 API 实例数量
docker-compose up -d --scale api=3
```

## 10. 更新日志

### v0.2.0 (2026-03-04)
- **安全修复**: Token 撤销机制优化
  - 生成新 Token 时自动撤销旧 Token
  - 防止刷新二维码后旧链接仍可使用
- **性能优化**: QR 码生成超时增加到 10 秒
- **数据库**: 新增 `revoked` 字段用于 Token 状态管理

### v0.1.0
- 初始版本发布

## 11. 技术支持

需要帮助？请查看：
- **项目文档**: `/fly-print-cloud/README.md`
- **API 文档**: `http://<服务器IP>/api/docs` (开发模式)
- **日志目录**: `docker-compose logs`
