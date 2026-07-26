import { expect, test, type Page, type Route } from '@playwright/test'

const fixedNow = new Date('2026-07-15T16:45:00+08:00')

const comments = [
  { cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '估计很少有人看得上的，可能都看不上。', timestamp: 1784102640 },
  { cid: 38672115, pid: 8401259, name_tag: '洞主', is_lz: true, text: '保得上本院的不去，保不上的也不要？', timestamp: 1784102700, quote: { cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '估计很少有人看得上的。' } },
  { cid: 38672220, pid: 8401259, name_tag: 'Bob', text: '基本入不了营。', timestamp: 1784103060 },
  { cid: 38672228, pid: 8401259, name_tag: 'Carol', text: '退一步说，申请默认就会鸽。', timestamp: 1784104920 },
]

const posts = Array.from({ length: 8 }, (_, index) => ({
  pid: 8401259 - index,
  text: index === 0 ? 'gsm 有保研到 jy 的吗？' : [
    '跨文化交流学发疯中，这个课到底在讲什么？',
    '家人暑假来北京旅游，酒店怎么选比较方便？',
    '模拟炒股今日记录，欢迎交流。',
    '贵性',
    '结婚后真的会变成家人吗？',
    '教员最近怎么没有上树洞？',
  ][index - 1],
  reply: index === 0 ? 11 : 2 + index,
  praise_num: 6 + index,
  timestamp: 1784102400 - index * 240,
  comment_list: index < 4 ? comments.slice(0, index === 0 ? 4 : 2).map((comment, commentIndex) => ({
    ...comment,
    cid: comment.cid - index * 10 - commentIndex,
    pid: 8401259 - index,
  })) : [],
}))

async function prepareClassic(page: Page) {
  await page.clock.setFixedTime(fixedNow)
  await page.addInitScript(() => {
    localStorage.setItem('pkustudio:layout-preset', 'classic')
    localStorage.setItem('pkustudio:classic:background', 'stars')
    localStorage.setItem('pkustudio:classic:color-mode', 'light')
  })
  await page.route('**/api/v1/**', fulfillClassicAPI)
}

async function fulfillClassicAPI(route: Route) {
  const request = route.request()
  const path = new URL(request.url()).pathname
  let data: unknown

  if (path === '/api/v1/local-tags') data = [
    { id: 1, name: '课程', color: '#285bc5' },
    { id: 2, name: '生活', color: '#287f78' },
  ]
  else if (path === '/api/v1/jobs') data = [
    { id: 'quality-job', type: 'sync_pids', status: 'running', completed_items: 17, failed_items: 0, total_items: 40, attempts: 1, created_at: '2026-07-15T08:00:00Z', updated_at: '2026-07-15T08:44:00Z' },
  ]
  else if (path === '/api/v1/session' || path === '/api/v1/session/probe') data = { checked: true, has_session: true, can_read_online: true, can_write_online: true, message: '会话正常' }
  else if (path === '/api/v1/notifications') data = { items: [], total: 0, page: 1 }
  else if (path === '/api/v1/posts/8401259') data = {
    post: { ...posts[0], text: 'gsm 有保研到 jy 的吗？' },
    comments,
    references: [],
    media: [],
    has_more_comments: false,
  }
  else if (path.startsWith('/api/v1/posts/')) {
    const pid = Number(path.split('/').at(-1))
    const post = posts.find((item) => item.pid === pid) ?? { ...posts[0], pid }
    data = { post, comments: [], references: [], media: [], has_more_comments: false }
  }
  else if (path === '/api/v1/posts') data = { items: posts, has_more: false }
  else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: path } }) })

  return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data }) })
}

async function openClassicList(page: Page) {
  await prepareClassic(page)
  await page.goto('/posts')
  await expect(page.getByRole('link', { name: '打开树洞 #8401259' })).toBeVisible()
}

test.describe('经典树洞视觉基线', () => {
  test('桌面列表和详情保持稳定', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 })
    await openClassicList(page)

    // GitHub's Windows runner renders the same fixed Chromium build with a
    // repeatable ~3% font-rasterization delta from the committed workstation
    // baseline. Keep this tolerance local to the list screenshot; all layout,
    // interaction, detail and responsive assertions remain strict.
    await expect(page).toHaveScreenshot('classic-list-light.png', {
      maxDiffPixelRatio: 0.035,
    })

    await page.getByRole('link', { name: '打开树洞 #8401259' }).click()
    await expect(page.getByRole('dialog', { name: '树洞 #8401259' })).toBeVisible()
    await expect(page).toHaveScreenshot('classic-detail-light.png', {
      maxDiffPixelRatio: 0.025,
    })
  })
})

