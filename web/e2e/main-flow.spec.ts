import { expect, test } from '@playwright/test'

test('dashboard to import, search, detail, and AI flow', async ({ page }) => {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    let data: unknown
    if (path === '/api/v1/health') data = { status: 'ok', posts: 1, comments: 1 }
    else if (path === '/api/v1/capabilities') data = { api_version: 'v1', schema_version: 3, fts5: true, archive_import: true, jobs: true, ai: false, live_search: false }
		else if (path === '/api/v1/jobs') data = []
		else if (path === '/api/v1/search/history') data = []
		else if (path === '/api/v1/ai/providers') data = [{ name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', configured: false }]
		else if (path === '/api/v1/ai/sessions') data = []
    else if (path === '/api/v1/imports' && route.request().method() === 'POST') data = {
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
    return route.fulfill({ status: path === '/api/v1/imports' ? 202 : 200, contentType: 'application/json', body: JSON.stringify({ data }) })
  })

  await page.goto('/')
  await expect(page.getByRole('heading', { name: '你的树洞资料，在本机慢慢长成档案' })).toBeVisible()

  await page.getByRole('link', { name: '归档导入' }).click()
  await page.locator('input[type=file]').setInputFiles({ name: 'sample.treehole.zip', mimeType: 'application/zip', buffer: Buffer.from('PK test') })
  await page.getByRole('button', { name: '预检并开始导入' }).click()
  await expect(page.getByText('预检完成 · v2')).toBeVisible()

  await page.getByRole('link', { name: '全文搜索' }).click()
  await page.getByRole('textbox', { name: '搜索关键词' }).fill('数据结构')
  await page.getByRole('button', { name: '搜索资料库' }).click()
  await page.getByText('数据结构课程体验').click()
  await expect(page.getByText('作业量适中')).toBeVisible()

  await page.getByRole('link', { name: 'AI 研究' }).click()
	await expect(page.getByRole('heading', { name: '从资料出发，而不是凭空回答' })).toBeVisible()
})
