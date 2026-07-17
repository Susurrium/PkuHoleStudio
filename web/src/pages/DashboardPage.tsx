import { useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Archive, Bot, Database, FolderKanban, Import, RefreshCw, Search } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import type { Job } from '../lib/types'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

export function DashboardPage() {
  const health = useQuery({ queryKey: ['health'], queryFn: api.health })
  const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
  useLiveJobRefresh(jobs.data ?? [])

  if (health.isLoading || jobs.isLoading) return <LoadingState label="正在打开本地工作台…" />
  if (health.error || jobs.error) return <ErrorState error={health.error || jobs.error} />

  const empty = (health.data?.posts ?? 0) === 0
  const currentJobs = (jobs.data ?? []).filter((job) => ['queued', 'running', 'paused', 'failed', 'partial'].includes(job.status)).slice(0, 3)
  return (
    <>
      <PageHeader eyebrow="LIBRARY" title="本地资料库" description="导入、归档、检索和分析保存在本机的树洞资料；断网时也可继续使用。" actions={
        <>
          <Link className="button-secondary" to="/sync"><RefreshCw size={16} />保存在线内容</Link>
          <Link className="button-primary" to="/posts"><Archive size={16} />开始浏览</Link>
        </>
      } />

      <section className="grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4" aria-label="资料库统计">
        <Metric icon={Archive} label="本地帖子" value={health.data?.posts ?? 0} note="可离线浏览" tone="coral" />
        <Metric icon={Database} label="本地评论" value={health.data?.comments ?? 0} note="包含评论全文" />
        <Metric icon={Search} label="本地搜索" value="可用" note={capabilities.data?.fts5 ? '帖子与评论全文检索' : '当前使用兼容检索模式'} />
        <Metric icon={RefreshCw} label="活动任务" value={(jobs.data ?? []).filter((job) => ['queued', 'running'].includes(job.status)).length} note="刷新页面不会丢失" />
      </section>

      {empty && <section className="mt-7"><EmptyState title="资料库还是空的" description="可以直接导入 Studio/Toolkit 兼容归档，也可以登录后同步关注洞或指定 PID。" action={<div className="flex flex-wrap justify-center gap-2"><Link className="button-primary" to="/imports"><Import size={16} />导入归档</Link><Link className="button-secondary" to="/sync"><RefreshCw size={16} />开始同步</Link></div>} /></section>}

      <section className="mt-7 grid gap-6 xl:grid-cols-[1.25fr_.75fr]">
        <div className="panel p-5 md:p-6">
          <div className="flex items-center justify-between gap-3"><div><p className="eyebrow">TASK STATUS</p><h2 className="mt-1 text-xl font-semibold">任务状态</h2></div><Link className="text-xs font-semibold text-teal hover:underline" to="/tasks">查看任务中心</Link></div>
          <div className="mt-5 grid gap-3">
            {currentJobs.length ? currentJobs.map((job) => <JobController key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-8 text-center text-sm text-ink-soft">当前没有运行中或需要处理的任务</p>}
          </div>
        </div>
        <div className="grid gap-6"><div className="panel p-5 md:p-6">
          <p className="eyebrow">QUICK START</p><h2 className="mt-1 text-xl font-semibold">常用入口</h2>
          <div className="mt-5 grid gap-3">
            <QuickLink to="/posts" icon={Archive} title="浏览资料库" note="按时间、来源和图片筛选" />
            <QuickLink to="/posts?focus=search" icon={Search} title="搜索资料" note="洞正文与评论一起搜索" />
            <QuickLink to="/imports" icon={Import} title="导入归档" note="Studio 原生支持 JSON 与 .treehole.zip" />
            <QuickLink to="/projects" icon={FolderKanban} title="整理研究项目" note="把一组树洞固定为可持续维护的研究范围" />
            <QuickLink to="/ai" icon={Bot} title="开始 AI 研究" note="按项目、PID、标签或时间范围研究本地资料" />
          </div>
        </div></div>
      </section>
    </>
  )
}

function Metric({ icon: Icon, label, value, note, tone }: { icon: typeof Archive; label: string; value: string | number; note: string; tone?: 'coral' }) {
  return <div className="panel p-4 sm:p-5"><div className={`grid size-9 place-items-center rounded-xl sm:size-10 ${tone === 'coral' ? 'bg-coral-soft text-coral' : 'bg-teal-soft text-teal'}`}><Icon size={18} /></div><p className="mt-3 text-2xl font-semibold tracking-tight sm:mt-5 sm:text-3xl">{typeof value === 'number' ? value.toLocaleString('zh-CN') : value}</p><p className="mt-1 text-sm font-semibold">{label}</p><p className="mt-1.5 text-[11px] leading-4 text-ink-soft sm:mt-2 sm:text-xs">{note}</p></div>
}

function QuickLink({ to, icon: Icon, title, note }: { to: string; icon: typeof Archive; title: string; note: string }) {
  return <Link to={to} className="flex items-center gap-3 rounded-xl border border-line bg-white/45 p-4 transition hover:border-teal/40 hover:bg-white/80"><div className="grid size-9 place-items-center rounded-lg bg-paper text-teal"><Icon size={17} /></div><div><p className="text-sm font-semibold">{title}</p><p className="mt-0.5 text-xs text-ink-soft">{note}</p></div></Link>
}

function JobController({ job }: { job: Job }) {
  const client = useQueryClient()
  const action = useMutation({ mutationFn: (value: 'pause' | 'resume' | 'cancel' | 'retry') => api.jobAction(job.id, value), onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }) })
  return <JobRow job={job} busy={action.isPending} onAction={(value) => action.mutate(value)} />
}

function useLiveJobRefresh(jobs: Job[]) {
  const client = useQueryClient()
  const ids = jobs.filter((job) => ['queued', 'running', 'paused'].includes(job.status)).map((job) => job.id).sort().join(',')
  useEffect(() => {
    if (!ids) return
    const sources = ids.split(',').map((id) => {
      const source = new EventSource(`/api/v1/jobs/${id}/events`)
      const refresh = () => client.invalidateQueries({ queryKey: ['jobs'] })
      for (const event of ['started', 'checkpoint', 'item_completed', 'item_failed', 'completed', 'partial', 'failed', 'cancelled', 'paused']) source.addEventListener(event, refresh)
      return source
    })
    return () => sources.forEach((source) => source.close())
  }, [client, ids])
}
