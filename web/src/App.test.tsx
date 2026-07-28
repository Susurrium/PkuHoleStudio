import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import App from './App'
import { CLASSIC_BACKGROUND_STORAGE_KEY, CLASSIC_COLOR_MODE_STORAGE_KEY, GITHUB_COLOR_MODE_STORAGE_KEY, LAYOUT_PRESET_STORAGE_KEY, useUIStore } from './store/ui'
import { ONBOARDING_STORAGE_KEY } from './components/OnboardingGuide'

beforeEach(() => window.localStorage.setItem(ONBOARDING_STORAGE_KEY, 'completed'))

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  useUIStore.getState().setLayoutPreset('studio')
  useUIStore.getState().setClassicBackground('stars')
  useUIStore.getState().setClassicColorMode('system')
  useUIStore.getState().setGithubColorMode('system')
  useUIStore.setState({ activeWorkspace: 'library', lastOnlineLocation: '/online', lastLibraryLocation: '/' })
  window.localStorage.clear()
  window.sessionStorage.clear()
})

function renderApp(path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><App /></MemoryRouter></QueryClientProvider>)
}

function json(data: unknown, status = 200) {
  return Promise.resolve(new Response(JSON.stringify({ data }), { status, headers: { 'Content-Type': 'application/json' } }))
}

function failure(code: string, message: string, status = 422) {
	return Promise.resolve(new Response(JSON.stringify({ error: { code, message, details: {} } }), { status, headers: { 'Content-Type': 'application/json' } }))
}

