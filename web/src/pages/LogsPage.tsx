import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Eraser, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { errorDescription, useFeedback } from '../components/Feedback'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'

export function LogsPage() {
	const [module, setModule] = useState('all')
	const [query, setQuery] = useState('')
	const client = useQueryClient()
	const { confirm, notify } = useFeedback()
	const logs = useQuery({ queryKey: ['logs', module, query], queryFn: () => api.logs(module, query) })
	const [streamed, setStreamed] = useState<import('../lib/types').LogLine[]>([])
	const [connected, setConnected] = useState(false)
	useEffect(() => {
		setStreamed([])
		const source = new EventSource(`/api/v1/logs/events?module=${encodeURIComponent(module)}&q=${encodeURIComponent(query)}`)
		source.addEventListener('ready', () => setConnected(true))
		source.addEventListener('line', (event) => {
			try { setStreamed((current) => [...current.slice(-1999), JSON.parse((event as MessageEvent).data)]) } catch { /* ignore malformed diagnostic lines */ }
		})
		source.onerror = () => { setConnected(false); logs.refetch() }
		return () => { source.close(); setConnected(false) }
	}, [module, query])
	const displayed = useMemo(() => [...(logs.data ?? []), ...streamed], [logs.data, streamed])
	const clear = useMutation({
		mutationFn: () => api.clearLogs(module),
		onSuccess: () => { setStreamed([]); client.invalidateQueries({ queryKey: ['logs'] }); notify({ title: '日志已清理', description: module === 'all' ? 'Crawler 与 TUI 日志已清空。' : `${module} 日志已清空。` }) },
		onError: (error) => notify({ tone: 'error', title: '日志清理失败', description: errorDescription(error) }),
	})
	const requestClear = async () => {
		const accepted = await confirm({ title: module === 'all' ? '清理全部运行日志？' : `清理 ${module} 日志？`, description: '清理后无法在页面恢复这些日志。帖子、评论、任务记录和本机设置不会受影响。', confirmLabel: '确认清理', tone: 'danger' })
		if (accepted) clear.mutate()
	}
	return <><PageHeader eyebrow="DIAGNOSTICS" title="运行日志" description="查看 Crawler 与 TUI 的实时运行信息；敏感凭据和本机数据目录会在接口层遮蔽。" actions={<><a className="button-secondary" href="/api/v1/diagnostics/bundle"><Download size={15} />下载安全诊断包</a><button className="button-secondary" disabled={logs.isFetching} onClick={() => logs.refetch()}><RefreshCw size={15} />{logs.isFetching ? '刷新中…' : '刷新'}</button><button className="button-secondary !text-coral" disabled={clear.isPending || displayed.length === 0} onClick={requestClear}><Eraser size={15} />{clear.isPending ? '清理中…' : '清理当前日志'}</button></>} /><div className="panel mb-5 grid gap-4 p-4 sm:grid-cols-2"><label className="text-xs font-medium text-ink-soft">日志来源<select className="field mt-1.5" value={module} onChange={(event) => setModule(event.target.value)}><option value="all">全部来源</option><option value="crawler">Crawler</option><option value="tui">TUI</option></select></label><label className="text-xs font-medium text-ink-soft">页面内筛选<input className="field mt-1.5" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="输入关键词筛选日志" /></label><p className={`sm:col-span-2 text-xs ${connected ? 'text-teal' : 'text-ink-soft'}`}>{connected ? '● 实时日志已连接，新内容会自动追加' : '○ 实时连接暂不可用，仍可手动刷新快照'}</p></div>{logs.isLoading ? <LoadingState label="正在读取日志…" /> : logs.error ? <ErrorState error={logs.error} /> : <div className="panel overflow-hidden"><div className="flex items-center justify-between border-b border-line px-4 py-3"><span className="text-xs font-semibold">日志输出</span><span className="badge">{displayed.length} 行</span></div><pre className="max-h-[65vh] overflow-auto whitespace-pre-wrap p-5 font-mono text-xs leading-6">{displayed.length ? displayed.map((item) => `[${item.module}] ${item.line}`).join('\n') : '暂无符合条件的日志'}</pre></div>}</>
}
