import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import App from './App'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function renderApp(path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><App /></MemoryRouter></QueryClientProvider>)
}

function json(data: unknown, status = 200) {
  return Promise.resolve(new Response(JSON.stringify({ data }), { status, headers: { 'Content-Type': 'application/json' } }))
}

describe('PkuHoleStudio Web', () => {
  it('shows the empty archive guidance on the dashboard', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const path = String(input)
      if (path.endsWith('/health')) return json({ status: 'ok', posts: 0, comments: 0 })
      if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 3, fts5: true, archive_import: true, jobs: true, ai: false, live_search: false })
      if (path.includes('/jobs')) return json([])
      throw new Error(`unexpected request ${path}`)
    }))
    renderApp('/')
    expect(await screen.findByText('资料库还是空的')).toBeInTheDocument()
    expect(screen.getByText('FTS5')).toBeInTheDocument()
  })

  it('keeps a search in the URL-backed page and renders a result', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const path = String(input)
		if (path.includes('/search?q=alpha')) return json({ items: [{ pid: 123456, text: 'alpha result', reply: 2 }], has_more: false })
		if (path.includes('/search/history')) return json([])
      throw new Error(`unexpected request ${path}`)
    }))
    const user = userEvent.setup()
    renderApp('/search')
    await user.type(screen.getByPlaceholderText('课程名、教师、关键词或 #PID'), 'alpha')
    await user.click(screen.getByRole('button', { name: '搜索资料库' }))
    expect(await screen.findByText('alpha result')).toBeInTheDocument()
    expect(screen.getByText('#123456')).toBeInTheDocument()
  })

	it('uploads an archive and displays its preflight report', async () => {
    vi.stubGlobal('fetch', vi.fn(() => json({
      job: { id: 'job-1', type: 'import_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
      preflight: { format: 'legacy-v1', status: 'completed', hash: 'abc', run_id: 'legacy-abc', counts: { items: 1, valid_items: 1, comments: 0 }, issues: [] },
    })))
    const user = userEvent.setup()
    const { container } = renderApp('/imports')
    const input = container.querySelector('input[type=file]') as HTMLInputElement
    await user.upload(input, new File(['{"holes":[]}'], 'archive.json', { type: 'application/json' }))
    await user.click(screen.getByRole('button', { name: '预检并开始导入' }))
    await waitFor(() => expect(screen.getByText('预检完成 · legacy-v1')).toBeInTheDocument())
    expect(screen.getByText('job-1')).toBeInTheDocument()
	})

	it('shows a failed preflight and does not render a queued import job', async () => {
		const failure = { error: {
			code: 'archive_no_valid_items', message: 'archive contains no valid items', details: {
				preflight: { format: 'v2', status: 'failed', hash: 'bad', run_id: 'run-bad', counts: { items: 3, valid_items: 0, skipped_items: 3 }, issues: [{ severity: 'error', code: 'invalid_hole', message: 'bad field' }] },
			},
		} }
		vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify(failure), { status: 422, headers: { 'Content-Type': 'application/json' } }))))
		const user = userEvent.setup()
		const { container } = renderApp('/imports')
		const input = container.querySelector('input[type=file]') as HTMLInputElement
		await user.upload(input, new File(['zip'], 'archive.treehole.zip', { type: 'application/zip' }))
		await user.click(screen.getByRole('button', { name: '预检并开始导入' }))
		expect(await screen.findByText('预检未通过 · v2')).toBeInTheDocument()
		expect(screen.getByText('没有可导入的有效帖子，未创建导入任务。请查看下方错误详情。')).toBeInTheDocument()
		expect(screen.queryByText('queued')).not.toBeInTheDocument()
	})

	it('creates a persistent export job and restores it in export history', async () => {
		const job = { id: 'export-1', type: 'export_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
		let rows: unknown[] = []
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/exports/jobs') && init?.method === 'POST') { rows = [job]; return json(job, 202) }
			if (path.endsWith('/exports/jobs')) return json(rows)
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/imports')
		await user.click(await screen.findByRole('button', { name: '创建 archive v2 任务' }))
		expect(await screen.findByText('export-1')).toBeInTheDocument()
		expect(screen.getByText('queued')).toBeInTheDocument()
	})

	it('shows provider guidance when AI is not configured', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/ai/providers')) return json([{ name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', configured: false }])
			if (path.includes('/ai/sessions')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/ai')
		expect(await screen.findByText('AI Provider 尚未配置')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: '发送问题' })).toBeDisabled()
	})

	it('writes AI settings without requiring the existing API key to be returned', async () => {
		let saved = ''
		const settings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'DeepSeek', ai_base_url: 'https://api.deepseek.com', ai_model: 'deepseek-chat', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 5, ai_api_key_configured: true, restart_required: false }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 4, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, online_sync: true })
			if (path.endsWith('/ai/providers')) return json([{ name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', configured: true }])
			if (path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings') && init?.method === 'PUT') { saved = String(init.body); return json({ ...settings, ai_enabled: true, restart_required: true }) }
			if (path.endsWith('/settings')) return json(settings)
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/settings')
		const key = await screen.findByLabelText(/API key/)
		expect(key).toHaveAttribute('placeholder', '已配置；不会回显')
		await user.click(screen.getByLabelText('启用 AI'))
		await user.click(screen.getByRole('button', { name: '保存 AI 设置' }))
		await waitFor(() => expect(saved).toContain('"ai_enabled":true'))
		expect(saved).not.toContain('existing')
		expect(await screen.findByText(/设置已安全写入/)).toBeInTheDocument()
	})

	it('creates a native PID sync job after an online session is verified', async () => {
		let createdBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/jobs') && init?.method === 'POST') {
				createdBody = String(init.body)
				return json({ id: 'sync-1', type: 'sync_pids', status: 'queued', completed_items: 0, failed_items: 0, total_items: 2, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }, 202)
			}
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/sync')
		await user.type(await screen.findByPlaceholderText('1234567, 2345678'), '123456, 234567')
		await user.click(screen.getByRole('button', { name: '同步 2 个 PID' }))
		await waitFor(() => expect(createdBody).not.toBe(''))
		expect(JSON.parse(createdBody)).toEqual({ type: 'sync_pids', payload: { pids: [123456, 234567] } })
	})

	it('completes an IAAA SMS challenge with the original credentials', async () => {
		let challengeBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session')) return json({ checked: false, has_session: false, can_read_online: false, can_write_online: false })
			if (path.endsWith('/session/login')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false, challenge: 'sms', challenge_stage: 'iaaa', message: '短信验证码已发送至 138****0000' })
			if (path.endsWith('/session/challenge')) {
				challengeBody = String(init?.body)
				return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			}
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/sync')
  await user.click(await screen.findByRole('button', { name: '在 Studio 中登录' }))
		await user.type(await screen.findByPlaceholderText('北大学号（无需邮箱后缀）'), '1234567890')
		await user.type(screen.getByPlaceholderText('密码（不会由网页保存）'), 'secret')
		await user.click(screen.getByRole('button', { name: '登录并保存本机会话' }))
		await user.type(await screen.findByPlaceholderText('验证码'), '654321')
		await user.click(screen.getByRole('button', { name: '继续登录' }))
		await waitFor(() => expect(challengeBody).not.toBe(''))
		expect(JSON.parse(challengeBody)).toEqual({ stage: 'iaaa', challenge: 'sms', username: '1234567890', password: 'secret', code: '654321' })
	})

	it('renders AI search trace, streamed delta, and a source link', async () => {
		const session = { id: 'session-1', title: 'Research', mode: 'local', provider: 'fake', model: 'fake-model', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/ai/providers')) return json([{ name: 'fake', base_url: 'http://local', model: 'fake-model', configured: true }])
			if (path.includes('/ai/sessions/session-1/messages')) return json({ session_id: 'session-1', status: 'started' }, 202)
			if (path.endsWith('/ai/sessions/session-1')) return json({ session, messages: [] })
			if (path.includes('/ai/sessions') && init?.method === 'POST') return json(session, 201)
			if (path.includes('/ai/sessions')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		vi.stubGlobal('EventSource', MockEventSource)
		const user = userEvent.setup()
		renderApp('/ai')
		await user.type(await screen.findByPlaceholderText('基于本地资料提出问题…'), 'alpha question')
		await user.click(screen.getByRole('button', { name: '发送问题' }))
		await waitFor(() => expect(MockEventSource.latest).toBeTruthy())
		MockEventSource.latest!.emit('search_started', { round: 1, query: 'alpha', reason: 'find evidence' })
		MockEventSource.latest!.emit('search_result', { round: 1, query: 'alpha', matches: 2 })
		MockEventSource.latest!.emit('delta', { delta: 'grounded answer' })
		MockEventSource.latest!.emit('source', { pid: 12345, cid: 101, snippet: 'evidence' })
		expect(await screen.findByText('grounded answer')).toBeInTheDocument()
		expect(screen.getByText('第 1 轮：alpha · find evidence')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: '#12345/C101' })).toHaveAttribute('href', '/posts/12345#comment-101')
		MockEventSource.latest!.emit('completed', {})
	})
})

class MockEventSource {
	static latest: MockEventSource | null = null
	listeners = new Map<string, ((event: MessageEvent) => void)[]>()
	onerror: (() => void) | null = null
	constructor(public url: string) { MockEventSource.latest = this }
	addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
		const callback = listener as (event: MessageEvent) => void
		this.listeners.set(type, [...(this.listeners.get(type) ?? []), callback])
	}
	emit(type: string, data: unknown) { for (const listener of this.listeners.get(type) ?? []) listener(new MessageEvent(type, { data: JSON.stringify(data) })) }
	close() {}
}
