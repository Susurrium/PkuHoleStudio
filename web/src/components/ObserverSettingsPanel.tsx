import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, AlertTriangle, Cloud, KeyRound, MessageSquareText, RefreshCw, Save, ShieldAlert, ShieldCheck, Wifi, WifiOff } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { ObserverSettingsUpdate, ObserverStatus, ObserverTrafficStatus } from '../lib/types'
import { errorDescription, useFeedback } from './Feedback'

const defaultSettings: ObserverSettingsUpdate = {
	enabled: false,
	base_url: '',
	request_timeout_seconds: 15,
	auto_sync_on_start: true,
	sync_interval_minutes: 5,
	sync_before_export: true,
}

export function ObserverSettingsPanel() {
	const client = useQueryClient()
	const { notify } = useFeedback()
	const settings = useQuery({ queryKey: ['observer-settings'], queryFn: api.observerSettings, retry: false })
	const status = useQuery({
		queryKey: ['observer-status'],
		queryFn: api.observerStatus,
		enabled: settings.data?.enabled === true,
		retry: false,
		refetchInterval: (query) => query.state.data?.challenge_required ? 5_000 : 15_000,
	})
	const [draft, setDraft] = useState<ObserverSettingsUpdate>(defaultSettings)
	const [apiToken, setAPIToken] = useState('')
	const [code, setCode] = useState('')

	useEffect(() => {
		if (!settings.data) return
		setDraft({
			enabled: settings.data.enabled,
			base_url: settings.data.base_url,
			request_timeout_seconds: settings.data.request_timeout_seconds,
			auto_sync_on_start: settings.data.auto_sync_on_start,
			sync_interval_minutes: settings.data.sync_interval_minutes,
			sync_before_export: settings.data.sync_before_export,
		})
	}, [settings.data])

	const refreshStatus = () => client.invalidateQueries({ queryKey: ['observer-status'] })
	const save = useMutation({
		mutationFn: () => api.updateObserverSettings({ ...draft, api_token: apiToken.trim() || undefined }),
		onSuccess: (saved) => {
			setAPIToken('')
			client.setQueryData(['observer-settings'], saved)
			void refreshStatus()
			notify({ title: 'Observer 设置已保存', description: saved.enabled ? 'Studio 将通过本机后端连接你的 Observer。' : 'Observer 集成当前已停用。' })
		},
		onError: (error) => notify({ tone: 'error', title: 'Observer 设置保存失败', description: errorDescription(error) }),
	})
	const test = useMutation({
		mutationFn: api.testObserverConnection,
		onSuccess: (result) => {
			void refreshStatus()
			notify({ tone: result.ok ? 'success' : 'error', title: result.ok ? 'Observer 连接成功' : 'Observer 连接失败', description: result.message || [result.instance_id, result.api_version].filter(Boolean).join(' · ') })
		},
		onError: (error) => notify({ tone: 'error', title: 'Observer 连接测试失败', description: errorDescription(error) }),
	})
	const sync = useMutation({
		mutationFn: api.syncObserver,
		onSuccess: (result) => {
			void Promise.all([refreshStatus(), client.invalidateQueries({ queryKey: ['removed-posts'] })])
			notify({ title: 'Observer 同步完成', description: `收到 ${result.events_received} 个事件，应用 ${result.events_applied} 个，导入 ${result.snapshots_imported} 份快照。` })
		},
		onError: (error) => notify({ tone: 'error', title: 'Observer 同步失败', description: errorDescription(error) }),
	})
	const submit = useMutation({
		mutationFn: () => api.submitObserverChallenge(code.trim()),
		onSuccess: (next) => {
			setCode('')
			client.setQueryData(['observer-status'], next)
			notify({ title: next.challenge_required ? '验证码已提交' : 'Observer 已恢复登录', description: next.challenge_required ? 'Observer 仍在等待认证，请检查验证码是否正确或是否已过期。' : '服务器扫描会自动继续。' })
		},
		onError: (error) => notify({ tone: 'error', title: '验证码提交失败', description: errorDescription(error) }),
	})
	const resend = useMutation({
		mutationFn: api.resendObserverChallenge,
		onSuccess: (next) => { client.setQueryData(['observer-status'], next); notify({ title: '已请求重新发送验证码', description: '请查看绑定手机；服务端会限制重复发送频率。' }) },
		onError: (error) => notify({ tone: 'error', title: '未能重新发送验证码', description: errorDescription(error) }),
	})
	const retryLogin = useMutation({
		mutationFn: api.retryObserverLogin,
		onSuccess: (next) => { client.setQueryData(['observer-status'], next); notify({ title: 'Observer 已重新发起登录', description: next.challenge_required ? '上游要求完成一次验证。' : '登录流程正在继续。' }) },
		onError: (error) => notify({ tone: 'error', title: '未能重新登录', description: errorDescription(error) }),
	})

	return <section id="observer-service" className="panel scroll-mt-32 p-5 lg:col-span-2 md:p-6">
		<div className="flex flex-wrap items-start justify-between gap-4">
			<div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><Cloud size={21} /></div><div><p className="eyebrow">SELF-HOSTED OBSERVER</p><h2 className="mt-1 text-lg font-semibold">自建热点与删除追踪</h2><p className="mt-1 max-w-2xl text-xs leading-5 text-ink-soft">Studio 只通过本机后端访问你的 Observer；API Token 不会交给浏览器直连，也不会从设置接口回显。</p></div></div>
			{settings.data && <span className={`badge gap-1 ${settings.data.enabled ? '!border-teal/30 !text-teal' : ''}`}>{settings.data.enabled ? <><ShieldCheck size={12} />已启用</> : <><WifiOff size={12} />已停用</>}</span>}
		</div>

		{settings.isLoading ? <div className="mt-5 animate-pulse rounded-xl border border-line bg-paper/45 p-5 text-sm text-ink-soft">正在读取 Observer 设置…</div> : settings.error ? <div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="alert"><div><p className="font-semibold text-coral">Observer 设置接口不可用</p><p className="mt-1 text-xs text-ink-soft">{errorDescription(settings.error)}</p></div><button className="button-secondary" onClick={() => settings.refetch()}><RefreshCw size={14} />重试</button></div> : <>
			<div className="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1.25fr)_minmax(300px,.75fr)]">
				<form className="rounded-xl border border-line bg-white/40 p-4" onSubmit={(event) => { event.preventDefault(); save.mutate() }}>
					<Toggle checked={draft.enabled} title="启用自建 Observer" description="热点、删除记录和服务器同步将优先使用这个地址。" onChange={(enabled) => setDraft((current) => ({ ...current, enabled }))} />
					<label className="mt-4 block text-xs font-medium text-ink-soft">Observer HTTPS 地址<input className="field mt-1.5" type="url" required={draft.enabled} placeholder="https://observer.example.com" value={draft.base_url} onChange={(event) => setDraft((current) => ({ ...current, base_url: event.target.value }))} /></label>
					<label className="mt-4 block text-xs font-medium text-ink-soft">API Token<input className="field mt-1.5" type="password" autoComplete="new-password" value={apiToken} placeholder={settings.data?.api_token_configured ? '已配置；留空保留现有 Token' : '输入 Observer API Token'} onChange={(event) => setAPIToken(event.target.value)} /></label>
					<div className="mt-4 grid gap-3 sm:grid-cols-2"><label className="text-xs font-medium text-ink-soft">请求超时（秒）<input className="field mt-1.5" type="number" min="1" max="120" value={draft.request_timeout_seconds} onChange={(event) => setDraft((current) => ({ ...current, request_timeout_seconds: Number(event.target.value) }))} /></label><label className="text-xs font-medium text-ink-soft">自动同步间隔（分钟）<input className="field mt-1.5" type="number" min="1" max="1440" value={draft.sync_interval_minutes} onChange={(event) => setDraft((current) => ({ ...current, sync_interval_minutes: Number(event.target.value) }))} /></label></div>
					<div className="mt-4 grid gap-2 sm:grid-cols-2"><Toggle compact checked={draft.auto_sync_on_start} title="启动后自动同步" description="打开 Studio 时拉取新事件。" onChange={(auto_sync_on_start) => setDraft((current) => ({ ...current, auto_sync_on_start }))} /><Toggle compact checked={draft.sync_before_export} title="导出前先同步" description="确保删除快照进入本地导出。" onChange={(sync_before_export) => setDraft((current) => ({ ...current, sync_before_export }))} /></div>
					<div className="mt-5 flex flex-wrap gap-2"><button className="button-primary" disabled={save.isPending || (draft.enabled && (!draft.base_url.trim() || (!settings.data?.api_token_configured && !apiToken.trim())))}><Save size={15} />{save.isPending ? '正在保存…' : '保存 Observer 设置'}</button><button type="button" className="button-secondary" disabled={test.isPending || !settings.data?.enabled || !settings.data.api_token_configured} onClick={() => test.mutate()}><Activity size={15} />{test.isPending ? '正在测试…' : '测试已保存连接'}</button></div>
				</form>
				<ObserverRuntimeStatus status={status.data} loading={status.isLoading || status.isFetching} error={status.error} refresh={() => status.refetch()} sync={() => sync.mutate()} syncing={sync.isPending} retryLogin={() => retryLogin.mutate()} retrying={retryLogin.isPending} />
			</div>
			{status.data?.challenge_required && <ObserverChallengeForm status={status.data} code={code} setCode={setCode} submit={() => submit.mutate()} submitting={submit.isPending} resend={() => resend.mutate()} resending={resend.isPending} />}
		</>}
	</section>
}

