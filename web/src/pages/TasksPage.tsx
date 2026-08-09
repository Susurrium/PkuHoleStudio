import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { CircleAlert, ListTodo } from 'lucide-react'
import { errorDescription } from '../components/Feedback'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { Job } from '../lib/types'

const activeStatuses = new Set(['queued', 'running'])
const attentionStatuses = new Set(['failed', 'partial'])

export function TasksPage() {
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 5_000 })
  if (jobs.isLoading) return <LoadingState label="正在读取任务…" />
	if (jobs.error) return <><PageHeader eyebrow="后台任务" title="任务" description="查看后台工作进度和结果。" /><FriendlyError title="暂时无法读取任务" description="请刷新页面后重试；正在运行的任务不会因此停止。" error={jobs.error} /></>

	const rows = jobs.data ?? []
	const retiredMonitors = rows.filter(isRetiredMonitor)
	const active = newestFirst(rows.filter((job) => activeStatuses.has(job.status) && !isRetiredMonitor(job)))
	const paused = newestFirst(rows.filter((job) => job.status === 'paused' && !isRetiredMonitor(job)))
	const attention = newestFirst([...retiredMonitors, ...rows.filter((job) => attentionStatuses.has(job.status))])
	const history = newestFirst(rows.filter((job) => job.status === 'completed'))
	const cancelled = newestFirst(rows.filter((job) => job.status === 'cancelled'))

  return <>
	<PageHeader eyebrow="后台任务" title="任务" description="查看正在进行的工作、处理失败任务，或回看完成结果。任务运行时可以离开本页。" />
	{!rows.length ? <EmptyState title="还没有任务" description="开始保存在线内容、导入、导出或资料维护后，进度会显示在这里。" action={<ListTodo size={19} />} /> : <div className="grid gap-7">
	  <TaskSection title="正在进行" description="这里的任务会自动更新进度" jobs={active} empty="当前没有进行中的任务" />
	  {attention.length > 0 && <TaskSection title="需要处理" description="按卡片提示重试、取消或重新发起" jobs={attention} icon={<CircleAlert size={18} className="text-coral" />} />}
	  {paused.length > 0 && <TaskSection title="已暂停" description="可以继续的任务会从原进度恢复" jobs={paused} />}
	  {(history.length > 0 || cancelled.length > 0) && <TaskHistory completed={history.slice(0, 30)} cancelled={cancelled.slice(0, 20)} />}
    </div>}
  </>
}

function TaskSection({ title, description, jobs, empty, icon }: { title: string; description: string; jobs: Job[]; empty?: string; icon?: ReactNode }) {
  return <section className="panel p-5 md:p-6"><div className="flex items-center justify-between gap-3"><div><div className="flex items-center gap-2"><h2 className="text-xl font-semibold">{title}</h2>{icon}</div><p className="mt-1 text-xs text-ink-soft">{description}</p></div><span className="badge">{jobs.length} 条</span></div><div className="mt-5 grid gap-3">{jobs.length ? jobs.map((job) => <TaskJob key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-6 text-center text-sm text-ink-soft">{empty}</p>}</div></section>
}

function TaskJob({ job }: { job: Job }) {
	const client = useQueryClient()
	const action = useMutation({ mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
	return <div><JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />{action.error && <InlineError message="未能更新任务，请稍后重试。" error={action.error} />}</div>
}

function TaskHistory({ completed, cancelled }: { completed: Job[]; cancelled: Job[] }) {
	return <details className="panel p-5 md:p-6"><summary className="cursor-pointer list-none"><div className="flex items-center justify-between gap-3"><div><h2 className="text-xl font-semibold">历史记录</h2><p className="mt-1 text-xs text-ink-soft">已完成和已取消的任务</p></div><span className="badge">{completed.length + cancelled.length} 条 · 展开</span></div></summary><div className="mt-5 grid gap-6 border-t border-line pt-5">{completed.length > 0 && <HistoryGroup title="已完成" jobs={completed} />}{cancelled.length > 0 && <HistoryGroup title="已取消" jobs={cancelled} />}</div></details>
}

function HistoryGroup({ title, jobs }: { title: string; jobs: Job[] }) {
	return <section><div className="mb-3 flex items-center justify-between gap-3"><h3 className="font-semibold">{title}</h3><span className="text-xs text-ink-soft">{jobs.length} 条</span></div><div className="grid gap-3">{jobs.map((job) => <TaskJob key={job.id} job={job} />)}</div></section>
}

function newestFirst(jobs: Job[]) {
	return [...jobs].sort((left, right) => new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime())
}

function InlineError({ message, error }: { message: string; error: unknown }) {
	return <div className="mt-2 text-xs text-coral" role="alert"><p>{message}</p><details className="mt-1 text-ink-soft"><summary className="cursor-pointer">查看技术详情</summary><p className="mt-1 break-all font-mono">{errorDescription(error)}</p></details></div>
}

function FriendlyError({ title, description, error }: { title: string; description: string; error: unknown }) {
	return <section className="panel border-coral/30 bg-coral-soft/30 p-6" role="alert"><h2 className="font-semibold">{title}</h2><p className="mt-1 text-sm leading-6 text-ink-soft">{description}</p><details className="mt-4 text-xs text-ink-soft"><summary className="cursor-pointer font-medium">查看技术详情</summary><p className="mt-2 break-all font-mono">{errorDescription(error)}</p></details></section>
}

function isRetiredMonitor(job: Job) {
	return job.type === 'monitor_latest' && ['queued', 'running', 'paused'].includes(job.status)
}
