import { expect, test } from '@playwright/test'

test('empty first run explains both workspaces and remembers completion', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 })
	await page.route('**/api/v1/**', async (route) => {
		const path = new URL(route.request().url()).pathname
		let data: unknown
		if (path === '/api/v1/health') data = { status: 'ok', posts: 0, comments: 0 }
		else if (path === '/api/v1/capabilities') data = { api_version: 'v1', schema_version: 4, fts5: true, archive_import: true, jobs: true }
		else if (path === '/api/v1/jobs' || path === '/api/v1/imports') data = []
		else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: path } }) })
		return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data }) })
	})

	await page.goto('/')
	const dialog = page.getByRole('dialog', { name: '先选择你现在要做的事' })
	await expect(dialog).toBeVisible()
	await dialog.getByRole('button', { name: /本地资料库/ }).click()
	await dialog.getByRole('button', { name: '继续' }).click()
	await expect(page.getByRole('heading', { name: '理解在线与本地的关系' })).toBeVisible()
	await page.getByRole('button', { name: '前往导入资料' }).click()
	await expect(page.getByRole('heading', { name: '导入与导出' })).toBeVisible()
	expect(await page.evaluate(() => localStorage.getItem('pkustudio:onboarding:v1'))).toBe('completed')
	await page.goto('/')
	await expect(page.getByRole('heading', { name: '本地资料库' })).toBeVisible()
	await expect(page.getByRole('dialog')).toHaveCount(0)
})

test('global task feedback hands completed saves to an already open local list', async ({ page }) => {
	let jobRequests = 0
	await page.route('**/api/v1/**', async (route) => {
		const path = new URL(route.request().url()).pathname
		let data: unknown
		if (path === '/api/v1/jobs') {
			jobRequests++
			data = [{ id: 'save-feedback', type: 'sync_pids', status: jobRequests === 1 ? 'running' : 'completed', scope: { pids: [701, 702] }, completed_items: jobRequests === 1 ? 0 : 2, failed_items: 0, total_items: 2, attempts: 1, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:02Z' }]
		}
		else if (path === '/api/v1/posts') data = { items: [{ pid: 701, text: '等待任务交接一' }, { pid: 702, text: '等待任务交接二' }], has_more: false }
		else if (path === '/api/v1/local-tags' || path === '/api/v1/search/history' || path === '/api/v1/projects') data = []
		else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: path } }) })
		return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data }) })
	})

	await page.goto('/posts')
	await expect(page.getByText('等待任务交接一')).toBeVisible()
	await expect(page.getByText('同步指定 PID 已完成')).toBeVisible({ timeout: 7_000 })
	await page.getByRole('link', { name: '整理这些树洞' }).click()
	await expect(page.getByText('已选择 2 个树洞')).toBeVisible()
})