function ObserverChallengeForm({ status, code, setCode, submit, submitting, resend, resending }: { status: ObserverStatus; code: string; setCode: (value: string) => void; submit: () => void; submitting: boolean; resend: () => void; resending: boolean }) {
	const kind = status.challenge === 'otp' ? 'otp' : 'sms'
	const stage = status.challenge_stage === 'treehole' ? '树洞二次认证' : 'IAAA'
	const label = kind === 'otp' ? '动态口令' : '短信验证码'
	return <form className="mt-4 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" onSubmit={(event) => { event.preventDefault(); if (code.trim()) submit() }}>
		<div className="flex items-start gap-3">{kind === 'sms' ? <MessageSquareText className="mt-0.5 shrink-0 text-coral" size={20} /> : <KeyRound className="mt-0.5 shrink-0 text-coral" size={20} />}<div><h3 className="font-semibold">{stage}需要{label}</h3><p className="mt-1 text-xs leading-5 text-ink-soft">Observer 已暂停新的上游扫描，但本地删除记录仍可浏览。输入本次{label}后，服务器会自动恢复。{status.masked_target ? ` 接收号码：${status.masked_target}` : ''}</p>{status.auth_reason && <p className="mt-1 text-xs text-ink-soft">{status.auth_reason}</p>}</div></div>
		<div className="mt-4 flex flex-col gap-2 sm:flex-row"><input className="field" inputMode="numeric" autoComplete="one-time-code" aria-label={`Observer ${label}`} placeholder={`输入${label}`} value={code} onChange={(event) => setCode(event.target.value)} /><button className="button-primary shrink-0" disabled={!code.trim() || submitting}><KeyRound size={15} />{submitting ? '正在验证…' : '提交验证码'}</button>{kind === 'sms' && <button type="button" className="button-secondary shrink-0" disabled={resending} onClick={resend}>{resending ? '正在请求…' : '重新发送'}</button>}</div>
	</form>
}

