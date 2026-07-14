import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Eraser, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'

export function LogsPage() {
	const [module, setModule] = useState('all')
	const [query, setQuery] = useState('')
	const client = useQueryClient()
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
	const clear = useMutation({ mutationFn: () => api.clearLogs(module), onSuccess: () => client.invalidateQueries({ queryKey: ['logs'] }) })
	return <><PageHeader eyebrow="DIAGNOSTICS" title="运行日志" description="查看 Crawler 与 TUI 日志。接口会遮蔽 Token、Authorization 和本机数据目录。" actions={<><a className="button-secondary" href="/api/v1/diagnostics/bundle"><Download size={15} />下载安全诊断包</a><button className="button-secondary" onClick={() => logs.refetch()}><RefreshCw size={15} />刷新</button><button className="button-secondary" disabled={clear.isPending} onClick={() => clear.mutate()}><Eraser size={15} />清理当前日志</button></>} /><div className="panel mb-5 grid gap-3 p-4 sm:grid-cols-2"><label className="text-xs text-ink-soft">模块<select className="field mt-1" value={module} onChange={(event) => setModule(event.target.value)}><option value="all">全部</option><option value="crawler">Crawler</option><option value="tui">TUI</option></select></label><label className="text-xs text-ink-soft">关键词<input className="field mt-1" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="筛选日志" /></label><p className={`text-xs ${connected ? 'text-teal' : 'text-ink-soft'}`}>{connected ? '● 实时日志已连接' : '○ 正在连接实时日志；仍可手动刷新'}</p></div>{logs.isLoading ? <LoadingState /> : logs.error ? <ErrorState error={logs.error} /> : <div className="panel overflow-hidden"><pre className="max-h-[65vh] overflow-auto whitespace-pre-wrap p-5 font-mono text-xs leading-6">{displayed.length ? displayed.map((item) => `[${item.module}] ${item.line}`).join('\n') : '暂无日志'}</pre></div>}</>
}
