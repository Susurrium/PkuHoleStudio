# Changelog

## Unreleased (v0.1.0-alpha.3)

- Studio archive v2 现在可选携带 `media/index.json` 与经过 SHA-256 校验的图片文件；旧 v2 仍可直接导入。
- 导入会保留帖子和评论的远程媒体元数据，缺失图片可通过 `repair_media` 持久任务补全，Web 详情支持离线图片和缺失状态。
- Markdown 阅读包会携带本地可用图片，并为尚未下载的媒体写入明确占位说明。
- 引用关系区分明确 PID、上下文推断和评论引用，并支持前向、反向展示及 `rebuild_references` 修复任务。
- 成功或取消的导入会清理暂存文件，启动时自动清理超过七天且未被活动任务使用的孤立文件。
- Web 资料库增加本地/在线双模式、关注和远程标签筛选；在线浏览不会自动写入本地资料库，可在详情页明确创建保存任务。
- Web 在线详情支持帖子与评论远程图片，Dashboard 增加最近 12 小时热榜。
- Web 在线模式增加多图片上传、发洞、普通/引用回复、点赞和关注；所有写操作复用 TUI 的 PostService 并限制为本机可写会话。
- 通知读取和已读操作迁入共享 NotificationService，Web 增加互动/系统通知页面，TUI 不再直接调用通知 Client。
- Web 增加退出本机会话并清除 Cookie/Token；同步页以 Studio 原生登录为主，Toolkit 仅保留为可选归档迁移工具。
- 持久任务增加顺序采集、持续监控、缩略图修复和暂存清理；Web 同步中心提供对应的高级采集与修复表单。
- 增加经过路径与凭据遮蔽的 Crawler/TUI 日志 Service、API 和 Web 日志页面。
- Web 增加课表与成绩页面；沿用 TUI 的实时在线读取逻辑，默认隐藏成绩，数据不进入资料库、归档、日志或 AI。
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
