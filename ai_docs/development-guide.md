# Fly Print Cloud 云打印管理系统 - 完整开发文档

## 一、项目需求文档

### 1.1 系统概述

Fly Print Cloud 是一个基于云-边协同架构的分布式打印管理系统。系统通过云端统一管理多个边缘节点（Edge Node），每个边缘节点管理本地的打印机设备，实现远程打印任务的分发、监控和管理。

### 1.2 核心业务需求

| 需求编号 | 需求名称 | 描述 |
|---------|---------|------|
| BR-001 | 远程打印管理 | 用户通过 Web 管理后台创建、监控和管理打印任务 |
| BR-002 | 边缘节点管理 | 管理分布在不同地理位置的边缘计算节点 |
| BR-003 | 打印机管理 | 统一管理所有边缘节点下的打印机设备 |
| BR-004 | 第三方打印接入 | 支持外部系统通过 API 提交打印任务 |
| BR-005 | 文件管理 | 上传和管理打印文件，支持多种文件格式 |

### 1.3 功能需求

#### 用户认证与授权
- **FR-001**: OAuth2 单点登录（SSO）集成，支持 Keycloak、Auth0、Google 等 OAuth2 提供商
- **FR-002**: 基于角色的访问控制（RBAC）：admin（管理员）、operator（操作员）、viewer（查看者）
- **FR-003**: 基于 scope 的细粒度权限控制
- **FR-004**: 自动用户同步 -- OAuth2 登录时自动创建本地用户

#### 边缘节点管理
- **FR-005**: Edge Node 自注册和心跳上报
- **FR-006**: 节点在线/离线状态自动检测（3分钟心跳超时）
- **FR-007**: 节点启用/禁用控制（云端下发状态变更）
- **FR-008**: 节点系统信息收集（OS、CPU、内存、磁盘、网络）

#### 打印机管理
- **FR-009**: Edge Node 自动注册本地打印机
- **FR-010**: 打印机能力上报（纸张、颜色、双面等）
- **FR-011**: 打印机别名设置和启用/禁用控制
- **FR-012**: 打印机状态实时更新（ready/printing/error/offline）

#### 打印任务管理
- **FR-013**: 创建打印任务（指定打印机、文件、打印参数）
- **FR-014**: 打印参数校验（根据打印机能力验证）
- **FR-015**: 任务生命周期管理（pending -> dispatched -> downloading -> printing -> completed/failed/cancelled）
- **FR-016**: 任务取消和重新打印
- **FR-017**: 任务重试机制（可配置最大重试次数）

#### 文件管理
- **FR-018**: 文件上传（支持 PDF、JPG、PNG、DOCX），限制 50MB
- **FR-019**: 文件下载（权限控制 -- 仅上传者或管理员）
- **FR-020**: 文件预览分发到 Edge Node（通过 Task Token 安全机制）

#### 数据统计
- **FR-021**: 仪表板展示系统概览（打印机数、节点数、任务数）
- **FR-022**: 7 天打印任务完成/失败趋势图

### 1.4 非功能需求

| 需求编号 | 类别 | 描述 |
|---------|------|------|
| NFR-001 | 安全性 | 密码使用 bcrypt 加密；Cookie 为 HTTP-only + SameSite=Lax |
| NFR-002 | 安全性 | CSRF 防护（OAuth2 state 参数）；Task Token 使用 HMAC-SHA256 签名 |
| NFR-003 | 安全性 | 参数化 SQL 查询防止注入攻击 |
| NFR-004 | 实时性 | WebSocket 长连接保证 Edge Node 与云端实时通信 |
| NFR-005 | 可扩展性 | Repository 模式的数据层设计，支持数据库切换 |
| NFR-006 | 可靠性 | 打印任务重试机制，默认最多 3 次 |
| NFR-007 | 可部署性 | Docker Compose 一键部署 |
| NFR-008 | 数据完整性 | 软删除机制保护关键数据（用户、节点） |

---

## 二、系统架构文档

### 2.1 整体架构

```
                          +-----------+
                          |  Browser  |
                          |  (React)  |
                          +-----+-----+
                                |
                          +-----v-----+
                          |   Nginx   |
                          | (Reverse  |
                          |  Proxy)   |
                          +-----+-----+
                                |
              +-----------------+-----------------+
              |                 |                 |
        +-----v-----+   +-----v-----+   +-------v-------+
        |  Static    |   |  API Route|   | WebSocket     |
        | Admin SPA  |   | /api/v1/* |   | /api/v1/edge/ws|
        +------------+   +-----+-----+   +-------+-------+
                                |                 |
                          +-----v-----+   +-------v-------+
                          |  Go API   |   | WS Manager    |
                          |  (Gin)    |   | (gorilla/ws)  |
                          +-----+-----+   +-------+-------+
                                |                 |
                          +-----v-----------------v-------+
                          |        Repository Layer       |
                          +-----+-------------------------+
                                |
                          +-----v-----+     +-----------+
                          | PostgreSQL|     |   Redis   |
                          |   (15)    |     | (7-alpine)|
                          +-----------+     +-----------+

              +---+           +---+           +---+
              |EN1|           |EN2|           |EN3|
              +---+           +---+           +---+
           (Edge Nodes connect to WS Manager via WebSocket)
```

### 2.2 技术栈

| 层级 | 技术 | 版本 | 用途 |
|------|------|------|------|
| **前端框架** | React + TypeScript | 18.2 / TS 4.7 | 管理后台 SPA |
| **UI 组件库** | Ant Design | 5.12.8 | UI 组件和布局 |
| **图表库** | ECharts | 5.4.3 | 数据可视化 |
| **前端路由** | React Router | 6.20.1 | 客户端路由 |
| **前端构建** | Create React App | 5.0.1 | 构建工具链 |
| **后端框架** | Gin (Go) | - | HTTP 路由和中间件 |
| **WebSocket** | gorilla/websocket | - | 实时双向通信 |
| **数据库** | PostgreSQL | 15 | 主数据存储 |
| **缓存** | Redis | 7-alpine | 会话和缓存（预留） |
| **认证** | OAuth2 + JWT | - | 身份认证和授权 |
| **密码哈希** | bcrypt | - | 密码安全存储 |
| **配置管理** | Viper | - | 多源配置加载 |
| **反向代理** | Nginx | alpine | 统一入口、静态文件、WS 代理 |
| **容器化** | Docker + Compose | - | 一键部署 |

