import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import App from './App'
import { ONBOARDING_STORAGE_KEY } from './components/OnboardingGuide'
import { useUIStore } from './store/ui'

beforeEach(() => window.localStorage.setItem(ONBOARDING_STORAGE_KEY, 'completed'))
afterEach(() => { cleanup(); vi.restoreAllMocks(); window.localStorage.clear(); window.sessionStorage.clear(); useUIStore.getState().setLayoutPreset('studio') })

function fixture({ failure = false } = {}) {
	let refreshed = false
	let fullUpdates = 0
	const calls: { path: string; method?: string; body?: string }[] = []
	const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
		const path = String(input)
		calls.push({ path, method: init?.method, body: String(init?.body || '') })
		let data: unknown = []
		if (path.endsWith('/capabilities')) data = { api_version: 'v1', schema_version: 14, post_updates: true, jobs: true, online_sync: true }
		else if (path.includes('/session')) data = { checked: true, has_session: true, can_read_online: true, can_write_online: true }
		else if (path.endsWith('/post-updates')) data = { account: 'account-a', enabled: false, interval_minutes: 60, pids: [], targets: [], next_run: '2026-10-17T10:00:00Z', running: false, revision: 0 }
		else if (path.endsWith('/posts/12345/refresh')) {
			if (JSON.parse(String(init?.body)).full) { fullUpdates++; data = { id: `update-${fullUpdates}`, type: 'sync_pids', status: 'queued', total_items: 1, completed_items: 0, failed_items: 0, attempts: 0 } }
			else if (failure) return new Response(JSON.stringify({ error: { code: 'refresh_failed', message: '会话失效，本地内容已保留' } }), { status: 502, headers: { 'Content-Type': 'application/json' } })
			else { refreshed = true; data = { pid: 12345, comments: 1 } }
		}
		else if (/\/jobs\/update-\d+$/.test(path)) data = { id: path.split('/').at(-1), type: 'sync_pids', status: 'completed', total_items: 1, completed_items: 1, failed_items: 0, attempts: 1 }
		else if (/\/posts\/12345\?/.test(path)) data = { post: { pid: 12345, text: refreshed ? '取得更新后的正文' : '可离线阅读的正文', likenum: refreshed ? 3 : 20 }, comments: refreshed ? [{ cid: 2, pid: 12345, text: '新增评论' }] : [], references: [], media: [], local_state: 'saved', has_more_comments: false }
		else if (path.includes('/posts?')) data = { items: [{ pid: 12345, text: '本地树洞', local_state: 'saved' }], has_more: false }
		else if (path.includes('/references')) data = { root: 12345, nodes: [], edges: [] }
		else if (path.includes('/note')) data = { content: '' }
		else if (path.endsWith('/health')) data = { status: 'ok', posts: 1, comments: refreshed ? 1 : 0 }
		return new Response(JSON.stringify({ data }), { status: init?.method === 'POST' && path.endsWith('/refresh') && fullUpdates ? 202 : 200, headers: { 'Content-Type': 'application/json' } })
	})
	vi.stubGlobal('fetch', fetcher)
	return { calls, fullUpdates: () => fullUpdates }
}

function openDetail() {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
	render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/posts/12345']}><App /></MemoryRouter></QueryClientProvider>)
}

describe.each(['studio', 'github', 'classic'] as const)('saved post updates in %s', (preset) => {
	it('refreshes on entry and allows another complete update after success', async () => {
		useUIStore.getState().setLayoutPreset(preset)
		const backend = fixture()
		openDetail()
		expect(await screen.findByText('取得更新后的正文')).toBeInTheDocument()
		expect(screen.getByText('新增评论')).toBeInTheDocument()
		expect(backend.calls.some((call) => call.path.endsWith('/refresh') && call.body === '{"full":false}')).toBe(true)
		const user = userEvent.setup()
		await user.click(screen.getByRole('button', { name: '完整更新本地内容' }))
		await screen.findByText('本地更新任务已完成')
		await user.click(screen.getByRole('button', { name: '完整更新本地内容' }))
		await waitFor(() => expect(backend.fullUpdates()).toBe(2))
	})
	it('keeps the local detail readable when automatic refresh fails', async () => {
		useUIStore.getState().setLayoutPreset(preset)
		fixture({ failure: true })
		openDetail()
		expect(await screen.findByText('可离线阅读的正文')).toBeInTheDocument()
		expect(await screen.findByText(/自动刷新失败：会话失效/)).toBeInTheDocument()
	})
})
