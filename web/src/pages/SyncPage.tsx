import { FormEvent, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, KeyRound, ListRestart, Radio, RefreshCw, SearchCheck, ShieldCheck } from 'lucide-react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import type { AuthStatus, Job } from '../lib/types'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { errorDescription } from '../components/Feedback'
import { LoadingState } from '../components/States'
import { setOnlineSession } from '../features/online/session'
import { PostUpdateSettingsPanel } from '../components/PostUpdateSettingsPanel'

const syncTypes = new Set(['sync_followed', 'sync_pids', 'sync_latest', 'sync_pages', 'repair_comments', 'repair_media', 'repair_thumbnails', 'rebuild_references', 'cleanup_staging', 'fetch_images', 'save_raw_json'])

export function SyncPage() {
	const client = useQueryClient()
	const navigate = useNavigate()
	const [searchParams] = useSearchParams()
	const loginIntent = searchParams.get('intent') === 'login'
	const requestedNext = searchParams.get('next') || '/online'
	const next = requestedNext.startsWith('/') && !requestedNext.startsWith('//') ? requestedNext : '/online'
	const session = useQuery({ queryKey: ['session'], queryFn: api.session })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
	const setSession = (value: AuthStatus) => {
		client.setQueryData(['session'], value)
		setOnlineSession(client, value)
		if (loginIntent && value.can_read_online) navigate(next, { replace: true })
	}
  const probe = useMutation({ mutationFn: api.probeSession, onSuccess: setSession })
	const reload = useMutation({ mutationFn: api.reloadSession, onSuccess: setSession })
	const logout = useMutation({ mutationFn: api.logoutSession, onSuccess: setSession })
  const create = useMutation({ mutationFn: ({ type, payload }: { type: string; payload?: unknown }) => api.createJob(type, payload), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
	const autoProbeAttempted = useRef(false)
	const probeSession = probe.mutate
	const hasStoredSession = session.data?.has_session === true
	const sessionChecked = session.data?.checked === true

	useEffect(() => {
		if (!hasStoredSession || sessionChecked || autoProbeAttempted.current) return
		autoProbeAttempted.current = true
		probeSession()
	}, [hasStoredSession, probeSession, sessionChecked])

	useEffect(() => {
		if (loginIntent && session.data?.checked && session.data.can_read_online) navigate(next, { replace: true })
	}, [loginIntent, navigate, next, session.data?.can_read_online, session.data?.checked])

	if (session.isLoading) return <LoadingState label="正在打开登录…" />
	if (session.error) return <>
	<PageHeader eyebrow="在线保存" title="保存在线内容" description="把需要的在线树洞保存到本机。" />
	<FriendlyError title="暂时无法打开登录" description="请刷新页面后重试；已保存的本地资料不会受到影响。" error={session.error} />
  </>

  const status = session.data
  const online = Boolean(status?.checked && status.can_read_online)
  const syncJobs = (jobs.data ?? []).filter((job) => syncTypes.has(job.type))
  const recentJobs = syncJobs.slice(0, 3)
  return <>
	<PageHeader eyebrow={loginIntent ? '在线树洞' : '在线保存'} title={loginIntent ? '登录在线树洞' : '保存在线内容'} description={loginIntent ? '使用北大学号完成登录；成功后会直接进入在线树洞。' : '登录后选择要保存的范围；任务创建后可以离开本页。'} actions={<>{status?.has_session && <button className="button-secondary" disabled={logout.isPending} onClick={() => logout.mutate()}>{logout.isPending ? '正在退出…' : '退出会话'}</button>}<button className="button-secondary" disabled={probe.isPending} onClick={() => probe.mutate()}><SearchCheck size={16} />{probe.isPending ? '检测中…' : '重新检测'}</button></>} />
    <section className={`panel p-5 md:p-6 ${online ? 'border-teal/30' : ''}`}>
      <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex items-start gap-3"><div className={`grid size-11 shrink-0 place-items-center rounded-xl ${online ? 'bg-teal-soft text-teal' : 'bg-coral-soft text-coral'}`}>{online ? <ShieldCheck size={20} /> : <KeyRound size={20} />}</div><div><p className="font-semibold">{online ? '已经登录，可以保存在线内容' : status?.has_session ? '已找到本机登录信息' : '需要登录树洞'}</p><p className="mt-1 text-sm leading-6 text-ink-soft">{sessionDescription(status, online)}</p>{status?.challenge_reason && <details className="mt-2 text-xs text-ink-soft"><summary className="cursor-pointer text-coral">查看验证详情</summary><p className="mt-1 break-all">{status.challenge_reason}</p></details>}</div></div>
		{!online && <div className="w-full max-w-md lg:w-[420px]">
		  <LoginPanel status={status} onStatus={setSession} />
		  {!status?.challenge && <div className="mt-3 flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-xs"><button className="font-semibold text-ink-soft hover:text-ink" type="button" disabled={reload.isPending} onClick={() => reload.mutate()}>{reload.isPending ? '正在重新读取…' : '我刚在其他客户端登录，重新读取'}</button><Link className="font-semibold text-ink-soft hover:text-ink" to="/imports">只迁移已有资料 →</Link></div>}
		  {reload.error && <InlineError message="未能重新读取其他客户端的登录状态，请直接使用学号登录。" error={reload.error} />}
		</div>}
      </div>
    </section>

	{online ? <>
	<div className="mb-4 mt-7"><h2 className="text-xl font-semibold">选择要保存的内容</h2><p className="mt-1 text-sm text-ink-soft">推荐优先保存指定 PID，范围清楚，也方便后续整理和导出。</p></div>
    <section className="grid gap-5 xl:grid-cols-3">
      <PIDSyncCard disabled={!online || create.isPending} onSubmit={(pids, includeComments, includeMedia) => create.mutate({ type: 'sync_pids', payload: { pids, include_comments: includeComments, include_media: includeMedia } })} />
      <SyncCard icon={RefreshCw} title="保存我的关注" description="从你的关注列表保存帖子。首次建议只保存 1 页。" disabled={!online || create.isPending} onSubmit={(value) => create.mutate({ type: 'sync_followed', payload: { pages: value } })} />
      <SyncCard icon={Radio} title="保存最新时间线" description="保存公共最新时间线；这是可选资料来源。" disabled={!online || create.isPending} onSubmit={(value) => create.mutate({ type: 'sync_latest', payload: { start_page: 1, pages: value } })} />
    </section>
	<section className="panel mt-5 flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between"><div><h2 className="font-semibold">补全缺失图片</h2><p className="mt-1 text-sm leading-6 text-ink-soft">检查已保存的树洞，只下载本机尚未保存的图片。</p></div><button className="button-secondary shrink-0" disabled={!online || create.isPending} onClick={() => create.mutate({ type: 'repair_media', payload: {} })}>开始补全图片</button></section>
		<PostUpdateSettingsPanel />
		<AdvancedSync disabled={!online || create.isPending} onCreate={(type, payload) => create.mutate({ type, payload })} />
		{create.data && <div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-teal/25 bg-teal-soft/35 px-4 py-3 text-sm"><span className="font-medium text-teal">保存任务已创建，可以离开本页继续使用。</span><Link className="font-semibold text-teal hover:underline" to="/tasks">查看任务进度 →</Link></div>}
	</> : <section className="mt-6 rounded-2xl border border-dashed border-line bg-white/25 p-7 text-center"><KeyRound className="mx-auto text-ink-soft/45" size={25} /><h2 className="mt-3 font-semibold">登录后才能选择保存范围</h2><p className="mt-2 text-sm text-ink-soft">登录只用于读取你选择的树洞；浏览本地资料不需要登录。</p></section>}
    {create.error && <div className="mt-5"><FriendlyError title="未能创建保存任务" description="请确认登录仍然有效，然后重试。" error={create.error} /></div>}

    <section className="panel mt-7 p-5 md:p-6">
	  <div className="flex items-center justify-between gap-3"><h2 className="text-xl font-semibold">最近保存任务</h2><div className="flex items-center gap-2"><span className="badge">{syncJobs.length} 条</span><Link className="text-xs font-semibold text-teal hover:underline" to="/tasks">查看全部</Link></div></div>
      <div className="mt-5 grid gap-3">{jobs.isLoading ? <LoadingState label="正在读取保存任务…" /> : jobs.error ? <FriendlyError title="暂时无法读取保存任务" description="登录和创建新的保存任务不受影响，可以稍后重试。" error={jobs.error} /> : recentJobs.length ? recentJobs.map((job) => <SyncJob key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-8 text-center text-sm text-ink-soft">还没有保存任务。登录后从上方选择一种保存方式。</p>}</div>
    </section>
  </>
}

function AdvancedSync({ disabled, onCreate }: { disabled: boolean; onCreate: (type: string, payload: unknown) => void }) {
	const [startPage, setStartPage] = useState(1)
	const [pages, setPages] = useState(10)
	const [saveJSON, setSaveJSON] = useState(false)
	return <details className="panel mt-6 p-5">
		<summary className="cursor-pointer list-none"><div className="flex items-center justify-between gap-3"><div><h2 className="text-lg font-semibold">兼容与维护</h2><p className="mt-1 text-xs leading-5 text-ink-soft">仅用于旧版数据迁移或问题排查，日常保存无需设置。</p></div><span className="badge">按需展开</span></div></summary>
		<div className="mt-5 grid gap-5 border-t border-line pt-5 lg:grid-cols-2">
			<form className="rounded-xl border border-line bg-white/45 p-5" onSubmit={(event) => { event.preventDefault(); onCreate('sync_pages', { start_page: startPage, pages, options: { post_limit: 200, comment_limit: 200, save_json: saveJSON, fetch_images: false } }) }}>
				<h3 className="font-semibold">按旧版页码保存</h3>
				<p className="mt-2 text-sm leading-6 text-ink-soft">兼容以前按页采集的流程，从指定页开始逐页保存。</p>
				<div className="mt-4 grid grid-cols-2 gap-3"><label className="text-xs text-ink-soft">起始页<input className="field mt-1" type="number" min={1} value={startPage} onChange={(event) => setStartPage(Math.max(1, Number(event.target.value) || 1))} /></label><label className="text-xs text-ink-soft">页数<input className="field mt-1" type="number" min={1} max={50} value={pages} onChange={(event) => setPages(Math.max(1, Math.min(50, Number(event.target.value) || 1)))} /></label></div>
				<label className="mt-3 inline-flex items-center gap-2 text-xs text-ink-soft"><input type="checkbox" checked={saveJSON} onChange={(event) => setSaveJSON(event.target.checked)} />同时保留排查用原始响应</label>
				<button className="button-primary mt-4 w-full" disabled={disabled}>创建旧版页码任务</button>
			</form>
			<div className="rounded-xl border border-line bg-white/45 p-5">
				<h3 className="font-semibold">诊断数据</h3>
				<p className="mt-2 text-sm leading-6 text-ink-soft">仅在维护者要求提供原始响应时生成。</p>
				<div className="mt-4 grid gap-2"><button type="button" className="button-secondary" disabled={disabled} onClick={() => onCreate('save_raw_json', {})}>生成诊断数据文件</button><Link className="mt-1 text-center text-xs font-semibold text-teal hover:underline" to="/maintenance">其他本地维护 →</Link></div>
			</div>
		</div>
	</details>
}

function LoginPanel({ status, onStatus }: { status?: AuthStatus; onStatus: (value: AuthStatus) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const login = useMutation({ mutationFn: () => api.loginSession(username.trim(), password), onSuccess: (value) => { if (value.can_read_online || value.challenge_stage !== 'iaaa') setPassword(''); onStatus(value) } })
	const challenge = status?.challenge === 'sms' || status?.challenge === 'otp' ? status.challenge : undefined
	const stage = status?.challenge_stage ?? ''
	const verify = useMutation({ mutationFn: () => api.continueSession(stage, challenge!, username.trim(), password, code.trim()), onSuccess: (value) => { setCode(''); if (value.can_read_online) setPassword(''); onStatus(value) } })
	const resend = useMutation({ mutationFn: () => api.sendSessionSMS(stage === 'treehole' ? 'treehole' : 'iaaa', username.trim()), onSuccess: onStatus })
	const isOTP = challenge === 'otp'
	const verificationHelp = isOTP
		? '请打开“北京大学”App，进入“我的 → 手机令牌”，输入当前 6 位动态口令；此方式不会发送短信。'
		: stage === 'iaaa'
			? '统一身份认证要求短信验证；验证码通常会自动发送到绑定手机号。'
			: '树洞会话要求短信验证，请先发送树洞短信验证码。'
	if (challenge) return <form className="w-full max-w-md rounded-xl border border-line bg-white/55 p-4 lg:w-[420px]" onSubmit={(event) => { event.preventDefault(); if (code.trim()) verify.mutate() }}><p className="text-sm font-semibold">输入{isOTP ? '手机令牌（6 位动态口令）' : '短信验证码'}</p><p className="mt-1 text-xs leading-5 text-ink-soft">{verificationHelp}</p><label className="mt-3 block text-xs font-medium text-ink-soft">{isOTP ? '手机令牌' : '短信验证码'}<div className="mt-1.5 flex gap-2"><input className="field" value={code} onChange={(event) => setCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" placeholder={isOTP ? '6 位动态口令' : '短信验证码'} /><button className="button-primary shrink-0" disabled={!code.trim() || verify.isPending}>{verify.isPending ? '验证中…' : '继续登录'}</button></div></label>{challenge === 'sms' && <button className={stage === 'treehole' ? 'button-secondary mt-3' : 'mt-3 text-xs font-semibold text-teal hover:underline'} type="button" disabled={resend.isPending || (stage === 'iaaa' && !username.trim())} onClick={() => resend.mutate()}>{resend.isPending ? '正在发送…' : stage === 'treehole' ? '发送树洞短信验证码' : '没有收到？重新发送验证码'}</button>}{(verify.error || resend.error) && <InlineError message="验证没有完成，请检查验证码后重试。" error={verify.error || resend.error} />}</form>
  return <form className="grid w-full max-w-md gap-3 rounded-xl border border-line bg-white/55 p-4 lg:w-[420px]" onSubmit={(event) => { event.preventDefault(); if (username.trim() && password) login.mutate() }}><div><p className="text-sm font-semibold">使用北大学号登录</p><p className="mt-1 text-xs leading-5 text-ink-soft">密码只用于完成登录，不会由网页保存。</p></div><label className="text-xs font-medium text-ink-soft">学号<input className="field mt-1.5" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" placeholder="北大学号（无需邮箱后缀）" /></label><label className="text-xs font-medium text-ink-soft">密码<input className="field mt-1.5" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" placeholder="密码（不会由网页保存）" /></label><button className="button-primary" disabled={!username.trim() || !password || login.isPending}>{login.isPending ? '登录中…' : '登录'}</button>{login.error && <InlineError message="登录没有完成，请检查学号、密码或网络后重试。" error={login.error} />}</form>
}

function SyncCard({ icon: Icon, title, description, disabled, onSubmit }: { icon: typeof RefreshCw; title: string; description: string; disabled: boolean; onSubmit: (pages: number) => void }) {
  const [pages, setPages] = useState(1)
  return <form className="panel p-5" onSubmit={(event: FormEvent) => { event.preventDefault(); onSubmit(pages) }}><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={19} /></div><h2 className="mt-5 text-lg font-semibold">{title}</h2><p className="mt-2 min-h-12 text-sm leading-6 text-ink-soft">{description}</p><label className="mt-4 block text-xs font-medium text-ink-soft">页数（1–50）<input className="field mt-1.5" type="number" min={1} max={50} value={pages} onChange={(event) => setPages(Math.max(1, Math.min(50, Number(event.target.value) || 1)))} /></label><button className="button-primary mt-4 w-full" disabled={disabled}>创建保存任务</button></form>
}

function PIDSyncCard({ disabled, onSubmit }: { disabled: boolean; onSubmit: (pids: number[], includeComments: boolean, includeMedia: boolean) => void }) {
  const [value, setValue] = useState('')
	const [includeComments, setIncludeComments] = useState(true)
	const [includeMedia, setIncludeMedia] = useState(true)
  const pids = value.split(/[\s,，]+/).map(Number).filter((item) => Number.isInteger(item) && item > 0)
  return <form className="panel border-coral/25 p-5" onSubmit={(event) => { event.preventDefault(); if (pids.length) onSubmit([...new Set(pids)], includeComments, includeMedia) }}><div className="flex items-start justify-between gap-3"><div className="grid size-10 place-items-center rounded-xl bg-coral-soft text-coral"><ListRestart size={19} /></div><span className="badge !border-coral/25 !bg-coral-soft/45 !text-coral">推荐</span></div><h2 className="mt-5 text-lg font-semibold">保存指定 PID</h2><p className="mt-2 min-h-12 text-sm leading-6 text-ink-soft">保存帖子，并可完整读取评论和图片。输入多个 PID 时用空格或逗号分隔。</p><label className="mt-4 block text-xs font-medium text-ink-soft">PID 列表<textarea className="field mt-1.5 min-h-20 resize-y" value={value} onChange={(event) => setValue(event.target.value)} placeholder="1234567, 2345678" /></label><div className="mt-3 flex flex-wrap gap-4 text-xs text-ink-soft"><label className="inline-flex items-center gap-2"><input type="checkbox" checked={includeComments} onChange={(event) => setIncludeComments(event.target.checked)} />保存全部评论</label><label className="inline-flex items-center gap-2"><input type="checkbox" checked={includeMedia} onChange={(event) => setIncludeMedia(event.target.checked)} />下载图片到本机</label></div><button className="button-primary mt-4 w-full" disabled={disabled || pids.length === 0}>保存 {new Set(pids).size || ''} 个 PID</button></form>
}

function SyncJob({ job }: { job: Job }) {
	const client = useQueryClient()
	const action = useMutation({ mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
	const download = useMutation({ mutationFn: () => api.downloadRawJSONJob(job.id), onSuccess: ({ blob, filename }) => { const url = URL.createObjectURL(blob); const anchor = document.createElement('a'); anchor.href = url; anchor.download = filename; anchor.click(); URL.revokeObjectURL(url) } })
	return <div><JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />{action.error && <InlineError message="未能更新任务，请稍后重试。" error={action.error} />}{job.type === 'save_raw_json' && job.status === 'completed' && <div className="mt-2 flex justify-end"><button className="button-secondary" disabled={download.isPending} onClick={() => download.mutate()}><Download size={14} />下载诊断数据</button></div>}{download.error && <InlineError message="未能下载诊断数据，请重试。" error={download.error} />}</div>
}

function sessionDescription(status: AuthStatus | undefined, online: boolean) {
	if (online) return '登录状态有效。现在可以按 PID、关注列表或最新时间线保存内容。'
	if (status?.challenge) return '还需要完成一次验证码验证。'
	if (status?.has_session) return '点击“重新检测”确认登录是否仍然有效，或重新登录。'
	return '登录后可以把选择的在线内容保存到本机。'
}

function InlineError({ message, error }: { message: string; error: unknown }) {
	return <div className="mt-2 text-xs text-coral" role="alert"><p>{message}</p><details className="mt-1 text-ink-soft"><summary className="cursor-pointer">查看技术详情</summary><p className="mt-1 break-all font-mono">{errorDescription(error)}</p></details></div>
}

function FriendlyError({ title, description, error }: { title: string; description: string; error: unknown }) {
	return <section className="panel border-coral/30 bg-coral-soft/30 p-6" role="alert"><h2 className="font-semibold">{title}</h2><p className="mt-1 text-sm leading-6 text-ink-soft">{description}</p><details className="mt-4 text-xs text-ink-soft"><summary className="cursor-pointer font-medium">查看技术详情</summary><p className="mt-2 break-all font-mono">{errorDescription(error)}</p></details></section>
}
