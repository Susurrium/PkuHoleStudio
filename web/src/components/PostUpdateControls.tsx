import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { RefreshCw, Star } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { errorDescription } from './Feedback'
import { useSavePostToLocal } from '../features/posts/usePostDetail'
import { invalidatePostContent, usePostUpdatesAvailable, usePostUpdatesState } from '../features/posts/updates'

export function PostUpdateMonitor() {
	const client = useQueryClient()
	const state = usePostUpdatesState()
	const previous = useRef<string | undefined>(undefined)
	useEffect(() => {
		if (!state.data) return
		const revision = `${state.data.account || ''}:${state.data.revision}`
		if (previous.current !== undefined && previous.current !== revision) void invalidatePostContent(client)
		previous.current = revision
	}, [client, state.data])
	return null
}

export function PostUpdateControls({ pid, saved, source, refreshError, refreshing, onRefresh }: {
	pid: string | number; saved: boolean; source: 'local' | 'live'; refreshError?: Error | null; refreshing?: boolean; onRefresh?: () => void
}) {
	const client = useQueryClient()
	const available = usePostUpdatesAvailable()
	const state = usePostUpdatesState()
	const save = useSavePostToLocal(pid)
	const watched = state.data?.pids.includes(Number(pid)) ?? false
	const watch = useMutation({
		mutationFn: () => {
			if (!state.data?.account) throw new Error('请先登录树洞')
			const pids = watched ? state.data.pids.filter((value) => value !== Number(pid)) : [...state.data.pids, Number(pid)]
			return api.setPostUpdates({ enabled: pids.length > 0 && (watched ? state.data.enabled : true), interval_minutes: state.data.interval_minutes, pids })
		},
		onSuccess: (value) => client.setQueryData(['post-updates'], value),
	})
	if (!available) return null
	return <section className="panel mb-5 flex flex-wrap items-center gap-3 p-3 text-xs" aria-label={`树洞 #${pid} 的更新操作`}>
		{saved ? <button type="button" className="button-secondary" disabled={save.active || save.isPending} onClick={save.update}><RefreshCw size={14} />{save.active ? save.label : '完整更新本地内容'}</button> : source === 'live' && <button type="button" className="button-secondary" disabled={refreshing} onClick={onRefresh}><RefreshCw size={14} />刷新在线内容</button>}
		{saved && <button type="button" className="button-secondary" disabled={watch.isPending || state.isLoading || (!watched && (state.data?.pids.length ?? 0) >= 20)} onClick={() => watch.mutate()}><Star size={14} />{watch.isPending ? '正在检查关注状态…' : watched ? '停止持续更新' : '持续更新此洞'}</button>}
		{saved && <Link to="/sync#continuous" className="font-semibold text-teal hover:underline">更新设置</Link>}
		{refreshing && <span role="status">正在获取线上更新，本地内容仍可阅读…</span>}
		{(refreshError || watch.error || save.failure) && <span className="basis-full text-coral" role="alert">{refreshError ? `自动刷新失败：${errorDescription(refreshError)}` : watch.error ? errorDescription(watch.error) : save.failure}</span>}
		{save.isSuccess && <span role="status">本地更新任务已完成</span>}
	</section>
}
