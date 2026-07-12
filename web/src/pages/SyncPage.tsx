import { FormEvent, useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, ListRestart, Radio, RefreshCw, SearchCheck, ShieldCheck } from 'lucide-react'
import { api } from '../lib/api'
import type { AuthStatus, Job } from '../lib/types'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'

const syncTypes = new Set(['sync_followed', 'sync_pids', 'sync_latest', 'repair_comments', 'repair_media'])

export function SyncPage() {
  const client = useQueryClient()
  const [showLogin, setShowLogin] = useState(false)
  const session = useQuery({ queryKey: ['session'], queryFn: api.session })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
  const setSession = (value: AuthStatus) => client.setQueryData(['session'], value)
  const probe = useMutation({ mutationFn: api.probeSession, onSuccess: setSession })
	const logout = useMutation({ mutationFn: api.logoutSession, onSuccess: (value) => { setSession(value); setShowLogin(false) } })
  const create = useMutation({ mutationFn: ({ type, payload }: { type: string; payload?: unknown }) => api.createJob(type, payload), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })

  useEffect(() => {
    if (session.data?.has_session && !session.data.checked && !probe.isPending) probe.mutate()
  }, [session.data?.has_session, session.data?.checked])

  if (session.isLoading || jobs.isLoading) return <LoadingState label="正在打开同步中心…" />
  if (session.error || jobs.error) return <ErrorState error={session.error || jobs.error} />

  const status = session.data
  const online = Boolean(status?.checked && status.can_read_online)
  const recentJobs = (jobs.data ?? []).filter((job) => syncTypes.has(job.type)).slice(0, 10)
  return <>
    <PageHeader eyebrow="NATIVE SYNC" title="同步中心" description="直接从树洞同步关注、指定 PID 或最新时间线。同步写入本机资料库，并复用可暂停、恢复和重试的持久任务。" actions={<>{status?.has_session && <button className="button-secondary" disabled={logout.isPending} onClick={() => logout.mutate()}>{logout.isPending ? '正在退出…' : '退出本机会话'}</button>}<button className="button-secondary" disabled={probe.isPending} onClick={() => probe.mutate()}><SearchCheck size={16} />{probe.isPending ? '检测中…' : '检测登录状态'}</button></>} />
    <section className={`panel p-5 md:p-6 ${online ? 'border-teal/30' : ''}`}>
      <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex items-start gap-3"><div className={`grid size-11 shrink-0 place-items-center rounded-xl ${online ? 'bg-teal-soft text-teal' : 'bg-coral-soft text-coral'}`}>{online ? <ShieldCheck size={20} /> : <KeyRound size={20} />}</div><div><p className="font-semibold">{online ? '在线读取已就绪' : status?.has_session ? '发现本机凭据，尚未验证' : '需要登录树洞'}</p><p className="mt-1 text-sm leading-6 text-ink-soft">{status?.message || '点击检测登录状态，或在本页完成登录。'}</p>{status?.challenge_reason && <p className="mt-2 text-xs text-coral">{status.challenge_reason}</p>}</div></div>
        {!online && <div className="w-full max-w-md lg:w-[420px]">
          {!showLogin && !status?.challenge && <div className="rounded-xl border border-line bg-white/55 p-4"><p className="text-sm font-semibold">Studio 原生在线同步</p><p className="mt-1 text-xs leading-5 text-ink-soft">在这里登录后即可直接同步和使用在线功能。密码只用于当前登录请求，不由网页保存；已建立的会话保存在本机。</p><button className="button-primary mt-3 w-full" type="button" onClick={() => setShowLogin(true)}>在 Studio 中登录</button><a className="mt-3 block w-full text-center text-xs font-semibold text-ink-soft hover:text-ink" href="/imports">只想迁移资料？前往归档导入</a></div>}
          {(showLogin || Boolean(status?.challenge)) && <><LoginPanel status={status} onStatus={setSession} /><button className="mt-2 w-full text-xs text-ink-soft hover:text-ink" type="button" onClick={() => setShowLogin(false)}>收起备用登录</button></>}
        </div>}
      </div>
    </section>

    <section className="mt-6 grid gap-5 xl:grid-cols-3">
      <SyncCard icon={RefreshCw} title="同步关注" description="从你的关注列表读取帖子。首次建议只同步 1 页。" disabled={!online || create.isPending} onSubmit={(value) => create.mutate({ type: 'sync_followed', payload: { pages: value } })} />
      <PIDSyncCard disabled={!online || create.isPending} onSubmit={(pids) => create.mutate({ type: 'sync_pids', payload: { pids } })} />
      <SyncCard icon={Radio} title="同步最新时间线" description="读取公共最新时间线；这是可选资料来源。" disabled={!online || create.isPending} onSubmit={(value) => create.mutate({ type: 'sync_latest', payload: { start_page: 1, pages: value } })} />
    </section>
    {create.error && <div className="mt-5"><ErrorState error={create.error} /></div>}

    <section className="panel mt-7 p-5 md:p-6">
      <div className="flex items-center justify-between"><div><p className="eyebrow">SYNC RUNS</p><h2 className="mt-1 text-xl font-semibold">最近同步任务</h2></div><span className="badge">{recentJobs.length} 条</span></div>
      <div className="mt-5 grid gap-3">{recentJobs.length ? recentJobs.map((job) => <SyncJob key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-8 text-center text-sm text-ink-soft">还没有同步任务。登录后从上方选择一种同步方式。</p>}</div>
    </section>
  </>
}

function LoginPanel({ status, onStatus }: { status?: AuthStatus; onStatus: (value: AuthStatus) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const login = useMutation({ mutationFn: () => api.loginSession(username.trim(), password), onSuccess: (value) => { if (value.can_read_online || value.challenge_stage !== 'iaaa') setPassword(''); onStatus(value) } })
	const challenge = status?.challenge === 'sms' || status?.challenge === 'otp' ? status.challenge : undefined
	const stage = status?.challenge_stage ?? ''
	const verify = useMutation({ mutationFn: () => api.continueSession(stage, challenge!, username.trim(), password, code.trim()), onSuccess: (value) => { setCode(''); if (value.can_read_online) setPassword(''); onStatus(value) } })
	const resend = useMutation({ mutationFn: () => api.sendSessionSMS(username.trim()), onSuccess: onStatus })
	if (challenge) return <form className="w-full max-w-md rounded-xl border border-line bg-white/55 p-4 lg:w-[420px]" onSubmit={(event) => { event.preventDefault(); if (code.trim()) verify.mutate() }}><p className="text-sm font-semibold">输入{challenge === 'sms' ? '短信验证码' : '动态口令'}</p><p className="mt-1 text-xs leading-5 text-ink-soft">{stage === 'iaaa' ? '这是北大统一身份认证阶段的验证。' : '这是树洞会话阶段的验证。'}</p><div className="mt-3 flex gap-2"><input className="field" value={code} onChange={(event) => setCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" placeholder="验证码" /><button className="button-primary shrink-0" disabled={!code.trim() || verify.isPending}>{verify.isPending ? '验证中…' : '继续登录'}</button></div>{challenge === 'sms' && stage === 'iaaa' && <button className="mt-3 text-xs font-semibold text-teal hover:underline" type="button" disabled={resend.isPending} onClick={() => resend.mutate()}>{resend.isPending ? '正在重新发送…' : '没有收到？重新发送验证码'}</button>}{(verify.error || resend.error) && <p className="mt-2 text-xs text-coral">{verify.error?.message || resend.error?.message}</p>}</form>
  return <form className="grid w-full max-w-md gap-3 rounded-xl border border-line bg-white/55 p-4 lg:w-[420px]" onSubmit={(event) => { event.preventDefault(); if (username.trim() && password) login.mutate() }}><div><input className="field" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" placeholder="北大学号（无需邮箱后缀）" /><p className="mt-1.5 text-[11px] text-ink-soft">填写学号数字即可；若粘贴学校邮箱，程序会自动移除 @ 后缀。</p></div><input className="field" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" placeholder="密码（不会由网页保存）" /><button className="button-primary" disabled={!username.trim() || !password || login.isPending}>{login.isPending ? '登录中…' : '登录并保存本机会话'}</button>{login.error && <p className="text-xs text-coral">{login.error.message}</p>}</form>
}

function SyncCard({ icon: Icon, title, description, disabled, onSubmit }: { icon: typeof RefreshCw; title: string; description: string; disabled: boolean; onSubmit: (pages: number) => void }) {
  const [pages, setPages] = useState(1)
  return <form className="panel p-5" onSubmit={(event: FormEvent) => { event.preventDefault(); onSubmit(pages) }}><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={19} /></div><h2 className="mt-5 text-lg font-semibold">{title}</h2><p className="mt-2 min-h-12 text-sm leading-6 text-ink-soft">{description}</p><label className="mt-4 block text-xs font-medium text-ink-soft">页数（1–50）<input className="field mt-1.5" type="number" min={1} max={50} value={pages} onChange={(event) => setPages(Math.max(1, Math.min(50, Number(event.target.value) || 1)))} /></label><button className="button-primary mt-4 w-full" disabled={disabled}>创建同步任务</button></form>
}

function PIDSyncCard({ disabled, onSubmit }: { disabled: boolean; onSubmit: (pids: number[]) => void }) {
  const [value, setValue] = useState('')
  const pids = value.split(/[\s,，]+/).map(Number).filter((item) => Number.isInteger(item) && item > 0)
  return <form className="panel p-5" onSubmit={(event) => { event.preventDefault(); if (pids.length) onSubmit([...new Set(pids)]) }}><div className="grid size-10 place-items-center rounded-xl bg-coral-soft text-coral"><ListRestart size={19} /></div><h2 className="mt-5 text-lg font-semibold">同步指定 PID</h2><p className="mt-2 min-h-12 text-sm leading-6 text-ink-soft">输入一个或多个 PID，用空格或逗号分隔。</p><label className="mt-4 block text-xs font-medium text-ink-soft">PID 列表<textarea className="field mt-1.5 min-h-20 resize-y" value={value} onChange={(event) => setValue(event.target.value)} placeholder="1234567, 2345678" /></label><button className="button-primary mt-4 w-full" disabled={disabled || pids.length === 0}>同步 {new Set(pids).size || ''} 个 PID</button></form>
}

function SyncJob({ job }: { job: Job }) {
  const client = useQueryClient()
  const action = useMutation({ mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
  return <JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />
}