### 2.3 组件交互关系

**Nginx 反向代理分流规则：**
- `/` -> 静态文件（Admin SPA）
- `/api/*` -> Go API 后端 (`:8080`)
- `/auth/*` -> Go API 后端 (`:8080`，OAuth2 认证)
- `/api/v1/edge/ws` -> WebSocket 长连接（Go API 后端）

**Go API 内部分层：**
```
HTTP Request
    -> Middleware (Logger -> CORS -> OAuth2ResourceServer)
    -> Handler (业务逻辑层)
    -> Repository (数据访问层)
    -> PostgreSQL (数据库)
```

**WebSocket 通信模型：**
```
Edge Node <-> Connection (ReadPump/WritePump) <-> ConnectionManager
                                                      |
                                                   Handler 层
                                                (调用 SendToNode /
                                                 DispatchPrintJob 等)
```

---

## 三、模块实现文档

### 3.1 管理后台 (Admin) 前端模块

**目录结构：** `admin/src/`

| 文件 | 行数 | 功能 |
|------|------|------|
| `index.tsx` | - | React 18 应用入口 |
| `App.tsx` | ~262 | 根路由、Ant Design Layout 布局、侧边栏导航 |
| `components/pages/Dashboard.tsx` | ~403 | 仪表板 -- 统计卡片 + ECharts 趋势图 |
| `components/pages/EdgeNodes.tsx` | ~438 | 边缘节点管理 -- 表格列表、编辑名称、启用/禁用 |
| `components/pages/Printers.tsx` | ~614 | 打印机管理 -- 表格列表、筛选、别名编辑、启用/禁用/删除 |
| `components/pages/PrintJobs.tsx` | ~733 | 打印任务管理 -- 表格列表、状态筛选、取消/重打/删除 |
| `components/pages/PublicUpload.tsx` | ~189 | 公共文件上传（无需管理员登录，通过 URL token 认证） |
| `components/pages/Users.tsx` | - | 用户管理（开发中占位） |
| `components/pages/Settings.tsx` | - | 系统设置（开发中占位） |
| `services/api.ts` | ~177 | API 通信层 -- 单例 ApiService 类、Token 管理、统一请求 |
| `services/dashboard.ts` | ~139 | Dashboard 数据接口类型定义 |

**认证流程（前端视角）：**
1. `AdminApp` 组件 `useEffect` -> 调用 `GET /auth/me`
2. 成功 -> 获取 `access_token` 存入 `ApiService` 实例
3. 失败 -> `window.location.href = '/auth/login'` 重定向到 OAuth2 登录
4. 后续 API 请求 -> `Authorization: Bearer {token}` 自动附加

**状态管理：** 无集中式状态库，每个页面组件通过 `useState` 独立管理状态。

**API 通信模式：** 基于 Fetch API 的 `ApiService` 单例类，自动处理 Token 注入、错误包装、FormData 上传。

### 3.2 API 服务 (api) 后端模块

**入口文件：** `api/cmd/server/main.go`

**启动流程：**
1. `LoadConfig()` -> Viper 加载配置（YAML + 环境变量，前缀 `FLY_PRINT_`）
2. `database.InitDB()` -> 连接 PostgreSQL，配置连接池（MaxOpen=25, MaxIdle=5）
3. `database.CreateTables()` -> 自动建表和索引
4. 初始化 5 个 Repository：User、EdgeNode、Printer、PrintJob、File
5. `websocket.NewConnectionManager()` + `go manager.Run()` -> 启动 WebSocket 管理器事件循环
6. 初始化 7 个 Handler：OAuth2、User、EdgeNode、Printer、PrintJob、File、Dashboard
7. 配置 Gin 路由和中间件链
8. `router.Run()` -> 启动 HTTP 服务

**中间件链：**
- `LoggerMiddleware` -> 记录请求 IP、方法、路径、状态码、耗时
- `CORSMiddleware` -> 跨域支持（开发环境允许所有源）
- `OAuth2ResourceServer(scopes...)` -> Bearer Token 验证 + 角色/权限检查

### 3.3 WebSocket 通信模块

**目录：** `api/internal/websocket/`

**核心组件：**

| 文件 | 功能 |
|------|------|
| `manager.go` | `ConnectionManager` -- 管理所有 Edge Node WebSocket 连接，提供注册/注销/发送/广播接口 |
| `connection.go` | `Connection` -- 单个 WebSocket 连接封装，ReadPump(读) / WritePump(写) 双协程模型 |
| `handler.go` | `WebSocketHandler.HandleConnection` -- HTTP -> WebSocket 升级、Token 验证、权限检查 |
| `message.go` | 消息类型常量定义和结构体 |

**WebSocket 连接建立流程：**
1. Edge Node 发起 `GET /api/v1/edge/ws?node_id=xxx`（携带 `Authorization: Bearer xxx`）
2. `WebSocketHandler` 验证 Token -> 检查 `edge:heartbeat` scope -> 获取 `node_id`
3. gorilla/websocket Upgrader 升级为 WebSocket 连接
4. 创建 `Connection` 实例 -> 注册到 `ConnectionManager`
5. 启动 `go ReadPump()` 和 `go WritePump()` 协程

**消息类型：**

| 方向 | 类型 | 用途 |
|------|------|------|
| Edge -> Cloud | `edge_heartbeat` | 心跳（更新 last_heartbeat） |
| Edge -> Cloud | `printer_status` | 打印机状态更新（status、queue_length） |
| Edge -> Cloud | `job_update` | 打印任务状态更新（status、progress、error） |
| Edge -> Cloud | `submit_print_params` | 提交打印参数（携带 Task Token 验证） |
| Cloud -> Edge | `print_job` | 分发打印任务 |
| Cloud -> Edge | `preview_file` | 分发文件预览请求（携带 Task Token） |
| Cloud -> Edge | `node_state` | 节点启用/禁用状态变更 |
| Cloud -> Edge | `printer_state` | 打印机启用/禁用状态变更 |
| Cloud -> Edge | `printer_deleted` | 打印机删除通知 |