function ObserverRuntimeStatus({ status, loading, error, refresh, sync, syncing, retryLogin, retrying }: { status?: ObserverStatus; loading: boolean; error: unknown; refresh: () => unknown; sync: () => void; syncing: boolean; retryLogin: () => void; retrying: boolean }) {
	if (error) return <div className="rounded-xl border border-coral/30 bg-coral-soft/25 p-4"><div className="flex items-center gap-2 text-coral"><WifiOff size={17} /><h3 className="font-semibold">Observer 当前不可达</h3></div><p className="mt-2 text-xs leading-5 text-ink-soft">{errorDescription(error)}。已同步到本机的删除归档仍可使用。</p><button className="button-secondary mt-4" onClick={refresh}><RefreshCw size={14} />重新检测</button></div>
	if (!status) return <div className="rounded-xl border border-line bg-paper/45 p-4 text-sm text-ink-soft">{loading ? '正在读取 Observer 状态…' : '启用并保存 Observer 后显示运行状态。'}</div>
	const trafficBlocked = Boolean(status.traffic?.state && status.traffic.state !== 'normal')
	const healthy = status.connected && status.auth_state === 'authenticated' && status.baseline_completed && !status.stale && !status.coverage_degraded && !trafficBlocked
	return <div className="rounded-xl border border-line bg-paper/45 p-4" aria-live="polite">
		<div className="flex items-start justify-between gap-3"><div className="flex items-center gap-2">{healthy ? <Wifi className="text-teal" size={18} /> : <AlertTriangle className="text-coral" size={18} />}<div><h3 className="font-semibold">{healthy ? 'Observer 正常运行' : observerStatusTitle(status)}</h3><p className="mt-0.5 text-xs text-ink-soft">{observerAuthLabel(status.auth_state)}</p></div></div>{loading && <RefreshCw className="animate-spin text-ink-soft" size={14} aria-label="正在刷新" />}</div>
		<dl className="mt-4 grid gap-2 text-xs"><StatusRow label="服务实例" value={status.instance_id || '未知'} /><StatusRow label="最近成功扫描" value={formatDate(status.last_successful_scan_at)} /><StatusRow label="待处理队列" value={`${status.queue_depth ?? 0} 项`} /><StatusRow label="安全基线" value={status.baseline_completed ? '已完成' : '建立中（暂不确认删除）'} /><StatusRow label="扫描覆盖" value={status.coverage_degraded ? '覆盖不足，暂停删除确认' : '正常'} />{status.traffic && <StatusRow label="全局流量保护" value={observerTrafficLabel(status.traffic.state)} />}</dl>
		{status.last_error && <p className="mt-3 rounded-lg border border-coral/20 bg-coral-soft/20 px-3 py-2 text-xs leading-5 text-coral">{status.last_error}</p>}
		{status.auth_warning && <p className="mt-3 rounded-lg border border-teal/25 bg-teal-soft/20 px-3 py-2 text-xs leading-5 text-ink-soft">{status.auth_warning}。当前会话仍可用，但服务重启后可能需要重新登录。</p>}
		{trafficBlocked && status.traffic && <ObserverTrafficGuard status={status.traffic} />}
		<div className="mt-4 flex flex-wrap gap-2"><button className="button-secondary !min-h-9 text-xs" disabled={syncing || !status.connected} onClick={sync}><RefreshCw size={13} />{syncing ? '正在同步…' : '立即同步'}</button><button className="button-secondary !min-h-9 text-xs" disabled={retrying || trafficBlocked} onClick={retryLogin}>{retrying ? '正在登录…' : '重新登录'}</button><button className="button-secondary !min-h-9 !px-3 text-xs" onClick={refresh}>刷新状态</button></div>
	</div>
}

