# FlyPrint Cloud — Agent

按需加载（勿整仓通读）：

| 任务 | 文档 |
|------|------|
| 协议 / 目录 / 第三方与 Demo | `docs/agent/architecture-and-protocols.md` |
| 启动 / 路由 / 测试命令 | `docs/agent/operations-and-verification.md` |
| 发版 P0/P1 待办 | `docs/agent/release-plan.md`（上下文丢失时先读；与 Edge 同名文件同步） |
| 人类部署说明 | `README.md` |

## 硬规则

- 改前定位：路由 → 请求模型 → handler → repository → 前端 → Cloud-Edge 全链路。
- 禁止未确认的兜底、替代链路或协议分支；改方案先对话确认。
- 可先写小 demo；合入后不得保留重复实现。
- 改 schema：在 `InitTables` 兼容旧实例，并补 repository/handler/测试/清理。
- 改 Cloud-Edge 协议：同步 `message.go`、序列化测试、Cloud provider test、Edge consumer test。协议以 Go 源码为准，Swagger 不完整。
- 保留工作区已有改动；禁止 `docker compose down -v`（删卷）。
- 不提交密码、JWT/文件访问/MinIO 密钥或生产配置；`.env.example` 仅模板。
- 提交前检查 `git status --short`、相关 diff 与测试；源码变则更新受影响说明。
