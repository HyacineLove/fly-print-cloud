## Fly Print Admin 交接文档

### 1. 项目概览

- **项目名称**: `fly-print-cloud/admin`（管理后台前端）
- **职责**: 提供云端打印管理功能的后台界面，包括边缘节点、打印机、打印任务、OAuth2 客户端等的管理，以及独立的公共上传页面。
- **技术栈**:
  - React 18
  - TypeScript
  - React Router v6
  - Ant Design 5
  - Create React App (react-scripts 5)

### 2. 运行与构建

- **依赖安装**

  ```bash
  npm install
  ```

- **本地开发**

  ```bash
  npm start
  ```

  默认运行在 `http://localhost:3000`。

- **单独构建（在 admin 目录下执行）**

  ```bash
  npm run build
  ```

  产物在 `build/`，可用任意静态服务器（如 `serve` 或 Nginx）托管。  
  **此时以本目录下的 `.env` 为准**，构建时会读取 `REACT_APP_API_BASE_PATH`、`REACT_APP_AUTH_BASE_PATH` 等。

- **整体构建（在 fly-print-cloud 目录下使用 docker-compose）**

  在 `fly-print-cloud` 目录下使用统一的 `docker-compose.yml`：

  - 执行 `docker compose build` / `docker compose up -d` 会构建并启动 postgres、api、admin-console-builder、nginx 等服务。
  - **此时以 `fly-print-cloud` 的配置为准**：前端构建用到的 `REACT_APP_*` 由 docker-compose 的 `build.args` 传入，值来自 `fly-print-cloud/.env`（或 compose 环境）。**`admin/.env` 不会被打进镜像**（已通过 `admin/.dockerignore` 排除），因此无论 admin 目录下 `.env` 填什么，都不会影响 docker-compose 构建出的前端。
  - 若需修改 API/Auth 路径，请编辑 `fly-print-cloud/.env` 中的 `REACT_APP_API_BASE_PATH`、`REACT_APP_AUTH_BASE_PATH`，然后重新构建 `admin-console-builder`。

### 3. 环境变量与后端 API 配置（重点）

**用户只需修改 env/config，无需改代码**：路径、端口等均通过环境变量注入。  
- 在 **admin 目录** 直接 build/开发：只改 **admin/.env**（可参考 `admin/.env.example`）。  
- 使用 **docker-compose** 构建/运行：只改 **fly-print-cloud/.env**（可参考 `fly-print-cloud/.env.example`），包括数据库、API、OAuth2、`REACT_APP_*`、`HTTP_PORT` 等。  
本项目已做前后端路径解耦，所有后端路径前缀从环境变量读取。

- **相关环境变量**

  - `REACT_APP_API_BASE_PATH`
    - 说明: API 基础路径前缀。
    - 示例:
      - `/api/v1`
      - `/fly-print-api/api/v1`
  - `REACT_APP_API_URL`
    - 说明: 旧版兼容字段，可写完整 URL，例如 `http://host/fly-print-api`，前端只会取其中的 path `/fly-print-api`。
    - 建议: 如果能确定只用 path，优先用 `REACT_APP_API_BASE_PATH`，更直观。
  - `REACT_APP_AUTH_BASE_PATH`
    - 说明: 认证相关接口的前缀。
    - 示例:
      - `/auth`
      - `/fly-print-api/auth`

- **默认值**

  若未配置上述变量，则：

  - `API_BASE_PATH` 默认 `/api/v1`
  - `AUTH_BASE_PATH` 默认 `/auth`

- **`src/config.ts` 的职责**

  - 计算并导出：

    ```ts
    export const API_BASE_PATH: string;
    export const AUTH_BASE_PATH: string;
    export const buildApiUrl: (path: string) => string;
    export const buildAuthUrl: (path: string) => string;
    ```

  - 使用方式：
    - `buildApiUrl('/admin/printers')` →
      - 环境为 `/api/v1` 时: `/api/v1/admin/printers`
      - 环境为 `/fly-print-api/api/v1` 时: `/fly-print-api/api/v1/admin/printers`
    - `buildAuthUrl('token')` →
      - `/auth/token` 或 `/fly-print-api/auth/token`

- **单独 build 场景配置示例**

  在本目录 `.env` 中配置（可复制 `.env.example` 为 `.env` 后修改）：

  ```env
  # 直接走 /api/v1
  REACT_APP_API_BASE_PATH=/api/v1
  REACT_APP_AUTH_BASE_PATH=/auth

  # 或者走网关 /fly-print-api/api/v1
  # REACT_APP_API_BASE_PATH=/fly-print-api/api/v1
  # REACT_APP_AUTH_BASE_PATH=/fly-print-api/auth
  ```

