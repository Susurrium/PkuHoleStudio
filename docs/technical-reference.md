# PkuHoleStudio 技术参考

本文面向需要从源码构建、使用命令行或 API、维护部署和参与开发的读者。普通使用者请直接阅读 [README](../README.md)。

## 运行入口

所有入口共享同一套配置、数据库和服务层。

| 命令 | 用途 |
| --- | --- |
| `treehole` / `treehole web` | 启动本机 Web 与 API，并默认打开浏览器 |
| `treehole tui` | 启动键盘操作的 TUI |
| `treehole crawler` | 运行兼容的批量采集工具 |
| `treehole server` | 启动 API-only 服务 |
| `treehole rebuild-search-index` | 重建 SQLite 全文搜索索引 |

常用参数：

```bash
# 指定共享资料目录和 SQLite 文件
treehole --data-dir D:/PkuHoleStudio/profile \
  --db-path D:/PkuHoleStudio/profile/studio.db web

# 指定本机 Web 端口
treehole web --host 127.0.0.1 --port 8080 --open=true

# 兼容的页码采集
treehole crawler --start-page 1 --max-pages 10

# 补全旧版图片或缩略图缓存；输出目录服从 --data-dir
treehole crawler fetch-images
treehole crawler fetch-thumbnails --start-id 1 --end-id 100
```

`--data-dir` 控制配置、Cookie、日志、媒体、任务产物和旧版图片缓存的位置。默认 SQLite 配置为相对路径 `./treehole.db`；需要让数据库跟随资料目录时，应同时传入 `--db-path`。

Web 只允许绑定 `localhost` 或回环 IP。API-only 服务若确需监听局域网地址，必须显式启用远程访问，并提供至少 24 个字符的 Bearer Token：

```powershell
$env:PKUHOLE_SERVER_API_TOKEN = "请替换为至少24字符的随机值"
.\treehole-windows-amd64.exe server --host 0.0.0.0 --port 8081 --allow-remote
```

## 实现概览

- TUI、Crawler、Web/API 共用 `internal/app` 组合的 Post、Search、Sync、Media、Library、Settings、Archive、Observer、AI 与 Jobs 服务。
- 默认使用 SQLite，启用 WAL、忙等待和外键；保留 PostgreSQL 基础兼容。SQLite FTS5 使用 trigram/BM25，无法使用 FTS5 或查询过短时回退到普通匹配。
- 在线与本地是明确的两种数据源。在线浏览不会自动写库；保存指定 PID 时会逐页读取评论，并按需下载图片。
- 本地元数据包括标签、帖子笔记、评论笔记、研究项目和引用关系。
- 持久任务支持暂停、恢复、取消、失败重试、项目检查点和 SSE 事件补发。进程重启后，原运行中任务会恢复为暂停状态。
- 现代媒体服务按内容哈希保存文件并记录缺失或失败状态；旧图片/缩略图缓存工具仍作为 CLI/TUI 兼容入口。

## 归档、Toolkit 与 Observer

Studio 可预检并导入旧 `{holes, comments}` JSON 和 Archive v2 ZIP，也可导出 Archive 2.1 或 Markdown ZIP。归档可包含帖子、评论、来源、本地标签与笔记、媒体和媒体缺失状态。

Archive Contract 的权威说明与测试夹具位于：

- [Archive 规范](../internal/archive/testdata/contract/schemas/SPECIFICATION.md)
- [黄金归档说明](../internal/archive/testdata/contract/v2/archives/README.md)

Toolkit 是可选的独立发送端，不是 Studio 的运行依赖。Bridge 支持旧版一次性接收码，以及基于六位码核对、公钥信任、P-256 签名和文件 SHA-256 绑定票据的设备信任流程；归档进入 Studio 后仍需预检和本机确认。

