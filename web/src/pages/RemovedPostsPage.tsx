import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, ArchiveRestore, CloudOff, MessageCircle, RefreshCw, Search, Settings, ShieldAlert, Wifi } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { errorDescription, useFeedback } from '../components/Feedback'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import { compactNumber, formatTime } from '../lib/format'
import type { ObserverStatus, RemovedPostSummary } from '../lib/types'

type RemovedFilter = 'confirmed_unavailable' | 'restored' | 'all'

export function RemovedPostsPage() {
	const [searchParams, setSearchParams] = useSearchParams()
	const filter = normalizeFilter(searchParams.get('state'))
	const query = searchParams.get('query') ?? ''
	const rawCursor = searchParams.get('cursor')
	const cursor = rawCursor && /^\d+$/.test(rawCursor) ? Number(rawCursor) : undefined
	const [draftQuery, setDraftQuery] = useState(query)
	const client = useQueryClient()
	const { notify } = useFeedback()
	const status = useQuery({ queryKey: ['observer-status'], queryFn: api.observerStatus, retry: false, refetchInterval: 15_000 })
	const removed = useQuery({ queryKey: ['removed-posts', filter, query, cursor], queryFn: () => api.removedPosts({ state: filter === 'all' ? undefined : filter, query: query || undefined, cursor, limit: 30 }), placeholderData: (previous) => previous })
	const sync = useMutation({
		mutationFn: api.syncObserver,
		onSuccess: (result) => {
			void Promise.all([client.invalidateQueries({ queryKey: ['removed-posts'] }), client.invalidateQueries({ queryKey: ['observer-status'] })])
			notify({ title: 'Observer 同步完成', description: `应用 ${result.events_applied} 个事件，导入 ${result.snapshots_imported} 份快照。` })
		},
		onError: (error) => notify({ tone: 'error', title: 'Observer 同步失败', description: errorDescription(error) }),
	})
	const changeFilter = (next: RemovedFilter) => setSearchParams((current) => { const params = new URLSearchParams(current); if (next === 'confirmed_unavailable') params.delete('state'); else params.set('state', next); params.delete('cursor'); return params })
	const search = (event: FormEvent) => {
		event.preventDefault()
		setSearchParams((current) => { const params = new URLSearchParams(current); if (draftQuery.trim()) params.set('query', draftQuery.trim()); else params.delete('query'); params.delete('cursor'); return params })
	}

	return <>
		<PageHeader eyebrow="OBSERVER ARCHIVE" title="已删除或不可访问" description="这里展示 Observer 曾经完整见过、后来经过二次确认不可访问的洞。内容已同步到本机，Observer 离线时仍可查看。" actions={<><button className="button-primary" disabled={sync.isPending || status.data?.connected !== true} onClick={() => sync.mutate()}><RefreshCw className={sync.isPending ? 'animate-spin' : ''} size={15} />{sync.isPending ? '正在同步…' : '立即同步'}</button><Link className="button-secondary" to="/settings#observer-service"><Settings size={15} />Observer 设置</Link></>} />
		<ObserverArchiveBanner status={status.data} error={status.error} refresh={() => status.refetch()} />
		<div className="panel mb-5 p-3 sm:p-4">
			<div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between"><nav className="grid grid-cols-3 rounded-xl border border-line bg-paper/45 p-1" aria-label="删除记录状态"><FilterButton active={filter === 'confirmed_unavailable'} onClick={() => changeFilter('confirmed_unavailable')}>不可访问</FilterButton><FilterButton active={filter === 'restored'} onClick={() => changeFilter('restored')}>已恢复</FilterButton><FilterButton active={filter === 'all'} onClick={() => changeFilter('all')}>全部记录</FilterButton></nav><form className="flex min-w-0 gap-2 lg:w-[420px]" role="search" onSubmit={search}><label className="relative min-w-0 flex-1"><span className="sr-only">搜索删除归档</span><Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft" size={15} /><input className="field !pl-9" value={draftQuery} onChange={(event) => setDraftQuery(event.target.value)} placeholder="搜索 PID 或正文" /></label><button className="button-secondary shrink-0">搜索</button></form></div>
		</div>
		{removed.isLoading ? <LoadingState label="正在读取本地删除归档…" /> : removed.error ? <ErrorState error={removed.error} /> : !removed.data?.items.length ? <EmptyState title={query ? '没有匹配的删除归档' : filter === 'restored' ? '还没有恢复记录' : '还没有确认不可访问的洞'} description={query ? '可以清除搜索词或切换状态后重试。' : 'Observer 只会归档自己真实抓到过正文，并经过两次明确不可访问确认的洞；PID 缺口不会出现在这里。'} action={query ? <button className="button-secondary" onClick={() => { setDraftQuery(''); setSearchParams((current) => { const params = new URLSearchParams(current); params.delete('query'); params.delete('cursor'); return params }) }}>清除搜索</button> : undefined} /> : <>
			<div className={`grid gap-4 ${removed.isPlaceholderData ? 'opacity-60' : ''}`} aria-busy={removed.isFetching}>{removed.data.items.map((item) => <RemovedPostCard key={`${item.pid}-${item.observed_at}`} item={item} />)}</div>
			{removed.data.has_more && removed.data.next_cursor !== undefined && <div className="mt-5 flex justify-center"><button className="button-secondary" disabled={removed.isFetching} onClick={() => setSearchParams((current) => { const params = new URLSearchParams(current); params.set('cursor', String(removed.data!.next_cursor!)); return params })}>查看更早记录</button></div>}
		</>}
	</>
}