- **docker-compose 场景配置**

  在 `fly-print-cloud/.env` 中配置（复制 `fly-print-cloud/.env.example` 为 `.env` 后按需修改）。以下仅对 docker-compose 构建/运行生效，admin 本地的 `.env` 在 Docker 构建时会被忽略：

  ```env
  # 前端 API/Auth 路径（同域部署通常与 Nginx 代理一致）
  REACT_APP_API_BASE_PATH=/api/v1
  REACT_APP_AUTH_BASE_PATH=/auth

  # Nginx 对外端口，改端口只需改此配置
  HTTP_PORT=8012
  ```

  若使用同域部署且 Nginx 已将上述路径代理到后端，保持默认即可。后端或网关只要保证这些前缀下有对应路由。

### 4. 目录结构与模块职责

当前结构（省略 `node_modules/` 等非源码目录）：

```text
admin/
  .dockerignore          # Docker 构建时排除 .env，使 compose 构建以 fly-print-cloud/.env 为准
  .env                   # 仅本地开发/单独 build 时生效
  .env.example           # 配置模板，复制为 .env 后按需修改
  Dockerfile
  package.json
  package-lock.json
  tsconfig.json
  public/
    index.html
  src/
    index.tsx
    App.tsx
    config.ts
    services/
      api.ts
    utils/
      errorHandler.ts
    components/
      ErrorBoundary.tsx
      Loading.tsx
      pages/
        Dashboard.tsx
        EdgeNodes.tsx
        Printers.tsx
        PrintJobs.tsx
        OAuth2Clients.tsx
        Users.tsx
        Settings.tsx
        Login.tsx
        PublicUpload.tsx
```

- **入口文件**
  - `src/index.tsx`: 创建 React 根节点并渲染 `<App />`。
  - `src/App.tsx`: 包含路由、侧边菜单、头部、布局及登录态检查逻辑，是管理后台的主应用壳。

- **配置与基础设施**
  - `src/config.ts`:
    - 管理 API/Auth 前缀和 URL 构造函数，是后端地址配置的唯一入口。
  - `src/services/api.ts`:
    - 封装通用 API 调用逻辑，包括：
      - `get/post/put/delete`
      - 文件上传 `uploadFile`
      - 上传预检 `preflightUpload`
      - 下载文件 `downloadFile`
    - 内部统一通过 `buildApiUrl` 和 `buildAuthUrl` 访问后端。
  - `src/utils/errorHandler.ts`:
    - 处理全局错误提示、Message 弹窗等通用错误处理逻辑。
  - `src/components/Loading.tsx`:
    - 全屏/局部加载状态组件。
  - `src/components/ErrorBoundary.tsx`:
    - React 错误边界，负责捕获渲染期异常并展示友好错误信息。

- **页面组件 (`src/components/pages/*`)**
  - `Dashboard.tsx`:
    - 展示整体统计数据和趋势图表。
    - 直接使用 `fetch` + `buildApiUrl` 调用 `/admin/printers`、`/admin/edge-nodes`、`/admin/print-jobs` 等接口。
  - `EdgeNodes.tsx`:
    - 边缘节点管理：列表、排序、搜索、编辑名称、启停、删除（软删除说明在 UI 上）。
    - 所有请求统一走 `buildApiUrl('/admin/edge-nodes...')`。
  - `Printers.tsx`:
    - 打印机管理：列表、过滤、编辑别名、启停、删除。
    - 支持按 Edge Node、状态和名称搜索。
  - `PrintJobs.tsx`:
    - 云端打印任务管理：列表、按状态/节点/时间过滤、分页、取消任务。
  - `OAuth2Clients.tsx`:
    - OAuth2 客户端管理：创建、编辑、重置密钥、删除。
    - 用 `/auth/me` 获取 access token 再访问 `/admin/oauth2-clients`。
  - `Users.tsx`:
    - 当前实现是一个占位页面（简单内容），预留给后续用户管理功能。
  - `Settings.tsx`:
    - 同样是基础占位，后续可扩展为系统设置页面。
  - `Login.tsx`:
    - 登录页面，支持两种模式：
      - `GET /auth/mode` 返回 `keycloak` 时，直接跳转到 `/auth/login`。
      - 否则走本地 OAuth2 密码模式，调用 `/auth/token` 并手动写入 `access_token` cookie。
  - `PublicUpload.tsx`:
    - 无需登录的公共上传页面（扫描二维码打开）。
    - 逻辑：
      - 从 URL 读取 `token`、`node_id`、`printer_id`。
      - 使用 `buildApiUrl('/files/verify-upload-token?...')` 进行轻量校验。
      - 调用 `apiService.preflightUpload` 和 `apiService.uploadFile` 完成预检与真正上传。
      - 内置错误码与英文错误消息映射，向用户展示友好的中文提示。

