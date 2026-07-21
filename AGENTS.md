# FlyPrint Cloud Agent 操作规范

本文件只保留执行任务时必须遵守的通用规范。项目说明按主题拆分如下：

- [架构、技术栈与协议](docs/agent/architecture-and-protocols.md)
- [运行、接口与验证](docs/agent/operations-and-verification.md)
- [改动规则与风险清单](docs/agent/change-rules.md)
- [发版开发计划 P0/P1](docs/agent/release-plan.md)（plan-execute 权威待办；上下文丢失时先读）

## 必须遵守

- 修改前先定位路由、请求模型、handler、repository、前端调用和 Cloud-Edge 完整链路，确认数据流与状态流转后再改动。
- 不得擅自增加未获确认的兜底、替代链路或协议分支；需要改变既定方案时先在对话中提出建议并等待确认。
- 可以先创建小 demo 或离线测试验证可行性，确认后再合并到生产代码；合并后不得保留重复实现。
- 保留工作区已有改动；不要执行 `docker compose down -v` 等会删除持久化数据的命令。
- 不提交密码、JWT 签名密钥、文件访问密钥、MinIO 密钥或生产环境配置；`.env.example` 仅作模板。
- 提交前检查 `git status --short`、相关 diff 和测试结果；源代码变化后同步更新受影响的说明文档。

## 2026-07-21 本轮验证记录

- 节点删除采用节点与打印机软删除；活动终端票据和未完成第三方请求先取消，历史任务、订单、票据与回调记录保留。
- 第三方预览首次匹配允许当前 Edge 会话尚未保存票据哈希；绑定后要求票据哈希、会话 ID 和集成请求 ID 严格一致。
- Cloud API `go test ./...` 已通过；本轮变更已纳入 Git 提交。
- 发版待办见 `docs/agent/release-plan.md`。