**Task Token 安全机制：**
- 生成：HMAC-SHA256 签名，payload = `node_id:file_id:timestamp`
- 有效期：1 小时
- 用途：Edge Node 预览文件后提交打印参数时的身份验证

### 3.4 数据库操作模块

**目录：** `api/internal/database/`

**架构模式：** Repository 模式 -- 每个实体对应一个 Repository 结构体，封装所有 SQL 操作。

| Repository | 实体 | 主要操作 |
|-----------|------|---------|
| `UserRepository` | User | CRUD、OAuth2 用户同步、密码管理、邮箱/用户名唯一性检查 |
| `EdgeNodeRepository` | EdgeNode | Upsert、分页列表（状态过滤）、心跳更新、超时检测、软删除 |
| `PrinterRepository` | Printer | Upsert（name+edge_node_id 唯一）、按节点查询、批量启用/禁用 |
| `PrintJobRepository` | PrintJob | CRUD、多条件过滤、状态统计、关联查询 Edge Node ID |
| `FileRepository` | File | Create、GetByID |

**数据库初始化：** `database.go` 中的 `CreateTables()` 使用 `CREATE TABLE IF NOT EXISTS` 和 `CREATE INDEX IF NOT EXISTS` 实现幂等建表。

### 3.5 用户认证 (OAuth2) 模块

**文件：** `api/internal/handlers/oauth2_handler.go` + `api/internal/middleware/oauth2.go`

**OAuth2 Authorization Code Flow：**
```
Browser -> GET /auth/login
  -> 302 redirect to OAuth2 Provider (with state, redirect_uri, client_id, scope)

OAuth2 Provider -> callback GET /auth/callback?code=xxx&state=xxx
  -> Validate state (CSRF protection)
  -> Exchange authorization code for Tokens (access_token, refresh_token, id_token)
  -> Set HTTP-only Cookies
  -> syncUserOnLogin() sync user to local database
  -> 302 redirect to Admin Console homepage
```

**Token 验证策略（中间件）：**
1. 优先尝试 JWT 解析（适用于 Client Credentials Flow / 机器间通信）
2. JWT 解析失败则通过 UserInfo 端点验证（适用于 Authorization Code Flow）
3. 从多个来源提取角色：OIDC groups、roles、Keycloak realm_access / resource_access、OAuth2 scope

**权限模型：**
- admin 角色拥有所有权限（超级权限）
- 其他角色需要精确匹配所需 scope（AND 逻辑）

### 3.6 打印任务管理模块

**文件：** `api/internal/handlers/print_job_handler.go`

**任务创建流程：**
1. 接收请求 -> 校验参数（file_path 或 file_url 至少一个）
2. 自动生成任务名称（从 URL 或文件路径提取文件名）
3. 设置默认值：Copies=1, MaxRetries=3
4. 校验打印机能力（纸张、颜色、双面、份数 1-99）
5. 创建数据库记录（状态: pending）
6. 通过 WebSocket `DispatchPrintJob()` 分发到 Edge Node -> 状态变为 dispatched
7. 返回 201 Created

**任务状态机：**
```
pending -> dispatched -> downloading -> printing -> completed
                                          |
                                        failed
pending/dispatched -> cancelled (用户取消)

completed/failed/cancelled -> (重新打印) -> 创建新任务
```

**重新打印逻辑：**
- 基于原任务创建新任务（保留文件信息）
- 允许修改：PrinterID、Copies、PaperSize、ColorMode、DuplexMode
- 重新校验目标打印机能力

### 3.7 边缘节点管理模块

**文件：** `api/internal/handlers/edge_node_handler.go`

**节点注册：**
- Edge Node 首次连接时调用 `POST /api/v1/edge/register`
- 使用 `UpsertEdgeNode`（ON CONFLICT 语义）-- 新节点创建，已有节点更新

**心跳机制：**
- Edge Node 定期调用 `POST /api/v1/edge/heartbeat` 或通过 WebSocket 发送 `edge_heartbeat` 消息
- 云端在 `ListEdgeNodes` 时自动调用 `CheckAndUpdateOfflineNodes` 检查超时节点
- 超时阈值：3 分钟无心跳标记为 offline

**启用/禁用：**
- 管理员通过 `PUT /api/v1/admin/edge-nodes/:id` 修改 `enabled` 字段
- 变更时通过 WebSocket `DispatchNodeEnabledChange()` 实时通知 Edge Node

---

## 四、数据库设计文档

### 4.1 ER 关系图

```
  +-----------+       +------------------+       +-----------+
  |   users   |       |   edge_nodes     |       |   files   |
  +-----------+       +------------------+       +-----------+
  | PK: id    |       | PK: id           |       | PK: id    |
  | username  |       | name             |       | orig_name |
  | email     |       | status           |       | file_name |
  | pwd_hash  |       | enabled          |       | file_path |
  | ext_id    |       | version          |       | mime_type |
  | role      |       | last_heartbeat   |       | size      |
  | status    |       | location/lat/lng |       | upldr_id  |
  +-----------+       | ip/mac/network   |       +-----------+
       |              | os/cpu/mem/disk  |
       |              | conn_quality/lat |
       |              | deleted_at       |
       |              +--------+---------+
       |                       |
       |              +--------v---------+
       |              |    printers      |
       |              +------------------+
       |              | PK: id           |
       |              | name             |
       |              | display_name     |
       |              | model            |
       |              | status/enabled   |
       |              | capabilities(J)  |
       +------+       | FK: edge_node_id |
              |       +--------+---------+
              |                |
       +------v--------v------+
       |     print_jobs       |
       +----------------------+
       | PK: id               |
       | name, status         |
       | FK: printer_id       |
       | FK: user_id          |
       | file_path/file_url   |
       | copies/paper/color   |
       | duplex/retries       |
       +----------------------+
```

### 4.2 表结构详细设计

#### users 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK, DEFAULT gen_random_uuid() | 用户唯一标识 |
| username | VARCHAR(50) | UNIQUE, NOT NULL | 用户名 |
| email | VARCHAR(100) | UNIQUE, NOT NULL | 邮箱 |
| password_hash | VARCHAR(255) | | bcrypt 加密密码 |
| external_id | VARCHAR(255) | UNIQUE | OAuth2 外部 ID（sub claim） |
| role | VARCHAR(20) | NOT NULL, DEFAULT 'viewer' | 角色：admin/operator/viewer |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'active' | 状态：active/inactive |
| last_login | TIMESTAMP WITH TIME ZONE | | 最后登录时间 |
| created_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 创建时间 |
| updated_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 更新时间（触发器自动更新） |

