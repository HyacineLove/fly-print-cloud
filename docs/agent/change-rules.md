# FlyPrint Cloud Agent 改动规则与风险清单

## 9. Agent 改动规则与风险清单

- 先定位路由、请求模型、handler、repository 和前端调用的完整链路，再修改单点；不要只改 Swagger 或前端类型。
- 新增/修改数据库字段必须在 `InitTables` 兼容旧实例，并补 repository、handler、测试和数据清理逻辑；长期方案应引入有版本/回滚的迁移工具。
- 新增/修改 Cloud-Edge 协议必须同时更新消息结构、序列化测试、Cloud provider test 和 Edge consumer test。
- 不要把默认密码、JWT 签名密钥、文件访问密钥、MinIO 密钥带入提交或生产环境；`.env.example` 仅是模板。
- 不要在保留数据的环境执行 `docker compose down -v`；它会删除命名卷。
- 生产固定 MinIO 镜像版本，明确 HTTPS 下的 `COOKIE_SECURE`、CORS、外部 URL 和 Keycloak issuer。
- 注意当前 `api/internal/middleware/oauth2.go` 的 JWT 解析/外部 UserInfo 双路径，任何安全相关改动都必须重新核对真实签名验证和 issuer 配置。
- 保留工作区已有改动；本次调查时 Git 已存在 `docs/superpowers` 两个删除项以及 `.claude/`、`README.md` 未跟踪项，除非用户明确要求，不要恢复、删除或重写它们。
- 仅新增文档时不要顺手格式化或重构业务代码；提交前检查 `git status --short` 和 `git diff -- AGENTS.md`。