function Toggle({ checked, title, description, onChange, compact = false }: { checked: boolean; title: string; description: string; onChange: (checked: boolean) => void; compact?: boolean }) {
	return <label className={`flex cursor-pointer items-start gap-3 rounded-xl border ${compact ? 'p-3' : 'p-4'} transition ${checked ? 'border-teal/40 bg-teal-soft/25' : 'border-line bg-white/40'}`}><input className="mt-1" type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} /><span><span className="block text-sm font-semibold">{title}</span><span className="mt-1 block text-xs leading-5 text-ink-soft">{description}</span></span></label>
}

function StatusRow({ label, value }: { label: string; value: string }) {
	return <div className="flex items-start justify-between gap-3"><dt className="text-ink-soft">{label}</dt><dd className="min-w-0 break-all text-right font-medium">{value}</dd></div>
}

function ObserverTrafficGuard({ status }: { status: ObserverTrafficStatus }) {
	const circuitOpen = status.state === 'circuit_open'
	return <div className="mt-3 rounded-lg border border-coral/25 bg-coral-soft/25 p-3 text-xs" role="status"><div className="flex items-center gap-2 text-coral"><ShieldAlert size={15} /><p className="font-semibold">{circuitOpen ? '全局流量熔断详情' : status.state === 'backoff' ? '全局退避详情' : '全局流量保护详情'}</p></div><dl className="mt-2 grid gap-1.5 text-ink-soft">{status.blocked_until && <StatusRow label="预计恢复" value={formatDate(status.blocked_until)} />}{status.reason && <StatusRow label="触发原因" value={observerTrafficReason(status.reason)} />}<StatusRow label="连续限流" value={`${status.consecutive_rate_limits ?? 0} 次`} /><StatusRow label="连续服务故障" value={`${status.consecutive_service_failures ?? 0} 次`} /></dl><p className="mt-2 leading-5 text-ink-soft">Observer 会在保护窗口结束后自动尝试恢复；无需反复点击重新登录。</p></div>
}

