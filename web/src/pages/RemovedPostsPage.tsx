import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, ArchiveRestore, CloudOff, RefreshCw, Search, Settings, ShieldAlert, Wifi } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { errorDescription } from '../components/Feedback'
import { PageHeader } from '../components/PageHeader'
import { PostMetrics } from '../components/PostMetrics'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import type { ObserverStatus, RemovedPostSummary } from '../lib/types'

type RemovedFilter = 'confirmed_unavailable' | 'restored' | 'all'

export function RemovedPostsPage() {
	const [searchParams, setSearchParams] = useSearchParams()
	const filter = normalizeFilter(searchParams.get('state'))
	const query = searchParams.get('query') ?? ''
	const rawCursor = searchParams.get('cursor')
	const cursor = rawCursor && /^\d+$/.test(rawCursor) ? Number(rawCursor) : undefined
	const [draftQuery, setDraftQuery] = useState(query)
	const status = useQuery({ queryKey: ['observer-status'], queryFn: api.observerStatus, retry: false, refetchInterval: 15_000 })
	const removed = useQuery({
		queryKey: ['removed-posts', filter, query, cursor],
		queryFn: () => api.removedPosts({ state: filter, query: query || undefined, cursor, limit: 30 }),
		placeholderData: (previous) => previous,
		refetchInterval: 60_000,
	})
	const changeFilter = (next: RemovedFilter) => setSearchParams((current) => { const params = new URLSearchParams(current); if (next === 'confirmed_unavailable') params.delete('state'); else params.set('state', next); params.delete('cursor'); return params })
	const search = (event: FormEvent) => {
		event.preventDefault()
		setSearchParams((current) => { const params = new URLSearchParams(current); if (draftQuery.trim()) params.set('query', draftQuery.trim()); else params.delete('query'); params.delete('cursor'); return params })
	}

	return <>
		<PageHeader eyebrow="Observer 实时记录" title="最近删除" description="直接查看 Observer 已确认不可访问或后来恢复的树洞；需要离线使用时再单独保存到本机。" actions={<><button className="button-primary" disabled={removed.isFetching} onClick={() => void Promise.all([removed.refetch(), status.refetch()])}><RefreshCw className={removed.isFetching ? 'animate-spin' : ''} size={15} />{removed.isFetching ? '正在刷新…' : '刷新记录'}</button><Link className="button-secondary" to="/settings#observer-service"><Settings size={15} />服务设置</Link></>} />
		<ObserverArchiveBanner status={status.data} error={status.error} refresh={() => status.refetch()} />
		<RemovedSourceStatus source={removed.data?.source} stale={removed.data?.stale} message={removed.data?.message} total={removed.data?.total} />
		<div className="panel mb-5 p-3 sm:p-4">
			<div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between"><nav className="grid grid-cols-3 rounded-xl border border-line bg-paper/45 p-1" aria-label="删除记录状态"><FilterButton active={filter === 'confirmed_unavailable'} onClick={() => changeFilter('confirmed_unavailable')}>不可访问</FilterButton><FilterButton active={filter === 'restored'} onClick={() => changeFilter('restored')}>已恢复</FilterButton><FilterButton active={filter === 'all'} onClick={() => changeFilter('all')}>全部记录</FilterButton></nav><form className="flex min-w-0 gap-2 lg:w-[420px]" role="search" onSubmit={search}><label className="relative min-w-0 flex-1"><span className="sr-only">搜索删除记录 PID</span><Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft" size={15} /><input className="field !pl-9" inputMode="numeric" value={draftQuery} onChange={(event) => setDraftQuery(event.target.value)} placeholder="输入 PID，例如 8449490" /></label><button className="button-secondary shrink-0">搜索</button></form></div>
		</div>
		{removed.isLoading ? <LoadingState label="正在读取 Observer 最近删除…" /> : removed.error ? <ErrorState error={removed.error} /> : !removed.data?.items.length ? <EmptyState title={query ? '没有匹配的删除记录' : filter === 'restored' ? '还没有恢复记录' : '还没有确认不可访问的洞'} description={query ? '请确认 PID 正确，或切换状态后重试。' : 'Observer 只有在完整扫描和详情复核后才会确认不可访问。'} action={query ? <button className="button-secondary" onClick={() => { setDraftQuery(''); setSearchParams((current) => { const params = new URLSearchParams(current); params.delete('query'); params.delete('cursor'); return params }) }}>清除搜索</button> : undefined} /> : <>
			<div className={`grid gap-4 ${removed.isPlaceholderData ? 'opacity-60' : ''}`} aria-busy={removed.isFetching}>{removed.data.items.map((item) => <RemovedPostCard key={`${item.pid}-${item.observed_at}`} item={item} />)}</div>
			{removed.data.has_more && removed.data.next_cursor !== undefined && <div className="mt-5 flex justify-center"><button className="button-secondary" disabled={removed.isFetching} onClick={() => setSearchParams((current) => { const params = new URLSearchParams(current); params.set('cursor', String(removed.data!.next_cursor!)); return params })}>查看更早记录</button></div>}
		</>}
	</>
}

