import { FormEvent, useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, KeyRound, ListRestart, Radio, RefreshCw, SearchCheck, ShieldCheck } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import type { AuthStatus, Job } from '../lib/types'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { setOnlineSession } from '../features/online/session'

const syncTypes = new Set(['sync_followed', 'sync_pids', 'sync_latest', 'sync_pages', 'monitor_latest', 'repair_comments', 'repair_media', 'repair_thumbnails', 'rebuild_references', 'cleanup_staging', 'fetch_images', 'save_raw_json'])

export function SyncPage() {
  const client = useQueryClient()
  const [showLogin, setShowLogin] = useState(false)
  const session = useQuery({ queryKey: ['session'], queryFn: api.session })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
  const setSession = (value: AuthStatus) => {
    client.setQueryData(['session'], value)
    setOnlineSession(client, value)
  }
  const probe = useMutation({ mutationFn: api.probeSession, onSuccess: setSession })
	const reload = useMutation({ mutationFn: api.reloadSession, onSuccess: setSession })
	const logout = useMutation({ mutationFn: api.logoutSession, onSuccess: (value) => { setSession(value); setShowLogin(false) } })
  const create = useMutation({ mutationFn: ({ type, payload }: { type: string; payload?: unknown }) => api.createJob(type, payload), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })

  useEffect(() => {
    if (session.data?.has_session && !session.data.checked && !probe.isPending) probe.mutate()
  }, [session.data?.has_session, session.data?.checked])

  if (session.isLoading || jobs.isLoading) return <LoadingState label="正在打开同步中心…" />
  if (session.error || jobs.error) return <ErrorState error={session.error || jobs.error} />

  const status = session.data
  const online = Boolean(status?.checked && status.can_read_online)
  const syncJobs = (jobs.data ?? []).filter((job) => syncTypes.has(job.type))
  const recentJobs = syncJobs.slice(0, 3)
  return <>
	<PageHeader eyebrow="SAVE ONLINE" title="保存在线内容" description="登录后选择明确范围，将在线树洞保存进本地资料库；任务会在后台继续运行。" actions={<>{status?.has_session && <button className="button-secondary" disabled={logout.isPending} onClick={() => logout.mutate()}>{logout.isPending ? '正在退出…' : '退出会话'}</button>}<button className="button-secondary" disabled={probe.isPending} onClick={() => probe.mutate()}><SearchCheck size={16} />{probe.isPending ? '检测中…' : '重新检测'}</button></>} />
    <section className={`panel p-5 md:p-6 ${online ? 'border-teal/30' : ''}`}>
      <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex items-start gap-3"><div className={`grid size-11 shrink-0 place-items-center rounded-xl ${online ? 'bg-teal-soft text-teal' : 'bg-coral-soft text-coral'}`}>{online ? <ShieldCheck size={20} /> : <KeyRound size={20} />}</div><div><p className="font-semibold">{online ? '在线读取已就绪' : status?.has_session ? '发现本机凭据，尚未验证' : '需要登录树洞'}</p><p className="mt-1 text-sm leading-6 text-ink-soft">{status?.message || '点击检测登录状态，或在本页完成登录。'}</p>{status?.challenge_reason && <p className="mt-2 text-xs text-coral">{status.challenge_reason}</p>}</div></div>
        {!online && <div className="w-full max-w-md lg:w-[420px]">
		  {!showLogin && !status?.challenge && <div className="rounded-xl border border-line bg-white/55 p-4"><p className="text-sm font-semibold">选择登录方式</p><p className="mt-1 text-xs leading-5 text-ink-soft">Web 与 TUI 共用本机会话；如果以前登录过，优先直接载入。</p><button className="button-primary mt-3 w-full" type="button" disabled={reload.isPending} onClick={() => reload.mutate()}>{reload.isPending ? '正在载入…' : '载入 TUI 已登录会话'}</button><button className="button-secondary mt-2 w-full" type="button" onClick={() => setShowLogin(true)}>使用学号在这里登录</button>{reload.error && <p className="mt-2 text-xs text-coral">{reload.error.message}</p>}<Link className="mt-3 block w-full text-center text-xs font-semibold text-ink-soft hover:text-ink" to="/imports">只迁移已有资料，不需要登录 →</Link></div>}
		  {(showLogin || Boolean(status?.challenge)) && <><LoginPanel status={status} onStatus={setSession} />{!status?.challenge && <button className="mt-2 w-full text-xs text-ink-soft hover:text-ink" type="button" onClick={() => setShowLogin(false)}>返回登录方式</button>}</>}
        </div>}
      </div>
    </section>

	{online ? <>
	<div className="mb-4 mt-7"><p className="eyebrow">CHOOSE CONTENT</p><h2 className="mt-1 text-xl font-semibold">选择要保存的内容</h2><p className="mt-1 text-sm text-ink-soft">推荐优先保存指定 PID，范围最明确，也最适合后续整理和导出。</p></div>
    <section className="grid gap-5 xl:grid-cols-3">
      <PIDSyncCard disabled={!online || create.isPending} onSubmit={(pids, includeComments, includeMedia) => create.mutate({ type: 'sync_pids', payload: { pids, include_comments: includeComments, include_media: includeMedia } })} />
      <SyncCard icon={RefreshCw} title="保存我的关注" description="从你的关注列表保存帖子。首次建议只保存 1 页。" disabled={!online || create.isPending} onSubmit={(value) => create.mutate({ type: 'sync_followed', payload: { pages: value } })} />
      <SyncCard icon={Radio} title="保存最新时间线" description="保存公共最新时间线；这是可选资料来源。" disabled={!online || create.isPending} onSubmit={(value) => create.mutate({ type: 'sync_latest', payload: { start_page: 1, pages: value } })} />
    </section>
		<AdvancedSync disabled={!online || create.isPending} onCreate={(type, payload) => create.mutate({ type, payload })} />
		{create.data && <div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-teal/25 bg-teal-soft/35 px-4 py-3 text-sm"><span className="font-medium text-teal">保存任务已创建，可以离开本页继续使用。</span><Link className="font-semibold text-teal hover:underline" to="/tasks">查看任务进度 →</Link></div>}
	</> : <section className="mt-6 rounded-2xl border border-dashed border-line bg-white/25 p-7 text-center"><KeyRound className="mx-auto text-ink-soft/45" size={25} /><h2 className="mt-3 font-semibold">登录后才能选择同步范围</h2><p className="mt-2 text-sm text-ink-soft">登录只用于读取你明确选择的树洞内容；浏览本地资料不需要登录。</p></section>}
    {create.error && <div className="mt-5"><ErrorState error={create.error} /></div>}

    <section className="panel mt-7 p-5 md:p-6">
      <div className="flex items-center justify-between gap-3"><div><p className="eyebrow">RECENT SAVE RUNS</p><h2 className="mt-1 text-xl font-semibold">最近保存任务</h2></div><div className="flex items-center gap-2"><span className="badge">{syncJobs.length} 条</span><Link className="text-xs font-semibold text-teal hover:underline" to="/tasks">查看全部</Link></div></div>
      <div className="mt-5 grid gap-3">{recentJobs.length ? recentJobs.map((job) => <SyncJob key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-8 text-center text-sm text-ink-soft">还没有保存任务。登录后从上方选择一种保存方式。</p>}</div>
    </section>
  </>
}

function AdvancedSync({ disabled, onCreate }: { disabled: boolean; onCreate: (type: string, payload: unknown) => void }) {
	const [startPage, setStartPage] = useState(1)
	const [pages, setPages] = useState(10)
	const [monitorPages, setMonitorPages] = useState(3)
	const [interval, setInterval] = useState(60)
	const [thumbStart, setThumbStart] = useState(1)
	const [thumbEnd, setThumbEnd] = useState(100)
	const [saveJSON, setSaveJSON] = useState(false)
	const [convertWebP, setConvertWebP] = useState(false)
	return <details className="panel mt-6 p-5"><summary className="cursor-pointer list-none"><div className="flex items-center justify-between gap-3"><div><p className="eyebrow">ADVANCED</p><h2 className="mt-1 text-lg font-semibold">高级采集选项</h2><p className="mt-1 text-xs text-ink-soft">顺序采集、持续监控与旧版图片补全</p></div><span className="badge">按需展开</span></div></summary><div className="mt-5 grid gap-5 border-t border-line pt-5 xl:grid-cols-3"><form className="rounded-xl border border-line bg-white/45 p-5" onSubmit={(event) => { event.preventDefault(); onCreate('sync_pages', { start_page: startPage, pages, options: { post_limit: 200, comment_limit: 200, save_json: saveJSON, fetch_images: false, convert_webp: convertWebP } }) }}><h3 className="font-semibold">顺序采集页面</h3><p className="mt-2 text-sm text-ink-soft">从指定页开始逐页采集，每页完成后保存进度。</p><div className="mt-4 grid grid-cols-2 gap-3"><label className="text-xs text-ink-soft">起始页<input className="field mt-1" type="number" min={1} value={startPage} onChange={(event) => setStartPage(Math.max(1, Number(event.target.value) || 1))} /></label><label className="text-xs text-ink-soft">页数<input className="field mt-1" type="number" min={1} max={50} value={pages} onChange={(event) => setPages(Math.max(1, Math.min(50, Number(event.target.value) || 1)))} /></label></div><label className="mt-3 inline-flex items-center gap-2 text-xs text-ink-soft"><input type="checkbox" checked={saveJSON} onChange={(event) => setSaveJSON(event.target.checked)} />保留原始响应</label><button className="button-primary mt-4 w-full" disabled={disabled}>创建顺序采集任务</button></form><form className="rounded-xl border border-line bg-white/45 p-5" onSubmit={(event) => { event.preventDefault(); onCreate('monitor_latest', { pages: monitorPages, interval_seconds: interval, options: { post_limit: 200, comment_limit: 200, save_json: saveJSON, convert_webp: convertWebP } }) }}><h3 className="font-semibold">持续监控最新页</h3><p className="mt-2 text-sm text-ink-soft">循环检查前 N 页；退出程序后不会自动恢复。</p><div className="mt-4 grid grid-cols-2 gap-3"><label className="text-xs text-ink-soft">监控页数<input className="field mt-1" type="number" min={1} max={50} value={monitorPages} onChange={(event) => setMonitorPages(Math.max(1, Math.min(50, Number(event.target.value) || 1)))} /></label><label className="text-xs text-ink-soft">间隔秒<input className="field mt-1" type="number" min={15} value={interval} onChange={(event) => setInterval(Math.max(15, Number(event.target.value) || 60))} /></label></div><button className="button-primary mt-4 w-full" disabled={disabled}>启动持续监控</button></form><div className="rounded-xl border border-line bg-white/45 p-5"><h3 className="font-semibold">图片与原始数据</h3><p className="mt-2 text-sm text-ink-soft">补全旧版图片、缩略图，或保存缓存的原始响应。</p><div className="mt-4 grid grid-cols-2 gap-3"><input className="field" type="number" min={1} value={thumbStart} aria-label="缩略图起始 ID" onChange={(event) => setThumbStart(Math.max(1, Number(event.target.value) || 1))} /><input className="field" type="number" min={thumbStart} value={thumbEnd} aria-label="缩略图结束 ID" onChange={(event) => setThumbEnd(Math.max(thumbStart, Number(event.target.value) || thumbStart))} /></div><label className="mt-3 inline-flex items-center gap-2 text-xs text-ink-soft"><input type="checkbox" checked={convertWebP} onChange={(event) => setConvertWebP(event.target.checked)} />转换为 WebP</label><div className="mt-3 grid gap-2"><button className="button-secondary" disabled={disabled} onClick={() => onCreate('repair_media', {})}>补全缺失媒体</button><button className="button-secondary" disabled={disabled} onClick={() => onCreate('fetch_images', { convert_webp: convertWebP })}>补全旧版图片</button><button className="button-secondary" disabled={disabled || thumbEnd < thumbStart} onClick={() => onCreate('repair_thumbnails', { start_id: thumbStart, end_id: thumbEnd, convert_webp: convertWebP })}>补全缩略图范围</button><button className="button-secondary" disabled={disabled} onClick={() => onCreate('save_raw_json', {})}>保存缓存的原始 JSON</button><Link className="mt-1 text-center text-xs font-semibold text-teal hover:underline" to="/maintenance">其他本地维护 →</Link></div></div></div></details>
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
	if (challenge) return <form className="w-full max-w-md rounded-xl border border-line bg-white/55 p-4 lg:w-[420px]" onSubmit={(event) => { event.preventDefault(); if (code.trim()) verify.mutate() }}><p className="eyebrow">VERIFY</p><p className="mt-1 text-sm font-semibold">输入{challenge === 'sms' ? '短信验证码' : '动态口令'}</p><p className="mt-1 text-xs leading-5 text-ink-soft">{stage === 'iaaa' ? '统一身份认证需要确认是你本人；短信通常会自动发送。' : '树洞会话需要再次验证，请先发送树洞短信。'}</p><label className="mt-3 block text-xs font-medium text-ink-soft">验证码<div className="mt-1.5 flex gap-2"><input className="field" value={code} onChange={(event) => setCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" placeholder="验证码" /><button className="button-primary shrink-0" disabled={!code.trim() || verify.isPending}>{verify.isPending ? '验证中…' : '继续登录'}</button></div></label>{challenge === 'sms' && <button className={stage === 'treehole' ? 'button-secondary mt-3' : 'mt-3 text-xs font-semibold text-teal hover:underline'} type="button" disabled={resend.isPending || (stage === 'iaaa' && !username.trim())} onClick={() => resend.mutate()}>{resend.isPending ? '正在发送…' : stage === 'treehole' ? '发送树洞短信验证码' : '没有收到？重新发送验证码'}</button>}{(verify.error || resend.error) && <p className="mt-2 text-xs text-coral">{verify.error?.message || resend.error?.message}</p>}</form>
  return <form className="grid w-full max-w-md gap-3 rounded-xl border border-line bg-white/55 p-4 lg:w-[420px]" onSubmit={(event) => { event.preventDefault(); if (username.trim() && password) login.mutate() }}><div><p className="eyebrow">WEB LOGIN</p><p className="mt-1 text-sm font-semibold">使用北大学号登录</p><p className="mt-1 text-xs leading-5 text-ink-soft">凭据仅用于本次登录流程，网页不会保存密码。</p></div><label className="text-xs font-medium text-ink-soft">学号<input className="field mt-1.5" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" placeholder="北大学号（无需邮箱后缀）" /></label><label className="text-xs font-medium text-ink-soft">密码<input className="field mt-1.5" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" placeholder="密码（不会由网页保存）" /></label><button className="button-primary" disabled={!username.trim() || !password || login.isPending}>{login.isPending ? '登录中…' : '登录并保存本机会话'}</button>{login.error && <p className="text-xs text-coral">{login.error.message}</p>}</form>
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
  return <div><JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />{job.type === 'save_raw_json' && job.status === 'completed' && <div className="mt-2 flex justify-end"><button className="button-secondary" disabled={download.isPending} onClick={() => download.mutate()}><Download size={14} />下载原始 JSON</button></div>}{download.error && <p className="mt-2 text-xs text-coral">{download.error.message}</p>}</div>
}
