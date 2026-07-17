import { expect, test, type Page, type Route } from '@playwright/test'

const fixedNow = new Date('2026-07-15T16:45:00+08:00')
const comments = [
  { cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '第一条 GitHub Conversation 风格评论。', timestamp: 1784102640 },
  { cid: 38672115, pid: 8401259, name_tag: '洞主', is_lz: true, text: '洞主补充了更多上下文。', timestamp: 1784102700, quote: { cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '第一条 GitHub Conversation 风格评论。' } },
]
const posts = Array.from({ length: 7 }, (_, index) => ({
  pid: 8401259 - index,
  text: index === 0 ? '这是一条适合 Issues 列表展示的树洞正文' : `本地资料示例 ${index + 1}：课程、生活与校园信息`,
  reply: index + 2,
  praise_num: index * 3,
  timestamp: 1784102400 - index * 300,
  media_ids: index === 2 ? 'image-1' : '',
}))

interface GithubSessionState { canRead: boolean; canWrite: boolean }

async function prepareGithub(page: Page, colorMode: 'light' | 'dark' = 'light', sessionState: GithubSessionState = { canRead: true, canWrite: true }) {
  await page.clock.setFixedTime(fixedNow)
  await page.addInitScript((mode) => {
    localStorage.setItem('pkustudio:layout-preset', 'github')
    localStorage.setItem('pkustudio:github:color-mode', mode)
  }, colorMode)
  await page.route('**/api/v1/**', (route) => fulfillGithubAPI(route, sessionState))
}

async function fulfillGithubAPI(route: Route, sessionState: GithubSessionState) {
  const path = new URL(route.request().url()).pathname
  if (path === '/api/v1/posts' && route.request().method() === 'POST') {
    if (!sessionState.canWrite) return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: { code: 'online_session_expired', message: '会话已过期' } }) })
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: posts[0] }) })
  }
  let data: unknown
  if (path === '/api/v1/local-tags') data = [{ id: 1, name: '课程', color: '#0969da' }, { id: 2, name: '生活', color: '#1a7f37' }]
  else if (path === '/api/v1/health') data = { status: 'ok', posts: 7, comments: 18 }
  else if (path === '/api/v1/capabilities') data = { api_version: 'v1', schema_version: 3, fts5: true, archive_import: true, jobs: true, ai: true, live_search: true }
  else if (path === '/api/v1/jobs') data = [{ id: 'github-job', type: 'sync_pids', status: 'running', completed_items: 17, failed_items: 0, total_items: 40, attempts: 1, created_at: '2026-07-15T08:00:00Z', updated_at: '2026-07-15T08:44:00Z' }]
  else if (path === '/api/v1/posts/hot') data = { window_hours: 12, updated_at: 1784102400, approximate: true, stale: false, items: [{ id: 8401259, text: posts[0].text, reply: 2, follownum: 16 }] }
  else if (path === '/api/v1/session' || path === '/api/v1/session/probe') data = { checked: true, has_session: sessionState.canRead, can_read_online: sessionState.canRead, can_write_online: sessionState.canWrite, message: sessionState.canRead ? '会话正常' : '需要登录' }
  else if (path === '/api/v1/posts/8401259/tags') data = [{ id: 1, name: '课程', color: '#0969da' }]
  else if (path === '/api/v1/posts/8401259/note') data = { content: '这是一条只保存在本机的个人笔记。' }
  else if (path === '/api/v1/posts/8401259') data = { post: posts[0], comments, references: [{ kind: 'explicit', source_pid: 8401259, target_pid: 8401200 }], media: [], has_more_comments: false }
  else if (path === '/api/v1/posts') data = { items: posts, has_more: false }
  else if (path === '/api/v1/ai/providers' || path === '/api/v1/ai/sessions' || path === '/api/v1/imports' || path === '/api/v1/exports/jobs' || path === '/api/v1/raw-json/jobs' || path === '/api/v1/campus/schedule') data = []
  else if (path === '/api/v1/notifications') data = { items: [], total: 0, page: 1 }
  else if (path === '/api/v1/campus/scores') data = { courses: [], gpa: 0, total_credits: 0 }
  else if (path === '/api/v1/settings') data = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, restart_required: false, ai_providers: [] }
  else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: path } }) })
  return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data }) })
}

