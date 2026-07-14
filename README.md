# PkuHoleStudio

本地优先的北大树洞资料库、全文搜索与 AI 研究工作台。

PkuHoleStudio 从 [PKUHoleTUI](https://github.com/dfshfghj/PKUHoleTUI) 的完整历史演进而来，保留原有 TUI、Crawler、SQLite/PostgreSQL 基础兼容和旧版 REST API，同时增加共享 Service 层、版本化迁移、持久任务、FTS5、Toolkit 归档导入和内嵌 Web 客户端。

当前预览版：`v0.1.0-alpha.3`。变更记录见 [CHANGELOG.md](CHANGELOG.md)，本版安装说明和重点变化见 [v0.1.0-alpha.3 发布说明](docs/releases/v0.1.0-alpha.3.md)。

上游锚点为 `PKUHoleTUI@f9d6221e16b1659a453866f3980c30c0cb8067e6`，本仓库标签为 `upstream-pkuholetui-f9d6221`。

## 当前能力

- TUI 在线/离线浏览，以及原有点赞、关注、发帖和回复能力。
- SQLite 本地资料库；保留 PostgreSQL 基础兼容。
- SQLite FTS5 trigram/BM25 全文搜索，支持 PID、帖子、评论片段、来源、时间、图片和标签筛选。
- 持久化同步/导入任务，支持暂停、恢复、取消、失败重试和 SSE 事件重放。
- Studio 原生支持旧版 `{holes, comments}` JSON 与 archive v2 `.treehole.zip` 导入；兼容 Toolkit v1.3 的数组型 `image_size`。
- 原生 Web 同步中心：自动检测或重新载入 TUI 保存的本机会话，支持关注/指定 PID/公共时间线同步；账号密码登录保留为备用方式。
- Studio 可对指定 PID 直接执行“在线更新正文与全部评论 → 下载图片 → 生成归档”，并独立导入和导出带图片的 archive v2 或逐洞 Markdown ZIP；旧 v2 继续兼容，图片按 SHA-256 校验和去重。
- Toolkit 仅作为独立、可选的浏览器导出工具；也可用 5 分钟有效的一次性配对码把归档发送给 Studio，核心导入导出流程不依赖它。
- 帖子详情展示帖子/评论图片、缺失媒体状态，以及明确、推断、评论引用的前向和反向关系；本地引用图可按一层或两层展开并跳转。
- Web 资料库可在本地与在线树洞之间切换，支持关注、远程标签、在线详情和远程图片；只有明确点击保存或启动同步时才写入资料库。
- 登录后的 Web 在线模式支持图片上传、发洞、普通/引用回复、点赞和关注。
- Web 支持互动/系统通知、单条/全部已读和 PID 跳转；退出登录会清除本机保存的树洞会话。
- Web 同步中心支持顺序采集、持续监控、媒体/缩略图/引用修复和暂存清理，并提供经过敏感信息遮蔽的运行日志页面。
- Web 支持实时课表与成绩查看，敏感校园数据仅保留在当前页面内存。
- Web 详情支持只保存在本机的帖子标签、整洞笔记和评论笔记；设置页可创建、改名、着色和删除标签，远端同步不会覆盖这些元数据。
- Studio 原生 archive v2 会携带标签与笔记扩展；旧版归档仍可导入，导入扩展时只补充缺失元数据，不覆盖目标资料库已有笔记。Markdown 包也包含这些本地整理信息。
- Web 导出使用持久任务：刷新页面后可恢复进度和历史，完成后下载，失败可重试；导出文件默认保留 30 天并由清理任务回收。
- Web 高级采集补齐 TUI 的原始 API JSON 缓存/保存、旧版图片补全和可选 WebP 转换；原始 JSON 作为可下载、30 天自动清理的受管理产物保存，空缓存不会生成假成功任务。
- Web 大评论区支持按游标逐批加载；资料库与全文搜索支持可随 URL 保存的本地标签多选筛选。
- 设置页可安全编辑并测试 OpenAI-compatible Provider、模型和检索限制；API key 采用只写、不回显设计，保存后运行时原子应用。
- AI 设置支持最多 20 个 OpenAI-compatible Provider，能够新增、编辑、删除和切换活动模型；旧版单 Provider 配置会自动迁移，切换无需重启且不会影响正在生成的回答。
- 本地检索严格限制最多 5 轮；课程分析会去重并覆盖最多 10 名教师，保存每轮关键词、命中数、最终 PID/CID 来源和统一维度比较提示。
- React Web：总览、帖子、详情、搜索、同步、导入导出、设置，以及 AI 功能入口。
- OpenAI-compatible AI Provider、DeepSeek 模板、本地检索 Agent、选中内容问答和课程/教师分析。
- `/api/v1` 游标 API；旧版 API 路由继续保留。

AI 默认关闭；实时树洞搜索保持显式、独立且默认关闭。

## 构建

要求 Go 1.26、Node.js 24、npm 11。SQLite 使用 `go-sqlite3`，因此需要可用的 CGO C 编译器。

```bash
cd web
npm ci
npm test
npm run build

cd ..
go test -tags sqlite_fts5 ./...
go build -tags sqlite_fts5 -o treehole ./cmd
```

`server` 与 Web 资源始终参与正式构建，不再需要 `withserver` build tag。省略 `sqlite_fts5` 时程序仍可构建，但本地搜索会回退到兼容 LIKE 模式。

## 使用

```bash
# 默认 TUI
./treehole

# 原有采集命令
./treehole crawler --max-pages 10

# API-only 兼容入口
./treehole server --host 127.0.0.1 --port 8081

# API + 内嵌 React；默认打开浏览器
./treehole web --host 127.0.0.1 --port 8080 --open=true

# 明确让 TUI 和 Web 共用同一运行资料与数据库（两个命令使用完全相同的路径）
./treehole --data-dir D:/PkuHoleStudio/profile --db-path D:/PkuHoleStudio/profile/studio.db
./treehole --data-dir D:/PkuHoleStudio/profile --db-path D:/PkuHoleStudio/profile/studio.db web

# 修复 FTS 索引
./treehole rebuild-search-index
```

Web 默认只监听 `127.0.0.1`。首次启动会在 `data/` 下生成配置、Cookie 和日志文件；默认 SQLite 文件由 `data/config.json` 的 `database.db_file` 指定。`--data-dir` 和 `--db-path` 是所有子命令共享的持久参数；同时运行 TUI 与 Web 时，Web 独占持久任务执行器，TUI 不会争抢队列。

若 TUI 已能正常登录，请让 TUI 与 Web 使用同一 `--data-dir`，然后在 Web“同步中心”点击“载入 TUI 已登录会话”；Web 会重新读取共享的 `cookies.json`，不需要再次输入密码或验证码。也可直接在 Web 登录，IAAA 与树洞二次短信验证会调用各自正确的端点。密码只用于当前请求，不持久化、不写入配置且不会由 API 回显。浏览器树洞页面的 HttpOnly Cookie 不会被 Studio 网页读取；Toolkit 只作为独立的简易归档导出/迁移工具，不是 Studio 同步、导入或导出的依赖。

启用 DeepSeek 或其他 OpenAI-compatible Provider：

```json
{
  "ai": {
    "enabled": true,
    "allow_live_search": false,
    "max_search_rounds": 5,
    "provider": {
      "name": "DeepSeek",
      "base_url": "https://api.deepseek.com",
      "api_key": "",
      "model": "deepseek-chat",
      "temperature": 0.2,
      "max_output_tokens": 4096,
      "request_timeout_seconds": 120
    }
  }
}
```

API key 也可通过 `PKUHOLE_AI_API_KEY` 环境变量提供，网页不会回显密钥。

前端开发：

```bash
# 终端一：API
go run -tags sqlite_fts5 ./cmd server --host 127.0.0.1 --port 8080

# 终端二：Vite，自动代理 /api/v1
cd web
npm run dev
```

## API v1

所有新接口位于 `/api/v1`：

```text
GET  /health
GET  /capabilities
GET  /posts
GET  /posts/:pid
GET  /posts/:pid/comments
GET  /search
GET  /media/:id

GET  /session
POST /session/probe
POST /session/reload
POST /session/login
POST /session/sms
POST /session/challenge

GET  /jobs
POST /jobs
GET  /jobs/:id
GET  /jobs/:id/events
POST /jobs/:id/pause|resume|cancel|retry

POST /imports
GET  /imports/:id
POST /exports
POST /exports/jobs

GET  /ai/providers
GET  /ai/sessions
POST /ai/sessions
GET  /ai/sessions/:id
POST /ai/sessions/:id/messages
GET  /ai/sessions/:id/events
POST /ai/sessions/:id/cancel
```

错误统一返回：

```json
{
  "error": {
    "code": "invalid_input",
    "message": "cursor must be between 0 and 100",
    "details": { "field": "cursor" }
  }
}
```

任务事件接口使用 SSE。客户端可发送 `Last-Event-ID`，服务端会先从数据库补发缺失事件，再继续实时推送。

## 目录

```text
cmd/                 Cobra 命令、TUI/server/web 入口
internal/app/        应用组合根
internal/service/    TUI、Web 和任务共享的 Service 层
internal/db/         Repository、迁移、FTS 与持久任务存储
internal/jobs/       单写任务调度器和事件流
internal/archive/    Toolkit v1/v2 解析、预检、导入与 Studio 导出
server/              Gin 旧路由、API v1 与 SPA 托管
web/                 React + TypeScript + Vite 客户端和测试
```

## 测试

```bash
go test ./...
go test -tags sqlite_fts5 ./...

cd web
npm test
npm run e2e
```

Playwright 覆盖 Dashboard → 导入 → 搜索 → 帖子详情 → AI 入口主流程。发布工作流按“前端安装与测试 → 前端 build → Go test → Go build”执行。

发布前真实数据与真实模型验收见 [alpha.3 验收清单](docs/alpha3-acceptance.md)。

## 安全与隐私

- Web 默认仅绑定本机回环地址。
- 密码与验证码接口拒绝非回环来源；密码不会持久化或出现在 API 响应中。
- 归档上传采用随机暂存文件、大小限制、ZIP 路径/CRC/展开体积校验。
- `referenced` 归档内容作为上下文保存，但默认不出现在普通资料库列表和搜索中。
- API 不回显任务中的本地暂存路径。
- AI 与实时在线搜索默认未启用；未来启用时仍由独立开关控制。

## License

沿用上游项目的许可证；详见 [LICENSE](LICENSE)。