test('dashboard to import, search, detail, and AI flow', async ({ page }) => {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    let data: unknown
    if (path === '/api/v1/health') data = { status: 'ok', posts: 1, comments: 1 }
    else if (path === '/api/v1/capabilities') data = { api_version: 'v1', schema_version: 3, fts5: true, archive_import: true, jobs: true, ai: false, live_search: false }
		else if (path === '/api/v1/jobs') data = []
		else if (path === '/api/v1/imports' && route.request().method() === 'GET') data = []
		else if (path === '/api/v1/exports/jobs') data = []
		else if (path === '/api/v1/search/history') data = []
		else if (path === '/api/v1/ai/providers') data = [{ name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', configured: false }]
		else if (path === '/api/v1/ai/sessions') data = []
		else if (path === '/api/v1/session/probe') data = { checked: true, has_session: false, can_read_online: false, can_write_online: false, message: '请先登录' }
    else if (path.startsWith('/api/v1/posts/hot')) data = { items: [], source: 'live_recent', window_hours: 12, updated_at: 1_784_100_000, stale: false, approximate: true }
	else if (path === '/api/v1/imports/preflight' && route.request().method() === 'POST') data = {
		token: 'preview-1', status: 'awaiting_confirmation', expires_at: '2026-01-01T01:00:00Z', filename: 'sample.treehole.zip',
		preflight: { format: 'v2', status: 'completed', hash: 'abc', run_id: 'run-1', counts: { items: 1, valid_items: 1, comments: 1 }, issues: [] },
	}
	else if (path === '/api/v1/imports/preflight/preview-1/confirm' && route.request().method() === 'POST') data = {
		token: 'preview-1', status: 'queued', expires_at: '2026-01-01T01:00:00Z', filename: 'sample.treehole.zip',
		job: { id: 'import-1', type: 'import_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
		preflight: { format: 'v2', status: 'completed', hash: 'abc', run_id: 'run-1', counts: { items: 1, valid_items: 1, comments: 1 }, issues: [] },
	}
    else if (path === '/api/v1/search') data = { items: [{ pid: 123456, text: '数据结构课程体验', reply: 1, timestamp: 1767225600 }], has_more: false }
    else if (path === '/api/v1/posts/123456') data = {
      post: { pid: 123456, text: '数据结构课程体验', reply: 1, timestamp: 1767225600 },
      comments: [{ cid: 1001, pid: 123456, name_tag: 'Alice', text: '作业量适中', timestamp: 1767225700 }],
      references: [], has_more_comments: false,
    }
    else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: 'not found', details: {} } }) })
	return route.fulfill({ status: path.endsWith('/confirm') ? 202 : 200, contentType: 'application/json', body: JSON.stringify({ data }) })
  })

  await page.goto('/')
  await expect(page.getByRole('heading', { name: '本地资料库' })).toBeVisible()

  await page.getByRole('link', { name: '导入与导出' }).click()
  await page.locator('input[type=file]').setInputFiles({ name: 'sample.treehole.zip', mimeType: 'application/zip', buffer: Buffer.from('PK test') })
	await page.getByRole('button', { name: '预检文件' }).click()
	await expect(page.getByText('文件检查通过')).toBeVisible()
	await expect(page.getByText('import-1')).toHaveCount(0)
	await page.getByRole('button', { name: '确认导入' }).click()
	await expect(page.getByText('任务已持久保存，正在等待本机执行器；刷新页面或重新启动程序不会丢失。')).toBeVisible()

  await page.getByRole('link', { name: '全部资料', exact: true }).click()
  await page.getByRole('textbox', { name: '搜索关键词' }).fill('数据结构')
  await page.getByRole('button', { name: '搜索', exact: true }).click()
  await page.getByText('数据结构课程体验').click()
  await expect(page.getByText('作业量适中')).toBeVisible()

	await page.getByRole('link', { name: 'AI 研究' }).click()
	await expect(page.getByRole('heading', { name: 'AI 研究台' })).toBeVisible()

	await page.getByRole('button', { name: '在线树洞' }).click()
	const navigation = page.getByRole('navigation', { name: '主导航' })
	await navigation.getByRole('link', { name: '互动消息' }).click()
	await expect(page.getByRole('heading', { name: '登录后查看通知' })).toBeVisible()
	await navigation.getByRole('link', { name: '课表与成绩' }).click()
	await expect(page.getByRole('heading', { name: '登录后使用校园信息' })).toBeVisible()
	await page.getByRole('button', { name: '本地资料库' }).click()
	await expect(page.getByRole('heading', { name: 'AI 研究台' })).toBeVisible()
})

test('Studio local selection organizes in place and carries context into export', async ({ page }) => {
	let batchTags: unknown
	await page.setViewportSize({ width: 390, height: 844 })
	await page.route('**/api/v1/**', async (route) => {
		const url = new URL(route.request().url())
		const path = url.pathname
		let data: unknown
		if (path === '/api/v1/posts') data = { items: [{ pid: 701, text: '移动端批量资料一', reply: 1 }, { pid: 702, text: '移动端批量资料二', reply: 2 }], has_more: false }
		else if (path === '/api/v1/search/history' || path === '/api/v1/exports/jobs') data = []
		else if (path === '/api/v1/exports/preview' && route.request().method() === 'POST') data = { format: 'treehole-v2', posts: 2, comments: 4, media: 1, missing_media: 0 }
		else if (path === '/api/v1/local-tags') data = [{ id: 8, name: '待整理', color: '#0f766e' }]
		else if (path === '/api/v1/projects') data = [{ id: 9, name: '课程项目', post_count: 0, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }]
		else if (path === '/api/v1/posts/batch/tags' && route.request().method() === 'POST') { batchTags = route.request().postDataJSON(); data = { updated: 2 } }
		else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: path } }) })
		return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data }) })
	})

	await page.goto('/posts')
	await page.getByRole('button', { name: '选择多个' }).click()
	await page.getByRole('button', { name: '选择树洞 #701' }).click()
	await page.getByRole('button', { name: '选择树洞 #702' }).click()
	await page.getByRole('button', { name: '添加标签' }).click()
	await page.getByRole('combobox', { name: '选择要添加的标签' }).selectOption('8')
	await page.getByRole('button', { name: '添加标签（2）' }).click()
	await expect(page.getByText('已为 2 个树洞添加标签')).toBeVisible()
	expect(batchTags).toEqual({ pids: [701, 702], tag_ids: [8] })
	const overflow = await page.evaluate(() => ({ width: window.innerWidth, scrollWidth: document.documentElement.scrollWidth }))
	expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.width)

	await page.getByRole('button', { name: '导出', exact: true }).click()
	await expect(page.getByRole('heading', { name: '打包本地资料' })).toBeVisible()
	await expect(page.getByLabel('导出范围')).toHaveValue('701, 702')
	await expect(page.getByText('预计导出范围').locator('..').getByText('2', { exact: true })).toBeVisible()
	await page.getByRole('link', { name: '返回来源页面' }).click()
	await expect(page.getByText('已选择 2 个树洞')).toBeVisible()
})

