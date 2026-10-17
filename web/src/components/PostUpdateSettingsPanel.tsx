import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { api } from '../lib/api'
import type { PostUpdateState } from '../lib/types'
import { errorDescription } from './Feedback'
import { usePostUpdatesAvailable, usePostUpdatesState } from '../features/posts/updates'

export function PostUpdateSettingsPanel() {
	const available = usePostUpdatesAvailable()
	const state = usePostUpdatesState()
	const section = useRef<HTMLElement>(null)
	const location = useLocation()
	useEffect(() => {
		if (!available || location.hash !== '#continuous') return
		const frame = requestAnimationFrame(() => section.current?.scrollIntoView({ block: 'start' }))
		return () => cancelAnimationFrame(frame)
	}, [available, location.hash])
	if (!available) return null
	return <section ref={section} id="continuous" className="panel mt-5 scroll-mt-24 p-5">
		<h2 className="text-lg font-semibold">持续更新关注洞</h2>
		<p className="mt-2 text-sm leading-6 text-ink-soft">选择已经保存、当前仍在关注的树洞，最多 20 个。程序运行期间执行，关闭浏览器后仍可继续。</p>
		<p className="mt-1 text-xs leading-5 text-ink-soft">每轮最多 100 次请求，未完成的分页下轮继续；所有读取共用请求间隔和限流退避。</p>
		{state.isLoading ? <p className="mt-3" role="status">正在读取设置…</p> : state.error ? <p className="mt-3 text-coral" role="alert">{errorDescription(state.error)}</p> : state.data?.account ? <UpdateSettingsForm key={`${state.data.account}:${state.data.enabled}:${state.data.interval_minutes}:${state.data.pids.join(',')}`} value={state.data} /> : <p className="mt-3 text-sm text-coral">{state.data?.error || '登录后可以配置持续更新。'}</p>}
	</section>
}

function UpdateSettingsForm({ value }: { value: PostUpdateState }) {
	const client = useQueryClient()
	const [enabled, setEnabled] = useState(value.enabled)
	const [interval, setInterval] = useState(value.interval_minutes)
	const [text, setText] = useState(value.pids.join(', '))
	const values = text.trim() ? text.trim().split(/[\s,，]+/) : []
	const pids = values.map(Number)
	const valid = pids.length <= 20 && pids.every((pid) => Number.isInteger(pid) && pid > 0) && new Set(pids).size === pids.length
	const save = useMutation({ mutationFn: () => api.setPostUpdates({ enabled, interval_minutes: interval, pids }), onSuccess: (state) => client.setQueryData(['post-updates'], state) })
	return <form className="mt-4 grid gap-3" onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate() }}>
		<label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />启用持续更新</label>
		<label className="text-sm">更新周期<select className="field mt-1" value={interval} onChange={(event) => setInterval(Number(event.target.value))}><option value={30}>每 30 分钟</option><option value={60}>每 60 分钟</option><option value={120}>每 120 分钟</option></select></label>
		<label className="text-sm">树洞 PID（最多 20 个）<textarea className="field mt-1" value={text} onChange={(event) => setText(event.target.value)} placeholder="使用空格或逗号分隔 PID" /></label>
		{!valid && <p className="text-xs text-coral" role="alert">请输入最多 20 个互不重复的正整数 PID。</p>}
		<button className="button-primary justify-self-start" disabled={!valid || save.isPending}>{save.isPending ? '正在验证并保存…' : '保存更新设置'}</button>
		<p className="text-xs text-ink-soft">{value.enabled ? `下轮计划：${new Date(value.next_run).toLocaleString('zh-CN')}` : '持续更新已关闭'}{value.running ? ' · 正在更新' : ''}</p>
		{value.targets.filter((target) => target.error || (target.page ?? 0) > 1).map((target) => <p className="text-xs text-coral" key={target.pid}>#{target.pid}：{target.error || `评论读取到第 ${target.page} 页，将继续补全`}</p>)}
		{(save.error || value.error) && <p className="text-xs text-coral" role="alert">{save.error ? errorDescription(save.error) : value.error}</p>}
		{save.isSuccess && <p className="text-xs text-teal" role="status">设置已保存</p>}
	</form>
}
