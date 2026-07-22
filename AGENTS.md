# FlyPrint Cloud — Agent

按需加载（勿整仓通读）：

| 任务 | 文档 |
|------|------|
| **开发计划 / 任务清单** | 工作区根目录 `FlyPrint开发计划.md`、`FlyPrint任务清单.md`（先读） |
| **全量归档（防上下文丢失）** | 工作区根目录 `FlyPrint总开发计划.md` |
| 协议 / 目录 / 第三方与 Demo | `docs/agent/architecture-and-protocols.md` |
| 启动 / 路由 / 测试命令 | `docs/agent/operations-and-verification.md` |
| 发版 P0/P1 待办（M0） | `docs/agent/release-plan.md`（与 Edge 同名文件同步） |
| **M0 完整交接手册（局域网主路径）** | `docs/M0演示交接手册.md` |
| **第三方接入指南（对接契约）** | `docs/第三方接入指南.md`（含 http(s) 双兼容） |
| 演示交付一页纸 | `docs/演示交付说明.md` |
| http(s)/ws(s) 双兼容 | Provider/file URL 校验；Edge 见对仓 `url_scheme.py` |
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
- **完成态**：`[x]` 仅表示已合入（及该项验收所要求的打包/预演）；「代码/单测通过」最多 `[~]`。细则见根目录 `FlyPrint任务清单.md`「用法」第 4 条；勾选须与 `docs/agent/release-plan.md` 同步。