#### edge_nodes 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | VARCHAR(100) | PK | 节点唯一标识（由 Edge Node 提供） |
| name | VARCHAR(100) | NOT NULL | 用户友好名称 |
| status | VARCHAR(20) | DEFAULT 'offline' | 状态：online/offline |
| enabled | BOOLEAN | DEFAULT true | 云端启用/禁用 |
| version | VARCHAR(50) | | 软件版本 |
| last_heartbeat | TIMESTAMP WITH TIME ZONE | | 最后心跳时间 |
| deleted_at | TIMESTAMP WITH TIME ZONE | | 软删除时间 |
| location | VARCHAR(255) | | 地理位置描述 |
| latitude | DECIMAL(10,7) | | 纬度 |
| longitude | DECIMAL(10,7) | | 经度 |
| ip_address | INET | | IP 地址 |
| mac_address | VARCHAR(17) | | MAC 地址 |
| network_interface | VARCHAR(50) | | 网络接口名 |
| os_version | VARCHAR(100) | | 操作系统版本 |
| cpu_info | TEXT | | CPU 信息 |
| memory_info | TEXT | | 内存信息 |
| disk_info | TEXT | | 磁盘信息 |
| connection_quality | VARCHAR(20) | | 连接质量 |
| latency | INTEGER | | 延迟(ms) |
| created_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 创建时间 |
| updated_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 更新时间 |

**索引：** `idx_edge_nodes_status`、`idx_edge_nodes_last_heartbeat`

#### printers 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK, DEFAULT gen_random_uuid() | 打印机唯一标识 |
| name | VARCHAR(100) | NOT NULL | CUPS 打印机名称 |
| display_name | VARCHAR(255) | | 用户友好显示名称 |
| model | VARCHAR(100) | | 打印机型号 |
| serial_number | VARCHAR(100) | | 序列号 |
| status | VARCHAR(20) | DEFAULT 'offline' | 状态：ready/printing/error/offline |
| enabled | BOOLEAN | DEFAULT true | 启用/禁用 |
| firmware_version | VARCHAR(50) | | 固件版本 |
| port_info | VARCHAR(100) | | 端口信息 |
| ip_address | INET | | 打印机 IP |
| mac_address | VARCHAR(17) | | MAC 地址 |
| network_config | TEXT | | 网络配置 |
| latitude | DECIMAL(10,7) | | 纬度 |
| longitude | DECIMAL(10,7) | | 经度 |
| location | VARCHAR(255) | | 位置描述 |
| capabilities | JSONB | | 能力信息 |
| edge_node_id | VARCHAR(100) | FK -> edge_nodes(id) | 所属 Edge Node |
| queue_length | INTEGER | DEFAULT 0 | 当前队列长度 |
| created_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 创建时间 |
| updated_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 更新时间 |

**唯一约束：** `(name, edge_node_id)`
**索引：** `idx_printers_edge_node_id`、`idx_printers_status`

**capabilities JSONB 结构：**
```json
{
  "paper_sizes": ["A4", "A3", "Letter"],
  "color_support": true,
  "duplex_support": true,
  "resolution": "1200x1200 dpi",
  "print_speed": "30 ppm",
  "media_types": ["plain", "glossy", "photo"]
}
```

#### print_jobs 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK, DEFAULT gen_random_uuid() | 任务唯一标识 |
| name | VARCHAR(200) | | 任务名称 |
| status | VARCHAR(20) | DEFAULT 'pending' | 状态（见下方状态枚举） |
| printer_id | UUID | FK -> printers(id) | 目标打印机 |
| user_id | UUID | FK -> users(id), NULLABLE | 提交用户 |
| user_name | VARCHAR(100) | | 用户名（冗余存储） |
| file_path | VARCHAR(500) | | 本地文件路径 |
| file_url | VARCHAR(1000) | | 第三方文件 URL |
| file_size | BIGINT | DEFAULT 0 | 文件大小(bytes) |
| page_count | INTEGER | DEFAULT 0 | 页数 |
| copies | INTEGER | DEFAULT 1 | 打印份数 |
| paper_size | VARCHAR(20) | | 纸张大小 |
| color_mode | VARCHAR(20) | | 颜色模式：color/grayscale |
| duplex_mode | VARCHAR(20) | | 双面模式：single/duplex |
| start_time | TIMESTAMP WITH TIME ZONE | | 开始打印时间 |
| end_time | TIMESTAMP WITH TIME ZONE | | 完成时间 |
| error_message | TEXT | | 错误信息 |
| retry_count | INTEGER | DEFAULT 0 | 已重试次数 |
| max_retries | INTEGER | DEFAULT 3 | 最大重试次数 |
| created_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 创建时间 |
| updated_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 更新时间 |

**状态枚举：** `pending` / `dispatched` / `downloading` / `printing` / `completed` / `failed` / `cancelled`

**索引：** `idx_print_jobs_status`、`idx_print_jobs_printer_id`、`idx_print_jobs_user_id`、`idx_print_jobs_created_at`

#### files 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK, DEFAULT gen_random_uuid() | 文件唯一标识 |
| original_name | VARCHAR(255) | NOT NULL | 原始文件名 |
| file_name | VARCHAR(255) | NOT NULL | 存储文件名（UUID + 扩展名） |
| file_path | VARCHAR(512) | NOT NULL | 存储路径 |
| mime_type | VARCHAR(100) | | MIME 类型 |
| size | BIGINT | | 文件大小(bytes) |
| uploader_id | VARCHAR(100) | | 上传者 ID |
| created_at | TIMESTAMP WITH TIME ZONE | DEFAULT NOW() | 创建时间 |

### 4.3 触发器

```sql
-- 自动更新 updated_at 字段
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ language 'plpgsql';

-- 应用于 users 表
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

---

## 五、API 接口文档

### 5.1 通用响应格式

**成功响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

**分页响应：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [ ... ],
    "total": 100,
    "page": 1,
    "page_size": 20,
    "total_pages": 5
  }
}
```

**错误响应：**
```json
{
  "code": 400,
  "message": "错误描述信息"
}
```