function RemovedSourceStatus({ source, stale, message, total }: { source?: string; stale?: boolean; message?: string; total?: number }) {
	if (!source) return null
	const remote = source === 'observer'
	return <div className={`mb-5 flex flex-wrap items-center gap-2 rounded-xl border px-4 py-3 text-xs ${remote && !stale ? 'border-teal/25 bg-teal-soft/20 text-teal' : 'border-coral/25 bg-coral-soft/20 text-coral'}`} role="status">
		{remote && !stale ? <Wifi size={15} /> : <CloudOff size={15} />}
		<span className="font-semibold">{remote && !stale ? '正在直接读取 Observer' : '正在显示本机缓存'}</span>
		{typeof total === 'number' && total > 0 ? <span className="text-ink-soft">共 {total} 条记录</span> : null}
		{message ? <span className="text-ink-soft">{message}</span> : null}
	</div>
}

function ObserverArchiveBanner({ status, error, refresh }: { status?: ObserverStatus; error: unknown; refresh: () => unknown }) {
	if (error) return <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="status"><div className="flex min-w-0 items-start gap-3"><CloudOff className="mt-0.5 shrink-0 text-coral" size={20} /><div><p className="font-semibold">删除记录服务暂时无法连接</p><p className="mt-1 text-xs leading-5 text-ink-soft">如有本机缓存，页面会继续显示并标明来源。</p><details className="mt-1 text-xs text-ink-soft"><summary className="cursor-pointer font-semibold">查看技术详情</summary><p className="mt-1 break-words">{errorDescription(error)}</p></details></div></div><button className="button-secondary" onClick={refresh}><RefreshCw size={14} />重新检测</button></div>
	if (!status) return null
	if (!status.configured || !status.enabled) return <div className="mb-5 flex items-start gap-3 rounded-xl border border-line bg-white/45 p-4"><ShieldAlert className="mt-0.5 shrink-0 text-ink-soft" size={20} /><div><p className="font-semibold">删除记录服务尚未启用</p><p className="mt-1 text-xs leading-5 text-ink-soft">已有本地记录仍可查看；启用服务后可直接读取 Observer 的最新记录。</p></div></div>
	if (!status.connected || status.challenge_required || status.stale || status.coverage_degraded) {
		const waitingForOTP = status.challenge_required && status.challenge === 'otp'
		return <div className="mb-5 flex items-start gap-3 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="status"><AlertTriangle className="mt-0.5 shrink-0 text-coral" size={20} /><div><p className="font-semibold">{status.challenge_required ? waitingForOTP ? '需要手机令牌' : '需要短信验证' : !status.connected ? '删除记录服务当前离线' : status.coverage_degraded ? '最近记录可能不完整' : '删除记录尚未更新'}</p><p className="mt-1 text-xs leading-5 text-ink-soft">{status.challenge_required ? waitingForOTP ? '前往设置提交北京大学 App 的 6 位动态口令；本地归档不受影响。' : '前往设置提交短信验证码；本地归档不受影响。' : '新的删除可能尚未同步，已有本地归档仍可正常查看。'}</p>{status.last_error ? <details className="mt-1 text-xs text-ink-soft"><summary className="cursor-pointer font-semibold">查看技术详情</summary><p className="mt-1 break-words">{status.last_error}</p></details> : null}</div></div>
	}
	return <div className="mb-5 flex flex-wrap items-center gap-2 rounded-xl border border-teal/25 bg-teal-soft/20 px-4 py-3 text-xs text-teal" role="status"><Wifi size={15} /><span className="font-semibold">删除记录已连接</span><span className="text-ink-soft">Observer 最近扫描 {formatISO(status.last_successful_scan_at)}</span></div>
}

function RemovedPostCard({ item }: { item: RemovedPostSummary }) {
	const unavailable = item.state === 'confirmed_unavailable'
	return <article className="panel overflow-hidden transition hover:-translate-y-0.5 hover:shadow-md">
		<Link className="block px-5 py-5 md:px-6" to={`/removed/${item.pid}`} aria-label={`查看删除记录 #${item.pid}`}>
			<div className="flex flex-wrap items-center justify-between gap-3"><div className="flex flex-wrap items-center gap-2"><span className="font-mono text-sm font-bold text-coral">#{item.pid}</span><span className={`badge ${unavailable ? '!border-coral/30 !bg-coral-soft/25 !text-coral' : '!border-teal/30 !bg-teal-soft/25 !text-teal'}`}>{unavailable ? '已删除或不可访问' : '已恢复'}</span><CompletenessBadge value={item.completeness} /></div><time className="text-xs text-ink-soft">{formatISO(item.observed_at)}</time></div>
			<p className="mt-4 line-clamp-4 whitespace-pre-wrap text-sm leading-7 md:text-base">{item.post?.text || '（没有保存到正文）'}</p>
			<footer className="mt-4 flex flex-wrap items-center gap-4 text-xs text-ink-soft"><span>发布于 {formatTime(item.post?.timestamp)}</span>{item.post ? <PostMetrics post={item.post} /> : null}{item.restored_at && <span className="inline-flex items-center gap-1 text-teal"><ArchiveRestore size={13} />{formatISO(item.restored_at)} 恢复</span>}</footer>
		</Link>
	</article>
}

function CompletenessBadge({ value }: { value?: string }) {
	if (value === 'complete') return <span className="badge !border-teal/25 !text-teal">完整保存</span>
	if (value === 'partial') return <span className="badge !border-coral/25 !text-coral">部分保存</span>
	return <span className="badge">保存情况未知</span>
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
