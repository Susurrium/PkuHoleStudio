# PkuHoleStudio

本地优先的北大树洞资料库、全文搜索与 AI 研究工作台。

PkuHoleStudio 从 [PKUHoleTUI](https://github.com/dfshfghj/PKUHoleTUI) 的完整历史演进而来，保留原有 TUI、Crawler、SQLite/PostgreSQL 基础兼容和旧版 REST API，同时增加共享 Service 层、版本化迁移、持久任务、FTS5、Toolkit 归档导入和内嵌 Web 客户端。

上游锚点为 `PKUHoleTUI@f9d6221e16b1659a453866f3980c30c0cb8067e6`，本仓库标签为 `upstream-pkuholetui-f9d6221`。

## 当前能力

- TUI 在线/离线浏览，以及原有点赞、关注、发帖和回复能力。
- SQLite 本地资料库；保留 PostgreSQL 基础兼容。
- SQLite FTS5 trigram/BM25 全文搜索，支持 PID、帖子、评论片段、来源、时间、图片和标签筛选。
- 持久化同步/导入任务，支持暂停、恢复、取消、失败重试和 SSE 事件重放。
- PkuHoleToolkit 旧版 `{holes, comments}` JSON 与 archive v2 `.treehole.zip` 导入。
- React Web：总览、帖子、详情、搜索、导入、设置，以及 AI 功能入口。
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

# 修复 FTS 索引
./treehole rebuild-search-index
```

Web 默认只监听 `127.0.0.1`。首次启动会在 `data/` 下生成配置、Cookie 和日志文件；默认 SQLite 文件由 `data/config.json` 的 `database.db_file` 指定。

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

GET  /jobs
POST /jobs
GET  /jobs/:id
GET  /jobs/:id/events
POST /jobs/:id/pause|resume|cancel|retry

POST /imports
GET  /imports/:id

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
internal/archive/    Toolkit v1/v2 解析、预检和导入
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

## 安全与隐私

- Web 默认仅绑定本机回环地址。
- 归档上传采用随机暂存文件、大小限制、ZIP 路径/CRC/展开体积校验。
- `referenced` 归档内容作为上下文保存，但默认不出现在普通资料库列表和搜索中。
- API 不回显任务中的本地暂存路径。
- AI 与实时在线搜索默认未启用；未来启用时仍由独立开关控制。

## License

沿用上游项目的许可证；详见 [LICENSE](LICENSE)。