**错误码说明：**

| HTTP 状态码 | 含义 | 使用场景 |
|------------|------|---------|
| 200 | OK | 请求成功 |
| 201 | Created | 资源创建成功 |
| 400 | Bad Request | 请求参数错误、验证失败 |
| 401 | Unauthorized | 未认证或 Token 失效 |
| 403 | Forbidden | 权限不足 |
| 404 | Not Found | 资源不存在 |
| 500 | Internal Server Error | 服务器内部错误 |

### 5.2 认证相关接口

| 方法 | 路径 | 认证 | 描述 |
|------|------|------|------|
| GET | `/auth/login` | 无 | 发起 OAuth2 授权（302 重定向） |
| GET | `/auth/callback` | 无 | OAuth2 回调处理 |
| GET | `/auth/me` | Cookie | 获取当前用户信息和 access_token |
| GET | `/auth/verify` | Cookie | 验证认证状态（Nginx auth_request） |
| GET/POST | `/auth/logout` | Cookie | 登出（清除 cookie + 可选 OAuth2 登出） |

**GET /auth/me 响应示例：**
```json
{
  "code": 200,
  "message": "authenticated",
  "data": {
    "sub": "user-uuid",
    "preferred_username": "john",
    "email": "john@example.com",
    "roles": ["admin", "fly-print-admin"],
    "access_token": "eyJ..."
  }
}
```

### 5.3 用户管理接口

| 方法 | 路径 | 权限 | 描述 |
|------|------|------|------|
| GET | `/api/v1/admin/users` | admin | 用户列表（?page=1&page_size=20） |
| POST | `/api/v1/admin/users` | admin | 创建用户 |
| GET | `/api/v1/admin/users/:id` | admin | 用户详情 |
| PUT | `/api/v1/admin/users/:id` | admin | 更新用户 |
| DELETE | `/api/v1/admin/users/:id` | admin | 删除用户（软删除） |
| PUT | `/api/v1/admin/users/:id/password` | admin | 修改密码 |
| GET | `/api/v1/admin/profile` | 任何认证用户 | 当前用户业务信息 |

**POST /api/v1/admin/users 请求体：**
```json
{
  "username": "newuser",
  "email": "user@example.com",
  "password": "secret123",
  "role": "operator"
}
```

| 字段 | 类型 | 必需 | 校验规则 |
|------|------|------|---------|
| username | string | 是 | 3-50 字符 |
| email | string | 是 | 有效邮箱格式 |
| password | string | 是 | >= 6 字符 |
| role | string | 是 | admin / operator / viewer |

**PUT /api/v1/admin/users/:id 请求体：**
```json
{
  "username": "updateduser",
  "email": "updated@example.com",
  "role": "admin",
  "status": "active"
}
```

**PUT /api/v1/admin/users/:id/password 请求体：**
```json
{
  "new_password": "newpassword123"
}
```

### 5.4 边缘节点管理接口

| 方法 | 路径 | 权限 | 描述 |
|------|------|------|------|
| POST | `/api/v1/edge/register` | edge:register | 节点注册 |
| POST | `/api/v1/edge/heartbeat` | edge:heartbeat | 心跳上报 |
| GET | `/api/v1/admin/edge-nodes` | admin/operator | 节点列表（?page&page_size&status） |
| GET | `/api/v1/admin/edge-nodes/:id` | admin/operator | 节点详情 |
| PUT | `/api/v1/admin/edge-nodes/:id` | admin/operator | 更新节点 |
| DELETE | `/api/v1/admin/edge-nodes/:id` | admin/operator | 删除节点（软删除） |

**POST /api/v1/edge/register 请求体：**
```json
{
  "node_id": "edge-node-001",
  "name": "Office Node"
}
```

| 字段 | 类型 | 必需 | 校验规则 |
|------|------|------|---------|
| node_id | string | 是 | 1-100 字符 |
| name | string | 是 | 1-100 字符 |

**POST /api/v1/edge/heartbeat 请求体：**
```json
{
  "node_id": "edge-node-001"
}
```

**GET /api/v1/admin/edge-nodes 查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| status | string | - | 过滤状态（online/offline） |

