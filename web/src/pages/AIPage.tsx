import { FormEvent, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, BookOpenCheck, CircleStop, MessageSquarePlus, Search, Send, Sparkles } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import type { AIEvidenceRef, AIEvidenceReport, AIClaimStatus, AIRun, AIScope, AISession, AISource } from '../lib/types'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'

type Mode = AISession['mode']

export function AIPage() {
  const client = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedMode = searchParams.get('mode')
  const initialMode: Mode = requestedMode === 'selected' || requestedMode === 'course' ? requestedMode : 'local'
  const initialPIDs = searchParams.get('pids') ?? ''
  const providers = useQuery({ queryKey: ['ai-providers'], queryFn: api.aiProviders })
  const sessions = useQuery({ queryKey: ['ai-sessions'], queryFn: api.aiSessions })
  const localTags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
  const [selectedID, setSelectedID] = useState('')
	const [creatingNew, setCreatingNew] = useState(Boolean(requestedMode || initialPIDs))
  const [mode, setMode] = useState<Mode>(initialMode)
  const [prompt, setPrompt] = useState('')
  const [pids, setPIDs] = useState(initialPIDs)
  const [course, setCourse] = useState(searchParams.get('course') ?? '')
  const [teachers, setTeachers] = useState(searchParams.get('teachers') ?? '')
  const [fromDate, setFromDate] = useState(searchParams.get('from') ?? '')
  const [toDate, setToDate] = useState(searchParams.get('to') ?? '')
  const [selectedTagIDs, setSelectedTagIDs] = useState<number[]>([])
  const [mediaFilter, setMediaFilter] = useState<'any' | 'with' | 'without'>('any')
  const [running, setRunning] = useState(false)
  const [draftAnswer, setDraftAnswer] = useState('')
  const [trace, setTrace] = useState<string[]>([])
	const [liveSources, setLiveSources] = useState<AISource[]>([])
	const [liveEvidence, setLiveEvidence] = useState<AIEvidenceReport>()
	const [runError, setRunError] = useState('')
	const promptInput = useRef<HTMLTextAreaElement>(null)
	const attachedRun = useRef('')
  const detail = useQuery({ queryKey: ['ai-session', selectedID], queryFn: () => api.aiSession(selectedID), enabled: Boolean(selectedID) })
  const configured = providers.data?.some((provider) => provider.configured) ?? false
  const activeProvider = providers.data?.find((provider) => provider.active) ?? providers.data?.find((provider) => provider.configured)

  useEffect(() => {
    if (!selectedID && !creatingNew && sessions.data?.[0]) setSelectedID(sessions.data[0].id)
  }, [creatingNew, selectedID, sessions.data])

  useEffect(() => {
    if (!detail.data?.session || creatingNew) return
    const scope = detail.data.session.scope
    setPIDs(scope?.pids?.join(', ') ?? '')
    setCourse(scope?.course ?? '')
    setTeachers(scope?.teachers?.join('、') ?? '')
    setFromDate(timestampToDateInput(scope?.from))
    setToDate(timestampToDateInput(scope?.to))
    setSelectedTagIDs(scope?.tag_ids ?? [])
    setMediaFilter(scope?.has_media === true ? 'with' : scope?.has_media === false ? 'without' : 'any')
  }, [creatingNew, detail.data?.session])

  useEffect(() => {
    const run = detail.data?.latest_run
    if (!selectedID || run?.status !== 'running' || attachedRun.current === run.id) return
    attachedRun.current = run.id
    setDraftAnswer(''); setTrace([]); setLiveSources([]); setLiveEvidence(undefined); setRunError(''); setRunning(true)
    return openAIStream(selectedID, {
      delta: (value) => setDraftAnswer((current) => current + value),
      trace: (value) => setTrace((current) => [...current, value]),
      source: (value) => setLiveSources((current) => current.some((item) => item.origin === value.origin && item.pid === value.pid && item.cid === value.cid) ? current : [...current, value]),
	  evidence: setLiveEvidence,
	  done: async () => { if (attachedRun.current === run.id) attachedRun.current = ''; setRunning(false); await client.invalidateQueries({ queryKey: ['ai-session', selectedID] }); await client.invalidateQueries({ queryKey: ['ai-sessions'] }) },
      error: (message) => setRunError(message),
    })
  }, [client, detail.data?.latest_run?.id, detail.data?.latest_run?.status, selectedID])

  const cancel = useMutation({ mutationFn: () => api.cancelAI(selectedID), onSuccess: () => setRunning(false) })
	const parsedPIDs = useMemo(() => [...new Set(pids.split(/[\s,，]+/).map(Number).filter((value) => Number.isInteger(value) && value > 0))], [pids])
	const parsedTeachers = useMemo(() => teachers.split(/[、,，]+/).map((value) => value.trim()).filter(Boolean), [teachers])

	function startNew() {
		setSelectedID(''); setCreatingNew(true); setMode('local'); setPrompt(''); setPIDs(''); setCourse(''); setTeachers(''); setFromDate(''); setToDate(''); setSelectedTagIDs([]); setMediaFilter('any'); setDraftAnswer(''); setTrace([]); setLiveSources([]); setLiveEvidence(undefined); setRunError('')
		setSearchParams({}, { replace: true })
		requestAnimationFrame(() => promptInput.current?.focus())
	}

	function selectSession(id: string) {
		setSelectedID(id); setCreatingNew(false); setDraftAnswer(''); setTrace([]); setLiveSources([]); setLiveEvidence(undefined); setRunError('')
		setSearchParams({}, { replace: true })
	}

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!prompt.trim() || !configured || running) return
    let sessionID = selectedID
    let sessionMode = detail.data?.session.mode ?? mode
    const scope = buildScope(sessionMode, parsedPIDs, course, parsedTeachers, fromDate, toDate, selectedTagIDs, mediaFilter)
    if (!sessionID) {
	  const created = await api.createAISession(mode, prompt.trim().slice(0, 36), scope)
      sessionID = created.id
      sessionMode = created.mode
      setSelectedID(created.id)
		setSearchParams({}, { replace: true })
      await client.invalidateQueries({ queryKey: ['ai-sessions'] })
    }
		setDraftAnswer(''); setTrace([]); setLiveSources([]); setLiveEvidence(undefined); setRunError(''); setRunning(true)
    try {
      const started = await api.startAIMessage(sessionID, { prompt: prompt.trim(), replace_scope: true, ...scope })
	  if (started.run_id) attachedRun.current = started.run_id
      openAIStream(sessionID, {
        delta: (value) => setDraftAnswer((current) => current + value),
        trace: (value) => setTrace((current) => [...current, value]),
        source: (value) => setLiveSources((current) => current.some((item) => item.origin === value.origin && item.pid === value.pid && item.cid === value.cid) ? current : [...current, value]),
		evidence: setLiveEvidence,
			done: async (completed) => { if (!started.run_id || attachedRun.current === started.run_id) attachedRun.current = ''; setRunning(false); if (completed) setPrompt(''); setCreatingNew(false); await client.invalidateQueries({ queryKey: ['ai-session', sessionID] }); await client.invalidateQueries({ queryKey: ['ai-sessions'] }) },
			error: (message) => setRunError(message),
		})
	} catch (error) {
		setRunning(false)
		setRunError(error instanceof Error ? error.message : 'AI 请求启动失败')
	}
  }

  const currentMode = detail.data?.session.mode ?? mode
  const messages = detail.data?.messages ?? []
	const contextReady = currentMode === 'selected' ? parsedPIDs.length > 0 : currentMode === 'course' ? Boolean(course.trim()) : true
  const shownMessages = useMemo(() => running ? [...messages, { id: 'stream', session_id: selectedID, role: 'assistant' as const, content: draftAnswer, evidence: liveEvidence, created_at: new Date().toISOString(), sources: liveSources }] : messages, [draftAnswer, liveEvidence, liveSources, messages, running, selectedID])
	const latestRun = detail.data?.latest_run
	const runNotice = !running && latestRun && (latestRun.status === 'failed' || latestRun.status === 'interrupted') ? <RunFailureNotice run={latestRun} /> : null

  if (providers.isLoading || sessions.isLoading) return <LoadingState label="正在打开 AI 研究台…" />
  if (providers.error || sessions.error) return <ErrorState error={providers.error || sessions.error} />

  return <>
	<PageHeader eyebrow="RESEARCH" title="AI 研究台" description="让 AI 从本地树洞资料中查找证据、整理观点，并把回答和引用来源一起保存。" actions={<button className="button-secondary" disabled={running} onClick={startNew}><MessageSquarePlus size={16} />新建研究</button>} />
	{!configured && <div className="mb-6 rounded-2xl border border-coral/25 bg-coral-soft/45 p-5 text-sm leading-6"><p className="font-semibold text-coral">还没有可用的 AI 服务</p><p className="mt-1 text-ink-soft">先在设置中启用 AI 并选择服务商。保存后立即生效，不需要重启 Studio。</p><Link to="/settings" className="mt-3 inline-flex font-semibold text-teal hover:underline">前往 AI 设置 →</Link></div>}
	{configured && activeProvider && <div className="mb-4 rounded-xl border border-line bg-white/40 px-4 py-3 text-xs leading-5 text-ink-soft"><span className="font-semibold text-ink">当前模型：{activeProvider.name} · {activeProvider.model}</span>{!isLocalProvider(activeProvider.base_url) && <span>；提问时，命中的树洞原文会发送到该远程服务进行分析。</span>}</div>}
    <div className="grid min-h-[650px] gap-5 xl:grid-cols-[260px_1fr]">
	  <aside className="panel min-w-0 p-4"><div className="flex items-center justify-between px-1"><p className="eyebrow">RECENT</p><span className="badge">{sessions.data?.length ?? 0}</span></div><div className="mt-4 flex gap-2 overflow-x-auto pb-1 xl:grid xl:max-h-[570px] xl:overflow-y-auto xl:pb-0">{sessions.data?.map((session) => <button key={session.id} disabled={running} onClick={() => selectSession(session.id)} className={`w-56 shrink-0 rounded-xl border p-3 text-left transition disabled:cursor-not-allowed disabled:opacity-60 xl:w-auto ${selectedID === session.id && !creatingNew ? 'border-teal bg-teal-soft/45' : 'border-line bg-white/45 hover:border-teal/40'}`}><p className="truncate text-sm font-semibold">{session.title}</p><p className="mt-1 text-[10px] text-ink-soft">{modeLabel(session.mode)} · {session.model}</p></button>)}{!sessions.data?.length && <p className="w-full rounded-xl border border-dashed border-line p-5 text-center text-xs leading-5 text-ink-soft">发送第一个问题后，研究记录会保存在这里。</p>}</div></aside>
      <section className="panel flex min-h-[650px] flex-col overflow-hidden">
		<div className="flex-1 space-y-5 overflow-auto p-5 md:p-7">{runNotice}{selectedID && detail.isLoading ? <LoadingState /> : selectedID && detail.error ? <ErrorState error={detail.error} /> : shownMessages.length ? shownMessages.map((message) => <div key={message.id} className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}><div className={`max-w-[92%] rounded-2xl px-4 py-3 text-sm leading-7 ${message.role === 'user' ? 'bg-ink text-white md:max-w-[78%]' : 'border border-line bg-white/65 md:max-w-[92%]'}`}><ResearchContent content={message.content || (message.id === 'stream' ? '正在查找资料…' : '')} sources={message.sources ?? []} />{message.evidence ? <EvidenceReport report={message.evidence} sources={message.sources ?? []} /> : null}{message.sources?.length ? <EvidenceList sources={message.sources} /> : null}{message.trace && <details className="mt-3 border-t border-line/60 pt-2 text-xs text-ink-soft"><summary className="cursor-pointer font-medium">查看检索过程</summary><pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[10px] leading-5">{formatTrace(message.trace)}</pre></details>}</div></div>) : <Welcome onChoose={(nextMode, value) => { setSelectedID(''); setMode(nextMode); setPrompt(value); setCreatingNew(true); requestAnimationFrame(() => promptInput.current?.focus()) }} />}{trace.length > 0 && <div className="rounded-xl border border-teal/20 bg-teal-soft/30 p-4"><p className="eyebrow">研究进度</p><ul className="mt-2 space-y-1 text-xs leading-5 text-ink-soft">{trace.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul></div>}</div>
        <form className="border-t border-line bg-paper/75 p-4 md:p-5" onSubmit={submit}>
		  {!selectedID && <div className="mb-3 grid grid-cols-3 gap-2">{(['local', 'selected', 'course'] as Mode[]).map((item) => <button type="button" key={item} onClick={() => setMode(item)} className={`rounded-xl border px-3 py-2 text-xs font-semibold ${mode === item ? 'border-ink bg-ink text-white' : 'border-line bg-white/60'}`}>{modeLabel(item)}</button>)}</div>}
          {currentMode === 'selected' && <input className="field mb-3" value={pids} onChange={(event) => setPIDs(event.target.value)} placeholder="选中 PID，例如 123456, 234567" />}
          {currentMode === 'course' && <div className="mb-3 grid gap-2 sm:grid-cols-2"><input className="field" value={course} onChange={(event) => setCourse(event.target.value)} placeholder="课程名或别名" /><input className="field" value={teachers} onChange={(event) => setTeachers(event.target.value)} placeholder="教师，可用逗号分隔" /></div>}
		  {(currentMode === 'local' || currentMode === 'course') && <ScopeFilters fromDate={fromDate} toDate={toDate} selectedTagIDs={selectedTagIDs} mediaFilter={mediaFilter} tags={localTags.data ?? []} setFromDate={setFromDate} setToDate={setToDate} setSelectedTagIDs={setSelectedTagIDs} setMediaFilter={setMediaFilter} />}
		  <div className="flex items-end gap-2"><textarea ref={promptInput} aria-label="研究问题" className="field min-h-24 flex-1 resize-y py-3" value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder={currentMode === 'course' ? '想重点比较哪些方面？' : '基于本地资料提出问题…'} /><button className="button-primary !size-11 !p-0" type="submit" disabled={!configured || !contextReady || !prompt.trim() || running} aria-label="发送问题"><Send size={17} /></button>{running && <button type="button" className="button-secondary !size-11 !p-0 text-coral" onClick={() => cancel.mutate()} aria-label="停止生成"><CircleStop size={18} /></button>}</div>
		  {!contextReady && <p className="mt-2 text-xs text-coral">{currentMode === 'selected' ? '请先填写至少一个 PID。' : '请先填写课程名或别名。'}</p>}
		<p className="mt-2 text-[11px] text-ink-soft">{modeLabel(currentMode)} · 回答可能出错，请打开资料来源核对原文</p>
		{runError && <p className="mt-2 text-xs text-coral">{runError}</p>}
        </form>
      </section>
    </div>
  </>
}

function Welcome({ onChoose }: { onChoose: (mode: Mode, prompt: string) => void }) {
	const suggestions: { mode: Mode; icon: typeof Search; title: string; description: string; prompt: string }[] = [
		{ mode: 'local', icon: Search, title: '从全部资料查找', description: '自动检索相关帖子和评论', prompt: '总结本地资料中关于这个主题的主要观点，并列出来源。' },
		{ mode: 'selected', icon: Sparkles, title: '分析指定帖子', description: '只围绕你填写的 PID 回答', prompt: '比较这些帖子中的观点、争议和共同结论。' },
		{ mode: 'course', icon: BookOpenCheck, title: '整理课程评价', description: '按课程与教师汇总体验', prompt: '总结课程难度、作业量、考核方式和同学们的整体评价。' },
	]
	return <div className="grid min-h-[390px] place-items-center"><div className="w-full max-w-2xl text-center"><div className="mx-auto grid size-14 place-items-center rounded-2xl bg-ink text-white shadow-[5px_5px_0_#e4654f]"><Bot size={24} /></div><h2 className="mt-6 text-xl font-semibold">你想从资料中了解什么？</h2><p className="mt-2 text-sm leading-6 text-ink-soft">选择一种研究方式，再修改下方问题。AI 只会读取与你问题相关的资料。</p><div className="mt-6 grid gap-3 text-left md:grid-cols-3">{suggestions.map(({ mode, icon: Icon, title, description, prompt }) => <button key={mode} type="button" className="rounded-xl border border-line bg-white/55 p-4 transition hover:-translate-y-0.5 hover:border-teal/40 hover:bg-teal-soft/20" onClick={() => onChoose(mode, prompt)}><Icon size={18} className="text-teal" /><p className="mt-3 text-sm font-semibold">{title}</p><p className="mt-1 text-xs leading-5 text-ink-soft">{description}</p></button>)}</div></div></div>
}

function ScopeFilters({ fromDate, toDate, selectedTagIDs, mediaFilter, tags, setFromDate, setToDate, setSelectedTagIDs, setMediaFilter }: { fromDate: string; toDate: string; selectedTagIDs: number[]; mediaFilter: 'any' | 'with' | 'without'; tags: { id: number; name: string; color?: string }[]; setFromDate: (value: string) => void; setToDate: (value: string) => void; setSelectedTagIDs: (value: number[] | ((current: number[]) => number[])) => void; setMediaFilter: (value: 'any' | 'with' | 'without') => void }) {
	const active = Number(Boolean(fromDate || toDate)) + Number(selectedTagIDs.length > 0) + Number(mediaFilter !== 'any')
	return <details className="mb-3 rounded-xl border border-line bg-white/45 p-3 text-xs"><summary className="cursor-pointer font-semibold text-ink-soft">限定研究范围{active ? ` · 已启用 ${active} 项` : ''}</summary><div className="mt-3 grid gap-3"><div className="grid gap-2 sm:grid-cols-2"><label className="text-ink-soft">起始日期<input aria-label="研究起始日期" className="field mt-1" type="date" value={fromDate} onChange={(event) => setFromDate(event.target.value)} /></label><label className="text-ink-soft">结束日期<input aria-label="研究结束日期" className="field mt-1" type="date" value={toDate} onChange={(event) => setToDate(event.target.value)} /></label></div><label className="text-ink-soft">媒体范围<select aria-label="研究媒体范围" className="field mt-1" value={mediaFilter} onChange={(event) => setMediaFilter(event.target.value as 'any' | 'with' | 'without')}><option value="any">不限</option><option value="with">仅含图片</option><option value="without">仅纯文本</option></select></label>{tags.length ? <div><p className="text-ink-soft">本地标签</p><div className="mt-2 flex flex-wrap gap-2">{tags.map((tag) => <label key={tag.id} className="badge cursor-pointer" style={tag.color ? { borderColor: tag.color, color: tag.color } : undefined}><input className="hidden" type="checkbox" checked={selectedTagIDs.includes(tag.id)} onChange={() => setSelectedTagIDs((current) => current.includes(tag.id) ? current.filter((id) => id !== tag.id) : [...current, tag.id])} />{tag.name}{selectedTagIDs.includes(tag.id) ? ' ✓' : ''}</label>)}</div></div> : null}</div></details>
}

function modeLabel(mode: Mode) { return mode === 'selected' ? '选中内容' : mode === 'course' ? '课程分析' : '本地检索' }

function buildScope(mode: Mode, pids: number[], course: string, teachers: string[], fromDate: string, toDate: string, tagIDs: number[], mediaFilter: 'any' | 'with' | 'without'): AIScope {
	if (mode === 'selected') return { pids }
	return {
		course: mode === 'course' ? course.trim() : undefined,
		teachers: mode === 'course' ? teachers : undefined,
		from: dateInputToTimestamp(fromDate, false),
		to: dateInputToTimestamp(toDate, true),
		tag_ids: tagIDs.length ? tagIDs : undefined,
		has_media: mediaFilter === 'any' ? undefined : mediaFilter === 'with',
	}
}

function dateInputToTimestamp(value: string, endOfDay: boolean) {
	if (!value) return undefined
	const time = new Date(`${value}T${endOfDay ? '23:59:59' : '00:00:00'}`).getTime()
	return Number.isFinite(time) ? Math.floor(time / 1000) : undefined
}

function timestampToDateInput(value?: number) {
	if (!value) return ''
	const date = new Date(value * 1000)
	const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
	return local.toISOString().slice(0, 10)
}

function RunFailureNotice({ run }: { run: AIRun }) {
	return <div className="rounded-xl border border-coral/25 bg-coral-soft/30 p-4 text-xs leading-5 text-coral">
		<p className="font-semibold">{run.status === 'interrupted' ? '上次研究因 Studio 重启而中断' : '上次研究未能完成'}</p>
		<p className="mt-1 text-ink-soft">{run.error || '问题和研究范围已经保留，可以直接修改后重试。'}</p>
		{run.queries?.length ? <details className="mt-2 text-ink-soft"><summary className="cursor-pointer font-semibold">查看已执行的 {run.queries.length} 次检索</summary><ol className="mt-2 space-y-1 pl-4">{run.queries.map((query) => <li key={query.ordinal}><span className="font-mono text-teal">{query.query || query.tool}</span> · 命中 {query.matches} 条{query.reason ? ` · ${query.reason}` : ''}</li>)}</ol></details> : null}
	</div>
}

function isLocalProvider(baseURL: string) {
	try {
		const hostname = new URL(baseURL).hostname.toLowerCase()
		return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '::1'
	} catch {
		return false
	}
}

function openAIStream(sessionID: string, handlers: { delta: (value: string) => void; trace: (value: string) => void; source: (value: AISource) => void; evidence: (value: AIEvidenceReport) => void; done: (completed: boolean) => void; error: (message: string) => void }) {
  const source = new EventSource(`/api/v1/ai/sessions/${sessionID}/events`)
  source.addEventListener('delta', (event) => handlers.delta(JSON.parse((event as MessageEvent).data).delta ?? ''))
  source.addEventListener('search_started', (event) => { const data = JSON.parse((event as MessageEvent).data); handlers.trace(`第 ${data.round} 轮：${data.query} · ${data.reason || '检索资料'}`) })
  source.addEventListener('search_result', (event) => { const data = JSON.parse((event as MessageEvent).data); handlers.trace(`命中 ${data.matches} 条：${data.query}${data.variants?.length > 1 ? ` · 已扩展 ${data.variants.length} 种表达` : ''}`) })
  source.addEventListener('validation_retry', (event) => { const data = JSON.parse((event as MessageEvent).data); handlers.trace(`结构校验后正在重写：${(data.issues ?? []).join('；')}`) })
  source.addEventListener('evidence_check_started', (event) => { const data = JSON.parse((event as MessageEvent).data); handlers.trace(`正在核对 ${data.cited ?? 0} 条带引用结论的证据支持关系`) })
  source.addEventListener('evidence_check_failed', () => handlers.trace('语义核对未完成；已保留逐句引用绑定结果'))
  source.addEventListener('source', (event) => handlers.source(JSON.parse((event as MessageEvent).data)))
	source.addEventListener('evidence_report', (event) => handlers.evidence(JSON.parse((event as MessageEvent).data)))
	source.addEventListener('error', (event) => { try { handlers.error(JSON.parse((event as MessageEvent).data).message ?? 'AI 运行失败') } catch { handlers.error('AI 运行失败') }; source.close(); handlers.done(false) })
	source.addEventListener('completed', () => { source.close(); handlers.done(true) })
	source.addEventListener('cancelled', () => { source.close(); handlers.done(false) })
	source.onerror = () => { handlers.error('与 AI 的实时连接已断开，请保留问题后重试。'); source.close(); handlers.done(false) }
	return () => source.close()
}

function formatTrace(value: string) { try { return JSON.stringify(JSON.parse(value), null, 2) } catch { return value } }

function EvidenceReport({ report, sources }: { report: AIEvidenceReport; sources: AISource[] }) {
	const coverage = Math.round((report.summary.citation_coverage || 0) * 100)
	const statusText = report.checked ? '语义核对完成' : report.summary.unverified ? '部分引用尚未语义核对' : '引用绑定完成'
	return <details className="mt-3 rounded-xl border border-teal/20 bg-teal-soft/20 p-3 text-xs">
		<summary className="cursor-pointer font-semibold text-ink">证据覆盖 {coverage}% · {statusText}</summary>
		<div className="mt-2 flex flex-wrap gap-2 text-[10px] text-ink-soft"><span>支持 {report.summary.supported}</span><span>部分支持 {report.summary.partial}</span><span>无支持 {report.summary.unsupported}</span><span>未核对 {report.summary.unverified}</span><span>明确资料不足 {report.summary.insufficient}</span></div>
		<ol className="mt-3 grid gap-2">{report.claims.map((claim) => <li key={claim.ordinal} className="rounded-lg border border-line/70 bg-white/55 p-3 leading-5"><div className="flex items-start gap-2"><span className={`mt-0.5 shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${claimStatusClass(claim.status)}`}>{claimStatusLabel(claim.status)}</span><span className="text-ink">{claim.text}</span></div>{claim.reason && <p className="mt-1 text-ink-soft">{claim.reason}</p>}{claim.sources?.length ? <div className="mt-2 flex flex-wrap gap-2">{claim.sources.map((ref, index) => { const source = sourceForEvidenceRef(ref, sources); return <Link key={`${ref.origin}-${ref.pid}-${ref.cid ?? 0}-${index}`} className="font-mono font-semibold text-teal hover:underline" to={sourceHref(source)}>#{ref.pid}{ref.cid ? `/C${ref.cid}` : ''}</Link> })}</div> : null}</li>)}</ol>
	</details>
}

function claimStatusLabel(status: AIClaimStatus) {
	return status === 'supported' ? '支持' : status === 'partial' ? '部分支持' : status === 'unsupported' ? '无支持' : status === 'insufficient' ? '资料不足' : '未核对'
}

function claimStatusClass(status: AIClaimStatus) {
	return status === 'supported' ? 'bg-teal-soft text-teal' : status === 'partial' ? 'bg-coral-soft text-coral' : status === 'unsupported' ? 'bg-coral text-white' : 'bg-paper text-ink-soft'
}

function sourceForEvidenceRef(ref: AIEvidenceRef, sources: AISource[]): AISource {
	return sources.find((source) => (source.origin ?? 'local') === ref.origin && source.pid === ref.pid && source.cid === ref.cid) ?? ref
}

function EvidenceList({ sources }: { sources: AISource[] }) {
	return <details className="mt-3 border-t border-line/60 pt-3 text-xs"><summary className="cursor-pointer font-semibold text-ink-soft">查看实际引用的 {sources.length} 条证据</summary><div className="mt-3 grid gap-2">{sources.map((source, index) => <Link aria-label={`#${source.pid}${source.cid ? `/C${source.cid}` : ''}`} key={`${source.origin ?? 'local'}-${source.pid}-${source.cid ?? 0}-${index}`} to={sourceHref(source)} className="rounded-lg border border-line bg-paper/55 p-3 transition hover:border-teal/40"><span className="font-mono font-semibold text-teal">{source.origin === 'live' ? '在线 · ' : ''}#{source.pid}{source.cid ? `/C${source.cid}` : ''}</span>{source.snippet && <span className="mt-1 block line-clamp-3 leading-5 text-ink-soft">{source.snippet}</span>}</Link>)}</div></details>
}

function ResearchContent({ content, sources }: { content: string; sources: AISource[] }) {
	const lines = content.split(/\r?\n/)
	const blocks: ReactNode[] = []
	for (let index = 0; index < lines.length;) {
		const line = lines[index]
		if (!line.trim()) { index++; continue }
		if (index + 1 < lines.length && line.includes('|') && /^\s*\|?\s*:?-{3,}/.test(lines[index + 1])) {
			const headers = tableCells(line)
			index += 2
			const rows: string[][] = []
			while (index < lines.length && lines[index].includes('|') && lines[index].trim()) rows.push(tableCells(lines[index++]))
			blocks.push(<div key={`table-${index}`} className="my-3 overflow-x-auto rounded-lg border border-line"><table className="min-w-full border-collapse text-left text-xs"><thead className="bg-paper"><tr>{headers.map((cell, cellIndex) => <th key={cellIndex} className="border-b border-line px-3 py-2 font-semibold">{inlineResearchText(cell, sources)}</th>)}</tr></thead><tbody>{rows.map((row, rowIndex) => <tr key={rowIndex} className="border-b border-line/50 last:border-0">{headers.map((_, cellIndex) => <td key={cellIndex} className="px-3 py-2 align-top leading-5">{inlineResearchText(row[cellIndex] ?? '', sources)}</td>)}</tr>)}</tbody></table></div>)
			continue
		}
		const heading = line.match(/^(#{1,3})\s+(.+)$/)
		if (heading) { blocks.push(<h3 key={`heading-${index}`} className="mt-4 font-semibold">{inlineResearchText(heading[2], sources)}</h3>); index++; continue }
		if (/^[-*]\s+/.test(line)) {
			const items: string[] = []
			while (index < lines.length && /^[-*]\s+/.test(lines[index])) items.push(lines[index++].replace(/^[-*]\s+/, ''))
			blocks.push(<ul key={`list-${index}`} className="my-2 list-disc space-y-1 pl-5">{items.map((item, itemIndex) => <li key={itemIndex}>{inlineResearchText(item, sources)}</li>)}</ul>)
			continue
		}
		if (/^```/.test(line)) {
			const code: string[] = []
			index++
			while (index < lines.length && !/^```/.test(lines[index])) code.push(lines[index++])
			if (index < lines.length) index++
			blocks.push(<pre key={`code-${index}`} className="my-3 overflow-x-auto rounded-lg bg-ink p-3 font-mono text-xs leading-5 text-white"><code>{code.join('\n')}</code></pre>)
			continue
		}
		const paragraph: string[] = [line]
		index++
		while (index < lines.length && lines[index].trim() && !/^(#{1,3})\s+|^[-*]\s+|^```/.test(lines[index]) && !(index + 1 < lines.length && lines[index].includes('|') && /^\s*\|?\s*:?-{3,}/.test(lines[index + 1]))) paragraph.push(lines[index++])
		blocks.push(<p key={`paragraph-${index}`} className="my-2 whitespace-pre-wrap">{inlineResearchText(paragraph.join('\n'), sources)}</p>)
	}
	return <div>{blocks}</div>
}

function inlineResearchText(value: string, sources: AISource[]) {
	return value.split(/(\[#\d+(?:\/C\d+)?\]|\*\*[^*]+\*\*)/g).filter(Boolean).map((part, index) => {
		const citation = part.match(/^\[#(\d+)(?:\/C(\d+))?\]$/)
		if (citation) {
			const pid = Number(citation[1]); const cid = citation[2] ? Number(citation[2]) : undefined
			const source = sources.find((item) => item.pid === pid && item.cid === cid)
			return source ? <Link key={index} className="mx-0.5 font-mono font-semibold text-teal hover:underline" to={sourceHref(source)}>{part}</Link> : <span key={index} className="font-mono text-coral">{part}</span>
		}
		if (part.startsWith('**') && part.endsWith('**')) return <strong key={index}>{part.slice(2, -2)}</strong>
		return part
	})
}

function tableCells(value: string) { return value.trim().replace(/^\||\|$/g, '').split('|').map((cell) => cell.trim()) }
function sourceHref(source: AISource) { return `/posts/${source.pid}?source=${source.origin === 'live' ? 'live' : 'local'}&return_to=${encodeURIComponent('/ai')}${source.cid ? `#comment-${source.cid}` : ''}` }