### 5. 路由与页面导航

路由全部定义在 `App.tsx` 中，使用 React Router v6：

- 外层使用 `<Router>` 包裹。
- 路由结构：

  - `/upload` → `PublicUpload`（无需登录）
  - `/login` → `Login`
  - `/*` → 进入 `AdminApp`（需要登录）
    - `/` → `Dashboard`
    - `/edge-nodes` → `EdgeNodes`
    - `/printers` → `Printers`
    - `/print-jobs` → `PrintJobs`
    - `/users` → `Users`
    - `/oauth2-clients` → `OAuth2Clients`
    - `/settings` → `Settings`

侧边菜单项与这些路径一一对应，定义在 `App.tsx` 的 `menuItems` 中。

### 6. 认证与会话处理

- **登录态检查（AdminApp 内）**
  - 挂载时调用 `GET /auth/me`：
    - 若返回 401 或 `response.ok` 为 `false`，直接重定向到 `/login`。
    - 若成功，提取 `user_id`、`preferred_username`／`username` 等字段，存入本地 `user` 状态。
  - 检查期间展示 `<Loading fullscreen tip="加载用户信息..." />`。

- **登录流程（Login.tsx）**
  - 首先调用 `GET /auth/mode`：
    - 若 `mode === 'keycloak'`，则 `window.location.href = '/auth/login'`。
    - 否则展示用户名/密码表单。
  - 提交表单：
    - 调用 `POST /auth/token`（`application/x-www-form-urlencoded`）。
    - 若成功返回 `access_token`:
      - 按 `expires_in` 写入 `access_token` cookie。
      - 重定向到 `/`。

- **退出登录**
  - `App.tsx` 中 `handleLogout` 调用 `POST /auth/logout`（忽略错误），然后统一跳到 `/login`。

### 7. API 调用约定与示例

- **统一前缀约定**
  - 所有后端请求必须通过 `buildApiUrl` 和 `buildAuthUrl` 构建 URL。
  - 禁止再次直接写死 `/api/v1/...` 或 `/fly-print-api/...`。

- **推荐调用方式**
  - **通用 API**：优先使用 `apiService`，例如：

    ```ts
    // GET
    const res = await apiService.get('/admin/printers');

    // POST
    await apiService.post('/admin/oauth2-clients', payload);
    ```

  - **特殊情况（如 Dashboard 或现有服务类中直接使用 fetch）**：

    ```ts
    const res = await fetch(buildApiUrl('/admin/edge-nodes'), { headers: { Authorization: `Bearer ${token}` } });
    ```

- **错误处理**
  - 建议在捕获异常后使用 `ErrorHandler` 中的工具方法（`handleError`、`message` 等）展示友好提示。

### 8. 历史清理与约定

在本轮整理中，已经移除了一些不再使用的文件，以避免新开发者混淆：

- 已删除：
  - 旧版 Dashboard demo：`src/components/Dashboard.tsx`
  - 未使用的通用 hooks：`src/hooks/useAsync.ts`
  - 未使用的空状态组件：`src/components/EmptyState.tsx`
  - 未使用的 dashboard service：`src/services/dashboard.ts`
  - 已不再需要的开发代理：`src/setupProxy.js`

**重要约定**：

- 新增 API 时，请务必：
  - 通过 `buildApiUrl` / `buildAuthUrl` 构建请求路径。
  - 如果逻辑有一定复用度，优先在 `services/` 层封装，而不是直接在页面里散落 `fetch`。

### 9. 后续扩展建议（给接手者的参考）

当前结构对中小规模足够简单直接，如果后续功能继续增长，可以考虑：

- **按功能拆分 features 目录**

  ```text
  src/
    features/
      dashboard/
      printers/
      edge-nodes/
      print-jobs/
      oauth2-clients/
      users/
      settings/
      auth/
      public-upload/
  ```

  将对应页面、子组件和特定 hooks/service 聚合到一起，提升可维护性。

- **抽离路由配置**

  将路由表从 `App.tsx` 抽到独立文件（如 `routes.tsx`），在一个地方集中维护“路径 → 组件 → 菜单项”的映射。

- **统一类型定义**

  如果未来公共数据结构（如 `Printer`, `EdgeNode`, `PrintJob` 等）在多处使用，可以考虑建立 `src/types/` 集中管理。

---

如需快速上手，建议阅读顺序：

1. `src/App.tsx`（理解整体布局和路由结构）
2. `src/config.ts`（理解 API 前缀/环境变量策略）
3. `src/services/api.ts`（通用请求封装）
4. 按需查看 `src/components/pages/*` 中对应业务页面实现。