function observerTrafficLabel(state?: string) {
	if (state === 'normal') return '正常'
	if (state === 'backoff') return '全局退避中'
	if (state === 'circuit_open') return '熔断已开启'
	return state ? `未知状态（${state}）` : '状态未知'
}

function observerTrafficReason(reason: string) {
	return ({
		upstream_rate_limited: '上游要求限流退避',
		upstream_rate_limited_without_retry_after: '上游限流且未提供恢复时间',
		upstream_503_retry_after: '上游服务暂不可用并要求稍后重试',
		consecutive_upstream_5xx: '上游连续返回服务错误',
		upstream_unexpected_html: '上游返回了网页验证内容而不是 API 数据',
		upstream_invalid_json: '上游连续返回无法解析的 API 数据',
		upstream_transport_error: '上游连续发生网络连接或超时错误',
		treehole_sso_missing_token: '树洞登录响应缺少会话令牌',
	} as Record<string, string>)[reason] || reason
}

function observerAuthLabel(state?: string) {
	return ({ authenticated: '登录有效', reauthenticating: '正在自动重新登录', network_backoff: '网络退避中', challenge_required: '等待上游验证', credentials_invalid: '账号或密码无效', configuration_error: '服务器认证配置异常', starting: '正在启动', stopped: '服务已停止' } as Record<string, string>)[state || ''] || state || '认证状态未知'
}

function observerStatusTitle(status: ObserverStatus) {
	if (!status.connected) return 'Observer 当前不可达'
	if (status.challenge_required) return status.challenge === 'otp' ? '等待动态口令' : '等待短信验证'
	if (status.auth_state === 'configuration_error') return 'Observer 认证配置异常'
	if (status.traffic?.state === 'circuit_open') return 'Observer 已开启全局熔断'
	if (status.traffic?.state === 'backoff') return 'Observer 正在全局退避'
	if (status.traffic?.state && status.traffic.state !== 'normal') return 'Observer 全局流量保护已阻断请求'
	if (!status.baseline_completed) return '正在建立删除确认基线'
	if (status.stale) return '扫描数据已经过期'
	if (status.coverage_degraded) return '扫描覆盖不足'
	return 'Observer 需要关注'
}

function formatDate(value?: string) {
	if (!value) return '尚无成功记录'
	const date = new Date(value)
	return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
