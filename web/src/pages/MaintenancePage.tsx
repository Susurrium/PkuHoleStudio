import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BrushCleaning, ListTodo, Network, Search, ShieldCheck, Wrench } from 'lucide-react'
import { Link } from 'react-router-dom'
import { errorDescription, useFeedback } from '../components/Feedback'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { Job } from '../lib/types'

const maintenanceTypes = new Set(['rebuild_search_index', 'rebuild_references', 'cleanup_staging'])
const activeStatuses = new Set(['queued', 'running', 'paused'])
const maintenanceLabels: Record<string, string> = { rebuild_search_index: '重建搜索索引', rebuild_references: '重建引用关系', cleanup_staging: '清理过期暂存' }

export function MaintenancePage() {
	const client = useQueryClient()
	const { confirm, notify } = useFeedback()
	const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 5_000 })
	const create = useMutation({
		mutationFn: ({ type, payload }: { type: string; payload?: unknown }) => api.createJob(type, payload),
		onSuccess: (job) => {
			client.invalidateQueries({ queryKey: ['jobs'] })
			notify({ title: `${maintenanceLabels[job.type] || '维护'}任务已创建`, description: '任务会在后台继续执行，可在本页或任务中心查看进度。' })
		},
		onError: (error) => notify({ tone: 'error', title: '未能创建维护任务', description: errorDescription(error) }),
	})
	if (jobs.isLoading) return <LoadingState label="正在读取维护任务…" />
	if (jobs.error) return <ErrorState error={jobs.error} />
	const recent = (jobs.data ?? []).filter((job) => maintenanceTypes.has(job.type))
	const activeTypes = new Set(recent.filter((job) => activeStatuses.has(job.status)).map((job) => job.type))
	const startCleanup = async () => {
		const accepted = await confirm({
			title: '清理 7 天前的暂存文件？',
			description: '将删除过期的导入、导出和原始响应暂存文件。正式资料库、标签、笔记和已下载媒体不会被删除。',
			confirmLabel: '确认清理',
			tone: 'danger',
		})
		if (accepted) create.mutate({ type: 'cleanup_staging', payload: { retention_days: 7 } })
	}
	return <>
		<PageHeader
			eyebrow="LOCAL MAINTENANCE"
			title="资料库维护"
			description="仅在搜索、引用关系或暂存空间出现问题时运行。任务会持久保存，并在后台继续执行。"
			actions={<Link className="button-secondary" to="/tasks"><ListTodo size={15} />打开任务中心</Link>}
		/>
		<div className="mb-5 flex items-start gap-3 rounded-xl border border-teal/25 bg-teal-soft/25 p-4 text-sm leading-6 text-ink-soft"><ShieldCheck size={18} className="mt-0.5 shrink-0 text-teal" /><p>重建任务不会修改帖子、评论、标签和笔记；清理暂存会在执行前再次确认。同一种维护任务运行期间不能重复创建。</p></div>
		<section className="grid gap-5 lg:grid-cols-3">
			<MaintenanceCard icon={Search} title="重建搜索索引" description="当新导入内容搜不到，或索引校验异常时使用。正常搜索时无需定期执行。" action="开始重建搜索索引" active={activeTypes.has('rebuild_search_index')} pending={create.isPending && create.variables?.type === 'rebuild_search_index'} onClick={() => create.mutate({ type: 'rebuild_search_index' })} />
			<MaintenanceCard icon={Network} title="重建引用关系" description="重新扫描帖子与评论中的 #PID 和评论引用，用于修复详情页引用图。" action="开始重建引用关系" active={activeTypes.has('rebuild_references')} pending={create.isPending && create.variables?.type === 'rebuild_references'} onClick={() => create.mutate({ type: 'rebuild_references' })} />
			<MaintenanceCard icon={BrushCleaning} title="清理过期暂存" description="删除超过 7 天的导入、导出与原始响应暂存，不影响正式资料。" action="清理 7 天前暂存" tone="danger" active={activeTypes.has('cleanup_staging')} pending={create.isPending && create.variables?.type === 'cleanup_staging'} onClick={startCleanup} />
		</section>
		<section className="mt-7">
			<div className="mb-4 flex items-end justify-between gap-3"><div><p className="eyebrow">MAINTENANCE RUNS</p><h2 className="mt-1 text-xl font-semibold">最近维护记录</h2></div><span className="badge">{recent.length} 条</span></div>
			{recent.length ? <div className="panel grid gap-3 p-4 md:p-5">{recent.map((job) => <MaintenanceJob key={job.id} job={job} />)}</div> : <EmptyState title="还没有维护记录" description="资料库运行正常时，不需要主动执行这些操作。" action={<Wrench size={20} />} />}
		</section>
	</>
}

function MaintenanceCard({ icon: Icon, title, description, action, tone = 'default', active, pending, onClick }: { icon: typeof Wrench; title: string; description: string; action: string; tone?: 'default' | 'danger'; active: boolean; pending: boolean; onClick: () => void }) {
	return <section className={`panel p-5 ${tone === 'danger' ? 'border-coral/25' : ''}`}>
		<div className={`grid size-10 place-items-center rounded-xl ${tone === 'danger' ? 'bg-coral-soft text-coral' : 'bg-teal-soft text-teal'}`}><Icon size={19} /></div>
		<div className="mt-5 flex items-start justify-between gap-3"><h2 className="text-lg font-semibold">{title}</h2>{active && <span className="badge !border-teal/30 !text-teal">进行中</span>}</div>
		<p className="mt-2 min-h-16 text-sm leading-6 text-ink-soft">{description}</p>
		<button className={`${tone === 'danger' ? 'button-secondary !text-coral hover:!border-coral' : 'button-primary'} mt-4 w-full`} disabled={active || pending} onClick={onClick}>{pending ? '正在创建…' : active ? '已有任务正在执行' : action}</button>
	</section>
}

function MaintenanceJob({ job }: { job: Job }) {
	const client = useQueryClient()
	const { notify } = useFeedback()
	const action = useMutation({
		mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value),
		onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }),
		onError: (error) => notify({ tone: 'error', title: '任务操作失败', description: errorDescription(error) }),
	})
	return <JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />
}
