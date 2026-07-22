# Cloud 架构与协议

## 定位

云端控制面与文件服务；不直连现场打印机。边界协议权威：`api/internal/websocket/message.go` 及 Go 路由/模型（Swagger 不全）。

```text
api/cmd/server/          入口、路由、后台任务
api/internal/handlers/   HTTP
api/internal/database/   PG、InitTables、repository
api/internal/websocket/  云边长连接
api/internal/storage/    local / MinIO
admin/                   React 管理端 + 公共上传
nginx/ + docker-compose.yml
```

栈：Go 1.25 / Gin / PG / WS；认证 `builtin`|`keycloak`；存储 `local`|`minio`（下载建议 `proxy`）；前端 React 18 + Ant Design；Compose 入口默认 `:8012`。

## 终端结果协议

- `print_job` 目标：`job_id` + `printer_id` + `file_url` + `content_hash`。`printer_name` 不在线契约、禁止作路由回退。
- 投递 ACK（最多 3 次）= Edge 已持久化收件，≠ 打印完成。
- 终态（`completed/failed/canceled/unconfirmed`）须带稳定 UUID `event_id`；`processing` 为尽力而为。
- Cloud 校验节点拥有目标打印机后，终态与 `edge_job_update_receipts` 同事务；`job_update_ack/accepted` 允许 Edge 清 outbox；`rejected` 为协议错，Edge 保留可见故障且不重试。
- 同 `event_id` 幂等接受；同 ID 不同 node/job/status/payload hash → reject。终态单调；例外：`unconfirmed/dispatch_ack_timeout` 可被真实终态替换。

## 节点删除与预览绑定

- 节点删除=软删节点与打印机；历史任务/票据/集成请求/回调保留。同事务取消活动票据与未终态集成请求；清 ephemeral 映射后断 WS。禁止硬删打印机（`terminal_tickets.printer_id` FK 保留）。
- `waiting_terminal` 预览：`node_id`+`terminal_session_id` 匹配且 Edge ticket hash 仍为 NULL 时可首次绑定；绑定后后续须 hash 精确匹配。绑定后上报一次 `terminal_session_state`；用户确认参数前不建 `print_job`。

## 二维码入口

- 仅 Edge `/api/qr_code`。Cloud 回相对 `/entry?token=...`；Edge 用 `cloud.base_url` 拼并对 localhost 改写局域网 IP（http 演示用；HTTPS 用域名）。`cloud.base_url` 支持 http(s)，WS 为 ws(s)。不依赖 `EXTERNAL_API_URL` 绝对地址。
- `/entry` 校验上传凭证后签发独立 `terminal_ticket`；官方再发上传凭证进 `/upload`；第三方只传终端票据。

## 部署边界

- Cloud = 受控 Linux 上的认证/任务/文件/对象存储/云边通信。
- Edge = 一体机；Cloud 用独立可验证设备身份绑定 `node_id`，禁止用 MAC/共享 Client/请求参数推断身份。
- Edge 扩大暴露面（非回环等）须先确认并补鉴权。

## WebSocket（摘要）

Cloud→Edge：`print_job`、`preview_file`、`upload_token`、`node_state`、`config_update`、`report_status`、`error`  
Edge→Cloud：`edge_heartbeat`、`job_update`、`submit_print_params`、`request_upload_token`、`ack`  
文件 payload 带 `content_hash` + 短期 `file_access_token`；Edge 现用 hash 作缓存键并验格式，**尚未**对下载字节重算 SHA-256。

## 第三方交互式打印与 Demo

- 文件接管后不下发 `print_job`；`integration/terminal_dispatcher.go` 先发标准 `preview_file`（可选三项集成上下文 + 建议 `print_options`）。
- 用户确认 → `submit_print_params` 回传上下文；Cloud 同事务校验后**每个集成请求仅一个**标准任务。官方分支不要求集成字段。
- `allow_private_file_hosts` 默认关；开启后仍仅 `allowed_file_hosts` 精确主机，并拒绝环回/链路本地等。禁止当全局私网放行。
- Demo：`integration-demo/`，provider=`livacloud-demo`，路径 `/integration-demo/`。模拟 SSO/HMAC/callback；禁止在核心链路加 provider 专属分支。密钥粘贴到 `/integration-demo/setup`，不回显、不落日志。

## 已知缺口（勿当已交付）

Users/Settings 占位；Dashboard 失败可能假数据；无 CI；无版本化 DB 迁移；MinIO 常用 `latest`；E2E/断线/升级兼容未成门禁。