**GET /api/v1/admin/edge-nodes 响应示例：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "edge-node-001",
        "name": "Office Node",
        "status": "online",
        "enabled": true,
        "version": "1.0.0",
        "last_heartbeat": "2026-02-25T10:00:00Z",
        "location": "Beijing",
        "printer_count": 3,
        "ip_address": "192.168.1.100",
        "mac_address": "AA:BB:CC:DD:EE:FF",
        "os_version": "Ubuntu 22.04",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-02-25T10:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20,
    "total_pages": 1
  }
}
```

**PUT /api/v1/admin/edge-nodes/:id 请求体：**
```json
{
  "name": "Updated Node Name",
  "enabled": false
}
```

### 5.5 打印机管理接口

| 方法 | 路径 | 权限 | 描述 |
|------|------|------|------|
| GET | `/api/v1/admin/printers` | admin/operator | 打印机列表（?page&page_size&edge_node_id） |
| GET | `/api/v1/admin/printers/:id` | admin/operator | 打印机详情 |
| PUT | `/api/v1/admin/printers/:id` | admin/operator | 更新打印机（display_name / enabled） |
| DELETE | `/api/v1/admin/printers/:id` | admin/operator | 删除打印机 |
| GET | `/api/v1/printers` | print:submit | 第三方获取可用打印机列表 |
| POST | `/api/v1/edge/:node_id/printers` | edge:printer | Edge 注册打印机 |
| GET | `/api/v1/edge/:node_id/printers` | edge:printer | Edge 查询自身打印机 |
| DELETE | `/api/v1/edge/:node_id/printers/:printer_id` | edge:printer | Edge 删除打印机 |

**GET /api/v1/admin/printers 查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| edge_node_id | string | - | 按 Edge Node 筛选 |

**PUT /api/v1/admin/printers/:id 请求体（管理员）：**
```json
{
  "display_name": "3F Color Printer",
  "enabled": true
}
```

**POST /api/v1/edge/:node_id/printers 请求体（Edge Node）：**
```json
{
  "name": "HP_LaserJet_Pro",
  "model": "HP LaserJet Pro M404dn",
  "serial_number": "SN12345",
  "firmware_version": "2.1.0",
  "port_info": "usb://HP/LaserJet",
  "ip_address": "192.168.1.50",
  "mac_address": "11:22:33:44:55:66",
  "capabilities": {
    "paper_sizes": ["A4", "A3", "Letter"],
    "color_support": true,
    "duplex_support": true,
    "resolution": "1200x1200 dpi",
    "print_speed": "30 ppm",
    "media_types": ["plain", "glossy"]
  }
}
```

### 5.6 打印任务管理接口

| 方法 | 路径 | 权限 | 描述 |
|------|------|------|------|
| POST | `/api/v1/admin/print-jobs` | admin/operator | 创建打印任务 |
| GET | `/api/v1/admin/print-jobs` | admin/operator | 任务列表（?page&page_size&status&printer_id&user_id） |
| GET | `/api/v1/admin/print-jobs/:id` | admin/operator | 任务详情 |
| PUT | `/api/v1/admin/print-jobs/:id` | admin/operator | 更新任务 |
| DELETE | `/api/v1/admin/print-jobs/:id` | admin/operator | 删除任务 |
| POST | `/api/v1/admin/print-jobs/:id/cancel` | admin/operator | 取消任务 |
| POST | `/api/v1/admin/print-jobs/:id/reprint` | admin/operator | 重新打印 |
| POST | `/api/v1/print-jobs` | print:submit | 第三方提交打印任务 |
| GET | `/api/v1/print-jobs/:id` | print:submit | 第三方查询任务状态 |

**POST /api/v1/admin/print-jobs 请求体：**
```json
{
  "printer_id": "550e8400-e29b-41d4-a716-446655440000",
  "file_path": "/uploads/document.pdf",
  "file_url": "",
  "copies": 2,
  "paper_size": "A4",
  "color_mode": "grayscale",
  "duplex_mode": "duplex"
}
```

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| printer_id | string | 是 | 目标打印机 UUID |
| file_path | string | 条件必需 | 本地文件路径（与 file_url 至少一个） |
| file_url | string | 条件必需 | 第三方文件 URL（与 file_path 至少一个） |
| copies | int | 否 | 打印份数，默认 1，范围 1-99 |
| paper_size | string | 否 | 纸张大小，需在打印机支持列表中 |
| color_mode | string | 否 | color / grayscale |
| duplex_mode | string | 否 | single / duplex |

**GET /api/v1/admin/print-jobs 查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| status | string | - | 过滤状态 |
| printer_id | string | - | 按打印机 ID 过滤 |
| user_id | string | - | 按用户 ID 过滤 |

**POST /api/v1/admin/print-jobs/:id/cancel 响应：**
- 仅 `pending` / `dispatched` 状态的任务可取消
- 返回更新后的任务对象

**POST /api/v1/admin/print-jobs/:id/reprint 请求体：**
```json
{
  "printer_id": "550e8400-e29b-41d4-a716-446655440000",
  "copies": 1,
  "paper_size": "A4",
  "color_mode": "color",
  "duplex_mode": "single"
}
```
- 仅 `completed` / `failed` / `cancelled` 状态的任务可重新打印
- 创建新任务，原任务保持不变

### 5.7 文件管理接口

| 方法 | 路径 | 权限 | 描述 |
|------|------|------|------|
| POST | `/api/v1/files` | file:upload | 上传文件（multipart/form-data） |
| GET | `/api/v1/files/:id` | 认证用户 | 下载文件（仅上传者或 admin） |

**POST /api/v1/files：**
- Content-Type: `multipart/form-data`
- 表单字段 `file`: 文件（必需）
- 查询参数 `node_id`: 目标 Edge Node（可选，指定时触发文件预览分发）
- 支持格式：`.pdf`, `.jpg`, `.jpeg`, `.png`, `.docx`
- 大小限制：50MB（可配置）

**上传成功响应：**
```json
{
  "code": 200,
  "message": "File uploaded successfully",
  "data": {
    "id": "file-uuid",
    "original_name": "document.pdf",
    "mime_type": "application/pdf",
    "size": 1048576,
    "url": "/api/v1/files/file-uuid",
    "created_at": "2026-02-25T10:00:00Z"
  }
}
```

### 5.8 Dashboard 接口

| 方法 | 路径 | 权限 | 描述 |
|------|------|------|------|
| GET | `/api/v1/admin/dashboard/trends` | admin/operator | 最近7天打印任务趋势 |

**响应示例：**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    { "date": "2026-02-19", "completed": 15, "failed": 2 },
    { "date": "2026-02-20", "completed": 20, "failed": 1 },
    { "date": "2026-02-21", "completed": 18, "failed": 0 },
    { "date": "2026-02-22", "completed": 12, "failed": 3 },
    { "date": "2026-02-23", "completed": 25, "failed": 1 },
    { "date": "2026-02-24", "completed": 22, "failed": 2 },
    { "date": "2026-02-25", "completed": 8, "failed": 0 }
  ]
}
```

### 5.9 健康检查接口

| 方法 | 路径 | 认证 | 描述 |
|------|------|------|------|
| GET | `/health` | 无 | 基础健康检查 |
| GET | `/api/v1/health` | 无 | API 健康检查 |

### 5.10 权限 Scope 映射表

| Scope | 说明 | 适用角色 |
|-------|------|---------|
| `fly-print-admin` | 管理后台完全访问 | admin |
| `fly-print-operator` | 管理后台操作权限 | operator |
| `edge:register` | Edge Node 注册 | 机器账号 |
| `edge:heartbeat` | Edge Node 心跳 + WebSocket | 机器账号 |
| `edge:printer` | Edge Node 打印机管理 | 机器账号 |
| `print:submit` | 第三方打印 API | API 客户端 |
| `file:upload` | 文件上传 | API 客户端 |
| `file:read` | 文件下载 | API 客户端 |

**注意：** `admin` 角色自动拥有所有权限，无需精确匹配 scope。

---

## 六、部署配置文档

### 6.1 Docker Compose 服务编排

**服务清单：**

