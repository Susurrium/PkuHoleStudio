# Changelog

## Unreleased (v0.1.0-alpha.3)

- 增加 Toolkit → Studio 一次性本地配对导入：配对码 5 分钟过期，归档先预检并等待用户确认，不传输登录凭据。
- 同步中心改为优先自动验证已有本机会话，并把 Studio 账号密码登录折叠为备用方式。
- TUI 认证状态现在区分 IAAA 与树洞会话验证阶段，短信/动态口令会提交到正确端点。
- 修复 Toolkit v1.3 真实归档中数组型 `image_size` 无法导入的问题。
- 零有效记录的预检返回明确失败，不再创建无意义的导入任务。
- 增加原生 Web 同步中心、会话检测、本机登录及短信/动态口令挑战。
- 关注、指定 PID 和公共时间线同步会在同一事务内记录帖子来源。
- 增加本地 archive v2 与逐洞 Markdown ZIP 导出，并验证 archive v2 导入闭环。
- 修复多帖子 Markdown 导出在写入第二条索引时出现 `zip: write to closed file` 的问题。
- Web 登录自动识别学校邮箱并取 `@` 前账号，同时展示 IAAA 返回的具体失败原因。
- 按北大 IAAA 当前官方流程发送/重发短信验证码，并将验证码作为 `smsCode` 完成 OAuth 登录；与树洞会话后的二次验证分阶段处理。

## v0.1.0-alpha.2

- 增加 OpenAI-compatible Provider 与 DeepSeek 配置模板。
- 增加选中内容问答、本地最多五轮检索 Agent、PID/CID 来源和流式事件。
- 增加课程/教师多维分析与统一比较表工作流。
- AI 会话、消息、检索轨迹和来源持久化到本地数据库。

## v0.1.0-alpha.1

- 保留 PKUHoleTUI 的 TUI、Crawler、模型、客户端和旧版 API。
- 增加共享 App/Service 层、版本化数据库迁移和持久任务管理器。
- 增加 SQLite FTS5 trigram/BM25 搜索和搜索历史。
- 增加 PkuHoleToolkit v1/v2 归档预检、幂等导入和 partial report。
- 增加 `/api/v1`、任务 SSE、单文件嵌入式 React Web 客户端和 `treehole web`。
