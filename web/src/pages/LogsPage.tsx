import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Eraser, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'

export function LogsPage() {
	const [module, setModule] = useState('all')
	const [query, setQuery] = useState('')
	const client = useQueryClient()
	const logs = useQuery({ queryKey: ['logs', module, query], queryFn: () => api.logs(module, query), refetchInterval: 5000 })
	const clear = useMutation({ mutationFn: () => api.clearLogs(module), onSuccess: () => client.invalidateQueries({ queryKey: ['logs'] }) })
	return <><PageHeader eyebrow="DIAGNOSTICS" title="运行日志" description="查看 Crawler 与 TUI 日志。接口会遮蔽 Token、Authorization 和本机数据目录。" actions={<><button className="button-secondary" onClick={() => logs.refetch()}><RefreshCw size={15} />刷新</button><button className="button-secondary" disabled={clear.isPending} onClick={() => clear.mutate()}><Eraser size={15} />清理当前日志</button></>} /><div className="panel mb-5 grid gap-3 p-4 sm:grid-cols-2"><label className="text-xs text-ink-soft">模块<select className="field mt-1" value={module} onChange={(event) => setModule(event.target.value)}><option value="all">全部</option><option value="crawler">Crawler</option><option value="tui">TUI</option></select></label><label className="text-xs text-ink-soft">关键词<input className="field mt-1" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="筛选日志" /></label></div>{logs.isLoading ? <LoadingState /> : logs.error ? <ErrorState error={logs.error} /> : <div className="panel overflow-hidden"><pre className="max-h-[65vh] overflow-auto whitespace-pre-wrap p-5 font-mono text-xs leading-6">{logs.data?.length ? logs.data.map((item) => `[${item.module}] ${item.line}`).join('\n') : '暂无日志'}</pre></div>}</>
}