| 服务 | 镜像 | 容器名 | 端口 | 说明 |
|------|------|--------|------|------|
| postgres | postgres:15 | fly-print-postgres | 内部 5432 | 主数据库 |
| redis | redis:7-alpine | fly-print-redis | 内部 6379 | 缓存服务（预留） |
| api | 自建 (Go 1.21) | fly-print-api | 内部 8080 | Go 后端 API |
| admin-console-builder | 自建 (Node 18) | fly-print-admin-builder | 无 | 前端构建（一次性） |
| nginx | nginx:alpine | fly-print-nginx | 8180:8000 | 统一入口反向代理 |

**数据卷：**
- `postgres_data` -> PostgreSQL 数据持久化
- `redis_data` -> Redis 数据持久化
- `admin_build` -> 前端构建产物（admin-builder 输出 -> nginx 挂载）
- `file_uploads` -> 上传文件存储（/root/uploads）

**网络：** `fly-print-network`（bridge 模式）

### 6.2 环境变量配置

将 `example.env` 复制为 `.env` 并修改以下关键配置：

```env
# ===== 数据库配置 =====
POSTGRES_DB=fly_print_cloud
POSTGRES_USER=postgres
POSTGRES_PASSWORD=<your-strong-password>

# ===== OAuth2 配置 =====
# 根据实际 OAuth2 提供商配置
OAUTH2_CLIENT_ID=fly-print-admin-console
OAUTH2_CLIENT_SECRET=<your-client-secret>

# Keycloak 示例:
OAUTH2_AUTH_URL=https://<keycloak-host>/realms/master/protocol/openid-connect/auth
OAUTH2_TOKEN_URL=https://<keycloak-host>/realms/master/protocol/openid-connect/token
OAUTH2_USERINFO_URL=https://<keycloak-host>/realms/master/protocol/openid-connect/userinfo
OAUTH2_LOGOUT_URL=https://<keycloak-host>/realms/master/protocol/openid-connect/logout
OAUTH2_LOGOUT_REDIRECT_URI_PARAM=post_logout_redirect_uri

# Google 示例:
# OAUTH2_AUTH_URL=https://accounts.google.com/o/oauth2/auth
# OAUTH2_TOKEN_URL=https://oauth2.googleapis.com/token
# OAUTH2_USERINFO_URL=https://www.googleapis.com/oauth2/v2/userinfo

# 回调 URI - 必须与 OAuth2 提供商中注册的一致
OAUTH2_REDIRECT_URI=http://<your-domain>:8000/auth/callback

# Admin Console URL - 认证成功后重定向到管理控制台首页
ADMIN_CONSOLE_URL=http://<your-domain>:8000

# ===== 可选：默认管理员 =====
# 仅在首次部署时设置为 true
CREATE_DEFAULT_ADMIN=true
DEFAULT_ADMIN_PASSWORD=<admin-password>
```

### 6.3 部署步骤

```bash
# 1. 克隆项目
git clone <repo-url> && cd fly-print-cloud

# 2. 配置环境变量
cp example.env .env
# 编辑 .env 文件，填写实际配置

# 3. 启动所有服务
docker compose up -d

# 4. 查看服务状态
docker compose ps

# 5. 查看 API 日志
docker compose logs -f api

# 6. 访问管理后台
# 浏览器打开 http://<your-ip>:8180

# 7. 首次部署后，关闭默认管理员创建
# 编辑 .env，设置 CREATE_DEFAULT_ADMIN=false
# docker compose restart api
```

### 6.4 Nginx 代理配置说明

**监听端口：** 8000（容器内部），映射到宿主机 8180

**路由规则：**

| Location | 目标 | 说明 |
|----------|------|------|
| `/api/` | `http://api:8080/api/` | API 请求代理 |
| `/auth/` | `http://api:8080/auth/` | OAuth2 认证代理 |
| `/api/v1/edge/ws` | `http://api:8080/api/v1/edge/ws` | WebSocket 代理（含 Upgrade 头） |
| `/` | 静态文件 (`/usr/share/nginx/html`) | Admin SPA（try_files 支持 SPA 路由） |

**WebSocket 代理关键配置：**
- `proxy_http_version 1.1`
- `proxy_set_header Upgrade $http_upgrade`
- `proxy_set_header Connection $connection_upgrade`
- `proxy_read_timeout 86400` / `proxy_send_timeout 86400`（24小时长连接）

### 6.5 Go API Dockerfile 说明

```
# 构建阶段 (Go 1.21 Alpine)
- 使用 GOPROXY 加速国内依赖下载
- go mod tidy + go mod verify 确保依赖完整
- CGO_ENABLED=0 GOOS=linux 静态编译

# 运行阶段 (Alpine 最小镜像)
- 暴露端口 8080
- 运行编译后的二进制文件
```

### 6.6 Admin Console Dockerfile 说明

```
# 构建阶段 (Node 18 Alpine)
- 使用 npmmirror.com 镜像源加速安装
- npm ci 安装依赖
- npm run build 构建生产版本

# 输出阶段 (Alpine)
- 复制构建产物到 /app/build/
- 通过共享卷传递给 Nginx
```

---

## 七、前后端交互文档

### 7.1 通信协议概览

```
React Frontend (Admin SPA)
  |
  |-- HTTP/REST --> Nginx --> Go API (Gin)
  |   |-- GET/POST/PUT/DELETE
  |   |-- Authorization: Bearer {access_token}
  |   |-- Content-Type: application/json / multipart/form-data
  |
  |-- Cookie --> Nginx --> Go API (OAuth2)
      |-- HTTP-only cookies (access_token, refresh_token, id_token)

Edge Node
  |
  |-- HTTP/REST --> Go API
  |   |-- POST /api/v1/edge/register
  |   |-- POST /api/v1/edge/heartbeat
  |   |-- POST /api/v1/edge/:node_id/printers
  |
  |-- WebSocket --> Nginx --> Go API (WS Manager)
      |-- Upstream:  heartbeat, printer_status, job_update, submit_print_params
      |-- Downstream: print_job, preview_file, node_state, printer_state, printer_deleted
```

### 7.2 前端认证流程

```
AdminApp mount
  -> fetch('/auth/me')
  -> Success: extract access_token -> store in ApiService instance
  -> Failure: window.location.href = '/auth/login'
```

### 7.3 前端数据请求流程