describe('PkuHoleStudio Web', () => {
	it('guides an empty first-time user into the chosen workspace', async () => {
		window.localStorage.removeItem(ONBOARDING_STORAGE_KEY)
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/health')) return json({ status: 'ok', posts: 0, comments: 0 })
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 4, fts5: true })
			if (path.includes('/jobs')) return json([])
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false })
			if (path.includes('/posts/hot')) return json({ items: [], source: 'live_recent', window_hours: 12, updated_at: 1_784_100_000, stale: false, approximate: true })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/')
		const dialog = await screen.findByRole('dialog', { name: '先选择你现在要做的事' })
		await user.click(within(dialog).getByRole('button', { name: /在线树洞/ }))
		await user.click(within(dialog).getByRole('button', { name: '继续' }))
		expect(await screen.findByRole('heading', { name: '理解在线与本地的关系' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '进入在线树洞' }))
		expect(await screen.findByRole('heading', { name: '在线树洞' })).toBeInTheDocument()
		expect(window.localStorage.getItem(ONBOARDING_STORAGE_KEY)).toBe('completed')
	})

	it('switches between the Studio and classic layout from interface settings', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 4, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, online_sync: true })
			if (path.endsWith('/ai/providers') || path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings')) return json({ database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: '', ai_base_url: '', ai_model: '', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 5, ai_api_key_configured: false, restart_required: false, ai_providers: [] })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/settings')
		await user.click(await screen.findByRole('button', { name: /经典树洞/ }))
		expect(await screen.findByText('PkuHoleStudio · 经典树洞')).toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: '设置' })).toBeInTheDocument()
		expect(window.localStorage.getItem(LAYOUT_PRESET_STORAGE_KEY)).toBe('classic')
		await user.click(screen.getByText('工具'))
		await user.click(screen.getByRole('button', { name: '切回 Studio 界面' }))
		expect(screen.getAllByText('Personal archive')).toHaveLength(2)
		expect(window.localStorage.getItem(LAYOUT_PRESET_STORAGE_KEY)).toBe('studio')
	})

	it('switches to the GitHub preset from settings and persists its color mode', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 4, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, online_sync: true })
			if (path.endsWith('/ai/providers') || path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings')) return json({ database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: '', ai_base_url: '', ai_model: '', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 5, ai_api_key_configured: false, restart_required: false, ai_providers: [] })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/settings')
		await user.click(await screen.findByRole('button', { name: /GitHub 风格/ }))
		expect(await screen.findByText('local-first', {}, { timeout: 5_000 })).toBeInTheDocument()
		expect(screen.getByRole('heading', { name: '设置' })).toBeInTheDocument()
		expect(window.localStorage.getItem(LAYOUT_PRESET_STORAGE_KEY)).toBe('github')
		await user.click(screen.getByLabelText('账户与界面'))
		await user.click(screen.getByRole('button', { name: '深色' }))
		expect(window.localStorage.getItem(GITHUB_COLOR_MODE_STORAGE_KEY)).toBe('dark')
	})

	it('renders a GitHub-style issue list and opens the shared-detail conversation', async () => {
		useUIStore.getState().setLayoutPreset('github')
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/posts/321?source=local')) return json({ post: { pid: 321, text: 'GitHub 详情正文', timestamp: 2, reply: 1 }, comments: [{ cid: 88, pid: 321, name_tag: 'Alice', text: 'Conversation 评论', timestamp: 3 }], references: [], media: [{ id: 7, owner_type: 'post', owner_id: 321, variant: 'original', status: 'available' }], has_more_comments: false })
			if (path.includes('/posts?')) return json({ items: [{ pid: 321, text: 'Issues 风格列表正文', timestamp: 1, reply: 1, praise_num: 2 }], has_more: false })
			if (path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/posts/321/tags')) return json([])
			if (path.endsWith('/posts/321/note')) return json({ content: '' })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts')
		expect(await screen.findByText('Issues 风格列表正文')).toBeInTheDocument()
		expect(document.querySelector('.github-issues-box')).toBeInTheDocument()
		await user.click(screen.getByRole('link', { name: 'Issues 风格列表正文' }))
		expect(await screen.findByText('GitHub 详情正文')).toBeInTheDocument()
		expect(screen.getByText('Conversation 评论')).toBeInTheDocument()
		expect(document.querySelector('.github-conversation-layout')).toBeInTheDocument()
		expect(screen.getByRole('img', { name: '树洞 #321 的图片 1' })).toBeInTheDocument()
		expect(document.querySelectorAll('main')).toHaveLength(1)
	})

	it('opens a classic thread as a drawer and keeps the list behind it', async () => {
		useUIStore.getState().setLayoutPreset('classic')
		vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
		const fetchMock = vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/jobs')) return json([])
			if (path.includes('/posts/321?source=local')) return json({ post: { pid: 321, text: '抽屉中的完整正文', timestamp: 2, reply: 1 }, comments: [{ cid: 88, pid: 321, name_tag: 'Alice', text: '完整评论', timestamp: 3 }], references: [], media: [], has_more_comments: false })
			if (path.includes('/posts?')) return json({ items: [{ pid: 321, text: '经典列表正文', timestamp: 1, reply: 1, comment_list: [{ cid: 88, pid: 321, name_tag: 'Alice', text: '回复预览', timestamp: 2 }] }], has_more: false })
			if (path.endsWith('/local-tags')) return json([])
			throw new Error(`unexpected request ${path}`)
		})
		vi.stubGlobal('fetch', fetchMock)
		const user = userEvent.setup()
		renderApp('/posts')
		expect(await screen.findByText(/回复预览/)).toBeInTheDocument()
		await user.click(screen.getByRole('link', { name: '打开树洞 #321' }))
		expect(await screen.findByRole('dialog', { name: '树洞 #321' })).toBeInTheDocument()
		expect(screen.getByText('抽屉中的完整正文')).toBeInTheDocument()
		expect(screen.getByText('经典列表正文')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '关闭详情' }))
		expect(screen.queryByRole('dialog', { name: '树洞 #321' })).not.toBeInTheDocument()
		expect(screen.getByText('经典列表正文')).toBeInTheDocument()
		await waitFor(() => expect(screen.getByRole('link', { name: '打开树洞 #321' })).toHaveFocus())
	})

	it('opens classic task, account, and message drawers without losing the list', async () => {
		useUIStore.getState().setLayoutPreset('classic')
		const session = { checked: true, has_session: true, can_read_online: true, can_write_online: true, message: '在线会话正常' }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/posts/654?source=live')) return json({ post: { pid: 654, text: '通知关联的在线洞', reply: 0 }, comments: [], references: [], media: [], has_more_comments: false })
			if (path.includes('/posts?')) return json({ items: [{ pid: 321, text: '抽屉后仍保留的列表', reply: 0 }], has_more: false })
			if (path.endsWith('/local-tags')) return json([])
			if (path.includes('/jobs')) return json([{ id: 'job-active', type: 'sync_pids', status: 'running', completed_items: 1, failed_items: 0, total_items: 2, attempts: 1, created_at: '2026-07-15T10:00:00Z', updated_at: '2026-07-15T10:00:01Z' }])
			if (path.endsWith('/session') || path.endsWith('/session/probe')) return json(session)
			if (path.includes('/notifications?type=interactive')) return json({ items: [{ id: 7, pid: 654, title: '收到回复', content: '有人回复了你的洞', read: false, type: 'int_msg' }], total: 1, page: 1 })
			if (path.endsWith('/notifications/7/read') && init?.method === 'POST') return json({ id: 7, read: true })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts')
		expect(await screen.findByText('抽屉后仍保留的列表')).toBeInTheDocument()

		await user.click(screen.getByRole('button', { name: /后台任务/ }))
		expect(await screen.findByRole('dialog', { name: '后台任务' })).toBeInTheDocument()
		expect(screen.getByText('同步指定 PID')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '关闭后台任务' }))

		await user.click(screen.getByRole('button', { name: '账户' }))
		expect(await screen.findByRole('dialog', { name: '账户与界面' })).toBeInTheDocument()
		await user.selectOptions(screen.getByLabelText('背景'), 'dusk')
		await user.selectOptions(screen.getByLabelText('明暗'), 'dark')
		expect(window.localStorage.getItem(CLASSIC_BACKGROUND_STORAGE_KEY)).toBe('dusk')
		expect(window.localStorage.getItem(CLASSIC_COLOR_MODE_STORAGE_KEY)).toBe('dark')
		expect(document.querySelector('.classic-shell')).toHaveClass('classic-bg-dusk', 'classic-mode-dark')
		await user.click(screen.getByRole('button', { name: '关闭账户与界面' }))

		await user.click(screen.getByRole('button', { name: '消息' }))
		expect(await screen.findByRole('dialog', { name: '消息' })).toBeInTheDocument()
		expect(await screen.findByText('有人回复了你的洞')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '查看树洞' }))
		expect(await screen.findByRole('dialog', { name: '树洞 #654' })).toBeInTheDocument()
		expect(screen.getByText('通知关联的在线洞')).toBeInTheDocument()
		expect(screen.getByText('抽屉后仍保留的列表')).toBeInTheDocument()
	})

	it('restores and clears a classic publish draft within the browser session', async () => {
		useUIStore.getState().setLayoutPreset('classic')
		const online = { checked: true, has_session: true, can_read_online: true, can_write_online: true }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json(online)
			if (path.includes('/notifications?type=interactive')) return json({ items: [], total: 0, page: 1 })
			if (path.includes('/tags?source=live')) return json([])
			if (path.includes('/posts?')) return json({ items: [{ pid: 1, text: '在线列表', reply: 0 }], has_more: false })
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?source=live')
		await user.click(await screen.findByRole('button', { name: /发表/ }))
		const editor = screen.getByPlaceholderText('写下想发布的内容…')
		await user.type(editor, '尚未发表的会话草稿')
		await user.click(screen.getByRole('button', { name: '关闭发表新洞' }))
		await user.click(screen.getByRole('button', { name: /发表/ }))
		expect(screen.getByPlaceholderText('写下想发布的内容…')).toHaveValue('尚未发表的会话草稿')
		await user.click(screen.getByRole('button', { name: '清空草稿' }))
		expect(screen.getByPlaceholderText('写下想发布的内容…')).toHaveValue('')
		expect(window.sessionStorage.getItem('pkustudio:classic:publish-draft')).toBeNull()
	})

	it('keeps the Studio publish draft after an error and clears it after success', async () => {
		let attempts = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/tags?source=live')) return json([])
			if (path.includes('/posts?')) return json({ items: [{ pid: 9, text: '在线列表', reply: 0 }], has_more: false })
			if (path.endsWith('/posts') && init?.method === 'POST') {
				attempts++
				return attempts === 1 ? failure('remote_rejected', '暂时不能发布') : json({ pid: 10, text: '会保留的草稿' }, 201)
			}
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?source=live')
		await user.click(await screen.findByText('发布新洞'))
		const editor = screen.getByPlaceholderText('写下想发布的内容…')
		await user.type(editor, '会保留的草稿')
		await user.click(screen.getByRole('button', { name: '确认发布' }))
		expect(await screen.findByText(/发布失败：暂时不能发布/)).toBeInTheDocument()
		expect(editor).toHaveValue('会保留的草稿')
		await user.click(screen.getByRole('button', { name: '确认发布' }))
		expect(await screen.findByText('树洞 #10 已发布')).toBeInTheDocument()
		expect(editor).toHaveValue('')
	})

	it('keeps the classic publish draft after an error and closes after success', async () => {
		useUIStore.getState().setLayoutPreset('classic')
		let attempts = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/tags?source=live')) return json([])
			if (path.includes('/notifications?type=interactive')) return json({ items: [], total: 0, page: 1 })
			if (path.includes('/jobs')) return json([])
			if (path.includes('/posts?')) return json({ items: [{ pid: 9, text: '经典在线列表', reply: 0 }], has_more: false })
			if (path.endsWith('/posts') && init?.method === 'POST') {
				attempts++
				return attempts === 1 ? failure('remote_rejected', '远端拒绝发布') : json({ pid: 11, text: '经典草稿' }, 201)
			}
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?source=live')
		await user.click(await screen.findByRole('button', { name: /发表/ }))
		const editor = screen.getByPlaceholderText('写下想发布的内容…')
		await user.type(editor, '经典草稿')
		await user.click(screen.getByRole('button', { name: '确认发表' }))
		expect(await screen.findByText(/发表失败：远端拒绝发布/)).toBeInTheDocument()
		expect(editor).toHaveValue('经典草稿')
		await user.click(screen.getByRole('button', { name: '确认发表' }))
		expect(await screen.findByText('树洞 #11 已发表')).toBeInTheDocument()
		expect(screen.queryByRole('dialog', { name: '发表新洞' })).not.toBeInTheDocument()
	})

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
    expect(screen.getByText('本地搜索')).toBeInTheDocument()
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
	await user.type(await screen.findByPlaceholderText('课程名、教师、关键词或 PID'), 'alpha')
    await user.click(screen.getByRole('button', { name: '搜索' }))
    expect(await screen.findByText('alpha result')).toBeInTheDocument()
    expect(screen.getByText('#123456')).toBeInTheDocument()
  })

	it('renders a completed empty hot ranking as an empty state', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/health')) return json({ status: 'ok', posts: 1, comments: 2 })
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 3, fts5: true })
			if (path.includes('/jobs')) return json([])
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false })
			if (path.includes('/posts/hot')) return json({ items: [], source: 'observer_cache', window_hours: 24, updated_at: 1_784_100_000, stale: true, approximate: false, message: 'Observer 暂无新数据' })
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/online')
		expect(await screen.findByText('当前时间范围内没有可用热榜数据')).toBeInTheDocument()
		expect(screen.queryByText('正在读取热榜…')).not.toBeInTheDocument()
		expect(screen.getByText('Observer 暂无新数据')).toBeInTheDocument()
	})

	it('labels a stale hot ranking and allows manual refresh', async () => {
		let hotRequests = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/health')) return json({ status: 'ok', posts: 1, comments: 2 })
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 3, fts5: true })
			if (path.includes('/jobs')) return json([])
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false })
			if (path.includes('/posts/hot')) { hotRequests++; return json({ items: [{ id: 123, text: '降级热榜', follownum: 8, reply: 3 }], source: 'observer_cache', window_hours: 24, updated_at: 1_784_100_000, stale: true, approximate: false, message: '当前展示最近 24 小时结果' }) }
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/online')
		expect(await screen.findByRole('heading', { name: '最近 24 小时热榜' })).toBeInTheDocument()
		expect(screen.getByText('数据范围已降级', { exact: false })).toBeInTheDocument()
		expect(screen.getByText('降级热榜')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '刷新热榜' }))
		await waitFor(() => expect(hotRequests).toBe(2))
	})

	it('shows whether online posts are already local and creates an explicit save job', async () => {
		let createdBody = ''
		let saved = false
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/posts/hot')) return json({ items: [], source: 'live_recent', window_hours: 12, updated_at: 1_784_100_000, stale: false, approximate: true })
			if (path.includes('/posts?source=live')) return json({ items: [{ pid: 101, text: '已经保存的在线洞', reply: 1, local_state: 'saved' }, { pid: 102, text: '尚未保存的在线洞', reply: 0, local_state: saved ? 'saved' : 'not_saved' }], has_more: false })
			if (path.includes('/posts?') && path.includes('source=local')) return json({ items: [{ pid: 102, text: '已交接到本地的树洞', reply: 0, local_state: 'saved' }], has_more: false })
			if (path.endsWith('/local-tags') || path.includes('/search/history') || path.endsWith('/projects')) return json([])
			if (path.includes('/jobs') && init?.method === 'POST') {
				createdBody = String(init.body)
				return json({ id: 'save-102', type: 'sync_pids', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }, 202)
			}
			if (path.endsWith('/jobs/save-102')) {
				saved = true
				return json({ id: 'save-102', type: 'sync_pids', status: 'completed', completed_items: 1, failed_items: 0, total_items: 1, attempts: 1, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:01Z' })
			}
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/online')
		expect(await screen.findByText('已经保存的在线洞')).toBeInTheDocument()
		expect(screen.getByText('已保存到本地')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '保存到本地' }))
		await waitFor(() => expect(JSON.parse(createdBody)).toEqual({ type: 'sync_pids', payload: { pids: [102], include_comments: true, include_media: true } }))
		await user.click(await screen.findByRole('link', { name: '前往本地整理' }))
		expect(await screen.findByText('已选择 1 个树洞')).toBeInTheDocument()
		expect(screen.getByText('已交接到本地的树洞')).toBeInTheDocument()
	})

	it('shows a save failure reason and retries the durable job', async () => {
		let retryCalls = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			const base = { id: 'save-103', type: 'sync_pids', completed_items: 0, failed_items: 1, total_items: 1, attempts: 1, error: '在线会话已过期', created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:01Z' }
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/posts/hot')) return json({ items: [], source: 'live_recent', window_hours: 12, updated_at: 1_784_100_000, stale: false, approximate: true })
			if (path.includes('/posts?source=live')) return json({ items: [{ pid: 103, text: '等待保存的在线洞', reply: 0, local_state: 'not_saved' }], has_more: false })
			if (path.endsWith('/jobs') && init?.method === 'POST') return json({ ...base, status: 'queued', failed_items: 0, attempts: 0 }, 202)
			if (path.endsWith('/jobs/save-103/retry') && init?.method === 'POST') { retryCalls++; return json({ ...base, status: 'queued', failed_items: 0, error: '' }) }
			if (path.endsWith('/jobs/save-103')) return json({ ...base, status: 'failed' })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/online')
		await user.click(await screen.findByRole('button', { name: '保存到本地' }))
		expect(await screen.findByText('在线会话已过期')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '保存失败，点击重试' }))
		await waitFor(() => expect(retryCalls).toBe(1))
	})

	it('selects multiple online posts in place and creates one background save job', async () => {
		let createdBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/tags?source=live')) return json([])
			if (path.includes('/posts?') && path.includes('source=live')) return json({ items: [
				{ pid: 201, text: '批量选择一', reply: 0, local_state: 'not_saved' },
				{ pid: 202, text: '批量选择二', reply: 0, local_state: 'not_saved' },
			], has_more: false })
			if (path.endsWith('/jobs') && init?.method === 'POST') {
				createdBody = String(init.body)
				return json({ id: 'batch-save', type: 'sync_pids', status: 'queued', completed_items: 0, failed_items: 0, total_items: 2, attempts: 0, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }, 202)
			}
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?source=live')
		await user.click(await screen.findByRole('button', { name: '选择多个' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #201' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #202' }))
		await user.click(screen.getByRole('button', { name: '保存到本地（2）' }))
		await waitFor(() => expect(createdBody).not.toBe(''))
		expect(JSON.parse(createdBody)).toEqual({ type: 'sync_pids', payload: { pids: [201, 202], include_comments: true, include_media: true } })
		expect(await screen.findByText('已创建 2 个树洞的保存任务')).toBeInTheDocument()
	})

	it('organizes selected local posts without replacing existing metadata', async () => {
		let tagBody = ''
		let projectBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/posts?')) return json({ items: [{ pid: 701, text: '本地批量一' }, { pid: 702, text: '本地批量二' }], has_more: false })
			if (path.endsWith('/search/history')) return json([])
			if (path.endsWith('/posts/batch/tags') && init?.method === 'POST') { tagBody = String(init.body); return json({ updated: 2 }) }
			if (path.endsWith('/posts/batch/projects') && init?.method === 'POST') { projectBody = String(init.body); return json({ updated: 2 }) }
			if (path.endsWith('/local-tags')) return json([{ id: 8, name: '待整理', color: '#0f766e' }])
			if (path.endsWith('/projects')) return json([{ id: 9, name: '课程项目', post_count: 1, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts')
		await user.click(await screen.findByRole('button', { name: '选择多个' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #701' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #702' }))
		await user.click(screen.getByRole('button', { name: '添加标签' }))
		await user.selectOptions(screen.getByRole('combobox', { name: '选择要添加的标签' }), '8')
		await user.click(screen.getByRole('button', { name: '添加标签（2）' }))
		expect(await screen.findByText('已为 2 个树洞添加标签')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '加入项目' }))
		await user.selectOptions(screen.getByRole('combobox', { name: '选择要加入的项目' }), '9')
		await user.click(screen.getByRole('button', { name: '加入项目（2）' }))
		expect(await screen.findByText('已将 2 个树洞加入项目')).toBeInTheDocument()
		expect(JSON.parse(tagBody)).toEqual({ pids: [701, 702], tag_ids: [8] })
		expect(JSON.parse(projectBody)).toEqual({ pids: [701, 702], project_ids: [9] })
	})

	it('carries a local selection into export and preserves the return context', async () => {
		let repairBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/posts?')) return json({ items: [{ pid: 801, text: '准备导出一' }, { pid: 802, text: '准备导出二' }], has_more: false })
			if (path.endsWith('/exports/preview')) return json({ format: 'treehole-v2', posts: 2, comments: 5, media: 3, missing_media: 1 })
			if (path.endsWith('/jobs') && init?.method === 'POST') {
				repairBody = String(init.body)
				return json({ id: 'repair-media-1', type: 'repair_media', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }, 202)
			}
			if (path.endsWith('/search/history') || path.endsWith('/local-tags') || path.endsWith('/exports/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?sort=asc')
		await user.click(await screen.findByRole('button', { name: '选择多个' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #801' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #802' }))
		await user.click(screen.getByRole('button', { name: '导出' }))
		expect(await screen.findByRole('heading', { name: '打包本地资料' })).toBeInTheDocument()
		expect(screen.getByLabelText('导出范围')).toHaveValue('801, 802')
		expect(screen.getByText('已带入 2 个树洞')).toBeInTheDocument()
		const missingMediaLabel = await screen.findByText('缺失图片')
		expect(missingMediaLabel.previousElementSibling).toHaveTextContent('1')
		await user.click(screen.getByRole('button', { name: '先补全缺失图片' }))
		await waitFor(() => expect(JSON.parse(repairBody)).toEqual({ type: 'repair_media', payload: {} }))
		expect(screen.getByRole('link', { name: '查看补全进度' })).toHaveAttribute('href', '/tasks')
		await user.click(screen.getByRole('link', { name: '返回来源页面' }))
		expect(await screen.findByText('已选择 2 个树洞')).toBeInTheDocument()
	})

	it('hands a completed PID sync job to the selected local workspace', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/jobs?limit=50')) return json([{ id: 'sync-finished', type: 'sync_pids', status: 'completed', scope: { pids: [901, 902] }, completed_items: 2, failed_items: 0, total_items: 2, attempts: 1, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:01Z' }])
			if (path.includes('/posts?') && path.includes('source=local')) return json({ items: [{ pid: 901, text: '交接一' }, { pid: 902, text: '交接二' }], has_more: false })
			if (path.endsWith('/local-tags') || path.includes('/search/history') || path.endsWith('/projects')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/tasks')
		await user.click(await screen.findByRole('link', { name: '前往本地整理（2）' }))
		expect(await screen.findByText('已选择 2 个树洞')).toBeInTheDocument()
		expect(screen.getByText('交接一')).toBeInTheDocument()
		expect(screen.getByText('交接二')).toBeInTheDocument()
	})

	it('shows continuous monitoring as resumable state instead of fake 100 percent progress', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/jobs?limit=50')) return json([{ id: 'monitor-1', type: 'monitor_latest', status: 'paused', checkpoint: { cycle: 3, page: 2 }, completed_items: 3, failed_items: 0, total_items: 3, attempts: 1, error: 'process restarted', created_at: '2026-07-15T10:00:00Z', updated_at: '2026-07-15T10:05:00Z' }])
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/tasks')
		expect(await screen.findByText('需要恢复')).toBeInTheDocument()
		expect(screen.getByText('已进行 3 轮 · 最近检查第 2 页')).toBeInTheDocument()
		expect(screen.queryByText('100%')).not.toBeInTheDocument()
	})

	it('restores the last online page and its publish draft after visiting the library workspace', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/tags?source=live')) return json([])
			if (path.includes('/posts?') && path.includes('source=live')) return json({ items: [{ pid: 1, text: '在线工作区列表', local_state: 'not_saved' }], has_more: false })
			if (path.endsWith('/health')) return json({ status: 'ok', posts: 1, comments: 0 })
			if (path.endsWith('/capabilities')) return json({ fts5: true })
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?source=live')
		await user.click(await screen.findByText('发布新洞'))
		await user.type(screen.getByPlaceholderText('写下想发布的内容…'), '跨工作区保留的草稿')
		await user.click(screen.getAllByRole('button', { name: '本地资料库' })[0])
		expect(await screen.findByText('已切换到本地资料库')).toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: '本地资料库' })).toBeInTheDocument()
		await user.click(screen.getAllByRole('button', { name: '在线树洞' })[0])
		expect(await screen.findByText('已切换到在线树洞')).toBeInTheDocument()
		expect(await screen.findByText('在线工作区列表')).toBeInTheDocument()
		expect(screen.getByPlaceholderText('写下想发布的内容…')).toHaveValue('跨工作区保留的草稿')
	})

	it('opens a post when the user clicks anywhere on its card', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/posts/321?')) return json({ post: { pid: 321, text: '整卡点击后的正文', timestamp: 1, reply: 0 }, comments: [], references: [], media: [], has_more_comments: false })
			if (path.includes('/posts?')) return json({ items: [{ pid: 321, text: '整卡可点击', timestamp: 1, reply: 0 }], has_more: false })
			if (path.includes('/search/history') || path.endsWith('/local-tags') || path.endsWith('/posts/321/tags')) return json([])
			if (path.endsWith('/posts/321/note')) return json({ owner_type: 'post', owner_id: 321, content: '' })
			if (path.includes('/posts/321/references')) return json({ root: 321, nodes: [{ pid: 321, text: '整卡点击后的正文' }], edges: [] })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts')
		await user.click(await screen.findByRole('link', { name: '打开树洞 #321' }))
		expect(await screen.findByText('整卡点击后的正文')).toBeInTheDocument()
	})

	it('keeps the last online timeline visible when a manual refresh fails', async () => {
		let postRequests = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.endsWith('/tags')) return json([])
			if (path.includes('/posts?')) {
				postRequests++
				if (postRequests > 1) return Promise.reject(new Error('network unavailable'))
				return json({ items: [{ pid: 654, text: '上一次成功读取的在线内容', timestamp: 1, reply: 0 }], has_more: false })
			}
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts?source=live')
		expect(await screen.findByText('上一次成功读取的在线内容')).toBeInTheDocument()
		await user.click(await screen.findByRole('button', { name: '刷新' }))
		expect(await screen.findByText('刷新失败，继续显示上次结果')).toBeInTheDocument()
		expect(screen.getByText('上一次成功读取的在线内容')).toBeInTheDocument()
	})

	it('opens a durable research project and carries its posts into AI scope', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/projects')) return json([{ id: 1, name: '选课研究', description: '整理课程评价', color: '#0f766e', post_count: 1, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }])
			if (path.endsWith('/projects/1/posts')) return json([{ pid: 8133824, text: '项目中的树洞', reply: 2 }])
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/projects?project=1')
		expect(await screen.findByRole('heading', { name: '选课研究' })).toBeInTheDocument()
		expect(await screen.findByText('项目中的树洞')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: '研究项目内容' })).toHaveAttribute('href', '/ai?mode=selected&pids=8133824')
	})

	it('searches, sorts, and batch removes posts inside a project', async () => {
		let removeBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/projects/1/posts/remove') && init?.method === 'POST') { removeBody = String(init.body); return json({ updated: 2 }) }
			if (path.endsWith('/projects')) return json([{ id: 1, name: '批量整理项目', description: '', color: '#0f766e', post_count: 3, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:00Z' }])
			if (path.endsWith('/projects/1/posts')) return json([
				{ pid: 901, text: '第一条课程资料', reply: 1, timestamp: 100 },
				{ pid: 902, text: '第二条生活资料', reply: 8, timestamp: 300 },
				{ pid: 903, text: '第三条课程资料', reply: 3, timestamp: 200 },
			])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/projects?project=1')
		const search = await screen.findByPlaceholderText('搜索项目内正文或 PID')
		await user.type(search, '第二条')
		expect(screen.getByText('第二条生活资料')).toBeInTheDocument()
		expect(screen.queryByText('第一条课程资料')).not.toBeInTheDocument()
		await user.clear(search)
		await user.selectOptions(screen.getByRole('combobox', { name: '项目资料排序' }), 'comments')
		const cards = screen.getAllByRole('link', { name: /打开树洞 #/ })
		expect(cards[0]).toHaveAttribute('aria-label', '打开树洞 #902')
		await user.click(screen.getByRole('button', { name: '选择多个' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #902' }))
		await user.click(screen.getByRole('button', { name: '选择树洞 #903' }))
		await user.click(screen.getByRole('button', { name: '移出项目' }))
		const dialog = await screen.findByRole('alertdialog')
		await user.click(within(dialog).getByRole('button', { name: '移出项目' }))
		expect(await screen.findByText('已从项目移出 2 个树洞')).toBeInTheDocument()
		expect(JSON.parse(removeBody)).toEqual({ pids: [902, 903] })
	})

	it('groups background work in the task center and localizes its statuses', async () => {
		const base = { completed_items: 0, failed_items: 0, total_items: 2, attempts: 1, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:01Z' }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/jobs?')) return json([{ ...base, id: 'sync-running', type: 'sync_pids', status: 'running' }, { ...base, id: 'export-failed', type: 'export_archive', status: 'failed' }])
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/tasks')
		expect(await screen.findByText('正在运行')).toBeInTheDocument()
		expect(screen.getByText('失败')).toBeInTheDocument()
		expect(screen.getByText('需要处理')).toBeInTheDocument()
	})

	it('uploads an archive and displays its preflight report', async () => {
		const job = { id: 'job-1', type: 'import_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
		const preflight = { format: 'legacy-v1', status: 'completed', hash: 'abc', run_id: 'legacy-abc', counts: { items: 1, valid_items: 1, comments: 0 }, issues: [] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/imports?')) return json([])
			if (path.endsWith('/exports/jobs')) return json([])
			if (path.endsWith('/jobs/job-1')) return json(job)
			if (path.endsWith('/imports/preflight') && init?.method === 'POST') return json({ token: 'preview-1', status: 'awaiting_confirmation', expires_at: '2026-01-01T01:00:00Z', filename: 'archive.json', preflight })
			if (path.endsWith('/imports/preflight/preview-1/confirm') && init?.method === 'POST') return json({ token: 'preview-1', status: 'queued', expires_at: '2026-01-01T01:00:00Z', filename: 'archive.json', preflight, job }, 202)
			throw new Error(`unexpected request ${path}`)
		}))
    const user = userEvent.setup()
    const { container } = renderApp('/imports')
	await screen.findByRole('heading', { name: '导入与导出' })
    const input = container.querySelector('input[type=file]') as HTMLInputElement
    await user.upload(input, new File(['{"holes":[]}'], 'archive.json', { type: 'application/json' }))
	await user.click(screen.getByRole('button', { name: '预检文件' }))
	await waitFor(() => expect(screen.getByText('文件检查通过')).toBeInTheDocument())
		expect(screen.queryByText('job-1')).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '确认导入' }))
    expect(screen.getByText('job-1')).toBeInTheDocument()
		expect(screen.getByText(/任务已持久保存/)).toBeInTheDocument()
	})

	it('shows a failed preflight and does not render a queued import job', async () => {
		const preflight = { format: 'v2', status: 'failed', hash: 'bad', run_id: 'run-bad', counts: { items: 3, valid_items: 0, skipped_items: 3 }, issues: [{ severity: 'error', code: 'invalid_hole', message: 'bad field' }] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/imports?')) return json([])
			if (path.endsWith('/exports/jobs')) return json([])
			if (path.endsWith('/imports/preflight') && init?.method === 'POST') return json({ token: 'preview-bad', status: 'awaiting_confirmation', expires_at: '2026-01-01T01:00:00Z', filename: 'archive.treehole.zip', preflight })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		const { container } = renderApp('/imports')
		await screen.findByRole('heading', { name: '导入与导出' })
		const input = container.querySelector('input[type=file]') as HTMLInputElement
		await user.upload(input, new File(['zip'], 'archive.treehole.zip', { type: 'application/zip' }))
		await user.click(screen.getByRole('button', { name: '预检文件' }))
		expect(await screen.findByText('文件检查未通过')).toBeInTheDocument()
		expect(screen.getByText('没有可导入的有效帖子，因此没有创建任务。请查看下方问题详情。')).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: '确认导入' })).not.toBeInTheDocument()
		expect(screen.queryByText('queued')).not.toBeInTheDocument()
	})

	it('detects an already imported archive before creating another job', async () => {
		const preflight = { format: 'v2', status: 'completed', hash: 'same-hash', run_id: 'same-run', duplicate: true, counts: { items: 4, valid_items: 4 }, issues: [] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/imports?')) return json([])
			if (path.endsWith('/exports/jobs')) return json([])
			if (path.endsWith('/imports/preflight') && init?.method === 'POST') return json({ token: 'preview-duplicate', status: 'awaiting_confirmation', expires_at: '2026-01-01T01:00:00Z', filename: 'archive.treehole.zip', preflight })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		const { container } = renderApp('/imports')
		await screen.findByRole('heading', { name: '导入与导出' })
		const input = container.querySelector('input[type=file]') as HTMLInputElement
		await user.upload(input, new File(['zip'], 'archive.treehole.zip', { type: 'application/zip' }))
		await user.click(screen.getByRole('button', { name: '预检文件' }))
		expect(await screen.findByText('检测到重复归档')).toBeInTheDocument()
		expect(screen.getByText('这个归档已经成功导入过，无需再次创建任务。')).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: '确认导入' })).not.toBeInTheDocument()
		expect(screen.getByRole('button', { name: '关闭并删除暂存文件' })).toBeInTheDocument()
	})

	it('creates a persistent export job and restores it in export history', async () => {
		const job = { id: 'export-1', type: 'export_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
		let rows: unknown[] = []
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/imports?')) return json([])
			if (path.endsWith('/exports/preview')) return json({ format: 'treehole-v2', posts: 1, comments: 2, media: 0, missing_media: 0 })
			if (path.endsWith('/exports/jobs') && init?.method === 'POST') { rows = [job]; return json(job, 202) }
			if (path.endsWith('/exports/jobs')) return json(rows)
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/imports')
		await user.click(await screen.findByRole('button', { name: '导出资料' }))
		await user.click(await screen.findByRole('button', { name: '创建 archive v2 任务' }))
		expect(await screen.findByText('export-1')).toBeInTheDocument()
		expect(screen.getByText('等待中')).toBeInTheDocument()
	})

	it('restores completed import history and report after a page refresh', async () => {
		const report = { format: 'v2', status: 'completed', hash: 'hash', run_id: 'run', counts: { items: 3, valid_items: 3, comments: 226 }, issues: [] }
		const job = { id: 'import-finished', type: 'import_archive', status: 'completed', checkpoint: report, completed_items: 1, failed_items: 0, total_items: 1, attempts: 1, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:01Z' }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/imports?')) return json([job])
			if (path.endsWith('/exports/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/imports')
		expect(await screen.findByText('import-finished')).toBeInTheDocument()
		await userEvent.setup().click(screen.getByText('查看最终导入报告'))
		expect(screen.getByText('226')).toBeInTheDocument()
		expect(screen.getByText('评论')).toBeInTheDocument()
	})

	it('shows provider guidance when AI is not configured', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/ai/providers')) return json([{ name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', configured: false }])
			if (path.includes('/ai/sessions')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/ai')
		expect(await screen.findByText('还没有可用的 AI 服务')).toBeInTheDocument()
		expect(screen.getByText(/保存后立即生效，不需要重启/)).toBeInTheDocument()
		expect(screen.queryByText(/环境变量后重启/)).not.toBeInTheDocument()
		expect(screen.getByRole('button', { name: '发送问题' })).toBeDisabled()
	})

	it('writes AI settings without requiring the existing API key to be returned', async () => {
		let saved = ''
		const provider = { id: 'deepseek', name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', temperature: 0.2, max_output_tokens: 4096, request_timeout_seconds: 120, api_key_configured: true, active: true }
		const settings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'DeepSeek', ai_base_url: 'https://api.deepseek.com', ai_model: 'deepseek-chat', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 5, ai_api_key_configured: true, restart_required: false, ai_active_provider: 'deepseek', ai_providers: [provider] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 4, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, online_sync: true })
			if (path.endsWith('/ai/providers')) return json([{ id: 'deepseek', name: 'DeepSeek', base_url: 'https://api.deepseek.com', model: 'deepseek-chat', configured: true, active: true }])
			if (path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings/ai/providers/deepseek/test') && init?.method === 'POST') return json({ provider_id: 'deepseek', latency_ms: 42, model: 'deepseek-chat' })
			if (path.endsWith('/settings') && init?.method === 'PUT') { saved = String(init.body); return json({ ...settings, ai_enabled: true, restart_required: false }) }
			if (path.endsWith('/settings')) return json(settings)
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: '工作区与界面' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '重新查看入门引导' }))
		expect(await screen.findByRole('dialog', { name: '先选择你现在要做的事' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '不再自动显示' }))
		await user.click(await screen.findByRole('button', { name: '测试' }))
		expect(await screen.findByText('连接成功 · 42 ms')).toBeInTheDocument()
		await user.click(await screen.findByRole('button', { name: '编辑' }))
		const key = screen.getByLabelText(/API key/)
		expect(key).toHaveAttribute('placeholder', '已配置；不会回显')
		await user.click(screen.getByRole('button', { name: '取消' }))
		await user.click(screen.getByLabelText('启用 AI 研究'))
		await user.click(screen.getByRole('button', { name: '保存策略' }))
		await waitFor(() => expect(saved).toContain('"ai_enabled":true'))
		expect(saved).not.toContain('existing')
		expect(await screen.findByText(/设置已安全写入/)).toBeInTheDocument()
	})

	it('guides signed-out users to login instead of showing a notification error', async () => {
		let notificationRequests = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false, message: '本机会话已过期' })
			if (path.includes('/notifications?')) { notificationRequests++; return json({ items: [], total: 0, page: 1 }) }
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/notifications')
		expect(await screen.findByText('登录后查看通知')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: '前往同步中心登录' })).toHaveAttribute('href', '/sync')
		expect(notificationRequests).toBe(0)
	})

	it('only requests scores after explicit reveal and removes them from the page when hidden', async () => {
		let scoreRequests = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: false })
			if (path.endsWith('/campus/scores')) {
				scoreRequests++
				return json({ gpa: '3.82', total_credit: '100', passed_credit: '98', course_count: '20', gpa_terms: [], scores: [{ year_term: '2025-2026-2', name: '测试课程', credit: '3', score: '95', category: '专业课' }] })
			}
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/campus?view=scores')
		expect(await screen.findByText('成绩默认保持隐藏')).toBeInTheDocument()
		expect(scoreRequests).toBe(0)
		await user.click(screen.getByRole('button', { name: '加载并显示成绩' }))
		expect(await screen.findByText('3.82')).toBeInTheDocument()
		expect(scoreRequests).toBe(1)
		await user.click(screen.getByRole('button', { name: '隐藏成绩' }))
		expect(screen.queryByText('3.82')).not.toBeInTheDocument()
		expect(screen.getByRole('button', { name: '重新显示成绩' })).toBeInTheDocument()
	})

	it('requires confirmation before deleting expired staging files', async () => {
		let createdBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/jobs') && init?.method === 'POST') {
				createdBody = String(init.body)
				return json({ id: 'cleanup-1', type: 'cleanup_staging', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }, 202)
			}
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/maintenance')
		await user.click(await screen.findByRole('button', { name: '清理 7 天前暂存' }))
		expect(screen.getByRole('alertdialog')).toBeInTheDocument()
		expect(createdBody).toBe('')
		await user.click(screen.getByRole('button', { name: '确认清理' }))
		await waitFor(() => expect(JSON.parse(createdBody)).toEqual({ type: 'cleanup_staging', payload: { retention_days: 7 } }))
		expect(await screen.findByText('清理过期暂存任务已创建')).toBeInTheDocument()
	})

	it('shows live diagnostic lines and falls back to the snapshot after an SSE error', async () => {
		let snapshotRequests = 0
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/logs?')) { snapshotRequests++; return json([]) }
			throw new Error(`unexpected request ${path}`)
		}))
		vi.stubGlobal('EventSource', MockEventSource)
		renderApp('/logs')
		await waitFor(() => expect(MockEventSource.latest?.url).toContain('/api/v1/logs/events'))
		MockEventSource.latest!.emit('ready', { connected: true })
		expect(await screen.findByText('● 实时日志已连接，新内容会自动追加')).toBeInTheDocument()
		MockEventSource.latest!.emit('line', { module: 'crawler', line: 'sync completed' })
		expect(await screen.findByText(/\[crawler\] sync completed/)).toBeInTheDocument()
		MockEventSource.latest!.fail()
		expect(await screen.findByText('○ 实时连接暂不可用，仍可手动刷新快照')).toBeInTheDocument()
		await waitFor(() => expect(snapshotRequests).toBeGreaterThan(1))
	})

	it('restores a paged CID deep link after refreshing a post', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.includes('/posts/123/comments') && path.includes('cursor=50')) return json({ items: [{ cid: 75, pid: 123, text: 'target comment', timestamp: 2 }], next_cursor: 75, has_more: false })
			if (path.includes('/posts/123?')) return json({ post: { pid: 123, text: 'post', timestamp: 1, reply: 75 }, comments: [{ cid: 1, pid: 123, text: 'first comment', timestamp: 1 }], references: [], media: [], next_comment_cursor: 50, has_more_comments: true })
			if (path.endsWith('/local-tags') || path.endsWith('/posts/123/tags')) return json([])
			if (path.endsWith('/posts/123/note')) return json({ owner_type: 'post', owner_id: 123, content: '' })
			if (path.includes('/posts/123/references')) return json({ root: 123, nodes: [{ pid: 123, text: 'post' }], edges: [] })
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/posts/123?comment_cursor=75#comment-75')
		expect(await screen.findByText('target comment')).toBeInTheDocument()
		expect(screen.getByText('已载入 2 / 75')).toBeInTheDocument()
	})

	it('focuses the live reply composer when quoting a comment', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/session/probe')) return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			if (path.includes('/posts/456?')) return json({ post: { pid: 456, text: 'live post', timestamp: 1, reply: 1 }, comments: [{ cid: 9, pid: 456, text: 'quote me', timestamp: 2 }], references: [], media: [], has_more_comments: false })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/posts/456?source=live')
		await user.click(await screen.findByRole('button', { name: '引用回复' }))
		const composer = screen.getByRole('textbox', { name: '回复内容' })
		expect(composer).toHaveFocus()
		expect(composer).toHaveAttribute('placeholder', '回复并引用 C9…')
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
		await user.click(screen.getByRole('button', { name: '保存 2 个 PID' }))
		await waitFor(() => expect(createdBody).not.toBe(''))
		expect(JSON.parse(createdBody)).toEqual({ type: 'sync_pids', payload: { pids: [123456, 234567], include_comments: true, include_media: true } })
	})

	it('reloads a session saved by the TUI without asking for credentials', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session/reload') && init?.method === 'POST') return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true, message: '在线会话可用' })
			if (path.endsWith('/session')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false })
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/sync')
		await user.click(await screen.findByRole('button', { name: '载入 TUI 已登录会话' }))
		expect(await screen.findByText('在线读取已就绪')).toBeInTheDocument()
	})

	it('starts offline reference maintenance from its dedicated page', async () => {
		let createdBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/jobs') && init?.method === 'POST') {
				createdBody = String(init.body)
				return json({ id: 'maint-1', type: 'rebuild_references', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }, 202)
			}
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/maintenance')
		await user.click(await screen.findByRole('button', { name: '开始重建引用关系' }))
		await waitFor(() => expect(JSON.parse(createdBody)).toEqual({ type: 'rebuild_references', payload: {} }))
	})

	it('creates a Studio-native live capture and image export job', async () => {
		let createdBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.includes('/imports?')) return json([])
			if (path.endsWith('/exports/preview')) return json({ format: 'treehole-v2', posts: 0, comments: 0, media: 0, missing_media: 0 })
			if (path.endsWith('/exports/jobs') && init?.method === 'POST') {
				createdBody = String(init.body)
				return json({ id: 'capture-export', type: 'export_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 2, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }, 202)
			}
			if (path.endsWith('/exports/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/imports')
		await user.click(screen.getByRole('button', { name: '导出资料' }))
		await user.type(await screen.findByPlaceholderText('留空导出全部；或输入 1234567, 2345678'), '8328353')
		await user.click(screen.getByLabelText('导出前在线更新指定 PID'))
		await user.click(screen.getByRole('button', { name: '创建 archive v2 任务' }))
		await waitFor(() => expect(JSON.parse(createdBody)).toEqual({ format: 'treehole-v2', pids: [8328353], include_comments: true, capture_live: true, include_media: true }))
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
		await user.click(await screen.findByRole('button', { name: '使用学号在这里登录' }))
		await user.type(await screen.findByPlaceholderText('北大学号（无需邮箱后缀）'), '1234567890')
		await user.type(screen.getByPlaceholderText('密码（不会由网页保存）'), 'secret')
		await user.click(screen.getByRole('button', { name: '登录并保存本机会话' }))
		await user.type(await screen.findByPlaceholderText('短信验证码'), '654321')
		await user.click(screen.getByRole('button', { name: '继续登录' }))
		await waitFor(() => expect(challengeBody).not.toBe(''))
		expect(JSON.parse(challengeBody)).toEqual({ stage: 'iaaa', challenge: 'sms', username: '1234567890', password: 'secret', code: '654321' })
	})

	it('identifies an IAAA mobile-token challenge without offering SMS resend', async () => {
		let challengeBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session')) return json({ checked: false, has_session: false, can_read_online: false, can_write_online: false })
			if (path.endsWith('/session/login')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false, challenge: 'otp', challenge_stage: 'iaaa', message: '统一身份认证要求手机令牌验证' })
			if (path.endsWith('/session/challenge')) {
				challengeBody = String(init?.body)
				return json({ checked: true, has_session: true, can_read_online: true, can_write_online: true })
			}
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/sync')
		await user.click(await screen.findByRole('button', { name: '使用学号在这里登录' }))
		await user.type(await screen.findByPlaceholderText('北大学号（无需邮箱后缀）'), '1234567890')
		await user.type(screen.getByPlaceholderText('密码（不会由网页保存）'), 'secret')
		await user.click(screen.getByRole('button', { name: '登录并保存本机会话' }))
		expect(await screen.findByText('输入手机令牌（6 位动态口令）')).toBeInTheDocument()
		expect(screen.getByText(/北京大学.*手机令牌.*不会发送短信/)).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: '没有收到？重新发送验证码' })).not.toBeInTheDocument()
		await user.type(screen.getByPlaceholderText('6 位动态口令'), '123456')
		await user.click(screen.getByRole('button', { name: '继续登录' }))
		await waitFor(() => expect(challengeBody).not.toBe(''))
		expect(JSON.parse(challengeBody)).toEqual({ stage: 'iaaa', challenge: 'otp', username: '1234567890', password: 'secret', code: '123456' })
	})

	it('uses the Treehole SMS endpoint for a Treehole-stage challenge', async () => {
		let smsBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/session')) return json({ checked: true, has_session: false, can_read_online: false, can_write_online: false })
			if (path.endsWith('/session/login')) return json({ checked: true, has_session: true, can_read_online: false, can_write_online: false, challenge: 'sms', challenge_stage: 'treehole', message: '需要短信验证' })
			if (path.endsWith('/session/sms')) { smsBody = String(init?.body); return json({ checked: true, has_session: true, can_read_online: false, can_write_online: false, challenge: 'sms', challenge_stage: 'treehole', message: '短信已发送' }) }
			if (path.includes('/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/sync')
		await user.click(await screen.findByRole('button', { name: '使用学号在这里登录' }))
		await user.type(screen.getByPlaceholderText('北大学号（无需邮箱后缀）'), '1234567890')
		await user.type(screen.getByPlaceholderText('密码（不会由网页保存）'), 'secret')
		await user.click(screen.getByRole('button', { name: '登录并保存本机会话' }))
		await user.click(await screen.findByRole('button', { name: '发送树洞短信验证码' }))
		await waitFor(() => expect(JSON.parse(smsBody)).toEqual({ stage: 'treehole' }))
	})

	it('keeps the AI workspace in new-research mode instead of reselecting history', async () => {
		const session = { id: 'old-session', title: '旧研究', mode: 'local', provider: 'fake', model: 'fake-model', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/ai/providers')) return json([{ name: 'fake', base_url: 'http://local', model: 'fake-model', configured: true }])
			if (path.includes('/ai/sessions/old-session')) return json({ session, messages: [{ id: 'old-message', session_id: session.id, role: 'assistant', content: '旧回答', created_at: session.created_at }] })
			if (path.includes('/ai/sessions')) return json([session])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/ai')
		await user.click(await screen.findByRole('button', { name: /旧研究/ }))
		expect(await screen.findByText('旧回答')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '新建研究' }))
		expect(await screen.findByText('你想从资料中了解什么？')).toBeInTheDocument()
		expect(screen.queryByText('旧回答')).not.toBeInTheDocument()
		await waitFor(() => expect(screen.getByRole('textbox', { name: '研究问题' })).toHaveFocus())
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
		MockEventSource.latest!.emit('evidence_report', { checked: true, summary: { total: 1, cited: 1, supported: 1, partial: 0, unsupported: 0, unverified: 0, insufficient: 0, citation_coverage: 1 }, claims: [{ ordinal: 1, text: 'grounded answer', status: 'supported', reason: '引文直接支持结论', sources: [{ origin: 'local', pid: 12345, cid: 101 }] }] })
		expect((await screen.findAllByText('grounded answer')).length).toBeGreaterThanOrEqual(2)
		expect(screen.getByText('第 1 轮：alpha · find evidence')).toBeInTheDocument()
		expect(screen.getByText('证据覆盖 100% · 语义核对完成')).toBeInTheDocument()
		expect(screen.getByText('引文直接支持结论')).toBeInTheDocument()
		for (const link of screen.getAllByRole('link', { name: '#12345/C101' })) expect(link).toHaveAttribute('href', '/posts/12345?source=local&return_to=%2Fai#comment-101')
		MockEventSource.latest!.emit('completed', {})
	})

	it('starts a durable selected research scope from an AI deep link', async () => {
		const session = { id: 'selected-session', title: '分析当前洞', mode: 'selected', provider: 'fake', model: 'fake-model', scope: { pids: [12345] }, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
		let createdBody = ''
		let messageBody = ''
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/ai/providers')) return json([{ name: 'fake', base_url: 'http://127.0.0.1:11434', model: 'fake-model', configured: true, active: true }])
			if (path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/ai/sessions/selected-session/messages')) { messageBody = String(init?.body); return json({ session_id: session.id, run_id: 'run-1', status: 'started' }, 202) }
			if (path.endsWith('/ai/sessions/selected-session')) return json({ session, messages: [], latest_run: { id: 'run-1', session_id: session.id, status: 'running', prompt: '分析这个洞', created_at: session.created_at, updated_at: session.updated_at } })
			if (path.endsWith('/ai/sessions') && init?.method === 'POST') { createdBody = String(init.body); return json(session, 201) }
			if (path.includes('/ai/sessions')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		vi.stubGlobal('EventSource', MockEventSource)
		const user = userEvent.setup()
		renderApp('/ai?mode=selected&pids=12345')
		expect(await screen.findByPlaceholderText('选中 PID，例如 123456, 234567')).toHaveValue('12345')
		await user.type(screen.getByLabelText('研究问题'), '分析这个洞')
		await user.click(screen.getByRole('button', { name: '发送问题' }))
		await waitFor(() => expect(createdBody).toContain('"pids":[12345]'))
		expect(messageBody).toContain('"replace_scope":true')
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
	fail() { this.onerror?.() }
	close() {}
}
