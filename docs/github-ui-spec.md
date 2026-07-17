# PkuHoleStudio GitHub 风格 UI 规格

状态：首版已实现（2026-07-15）
内部标识：`github`
用户可见名称：`GitHub 风格`

## 1. 定位

GitHub 风格 UI 是与 `studio`、`classic` 并列的第三套内置布局预设。它使用 GitHub 开源的 Primer 设计令牌、React 基础样式和 Octicons，复用其信息层级与交互语法，但继续使用 PkuHoleStudio 自己的名称、标识和业务术语。

不复制 GitHub.com 的页面源码、商标、Octocat、数据模型或不存在的产品概念。树洞不会被伪装成代码仓库，也不会出现没有业务含义的 Pull Request、分支、Open/Closed 状态。

## 2. 共享边界

三套 UI 共用以下能力：

- API 客户端、React Query 缓存键和错误解释。
- 在线会话与读写权限判断。
- 帖子列表查询、搜索、分页、发布和详情跳转。
- 详情读取、评论位置恢复、回复、点赞、关注和保存到本地。
- 本地标签、帖子笔记、评论笔记、媒体与引用关系。
- URL 查询参数、详情 `return_to` 和列表滚动位置。

共享能力位于 `web/src/features/posts/`。布局组件不得自行改变 API 请求语义或副作用触发条件。

## 3. 信息架构

### 3.1 App Header

- 左侧菜单打开全量导航抽屉。
- PkuHoleStudio 自有圆形 `P` 标识进入总览。
- 全局搜索支持正文关键词和 `#PID`。
- 右侧提供发表、任务、通知、账户与界面入口。
- Header 随页面正常滚动，不设置为固定栏。

### 3.2 项目上下文与导航

上下文显示 `PkuHoleStudio / treehole` 和 `local-first` 标签。主要导航包含：

1. 总览
2. 树洞
3. AI 研究
4. 任务
5. 通知

同步、导入导出、校园、资料维护、日志和设置进入“更多”菜单及全量导航抽屉。

### 3.3 页面映射

| PkuHoleStudio 能力 | GitHub 风格表现 |
| --- | --- |
| 总览 | Repository Overview / README 布局 |
| 树洞列表 | Issues 列表 |
| 树洞详情 | Conversation 时间线 |
| 发布新洞 | New Issue 风格编辑区 |
| 后台任务 | Actions / Workflow 语法 |
| 标签和笔记 | Detail Sidebar 元数据 |
| AI 研究 | Copilot 风格入口，保留原业务页 |
| 其余管理页面 | Primer 令牌适配的通用工作区 |

## 4. 帖子列表

- 本地资料和在线树洞使用分段来源选择。
- 搜索、图片、标签、关注范围和排序继续写入现有 URL 参数。
- 纯数字或 `#数字` 直接进入 `/posts/:pid`。
- 每行显示状态图标、正文摘要、PID、时间、来源、图片、点赞和评论数。
- 搜索命中的评论显示在对应帖子下方。
- 发布表单仅在在线会话具有写权限时可提交。
- 加载、空状态、错误和未登录状态使用独立且可操作的反馈区。

## 5. 帖子详情

- 根帖和评论使用 GitHub Conversation 卡片与时间线连接线。
- `Author` 标记对应洞主身份。
- 详情右栏承载本地标签、笔记、来源、引用关系和 AI 入口。
- 在线可写时显示回复、点赞和关注；只读或未登录状态不显示可提交的假操作。
- 评论分页和 `#comment-CID` 自动恢复使用共享控制层。
- 回复失败保留正文和文件；成功后清空并刷新详情。

## 6. 主题与持久化

| 键 | 值 |
| --- | --- |
| `pkustudio:layout-preset` | `studio` / `classic` / `github` |
| `pkustudio:github:color-mode` | `system` / `light` / `dark` |

GitHub 预设通过 `ThemeProvider` 和 `BaseStyles` 应用 Primer 主题，并且作为独立动态资源加载。自定义样式全部位于 `.github-preset-root` 或 `.github-shell` 作用域内。

未单独重写的业务页面通过令牌桥接适配：Studio 的 `paper/ink/line/teal/coral` 令牌在 GitHub 根中映射到 Primer 的背景、前景、边框、强调和危险语义色；通用面板、按钮、输入框、徽章和圆角同步收敛为 Primer 风格。

## 7. 响应式与无障碍

- 桌面内容最大宽度约 1280px。
- 900px 以下详情右栏移到正文下方。
- 700px 以下全局搜索压缩，列表筛选纵向排列，Conversation 使用窄头像和单栏元数据。
- 全量导航抽屉支持打开后聚焦、Tab 焦点循环、Escape 关闭、背景滚动锁定和关闭后焦点恢复。
- 所有图标按钮都有可访问名称。
- 遵守 `prefers-reduced-motion`。
- 390px、1024px 和 1280px 视口不得产生页面级横向溢出。

## 8. 代码结构

```text
web/src/
├── presets/registry.tsx
├── features/posts/
│   ├── usePostsExplorer.ts
│   └── usePostDetail.ts
└── github/
    ├── GithubPreset.tsx
    ├── GithubPresetRoot.tsx
    ├── GithubShell.tsx
    ├── GithubDashboardPage.tsx
    ├── GithubPostsPage.tsx
    ├── GithubPostDetailPage.tsx
    ├── GithubComponents.tsx
    └── github.css
```

## 9. 验收与回归

- 单元测试覆盖三套预设切换、GitHub 配色持久化、Issues 列表和 Conversation 详情。
- `npm run e2e:github-quality` 验证列表/详情视觉快照、响应式溢出、通用页面深色对比度、在线权限失效、主题和键盘导航。
- `npm run e2e:github-update` 只在设计确认后更新 GitHub 快照。
- 完整 `npm run e2e` 必须同时通过默认主流程、经典质量套件和 GitHub 质量套件。
- 视觉基线固定在 Windows Chromium 生成和验证；发布工作流的 E2E 平台必须与 `*-chromium-win32.png` 基线一致。
- 未取得 `can_write_online` 时不得渲染可提交的发布表单；失去 `can_read_online` 后不得继续显示缓存中的在线帖子。
- `npm run build` 必须保持 GitHub 资源独立分包。
