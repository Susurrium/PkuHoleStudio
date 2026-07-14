import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BrushCleaning, Network, Search, Wrench } from 'lucide-react'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { Job } from '../lib/types'

const maintenanceTypes = new Set(['rebuild_search_index', 'rebuild_references', 'cleanup_staging'])

export function MaintenancePage() {
	const client = useQueryClient()
	const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 5_000 })
	const create = useMutation({
		mutationFn: ({ type, payload }: { type: string; payload?: unknown }) => api.createJob(type, payload),
		onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }),
	})
	if (jobs.isLoading) return <LoadingState label="正在读取维护任务…" />
	if (jobs.error) return <ErrorState error={jobs.error} />
	const recent = (jobs.data ?? []).filter((job) => maintenanceTypes.has(job.type))
	return <>
		<PageHeader eyebrow="LOCAL MAINTENANCE" title="资料库维护" description="这些任务只处理本机数据库和暂存文件，不需要登录树洞。刷新页面或重启 Studio 后，任务记录与进度仍会保留。" />
		<section className="grid gap-5 lg:grid-cols-3">
			<MaintenanceCard icon={Search} title="重建搜索索引" description="当新导入内容搜不到，或索引校验异常时使用。不会修改帖子、评论、标签或笔记。" action="开始重建搜索索引" disabled={create.isPending} onClick={() => create.mutate({ type: 'rebuild_search_index' })} />
			<MaintenanceCard icon={Network} title="重建引用关系" description="重新扫描帖子与评论中的 #PID 和评论引用，修复详情页的引用关系图。" action="开始重建引用关系" disabled={create.isPending} onClick={() => create.mutate({ type: 'rebuild_references' })} />
			<MaintenanceCard icon={BrushCleaning} title="清理过期暂存" description="删除超过 7 天的导入、导出与原始响应暂存文件，不删除正式资料库内容。" action="清理 7 天前暂存" disabled={create.isPending} onClick={() => create.mutate({ type: 'cleanup_staging', payload: { retention_days: 7 } })} />
		</section>
		{create.error && <div className="mt-5"><ErrorState error={create.error} /></div>}
		<section className="panel mt-7 p-5 md:p-6">
			<div className="flex items-center justify-between"><div><p className="eyebrow">MAINTENANCE RUNS</p><h2 className="mt-1 text-xl font-semibold">维护任务记录</h2></div><span className="badge">{recent.length} 条</span></div>
			<div className="mt-5 grid gap-3">{recent.length ? recent.map((job) => <MaintenanceJob key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-8 text-center text-sm text-ink-soft">还没有执行过维护任务。</p>}</div>
		</section>
	</>
}

function MaintenanceCard({ icon: Icon, title, description, action, disabled, onClick }: { icon: typeof Wrench; title: string; description: string; action: string; disabled: boolean; onClick: () => void }) {
	return <section className="panel p-5"><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={19} /></div><h2 className="mt-5 text-lg font-semibold">{title}</h2><p className="mt-2 min-h-16 text-sm leading-6 text-ink-soft">{description}</p><button className="button-primary mt-4 w-full" disabled={disabled} onClick={onClick}>{action}</button></section>
}

function MaintenanceJob({ job }: { job: Job }) {
	const client = useQueryClient()
	const action = useMutation({ mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
	return <JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />
}