async function openGithubList(page: Page) {
  await prepareGithub(page)
  await page.goto('/posts')
  await expect(page.getByRole('link', { name: posts[0].text })).toBeVisible()
}

test.describe('GitHub 风格视觉基线', () => {
  test('Repository Overview 总览保持稳定', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await prepareGithub(page)
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'PkuHoleStudio' })).toBeVisible()
    await expect(page).toHaveScreenshot('github-dashboard-light.png')
  })

  test('Issues 列表和 Conversation 详情保持稳定', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await openGithubList(page)
    await expect(page).toHaveScreenshot('github-list-light.png')
    await page.getByRole('link', { name: posts[0].text }).click()
    await expect(page.getByText('第一条 GitHub Conversation 风格评论。', { exact: true })).toBeVisible()
    await expect(page).toHaveScreenshot('github-detail-light.png')
  })

  test('深色 Issues 列表和 Conversation 详情保持稳定', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await prepareGithub(page, 'dark')
    await page.goto('/posts')
    await expect(page.getByRole('link', { name: posts[0].text })).toBeVisible()
    await expect(page).toHaveScreenshot('github-list-dark.png')
    await page.getByRole('link', { name: posts[0].text }).click()
    await expect(page.getByText('第一条 GitHub Conversation 风格评论。', { exact: true })).toBeVisible()
    await expect(page).toHaveScreenshot('github-detail-dark.png')
  })
})