以打印任务列表为例：
```
PrintJobs component useEffect
  -> apiService.get('/admin/print-jobs?page=1&page_size=10&status=pending')
  -> ApiService.request()
    -> auto call getToken() to ensure token availability
    -> fetch(baseUrl + endpoint, { headers: { Authorization: Bearer xxx } })
  -> Response: { code, message, data: { items, total, page, page_size, total_pages } }
  -> Update component state: setPrintJobs(data.items), setTotal(data.total)
```

### 7.4 前端操作流程

以取消打印任务为例：
```
User clicks "Cancel" button
  -> Modal.confirm dialog
  -> apiService.put('/admin/print-jobs/{id}', { status: 'cancelled' })
  -> Success: message.success('Cancelled') -> refresh list
  -> Failure: message.error('Cancel failed: ...')
```

### 7.5 WebSocket 交互流程（Cloud <-> Edge）

**打印任务完整生命周期：**
```
1.  User creates print job (Frontend POST -> API)
2.  API creates database record (status: pending)
3.  API calls WS Manager.DispatchPrintJob() (status: dispatched)
4.  WS Manager sends print_job message to Edge Node via WebSocket
5.  Edge Node receives job, starts file download
6.  Edge -> Cloud: job_update (status: downloading)
7.  Edge finishes download, starts printing
8.  Edge -> Cloud: job_update (status: printing)
9.  Edge finishes printing
10. Edge -> Cloud: job_update (status: completed or failed)
11. Frontend sees latest status on next refresh/load
```

**文件预览打印流程：**
```
1. User uploads file (Frontend POST /api/v1/files?node_id=xxx)
2. API saves file -> generates Task Token (HMAC-SHA256)
3. WS Manager.DispatchPreviewFile() -> sends to Edge Node
4. Edge Node previews file -> user selects print params at edge side
5. Edge -> Cloud: submit_print_params (with Task Token)
6. Cloud validates Token -> creates PrintJob -> DispatchPrintJob
7. Edge executes printing
```

### 7.6 WebSocket 消息格式

**通用消息结构：**
```json
{
  "type": "message_type",
  "data": { ... }
}
```

**Edge -> Cloud 消息示例：**

心跳：
```json
{
  "type": "edge_heartbeat",
  "data": {
    "node_id": "edge-node-001",
    "timestamp": "2026-02-25T10:00:00Z"
  }
}
```

打印机状态更新：
```json
{
  "type": "printer_status",
  "data": {
    "printer_id": "printer-uuid",
    "status": "printing",
    "queue_length": 3
  }
}
```

任务状态更新：
```json
{
  "type": "job_update",
  "data": {
    "job_id": "job-uuid",
    "status": "completed",
    "progress": 100,
    "error_message": ""
  }
}
```

**Cloud -> Edge 消息示例：**

打印任务分发：
```json
{
  "type": "print_job",
  "data": {
    "id": "job-uuid",
    "name": "document.pdf",
    "printer_id": "printer-uuid",
    "file_path": "/uploads/document.pdf",
    "file_url": "",
    "copies": 2,
    "paper_size": "A4",
    "color_mode": "grayscale",
    "duplex_mode": "single"
  }
}
```

节点启用状态变更：
```json
{
  "type": "node_state",
  "data": {
    "node_id": "edge-node-001",
    "enabled": false
  }
}
```

### 7.7 数据格式约定

| 约定项 | 格式 | 示例 |
|--------|------|------|
| 时间 | ISO 8601 | `2026-02-25T10:30:00Z` |
| ID | UUID v4 | `550e8400-e29b-41d4-a716-446655440000` |
| 分页页码 | 从 1 开始 | `page=1` |
| 分页大小 | 默认 20 | `page_size=20` |
| 布尔值 | JSON | `true` / `false` |
| 空值 | JSON | `null` |

---

## 关键文件索引

| 文件路径 | 说明 |
|---------|------|
| `api/cmd/server/main.go` | 应用入口，路由配置 |
| `api/internal/config/config.go` | 配置加载和结构定义 |
| `api/internal/database/database.go` | 数据库初始化、建表 |
| `api/internal/database/user_repository.go` | 用户数据访问层 |
| `api/internal/database/edge_node_repository.go` | 节点数据访问层 |
| `api/internal/database/printer_repository.go` | 打印机数据访问层 |
| `api/internal/database/print_job_repository.go` | 任务数据访问层 |
| `api/internal/database/file_repository.go` | 文件数据访问层 |
| `api/internal/models/models.go` | 核心数据模型定义 |
| `api/internal/models/file.go` | 文件模型定义 |
| `api/internal/handlers/oauth2_handler.go` | OAuth2 认证处理器 |
| `api/internal/handlers/user_handler.go` | 用户管理处理器 |
| `api/internal/handlers/edge_node_handler.go` | 节点管理处理器 |
| `api/internal/handlers/printer_handler.go` | 打印机管理处理器 |
| `api/internal/handlers/print_job_handler.go` | 任务管理处理器 |
| `api/internal/handlers/file_handler.go` | 文件处理器 |
| `api/internal/handlers/dashboard_handler.go` | 仪表板处理器 |
| `api/internal/handlers/response.go` | 统一响应格式 |
| `api/internal/middleware/oauth2.go` | OAuth2 认证中间件 |
| `api/internal/middleware/common.go` | 日志/CORS 中间件 |
| `api/internal/websocket/manager.go` | WebSocket 连接管理器 |
| `api/internal/websocket/connection.go` | WebSocket 连接封装 |
| `api/internal/websocket/handler.go` | WebSocket 请求处理 |
| `api/internal/websocket/message.go` | WebSocket 消息类型定义 |
| `admin/src/App.tsx` | 前端根组件、路由、布局 |
| `admin/src/services/api.ts` | 前端 API 通信层 |
| `admin/src/components/pages/Dashboard.tsx` | 仪表板页面 |
| `admin/src/components/pages/EdgeNodes.tsx` | 节点管理页面 |
| `admin/src/components/pages/Printers.tsx` | 打印机管理页面 |
| `admin/src/components/pages/PrintJobs.tsx` | 任务管理页面 |
| `admin/src/components/pages/PublicUpload.tsx` | 公共上传页面 |
| `docker-compose.yml` | Docker 服务编排 |
| `example.env` | 环境变量示例 |
| `nginx/nginx.conf` | Nginx 主配置 |
| `nginx/conf.d/admin.conf` | Nginx 站点配置 |