Observer 同样是可选的独立服务。Studio 客户端只接受 HTTPS 服务地址，支持事件增量同步、删除快照校验、认证挑战、游标与重复事件处理。部署和连接方式见 [自托管 Observer 使用说明](self-hosted-observer.md)。

## AI

AI 默认关闭，支持 OpenAI-compatible Provider。Web 设置页可以新增、编辑、测试和切换 Provider；API key 只写入本机，不由 API 回显，也可通过 `PKUHOLE_AI_API_KEY` 环境变量提供。

支持三种研究范围：

- `selected`：指定最多 100 个 PID。
- `local`：在本地资料库中检索。
- `course`：按课程与最多 10 名教师整理证据。

研究会话、消息、检索轨迹、引用来源和证据检查结果会写入本地数据库。只有显式开启实时搜索后，工作流才可补充查询在线树洞。

兼容的单 Provider 配置示例：

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

## Studio API v1

新接口位于 `/api/v1`，旧版兼容路由继续保留。代表性路由如下：

```text
GET  /health
GET  /capabilities
GET  /posts
GET  /posts/hot?limit=10
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

任务事件接口使用 SSE。客户端可发送 `Last-Event-ID`，服务端会先从数据库补发缺失事件，再继续实时推送。归档导入任务只开放取消；已退役的 `monitor_latest` 历史任务也只开放取消。

上游树洞接口字段笔记保存在 [doc/api.md](../doc/api.md)，它不是 Studio API 文档。

## 从源码构建

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

`server` 与 Web 资源始终参与正式构建，不需要 `withserver` build tag。省略 `sqlite_fts5` 时仍可构建，但本地搜索会回退到兼容模式。

前端开发：

```bash
# 终端一：API
go run -tags sqlite_fts5 ./cmd server --host 127.0.0.1 --port 8080

# 终端二：Vite，自动代理 /api/v1
cd web
npm run dev
```

## 目录结构

```text
cmd/                 Cobra 命令、TUI、server 和 web 入口
internal/app/        应用组合根与任务处理器
internal/service/    TUI、Web 和任务共享的服务层
internal/db/         Repository、迁移、FTS 与任务存储
internal/jobs/       持久任务调度与事件流
internal/archive/    归档预检、导入、导出与契约测试
internal/tui/        终端界面
server/              Gin 兼容路由、API v1 与 SPA 托管
web/                 React、TypeScript、Vite 客户端和测试
```

界面与历史资料：

- [经典树洞界面规格](classic-treehole-ui-spec.md)
- [GitHub 风格界面规格](github-ui-spec.md)
- [alpha.3 验收清单](alpha3-acceptance.md)
- [上游基线记录](upstream-baseline.md)

## 测试与发布

```bash
go test ./...
go test -tags sqlite_fts5 ./...
go vet -tags sqlite_fts5 ./...

cd web
npm test
npm run build
npm run e2e
```

Playwright 覆盖主流程，以及经典树洞/GitHub 布局的响应式、键盘和视觉回归。CI 在 main 与 PR 上执行 Web 测试和构建、Go test/vet/build，并在 Windows Chromium 上运行完整 E2E。`v*` 标签还会构建 Linux、macOS、Windows 产物和 `SHA256SUMS.txt`，发布正文来自对应的 `docs/releases/<tag>.md`。

## 安全边界

- Web 只允许绑定本机回环地址。
- API-only 服务默认也绑定 `127.0.0.1`；非回环监听必须显式启用并使用 Bearer Token。
- 密码与验证码接口拒绝非本机来源，密码不会持久化或出现在 API 响应中。
- 配置、Cookie 和 AI key 是本机文件，不使用操作系统密钥链；应将资料目录视为私密数据。
- 归档上传使用随机暂存文件，并校验大小、ZIP 路径、CRC 和展开体积。
- AI 与实时在线搜索默认关闭；远程 AI Provider 会收到与问题相关的命中原文。
- `referenced` 归档内容可作为上下文保存，但默认不出现在普通资料列表与搜索中。