test.describe('GitHub 风格响应式与主题', () => {
  test('常用视口没有页面级横向溢出', async ({ page }) => {
    await openGithubList(page)
    for (const viewport of [
      { width: 1280, height: 800 },
      { width: 1024, height: 768 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport)
      const overflow = await page.evaluate(() => ({ innerWidth: window.innerWidth, scrollWidth: document.documentElement.scrollWidth }))
      expect(overflow.scrollWidth, `${viewport.width}px 视口发生页面横向溢出`).toBeLessThanOrEqual(overflow.innerWidth)
    }
    await page.setViewportSize({ width: 390, height: 844 })
    await page.getByRole('link', { name: posts[0].text }).click()
    await expect(page.locator('.github-conversation-layout')).toBeVisible()
    const detailOverflow = await page.evaluate(() => ({ innerWidth: window.innerWidth, scrollWidth: document.documentElement.scrollWidth }))
    expect(detailOverflow.scrollWidth, '390px 详情页发生页面横向溢出').toBeLessThanOrEqual(detailOverflow.innerWidth)
    expect(await page.locator('main').count()).toBe(1)
  })

  test('配色选择会持久化且不改变当前路由', async ({ page }) => {
    await openGithubList(page)
    await page.getByLabel('账户与界面').click()
    await page.getByRole('button', { name: '深色' }).click()
    await expect(page.locator('.github-preset-root')).toHaveAttribute('data-color-mode', 'dark')
    await expect.poll(() => page.evaluate(() => localStorage.getItem('pkustudio:github:color-mode'))).toBe('dark')
    await expect(page).toHaveURL(/\/posts$/)
  })

  test('深色模式下通用页面的强调控件保持可读', async ({ page }) => {
    await prepareGithub(page, 'dark')
    for (const path of ['/ai', '/sync', '/imports', '/notifications', '/campus']) {
      await page.goto(path)
      const emphasized = page.locator('.bg-ink.text-white:visible, .button-primary:visible')
      await expect(emphasized.first(), `${path} 缺少强调控件`).toBeVisible()
      const pairs = await emphasized.evaluateAll((elements) => elements.map((element) => {
        const style = getComputedStyle(element)
        return { foreground: style.color, background: style.backgroundColor }
      }))
      for (const pair of pairs) expect(contrastRatio(pair.foreground, pair.background), `${path} 的强调控件对比度不足`).toBeGreaterThanOrEqual(4.5)
    }
  })

  test('未登录时不会渲染或提交 URL 请求的发布表单', async ({ page }) => {
    let createAttempts = 0
    page.on('request', (request) => { if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/v1/posts') createAttempts += 1 })
    await prepareGithub(page, 'light', { canRead: false, canWrite: false })
    await page.goto('/posts?source=live&compose=true')
    await expect(page.getByText('需要先登录树洞')).toBeVisible()
    await expect(page.locator('.github-compose-panel')).toHaveCount(0)
    await expect(page.getByRole('link', { name: '登录后发表' })).toBeVisible()
    expect(createAttempts).toBe(0)
  })

  test('会话过期后隐藏发布区并清除在线列表缓存', async ({ page }) => {
    const sessionState = { canRead: true, canWrite: true }
    await prepareGithub(page, 'light', sessionState)
    await page.goto('/posts?source=live')
    await expect(page.locator('.github-issue-row')).toHaveCount(posts.length)
    await page.getByRole('button', { name: '发表新洞' }).click()
    const imageInput = page.locator('.github-compose-panel input[type="file"]')
    await imageInput.focus()
    await expect(imageInput).toBeFocused()
    await page.getByRole('textbox', { name: '新洞正文' }).fill('触发会话过期')
    sessionState.canRead = false
    sessionState.canWrite = false
    await page.getByRole('button', { name: '确认发表' }).click()
    await expect(page.getByText('需要先登录树洞')).toBeVisible()
    await expect(page.locator('.github-compose-panel')).toHaveCount(0)
    await expect(page.locator('.github-issue-row')).toHaveCount(0)
  })

  test('导航抽屉支持焦点循环、Escape 和焦点恢复', async ({ page }) => {
    await openGithubList(page)
    const trigger = page.getByRole('button', { name: '打开导航' })
    await trigger.click()
    const dialog = page.getByRole('dialog', { name: '全部导航' })
    await expect(dialog).toBeVisible()
    await expect(page.getByRole('button', { name: '关闭导航' })).toBeFocused()
    for (let index = 0; index < 16; index += 1) {
      await page.keyboard.press('Tab')
      expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)
    }
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    await expect(trigger).toBeFocused()
  })

  test('斜杠聚焦全局搜索，更多菜单导航后关闭并标记工具页', async ({ page }) => {
    await openGithubList(page)
    await page.keyboard.press('/')
    await expect(page.getByRole('textbox', { name: '全局搜索内容或 PID' })).toBeFocused()
    const more = page.locator('.github-underline-nav > details')
    await more.locator('summary').click()
    await more.getByRole('link', { name: '设置' }).click()
    await expect(page).toHaveURL(/\/settings$/)
    await expect.poll(() => more.evaluate((element) => (element as HTMLDetailsElement).open)).toBe(false)
    await expect(more).toHaveClass(/is-active/)
  })
})

function contrastRatio(foreground: string, background: string) {
  const foregroundLuminance = relativeLuminance(parseRGB(foreground))
  const backgroundLuminance = relativeLuminance(parseRGB(background))
  return (Math.max(foregroundLuminance, backgroundLuminance) + 0.05) / (Math.min(foregroundLuminance, backgroundLuminance) + 0.05)
}

function parseRGB(value: string) {
  const channels = value.match(/[\d.]+/g)?.slice(0, 3).map(Number)
  if (!channels || channels.length !== 3) throw new Error(`无法解析颜色 ${value}`)
  return channels
}

function relativeLuminance(channels: number[]) {
  const [red, green, blue] = channels.map((channel) => {
    const normalized = channel / 255
    return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue
}
