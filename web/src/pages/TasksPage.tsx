import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { CircleAlert, ListTodo } from 'lucide-react'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { Job } from '../lib/types'

const activeStatuses = new Set(['queued', 'running'])
const attentionStatuses = new Set(['failed', 'partial'])

export function TasksPage() {
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 5_000 })
  if (jobs.isLoading) return <LoadingState label="正在读取任务…" />
  if (jobs.error) return <ErrorState error={jobs.error} />

  const rows = jobs.data ?? []
  const active = rows.filter((job) => activeStatuses.has(job.status))
  const paused = rows.filter((job) => job.status === 'paused')
  const attention = rows.filter((job) => attentionStatuses.has(job.status))
  const history = rows.filter((job) => job.status === 'completed')
  const cancelled = rows.filter((job) => job.status === 'cancelled')

  return <>
    <PageHeader eyebrow="TASKS" title="任务" description="同步、导入、导出和资料维护都会在后台继续运行。你可以离开页面，稍后在这里查看进度或处理失败任务。" />
    {!rows.length ? <EmptyState title="还没有任务" description="开始同步、导入归档或执行资料维护后，任务会集中显示在这里。" action={<ListTodo size={19} />} /> : <div className="grid gap-7">
      <TaskSection title="正在进行" description="等待执行或正在运行的任务" jobs={active} empty="当前没有进行中的任务" />
      {paused.length > 0 && <TaskSection title="已暂停" description="进度已保存，可按需继续或取消" jobs={paused} />}
      {attention.length > 0 && <TaskSection title="需要处理" description="失败或部分完成，需要检查后重试" jobs={attention} icon={<CircleAlert size={18} className="text-coral" />} />}
      <TaskSection title="最近完成" description="已成功完成的任务" jobs={history.slice(0, 30)} empty="还没有已完成任务" />
      {cancelled.length > 0 && <TaskSection title="已取消" description="由用户终止的历史任务" jobs={cancelled.slice(0, 20)} />}
    </div>}
  </>
}

function TaskSection({ title, description, jobs, empty, icon }: { title: string; description: string; jobs: Job[]; empty?: string; icon?: ReactNode }) {
  return <section className="panel p-5 md:p-6"><div className="flex items-center justify-between gap-3"><div><div className="flex items-center gap-2"><h2 className="text-xl font-semibold">{title}</h2>{icon}</div><p className="mt-1 text-xs text-ink-soft">{description}</p></div><span className="badge">{jobs.length} 条</span></div><div className="mt-5 grid gap-3">{jobs.length ? jobs.map((job) => <TaskJob key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-6 text-center text-sm text-ink-soft">{empty}</p>}</div></section>
}

function TaskJob({ job }: { job: Job }) {
  const client = useQueryClient()
  const action = useMutation({ mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
  return <JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />
}