test.describe('经典树洞响应式布局', () => {
  test('桌面、超宽屏、平板和手机均无页面横向溢出', async ({ page }) => {
    await openClassicList(page)

    for (const viewport of [
      { width: 1280, height: 720 },
      { width: 2560, height: 1440 },
      { width: 1024, height: 768 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport)
      const overflow = await page.evaluate(() => ({
        innerWidth: window.innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        offenders: Array.from(document.querySelectorAll<HTMLElement>('body *')).flatMap((element) => {
          const rect = element.getBoundingClientRect()
          return rect.right > window.innerWidth + 1 || rect.left < -1
            ? [`${element.tagName.toLowerCase()}.${element.className || '(no-class)'} [${Math.round(rect.left)}, ${Math.round(rect.right)}]`]
            : []
        }).slice(0, 8),
      }))
      expect(overflow.scrollWidth, `${viewport.width}px 视口发生溢出：${overflow.offenders.join('；')}`).toBeLessThanOrEqual(overflow.innerWidth)
    }

    await page.setViewportSize({ width: 2560, height: 1440 })
    const desktopRoot = await page.locator('.classic-root-card').first().boundingBox()
    const desktopReply = await page.locator('.classic-reply-card').first().boundingBox()
    expect(desktopRoot?.width).toBeCloseTo(600, 0)
    expect(desktopReply?.width).toBeCloseTo(290, 0)

    await page.getByRole('link', { name: '打开树洞 #8401259' }).click()
    const ultrawideDrawer = await page.getByRole('dialog', { name: '树洞 #8401259' }).boundingBox()
    expect(ultrawideDrawer?.width).toBeGreaterThanOrEqual(1800)
    expect(ultrawideDrawer?.width).toBeLessThanOrEqual(1851)
    await page.keyboard.press('Escape')

    await page.setViewportSize({ width: 1024, height: 768 })
    await page.getByRole('link', { name: '打开树洞 #8401259' }).click()
    const tabletDrawer = await page.getByRole('dialog', { name: '树洞 #8401259' }).boundingBox()
    expect(tabletDrawer?.width).toBeCloseTo(1024 * 0.82, -1)
    await page.keyboard.press('Escape')

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.locator('.classic-mobile-nav')).toBeVisible()
    const threadLayout = await page.locator('.classic-thread-scroll').first().evaluate((element) => getComputedStyle(element).display)
    expect(threadLayout).toBe('grid')
    await page.getByRole('link', { name: '打开树洞 #8401259' }).click()
    const mobileDrawer = await page.getByRole('dialog', { name: '树洞 #8401259' }).boundingBox()
    expect(mobileDrawer?.width).toBeCloseTo(390, 0)
  })

  test('工具栏吸顶，抽屉遵守减少动态效果设置', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await openClassicList(page)
    await page.evaluate(() => window.scrollTo(0, 500))
    await expect.poll(async () => (await page.locator('.classic-toolbar').boundingBox())?.y ?? -1).toBeCloseTo(0, 0)

    await page.getByRole('button', { name: '账户' }).click()
    const dialog = page.getByRole('dialog', { name: '账户与界面' })
    await expect(dialog).toBeVisible()
    await expect.poll(() => dialog.evaluate((element) => getComputedStyle(element).animationName)).toBe('none')
    await expect.poll(() => page.locator('.classic-drawer-layer').evaluate((element) => getComputedStyle(element).animationName)).toBe('none')
  })
})

test.describe('经典树洞键盘可用性', () => {
  test('Enter 打开详情，焦点留在抽屉内，Escape 关闭并回到卡片', async ({ page }) => {
    await openClassicList(page)
    const card = page.getByRole('link', { name: '打开树洞 #8401259' })
    await card.focus()
    await page.keyboard.press('Enter')

    const dialog = page.getByRole('dialog', { name: '树洞 #8401259' })
    await expect(dialog).toBeVisible()
    await expect(page.getByRole('button', { name: '关闭详情' })).toBeFocused()

    for (let index = 0; index < 18; index += 1) {
      await page.keyboard.press('Tab')
      expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)
    }

    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    await expect(card).toBeFocused()
  })
})
