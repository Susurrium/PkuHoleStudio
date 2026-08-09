# PkuHoleStudio

PkuHoleStudio 是一款在自己电脑上运行的北大树洞客户端。你可以浏览和互动，也可以把需要的内容保存到本机，之后离线搜索、整理和导出。

当前预览版：`v0.1.0-alpha.9`。查看 [本版说明](docs/releases/v0.1.0-alpha.9.md) 或 [全部变更](CHANGELOG.md)。

本项目在 TUI、采集器和旧版接口兼容方面参考、复用和借鉴了 [PKUHoleTUI](https://github.com/dfshfghj/PKUHoleTUI) 的部分代码，感谢原项目作者。

## 一、现在可以做什么

- 浏览最新树洞、近期热榜、关注列表和互动消息，查看完整评论与图片。
- 登录后发洞、回复、引用回复、点赞和关注；也可以查看课表与成绩。
- 把指定 PID、关注列表或最新时间线保存到本机，并一并保存评论和图片。
- 在本地资料库中离线浏览和全文搜索，按时间排序，并按图片和本地标签筛选；在线与本地内容通过工作区切换。
- 为帖子和评论添加本地笔记、标签，或整理进研究项目；这些内容不会上传到树洞。
- 导入旧 JSON 或 Studio 迁移包，也可以导出 Studio 迁移包或 Markdown 阅读文件。
- 按需启用 AI 研究，从本地资料中查找证据、整理观点并保留引用来源。
- 按需连接自托管 Observer，查看近期热榜以及已经保存的删除记录。
- 在 Studio、经典树洞和 GitHub 三种界面之间切换；也可使用键盘操作的 TUI。

在线树洞和本地资料库是两个不同的工作区：在线浏览的内容不会自动保存，只有点击“保存到本地”或创建保存任务后，才会进入本地资料库。

## 二、安装与使用

### Windows 安装（推荐）

PkuHoleStudio 是免安装程序，不需要另装 Go、Node.js 或数据库。

1. 打开 [v0.1.0-alpha.9 下载页](https://github.com/Susurrium/PkuHoleStudio/releases/tag/v0.1.0-alpha.9)。
2. 下载 `treehole-windows-amd64.zip`，解压到一个固定且可写的目录，例如 `D:\PkuHoleStudio`。不要直接在压缩包内运行。
3. 在解压目录打开 PowerShell，运行：

   ```powershell
   .\treehole-windows-amd64.exe web
   ```

4. 浏览器会自动打开 `http://127.0.0.1:8080`。如果没有自动打开，请手动访问该地址。
5. 使用期间保持 PowerShell 窗口开启；按 `Ctrl+C` 停止程序。以后仍从同一目录运行同一条命令。

如果 Windows 提示这是未签名的应用，请先确认文件来自上面的 GitHub Release；然后在系统提示中选择继续运行。

### macOS

当前发布包适用于 Apple Silicon Mac：

```bash
tar -xzf treehole-darwin-arm64.tar.gz
chmod +x treehole-darwin-arm64
./treehole-darwin-arm64 web
```

如果 macOS 阻止首次运行，请在“系统设置 → 隐私与安全性”中确认打开。浏览器入口同样是 `http://127.0.0.1:8080`。

### Linux

当前发布包适用于使用 glibc 的 x86-64 Linux，不适用于 Alpine/musl：

```bash
tar -xzf treehole-linux-amd64.tar.gz
chmod +x treehole-linux-amd64
./treehole-linux-amd64 web
```

如果浏览器没有自动打开，请手动访问 `http://127.0.0.1:8080`。

### 第一次使用

1. 首次打开时，先选择“在线树洞”或“本地资料库”。之后可以随时切换。
2. 打开左侧的“登录与保存”。如果当前资料目录以前登录过，先点“载入已有登录状态”；否则选择“使用学号登录”。
3. 按页面提示完成短信验证码或北京大学 App 的 6 位手机令牌。密码只用于这次登录，不会由网页保存。
4. 推荐先使用“保存指定 PID”，保留默认勾选的“保存全部评论”和“下载图片到本机”。也可以保存关注列表或最新时间线。
5. 任务创建后可以离开页面；在“任务中心”查看进度。完成后切换到“本地资料库”即可搜索和整理。

如果你已经有旧资料，不需要先登录：打开“导入与导出”，选择文件，等待检查完成后再确认导入。

### 日常使用

- 想实时浏览、发洞或互动：切换到“在线树洞”。
- 想离线搜索和整理：切换到“本地资料库”，使用“全部资料”“研究项目”和帖子详情中的标签、笔记功能。
- 想保存看到的某个洞：在在线详情页点“保存到本地”；需要批量保存时，前往“登录与保存”。
- 图片没有保存完整：前往“登录与保存”，运行“补全缺失图片”。
- 想导入已有资料：打开“导入与导出 → 导入文件”，选择 JSON、ZIP 或 `.treehole.zip`，点“检查文件”，确认内容后再点“确认导入”。
- 想迁移到另一台电脑：打开“导入与导出 → 导出资料”，留空 PID 代表全部资料，点击“创建 Studio 迁移包”；任务完成后下载，再在另一台电脑导入。
- 想得到便于阅读的文件：在同一导出页面选择“导出为 Markdown”。
- 想使用 AI：前往“设置 → AI 服务”添加服务，再在“AI 使用方式”中启用，之后打开“AI 研究”。AI 默认关闭。
- 想使用删除记录和自建热榜：先部署 Observer，再按 [Observer 使用说明](docs/self-hosted-observer.md) 连接。

### 数据位置与升级

按上面的默认方式启动时：

- `data/` 保存设置、登录状态、日志、图片和导出文件。
- `treehole.db` 保存帖子、评论、标签、笔记、项目和 AI 记录。

升级时先停止程序，下载并解压新版，只替换可执行文件；保留原来的 `data/` 和 `treehole.db*`。如果要迁移全部本机状态，停止程序后复制这两部分即可。

需要把所有资料固定到其他位置时，请以后始终使用同一条命令：

```powershell
.\treehole-windows-amd64.exe --data-dir D:\PkuHoleStudio\data --db-path D:\PkuHoleStudio\data\treehole.db web
```

### 常见问题

- 双击程序后出现终端界面：这是 TUI。关闭后使用带 `web` 的命令启动浏览器界面。
- 浏览器没有打开：手动访问 `http://127.0.0.1:8080`。
- 提示 8080 端口被占用：运行 `.\treehole-windows-amd64.exe web --port 8081`，然后访问 `http://127.0.0.1:8081`。
- 突然看不到原来的资料或登录状态：通常是从不同目录启动，或改变了 `--data-dir` / `--db-path`。回到原目录并使用原命令。
- 本地搜索结果异常：停止 Web 后运行 `.\treehole-windows-amd64.exe rebuild-search-index`，完成后重新启动 Web。

源码构建、命令行采集、API、目录结构、测试和安全边界见 [技术参考](docs/technical-reference.md)。
