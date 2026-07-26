import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import App from './App'
import { ONBOARDING_STORAGE_KEY } from './components/OnboardingGuide'
import { useUIStore } from './store/ui'

beforeEach(() => window.localStorage.setItem(ONBOARDING_STORAGE_KEY, 'completed'))

afterEach(() => {
	cleanup()
	vi.restoreAllMocks()
	useUIStore.getState().setLayoutPreset('studio')
	useUIStore.setState({ activeWorkspace: 'library', lastOnlineLocation: '/online', lastLibraryLocation: '/' })
	window.localStorage.clear()
})

function renderApp(path: string) {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
	return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><App /></MemoryRouter></QueryClientProvider>)
}

function json(data: unknown, status = 200) {
	return Promise.resolve(new Response(JSON.stringify({ data }), { status, headers: { 'Content-Type': 'application/json' } }))
}

function failure(code: string, message: string, status = 502, details?: unknown) {
	return Promise.resolve(new Response(JSON.stringify({ error: { code, message, details } }), { status, headers: { 'Content-Type': 'application/json' } }))
}

function stubObserverTrafficStatus(traffic: Record<string, unknown>) {
	const observerSettings = { enabled: true, base_url: 'https://observer.example.com', api_token_configured: true, request_timeout_seconds: 15, auto_sync_on_start: true, sync_interval_minutes: 5, sync_before_export: true }
	const status = { configured: true, enabled: true, connected: true, stale: false, instance_id: 'observer-traffic', api_version: 'v1', auth_state: 'authenticated', challenge_required: false, coverage_degraded: false, baseline_completed: true, queue_depth: 0, traffic }
	const localSettings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'Local', ai_base_url: 'http://127.0.0.1', ai_model: 'test', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 3, ai_api_key_configured: false, restart_required: false, ai_active_provider: 'local', ai_providers: [] }
	vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
		const path = String(input)
		if (path.endsWith('/jobs?limit=50')) return json([])
		if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 12, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, live_search: true })
		if (path.endsWith('/ai/providers') || path.endsWith('/local-tags')) return json([])
		if (path.endsWith('/settings/observer')) return json(observerSettings)
		if (path.endsWith('/observer/status')) return json(status)
		if (path.endsWith('/settings')) return json(localSettings)
		throw new Error(`unexpected request ${path}`)
	}))
}

