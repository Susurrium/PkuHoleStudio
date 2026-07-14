import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, BookOpenCheck, CircleStop, MessageSquarePlus, Search, Send, Sparkles } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import type { AISession, AISource } from '../lib/types'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'

type Mode = AISession['mode']

export function AIPage() {
  const client = useQueryClient()
  const providers = useQuery({ queryKey: ['ai-providers'], queryFn: api.aiProviders })
  const sessions = useQuery({ queryKey: ['ai-sessions'], queryFn: api.aiSessions })
  const [selectedID, setSelectedID] = useState('')
  const [mode, setMode] = useState<Mode>('local')
  const [prompt, setPrompt] = useState('')
  const [pids, setPIDs] = useState('')
  const [course, setCourse] = useState('')
  const [teachers, setTeachers] = useState('')
  const [running, setRunning] = useState(false)
  const [draftAnswer, setDraftAnswer] = useState('')
  const [trace, setTrace] = useState<string[]>([])
	const [liveSources, setLiveSources] = useState<AISource[]>([])
	const [runError, setRunError] = useState('')
  const detail = useQuery({ queryKey: ['ai-session', selectedID], queryFn: () => api.aiSession(selectedID), enabled: Boolean(selectedID) })
  const configured = providers.data?.some((provider) => provider.configured) ?? false

  useEffect(() => {
    if (!selectedID && sessions.data?.[0]) setSelectedID(sessions.data[0].id)
  }, [selectedID, sessions.data])

  const create = useMutation({ mutationFn: ({ nextMode, title }: { nextMode: Mode; title: string }) => api.createAISession(nextMode, title), onSuccess: (session) => { setSelectedID(session.id); client.invalidateQueries({ queryKey: ['ai-sessions'] }) } })
  const cancel = useMutation({ mutationFn: () => api.cancelAI(selectedID), onSuccess: () => setRunning(false) })

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!prompt.trim() || !configured || running) return
    let sessionID = selectedID
    let sessionMode = detail.data?.session.mode ?? mode
    if (!sessionID) {
      const created = await api.createAISession(mode, prompt.trim().slice(0, 36))
      sessionID = created.id
      sessionMode = created.mode
      setSelectedID(created.id)
      await client.invalidateQueries({ queryKey: ['ai-sessions'] })
    }
    const parsedPIDs = pids.split(/[\s,，]+/).map(Number).filter((value) => Number.isInteger(value) && value > 0)
		setDraftAnswer(''); setTrace([]); setLiveSources([]); setRunError(''); setRunning(true)
    try {
      await api.startAIMessage(sessionID, { prompt: prompt.trim(), pids: sessionMode === 'selected' ? parsedPIDs : undefined, course: sessionMode === 'course' ? course.trim() : undefined, teachers: sessionMode === 'course' ? teachers.split(/[、,，]+/).map((value) => value.trim()).filter(Boolean) : undefined })
      openAIStream(sessionID, {
        delta: (value) => setDraftAnswer((current) => current + value),
        trace: (value) => setTrace((current) => [...current, value]),
        source: (value) => setLiveSources((current) => current.some((item) => item.pid === value.pid && item.cid === value.cid) ? current : [...current, value]),
			done: async () => { setRunning(false); setPrompt(''); await client.invalidateQueries({ queryKey: ['ai-session', sessionID] }); await client.invalidateQueries({ queryKey: ['ai-sessions'] }) },
			error: (message) => setRunError(message),
		})
	} catch (error) {
		setRunning(false)
		setRunError(error instanceof Error ? error.message : 'AI 请求启动失败')
	}
  }

  const currentMode = detail.data?.session.mode ?? mode
  const messages = detail.data?.messages ?? []
  const shownMessages = useMemo(() => running ? [...messages, { id: 'stream', session_id: selectedID, role: 'assistant' as const, content: draftAnswer, created_at: new Date().toISOString(), sources: liveSources }] : messages, [draftAnswer, liveSources, messages, running, selectedID])

  if (providers.isLoading || sessions.isLoading) return <LoadingState label="正在打开 AI 研究台…" />
  if (providers.error || sessions.error) return <ErrorState error={providers.error || sessions.error} />

  return <>
    <PageHeader eyebrow="LOCAL RESEARCH" title="AI 研究台" description="模型通过只读工具检索本地资料，回答、检索轨迹和 PID/CID 来源会一起保存。实时树洞搜索默认不可用。" actions={<button className="button-secondary" onClick={() => { setSelectedID(''); setMode('local'); setPrompt('') }}><MessageSquarePlus size={16} />新会话</button>} />
    {!configured && <div className="mb-6 rounded-2xl border border-coral/25 bg-coral-soft/45 p-5 text-sm leading-6"><p className="font-semibold text-coral">AI Provider 尚未配置</p><p className="mt-1 text-ink-soft">前往“设置”启用 AI，并配置或新增 Provider。保存后会立即用于新会话，无需重启；DeepSeek 模板已经内置，本地无鉴权服务可以留空 API key。</p><Link to="/settings" className="mt-3 inline-flex font-semibold text-teal hover:underline">打开 AI 设置 →</Link></div>}
    <div className="grid min-h-[650px] gap-5 xl:grid-cols-[260px_1fr]">
      <aside className="panel p-4"><div className="flex items-center justify-between px-1"><p className="eyebrow">SESSIONS</p><span className="badge">{sessions.data?.length ?? 0}</span></div><div className="mt-4 grid gap-2">{sessions.data?.map((session) => <button key={session.id} onClick={() => setSelectedID(session.id)} className={`rounded-xl border p-3 text-left transition ${selectedID === session.id ? 'border-teal bg-teal-soft/45' : 'border-line bg-white/45 hover:border-teal/40'}`}><p className="truncate text-sm font-semibold">{session.title}</p><p className="mt-1 font-mono text-[10px] text-ink-soft">{modeLabel(session.mode)} · {session.model}</p></button>)}{!sessions.data?.length && <p className="rounded-xl border border-dashed border-line p-5 text-center text-xs leading-5 text-ink-soft">发送第一个问题后，会话会保存在这里。</p>}</div></aside>
      <section className="panel flex min-h-[650px] flex-col overflow-hidden">
		<div className="flex-1 space-y-5 overflow-auto p-5 md:p-7">{selectedID && detail.isLoading ? <LoadingState /> : shownMessages.length ? shownMessages.map((message) => <div key={message.id} className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}><div className={`max-w-[88%] rounded-2xl px-4 py-3 text-sm leading-7 md:max-w-[78%] ${message.role === 'user' ? 'bg-ink text-white' : 'border border-line bg-white/65'}`}><p className="whitespace-pre-wrap">{message.content || (message.id === 'stream' ? '正在思考…' : '')}</p>{message.sources?.length ? <div className="mt-3 flex flex-wrap gap-1.5 border-t border-line/60 pt-3">{message.sources.slice(0, 18).map((source, index) => <Link key={`${source.pid}-${source.cid ?? 0}-${index}`} to={`/posts/${source.pid}${source.cid ? `#comment-${source.cid}` : ''}`} className="badge hover:border-teal hover:text-teal">#{source.pid}{source.cid ? `/C${source.cid}` : ''}</Link>)}</div> : null}{message.trace && <details className="mt-3 border-t border-line/60 pt-2 text-xs text-ink-soft"><summary className="cursor-pointer font-medium">查看已保存检索轨迹</summary><pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[10px] leading-5">{formatTrace(message.trace)}</pre></details>}</div></div>) : <Welcome />}{trace.length > 0 && <div className="rounded-xl border border-teal/20 bg-teal-soft/30 p-4"><p className="eyebrow">SEARCH TRACE</p><ul className="mt-2 space-y-1 text-xs leading-5 text-ink-soft">{trace.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul></div>}</div>
        <form className="border-t border-line bg-paper/75 p-4 md:p-5" onSubmit={submit}>
          {!selectedID && <div className="mb-3 grid grid-cols-3 gap-2">{(['local', 'selected', 'course'] as Mode[]).map((item) => <button type="button" key={item} onClick={() => setMode(item)} className={`rounded-xl border px-3 py-2 text-xs font-semibold ${mode === item ? 'border-ink bg-ink text-white' : 'border-line bg-white/60'}`}>{modeLabel(item)}</button>)}</div>}
          {currentMode === 'selected' && <input className="field mb-3" value={pids} onChange={(event) => setPIDs(event.target.value)} placeholder="选中 PID，例如 123456, 234567" />}
          {currentMode === 'course' && <div className="mb-3 grid gap-2 sm:grid-cols-2"><input className="field" value={course} onChange={(event) => setCourse(event.target.value)} placeholder="课程名或别名" /><input className="field" value={teachers} onChange={(event) => setTeachers(event.target.value)} placeholder="教师，可用逗号分隔" /></div>}
          <div className="flex items-end gap-2"><textarea className="field min-h-24 flex-1 resize-y py-3" value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder={currentMode === 'course' ? '想重点比较哪些方面？' : '基于本地资料提出问题…'} /><button className="button-primary !size-11 !p-0" type="submit" disabled={!configured || !prompt.trim() || running} aria-label="发送问题"><Send size={17} /></button>{running && <button type="button" className="button-secondary !size-11 !p-0 text-coral" onClick={() => cancel.mutate()} aria-label="停止生成"><CircleStop size={18} /></button>}</div>
		<p className="mt-2 text-[11px] text-ink-soft">{modeLabel(currentMode)} · 最多 5 轮本地检索 · 回答可能出错，请回到引用原文核对</p>
		{runError && <p className="mt-2 text-xs text-coral">{runError}</p>}
        </form>
      </section>
    </div>
  </>
}

function Welcome() { return <div className="grid min-h-[360px] place-items-center text-center"><div className="max-w-md"><div className="mx-auto grid size-14 place-items-center rounded-2xl bg-ink text-white shadow-[5px_5px_0_#e4654f]"><Bot size={24} /></div><h2 className="mt-6 text-xl font-semibold">从资料出发，而不是凭空回答</h2><p className="mt-3 text-sm leading-6 text-ink-soft">选择本地自动检索、指定帖子问答或课程分析。你会看到检索关键词、命中数量和最终来源。</p><div className="mt-5 flex justify-center gap-4 text-xs text-teal"><span className="inline-flex items-center gap-1"><Search size={13} />FTS 检索</span><span className="inline-flex items-center gap-1"><BookOpenCheck size={13} />课程对比</span><span className="inline-flex items-center gap-1"><Sparkles size={13} />流式回答</span></div></div></div> }

function modeLabel(mode: Mode) { return mode === 'selected' ? '选中内容' : mode === 'course' ? '课程分析' : '本地检索' }

function openAIStream(sessionID: string, handlers: { delta: (value: string) => void; trace: (value: string) => void; source: (value: AISource) => void; done: () => void; error: (message: string) => void }) {
  const source = new EventSource(`/api/v1/ai/sessions/${sessionID}/events`)
  source.addEventListener('delta', (event) => handlers.delta(JSON.parse((event as MessageEvent).data).delta ?? ''))
  source.addEventListener('search_started', (event) => { const data = JSON.parse((event as MessageEvent).data); handlers.trace(`第 ${data.round} 轮：${data.query} · ${data.reason || '检索资料'}`) })
  source.addEventListener('search_result', (event) => { const data = JSON.parse((event as MessageEvent).data); handlers.trace(`命中 ${data.matches} 条：${data.query}`) })
  source.addEventListener('source', (event) => handlers.source(JSON.parse((event as MessageEvent).data)))
	source.addEventListener('error', (event) => { try { handlers.error(JSON.parse((event as MessageEvent).data).message ?? 'AI 运行失败') } catch { handlers.error('AI 运行失败') }; source.close(); handlers.done() })
	for (const type of ['completed', 'cancelled']) source.addEventListener(type, () => { source.close(); handlers.done() })
	source.onerror = () => { source.close(); handlers.done() }
}

function formatTrace(value: string) { try { return JSON.stringify(JSON.parse(value), null, 2) } catch { return value } }