test('classic layout keeps search, list, and detail in one context', async ({ page }) => {
	await page.addInitScript(() => localStorage.setItem('pkustudio:layout-preset', 'classic'))
	await page.route('**/api/v1/**', async (route) => {
		const url = new URL(route.request().url())
		const path = url.pathname
		let data: unknown
		if (path === '/api/v1/local-tags') data = [{ id: 1, name: '课程', color: '#285bc5' }]
		else if (path === '/api/v1/jobs') data = [{ id: 'classic-job', type: 'sync_pids', status: 'running', completed_items: 1, failed_items: 0, total_items: 3, attempts: 1, created_at: '2026-07-15T10:00:00Z', updated_at: '2026-07-15T10:00:01Z' }]
		else if (path === '/api/v1/session' || path === '/api/v1/session/probe') data = { checked: true, has_session: true, can_read_online: true, can_write_online: true, message: '会话正常' }
		else if (path === '/api/v1/notifications') data = { items: [{ id: 9, pid: 8401259, title: '收到回复', content: '经典消息抽屉中的通知', read: false, type: 'int_msg' }], total: 1, page: 1 }
		else if (path === '/api/v1/posts/8401259') data = {
			post: { pid: 8401259, text: '完整洞内容', reply: 2, praise_num: 6, timestamp: 1784108400 },
			comments: [
				{ cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '第一条完整回复', timestamp: 1784108460 },
				{ cid: 38672115, pid: 8401259, name_tag: '洞主', is_lz: true, text: '洞主的回复', timestamp: 1784108520, quote: { cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '第一条完整回复' } },
			],
			references: [], media: [], has_more_comments: false,
		}
		else if (path === '/api/v1/posts') data = { items: [{
			pid: 8401259, text: '点击卡片任意位置打开经典详情', reply: 2, praise_num: 6, timestamp: 1784108400,
			comment_list: [
				{ cid: 38672079, pid: 8401259, name_tag: 'Alice', text: '横向回复一', timestamp: 1784108460 },
				{ cid: 38672115, pid: 8401259, name_tag: '洞主', is_lz: true, text: '横向回复二', timestamp: 1784108520 },
			],
		}], has_more: false }
		else return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'not_found', message: path } }) })
		return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data }) })
	})

	await page.goto('/posts')
	await expect(page.getByText('PkuHoleStudio · 经典树洞')).toBeVisible()
	await expect(page.getByText('横向回复一')).toBeVisible()
	await page.getByRole('button', { name: /后台任务/ }).click()
	await expect(page.getByRole('dialog', { name: '后台任务' })).toBeVisible()
	await expect(page.getByText('同步指定 PID')).toBeVisible()
	await page.goBack()
	await expect(page.getByRole('dialog', { name: '后台任务' })).toBeHidden()

	await page.getByRole('button', { name: '账户' }).click()
	await expect(page.getByRole('dialog', { name: '账户与界面' })).toBeVisible()
	await page.getByLabel('背景').selectOption('dusk')
	await page.getByLabel('明暗').selectOption('dark')
	await expect(page.locator('.classic-shell')).toHaveClass(/classic-bg-dusk/)
	await expect(page.locator('.classic-shell')).toHaveClass(/classic-mode-dark/)
	await page.getByLabel('明暗').selectOption('light')
	await page.getByRole('button', { name: '关闭账户与界面' }).click()

	await page.getByRole('button', { name: '消息' }).click()
	await expect(page.getByRole('dialog', { name: '消息' })).toBeVisible()
	await expect(page.getByText('经典消息抽屉中的通知')).toBeVisible()
	await page.getByRole('button', { name: '关闭消息' }).click()

	await page.getByRole('link', { name: '打开树洞 #8401259' }).click()
	await expect(page.getByRole('dialog', { name: '树洞 #8401259' })).toBeVisible()
	await expect(page.getByText('完整洞内容')).toBeVisible()
	await expect(page.getByText('点击卡片任意位置打开经典详情')).toBeVisible()
	await page.keyboard.press('Escape')
	await expect(page.getByRole('dialog', { name: '树洞 #8401259' })).toBeHidden()

	await page.getByRole('textbox', { name: '搜索内容或 PID' }).fill('#8401259')
	await page.getByRole('button', { name: '搜索', exact: true }).click()
	await expect(page.getByRole('dialog', { name: '树洞 #8401259' })).toBeVisible()

	await page.setViewportSize({ width: 390, height: 844 })
	const drawer = await page.getByRole('dialog', { name: '树洞 #8401259' }).boundingBox()
	expect(drawer?.width).toBeCloseTo(390, 1)
})