describe('Observer UI', () => {
	it('saves write-only Observer credentials and completes an SMS challenge', async () => {
		let savedBody = ''
		let challengeBody = ''
		const observerSettings = { enabled: true, base_url: 'https://observer.example.com', api_token_configured: true, request_timeout_seconds: 15, auto_sync_on_start: true, sync_interval_minutes: 5, sync_before_export: true }
		const challenged = { configured: true, enabled: true, connected: true, stale: false, instance_id: 'observer-1', api_version: 'v1', auth_state: 'challenge_required', challenge_required: true, challenge: 'sms', challenge_stage: 'iaaa', masked_target: '138****0000', last_successful_scan_at: '2026-07-16T08:00:00Z', coverage_degraded: false, baseline_completed: true, queue_depth: 2 }
		const localSettings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'Local', ai_base_url: 'http://127.0.0.1', ai_model: 'test', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 3, ai_api_key_configured: false, restart_required: false, ai_active_provider: 'local', ai_providers: [] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 12, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, live_search: true })
			if (path.endsWith('/ai/providers')) return json([])
			if (path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings/observer') && init?.method === 'PUT') { savedBody = String(init.body); return json(observerSettings) }
			if (path.endsWith('/settings/observer')) return json(observerSettings)
			if (path.endsWith('/observer/status')) return json(challenged)
			if (path.endsWith('/observer/auth/challenge/submit') && init?.method === 'POST') { challengeBody = String(init.body); return json({ ...challenged, auth_state: 'authenticated', challenge_required: false }) }
			if (path.endsWith('/settings')) return json(localSettings)
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: '自建热点与删除追踪' })).toBeInTheDocument()
		const token = await screen.findByLabelText('API Token')
		expect(token).toHaveAttribute('placeholder', '已配置；留空保留现有 Token')
		await user.type(token, 'rotated-secret')
		await user.click(screen.getByRole('button', { name: '保存 Observer 设置' }))
		await waitFor(() => expect(JSON.parse(savedBody)).toMatchObject({ enabled: true, base_url: 'https://observer.example.com', api_token: 'rotated-secret', auto_sync_on_start: true, sync_before_export: true }))
		const code = await screen.findByLabelText('Observer 短信验证码')
		await user.type(code, '123456')
		await user.click(screen.getByRole('button', { name: '提交验证码' }))
		await waitFor(() => expect(JSON.parse(challengeBody)).toEqual({ code: '123456' }))
		expect(await screen.findByText('Observer 已恢复登录')).toBeInTheDocument()
	})

	it('shows the Observer release and commit after a successful connection test', async () => {
		const observerSettings = { enabled: true, base_url: 'https://observer.example.com', api_token_configured: true, request_timeout_seconds: 15, auto_sync_on_start: true, sync_interval_minutes: 5, sync_before_export: true }
		const status = { configured: true, enabled: true, connected: true, stale: false, instance_id: 'observer-build', api_version: 'v1', auth_state: 'authenticated', challenge_required: false, coverage_degraded: false, baseline_completed: true, queue_depth: 0 }
		const localSettings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'Local', ai_base_url: 'http://127.0.0.1', ai_model: 'test', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 3, ai_api_key_configured: false, restart_required: false, ai_active_provider: 'local', ai_providers: [] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 12, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, live_search: true })
			if (path.endsWith('/ai/providers') || path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings/observer/test')) return json({ ok: true, instance_id: 'observer-build', api_version: 'v1', service_version: 'v0.1.0-alpha.1', commit: '0123456789abcdef', build_date: '2026-07-17T04:00:00Z', auth_state: 'authenticated', message: 'Observer connection succeeded' })
			if (path.endsWith('/settings/observer')) return json(observerSettings)
			if (path.endsWith('/observer/status')) return json(status)
			if (path.endsWith('/settings')) return json(localSettings)
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/settings')
		await user.click(await screen.findByRole('button', { name: '测试已保存连接' }))
		expect(await screen.findByText('v0.1.0-alpha.1 · 0123456789ab · observer-build · v1')).toBeInTheDocument()
	})

	it('labels a Treehole OTP challenge accurately and does not offer SMS resend', async () => {
		const observerSettings = { enabled: true, base_url: 'https://observer.example.com', api_token_configured: true, request_timeout_seconds: 15, auto_sync_on_start: true, sync_interval_minutes: 5, sync_before_export: true }
		const challenged = { configured: true, enabled: true, connected: true, stale: false, instance_id: 'observer-1', api_version: 'v1', auth_state: 'challenge_required', challenge_required: true, challenge: 'otp', challenge_stage: 'treehole', auth_reason: '需要令牌验证', coverage_degraded: false, baseline_completed: true, queue_depth: 0 }
		const localSettings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'Local', ai_base_url: 'http://127.0.0.1', ai_model: 'test', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 3, ai_api_key_configured: false, restart_required: false, ai_active_provider: 'local', ai_providers: [] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 12, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, live_search: true })
			if (path.endsWith('/ai/providers')) return json([])
			if (path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings/observer')) return json(observerSettings)
			if (path.endsWith('/observer/status')) return json(challenged)
			if (path.endsWith('/settings')) return json(localSettings)
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: '树洞二次认证需要动态口令' })).toBeInTheDocument()
		expect(screen.getByLabelText('Observer 动态口令')).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: '重新发送' })).not.toBeInTheDocument()
		expect(screen.queryByText('全局流量保护')).not.toBeInTheDocument()
	})

	it('shows an optional Observer-wide circuit breaker without confusing it with login failure', async () => {
		const observerSettings = { enabled: true, base_url: 'https://observer.example.com', api_token_configured: true, request_timeout_seconds: 15, auto_sync_on_start: true, sync_interval_minutes: 5, sync_before_export: true }
		const status = { configured: true, enabled: true, connected: true, stale: false, instance_id: 'observer-traffic', api_version: 'v1', auth_state: 'authenticated', challenge_required: false, coverage_degraded: false, baseline_completed: true, queue_depth: 4, traffic: { state: 'circuit_open', blocked_until: '2026-07-16T10:05:00Z', reason: 'consecutive_upstream_5xx', consecutive_rate_limits: 3, consecutive_service_failures: 5 } }
		const localSettings = { database_type: 'sqlite3', database_file: './treehole.db', ai_enabled: false, ai_live_search: false, ai_provider_name: 'Local', ai_base_url: 'http://127.0.0.1', ai_model: 'test', ai_temperature: 0.2, ai_max_output_tokens: 4096, ai_request_timeout_seconds: 120, ai_max_search_rounds: 3, ai_api_key_configured: false, restart_required: false, ai_active_provider: 'local', ai_providers: [] }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 12, fts5: true, archive_import: true, archive_export: true, jobs: true, ai: true, live_search: true })
			if (path.endsWith('/ai/providers') || path.endsWith('/local-tags')) return json([])
			if (path.endsWith('/settings/observer')) return json(observerSettings)
			if (path.endsWith('/observer/status')) return json(status)
			if (path.endsWith('/settings')) return json(localSettings)
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: 'Observer 已开启全局熔断' })).toBeInTheDocument()
		expect(screen.getByText('全局流量保护')).toBeInTheDocument()
		expect(screen.getByText('熔断已开启')).toBeInTheDocument()
		expect(screen.getByText('上游连续返回服务错误')).toBeInTheDocument()
		expect(screen.getByText('3 次')).toBeInTheDocument()
		expect(screen.getByText('5 次')).toBeInTheDocument()
		expect(screen.getByText(/保护窗口结束后自动尝试恢复/)).toBeInTheDocument()
		expect(screen.getByText('登录有效')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: '重新登录' })).toBeDisabled()
	})

	it('keeps normal traffic non-blocking and leaves manual login available', async () => {
		stubObserverTrafficStatus({ state: 'normal', consecutive_rate_limits: 0, consecutive_service_failures: 0 })
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: 'Observer 正常运行' })).toBeInTheDocument()
		const trafficRow = screen.getByText('全局流量保护').closest('div')
		expect(trafficRow).not.toBeNull()
		expect(within(trafficRow!).getByText('正常')).toBeInTheDocument()
		expect(screen.queryByText('全局流量保护详情')).not.toBeInTheDocument()
		expect(screen.getByRole('button', { name: '重新登录' })).toBeEnabled()
	})

	it('shows global backoff and disables manual login until recovery', async () => {
		stubObserverTrafficStatus({ state: 'backoff', blocked_until: '2026-07-16T10:05:00Z', reason: 'upstream_rate_limited', consecutive_rate_limits: 2, consecutive_service_failures: 0 })
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: 'Observer 正在全局退避' })).toBeInTheDocument()
		expect(screen.getByText('全局退避中')).toBeInTheDocument()
		expect(screen.getByText('全局退避详情')).toBeInTheDocument()
		expect(screen.getByText('上游要求限流退避')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: '重新登录' })).toBeDisabled()
	})

	it('conservatively blocks unknown non-normal traffic states', async () => {
		stubObserverTrafficStatus({ state: 'adaptive_pause', reason: 'future Observer guard', consecutive_rate_limits: 0, consecutive_service_failures: 1 })
		renderApp('/settings')
		expect(await screen.findByRole('heading', { name: 'Observer 全局流量保护已阻断请求' })).toBeInTheDocument()
		expect(screen.getByText('未知状态（adaptive_pause）')).toBeInTheDocument()
		expect(screen.getByText('全局流量保护详情')).toBeInTheDocument()
		expect(screen.getByText('future Observer guard')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: '重新登录' })).toBeDisabled()
	})

	it('keeps locally synced removed posts readable while Observer is offline', async () => {
		const availability = { pid: 8123456, state: 'confirmed_unavailable', observed_at: '2026-07-16T08:10:00Z', first_unavailable_at: '2026-07-16T08:00:00Z', last_unavailable_at: '2026-07-16T08:10:00Z', observer_id: 'observer-1', completeness: 'partial' }
		const post = { pid: 8123456, text: '删除前正文', timestamp: 1_784_100_000, reply: 1, praise_num: 12 }
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.endsWith('/observer/status')) return failure('observer_unavailable', '服务器暂时无法连接')
			if (path.includes('/removed/8123456')) return json({ availability, post, comments: [{ cid: 7, pid: 8123456, text: '已经保存的评论', name_tag: 'Alice', timestamp: 1_784_100_100 }], media: [] })
			if (path.includes('/removed?')) return json({ items: [{ ...availability, post }], has_more: false })
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/removed')
		expect(await screen.findByText('Observer 当前不可达')).toBeInTheDocument()
		expect(screen.getByText('删除前正文')).toBeInTheDocument()
		expect(screen.getByText('部分归档')).toBeInTheDocument()
		await user.click(screen.getByRole('link', { name: '查看删除归档 #8123456' }))
		expect(await screen.findByText('已经保存的评论')).toBeInTheDocument()
		expect(screen.getByText('删除前只来得及保存部分内容')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: '返回删除归档' })).toBeInTheDocument()
	})

	it('requires an explicit choice before exporting a stale local snapshot', async () => {
		const createBodies: string[] = []
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.includes('/imports?limit=50')) return json([])
			if (path.endsWith('/exports/preview') && init?.method === 'POST') return json({ format: 'treehole-v2', posts: 1, comments: 2, media: 0, missing_media: 0 })
			if (path.endsWith('/exports/jobs') && init?.method === 'POST') {
				createBodies.push(String(init.body))
				if (createBodies.length === 1) return failure('observer_sync_failed', '无法连接 Observer', 502, { can_export_local_snapshot: true })
				return json({ id: 'export-1', type: 'export_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-07-16T08:00:00Z', updated_at: '2026-07-16T08:00:00Z' }, 202)
			}
			if (path.endsWith('/exports/jobs')) return json([])
			throw new Error(`unexpected request ${path}`)
		}))
		const user = userEvent.setup()
		renderApp('/imports?view=export')
		await user.click(await screen.findByRole('button', { name: '创建 archive v2 任务' }))
		expect(await screen.findByText('导出前 Observer 同步失败')).toBeInTheDocument()
		expect(JSON.parse(createBodies[0])).not.toHaveProperty('sync_observer_before_export')
		await user.click(screen.getByRole('button', { name: '使用当前本地数据继续导出' }))
		await waitFor(() => expect(createBodies).toHaveLength(2))
		expect(JSON.parse(createBodies[1])).toMatchObject({ format: 'treehole-v2', sync_observer_before_export: false })
		expect(await screen.findByText('导出任务已创建，完成后可在下方下载。')).toBeInTheDocument()
	})

	it('does not invent content for a legacy availability record without a post', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const path = String(input)
			if (path.endsWith('/jobs?limit=50')) return json([])
			if (path.endsWith('/removed/99')) return json({ availability: { pid: 99, state: 'confirmed_unavailable', observed_at: '2026-07-16T08:00:00Z', observer_id: 'legacy', completeness: 'partial' }, comments: [], media: [] })
			throw new Error(`unexpected request ${path}`)
		}))
		renderApp('/removed/99')
		expect(await screen.findByRole('heading', { name: '这条记录没有可展示的正文' })).toBeInTheDocument()
		expect(screen.getByText(/不会用 PID 占位内容冒充/)).toBeInTheDocument()
	})
})