function ObserverArchiveBanner({ status, error, refresh }: { status?: ObserverStatus; error: unknown; refresh: () => unknown }) {
	if (error) return <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="status"><div className="flex min-w-0 items-start gap-3"><CloudOff className="mt-0.5 shrink-0 text-coral" size={20} /><div><p className="font-semibold">Observer 当前不可达</p><p className="mt-1 text-xs leading-5 text-ink-soft">{errorDescription(error)}。下面仍然是本机已经同步的数据。</p></div></div><button className="button-secondary" onClick={refresh}><RefreshCw size={14} />重新检测</button></div>
	if (!status) return null
	if (!status.configured || !status.enabled) return <div className="mb-5 flex items-start gap-3 rounded-xl border border-line bg-white/45 p-4"><ShieldAlert className="mt-0.5 shrink-0 text-ink-soft" size={20} /><div><p className="font-semibold">Observer 尚未启用</p><p className="mt-1 text-xs leading-5 text-ink-soft">可以浏览本机已有记录；配置自建 Observer 后才会继续收到新的删除事件。</p></div></div>
	if (!status.connected || status.challenge_required || status.stale || status.coverage_degraded) return <div className="mb-5 flex items-start gap-3 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="status"><AlertTriangle className="mt-0.5 shrink-0 text-coral" size={20} /><div><p className="font-semibold">{status.challenge_required ? 'Observer 等待短信验证' : !status.connected ? 'Observer 当前离线' : status.coverage_degraded ? 'Observer 扫描覆盖不足' : 'Observer 数据已经过期'}</p><p className="mt-1 text-xs leading-5 text-ink-soft">{status.last_error || (status.challenge_required ? '前往设置提交验证码；本地归档不受影响。' : '新的删除可能尚未同步，已有本地归档仍可正常查看。')}</p></div></div>
	return <div className="mb-5 flex flex-wrap items-center gap-2 rounded-xl border border-teal/25 bg-teal-soft/20 px-4 py-3 text-xs text-teal" role="status"><Wifi size={15} /><span className="font-semibold">Observer 正常运行</span><span className="text-ink-soft">最近扫描 {formatISO(status.last_successful_scan_at)}</span></div>
}

function RemovedPostCard({ item }: { item: RemovedPostSummary }) {
	const unavailable = item.state === 'confirmed_unavailable'
	return <article className="panel overflow-hidden transition hover:-translate-y-0.5 hover:shadow-md">
		<Link className="block px-5 py-5 md:px-6" to={`/removed/${item.pid}`} aria-label={`查看删除归档 #${item.pid}`}>
			<div className="flex flex-wrap items-center justify-between gap-3"><div className="flex flex-wrap items-center gap-2"><span className="font-mono text-sm font-bold text-coral">#{item.pid}</span><span className={`badge ${unavailable ? '!border-coral/30 !bg-coral-soft/25 !text-coral' : '!border-teal/30 !bg-teal-soft/25 !text-teal'}`}>{unavailable ? '已删除或不可访问' : '已恢复'}</span><CompletenessBadge value={item.completeness} /></div><time className="text-xs text-ink-soft">{formatISO(item.observed_at)}</time></div>
			<p className="mt-4 line-clamp-4 whitespace-pre-wrap text-sm leading-7 md:text-base">{item.post?.text || '（归档中没有正文）'}</p>
			<footer className="mt-4 flex flex-wrap items-center gap-4 text-xs text-ink-soft"><span>发布于 {formatTime(item.post?.timestamp)}</span><span className="inline-flex items-center gap-1"><MessageCircle size={13} />{compactNumber(item.post?.reply ?? 0)} 条评论</span>{item.post?.praise_num !== undefined && <span>点赞 {compactNumber(item.post.praise_num)}</span>}{item.restored_at && <span className="inline-flex items-center gap-1 text-teal"><ArchiveRestore size={13} />{formatISO(item.restored_at)} 恢复</span>}</footer>
		</Link>
	</article>
}

function CompletenessBadge({ value }: { value?: string }) {
	if (value === 'complete') return <span className="badge !border-teal/25 !text-teal">完整归档</span>
	if (value === 'partial') return <span className="badge !border-coral/25 !text-coral">部分归档</span>
	return <span className="badge">完整度未知</span>
}

function FilterButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
	return <button className={`rounded-lg px-3 py-2 text-xs font-semibold transition sm:px-5 ${active ? 'bg-ink text-white shadow-sm' : 'text-ink-soft hover:bg-white hover:text-ink'}`} aria-pressed={active} onClick={onClick}>{children}</button>
}

function normalizeFilter(value: string | null): RemovedFilter {
	return value === 'restored' || value === 'all' ? value : 'confirmed_unavailable'
}

function formatISO(value?: string) {
	if (!value) return '时间未知'
	const date = new Date(value)
	return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
