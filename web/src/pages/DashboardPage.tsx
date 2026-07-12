import { useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Archive, Bell, Database, Flame, Import, RefreshCw, Search } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import type { Job } from '../lib/types'
import { JobRow } from '../components/JobRow'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

export function DashboardPage() {
  const client = useQueryClient()
  const health = useQuery({ queryKey: ['health'], queryFn: api.health })
  const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
	const hotPosts = useQuery({ queryKey: ['hot-posts'], queryFn: api.hotPosts, retry: false, staleTime: 5 * 60_000 })
	const session = useQuery({ queryKey: ['session'], queryFn: api.session })
	const notifications = useQuery({ queryKey: ['notifications', 'interactive'], queryFn: () => api.notifications('interactive'), enabled: session.data?.has_session === true, retry: false })
  const createJob = useMutation({
    mutationFn: ({ type, payload }: { type: string; payload?: unknown }) => api.createJob(type, payload),
    onSuccess: () => client.invalidateQueries({ queryKey: ['jobs'] }),
  })
  useLiveJobRefresh(jobs.data ?? [])

  if (health.isLoading || jobs.isLoading) return <LoadingState label="正在打开本地工作台…" />
  if (health.error || jobs.error) return <ErrorState error={health.error || jobs.error} />

  const empty = (health.data?.posts ?? 0) === 0
  return (
    <>
      <PageHeader eyebrow="LIBRARY DESK" title="你的树洞资料，在本机慢慢长成档案" description="同步、检索与导入共享同一套本地资料库。任务可以离开页面继续运行，进度会在返回时恢复。" actions={
        <>
          <button className="button-secondary" disabled={createJob.isPending} onClick={() => createJob.mutate({ type: 'rebuild_search_index' })}><Search size={16} />重建索引</button>
          <Link className="button-primary" to="/sync"><RefreshCw size={16} />打开同步中心</Link>
        </>
      } />

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-label="资料库统计">
        <Metric icon={Archive} label="本地帖子" value={health.data?.posts ?? 0} note="可离线浏览" tone="coral" />
        <Metric icon={Database} label="本地评论" value={health.data?.comments ?? 0} note="包含评论全文" />
        <Metric icon={Search} label="搜索引擎" value={capabilities.data?.fts5 ? 'FTS5' : 'LIKE'} note={capabilities.data?.fts5 ? '中文 trigram 索引' : '兼容搜索模式'} />
        <Metric icon={RefreshCw} label="活动任务" value={(jobs.data ?? []).filter((job) => ['queued', 'running', 'paused'].includes(job.status)).length} note="刷新页面不会丢失" />
      </section>

      {empty && <section className="mt-7"><EmptyState title="资料库还是空的" description="可以直接导入 Studio/Toolkit 兼容归档，也可以登录后同步关注洞或指定 PID。" action={<div className="flex flex-wrap justify-center gap-2"><Link className="button-primary" to="/imports"><Import size={16} />导入归档</Link><Link className="button-secondary" to="/sync"><RefreshCw size={16} />开始同步</Link></div>} /></section>}

      <section className="mt-7 grid gap-6 xl:grid-cols-[1.25fr_.75fr]">
        <div className="panel p-5 md:p-6">
          <div className="flex items-center justify-between"><div><p className="eyebrow">RECENT RUNS</p><h2 className="mt-1 text-xl font-semibold">最近任务</h2></div><span className="badge">{jobs.data?.length ?? 0} 条</span></div>
          <div className="mt-5 grid gap-3">
            {jobs.data?.length ? jobs.data.slice(0, 6).map((job) => <JobController key={job.id} job={job} />) : <p className="rounded-xl border border-dashed border-line p-8 text-center text-sm text-ink-soft">还没有任务记录</p>}
          </div>
        </div>
        <div className="grid gap-6">{session.data?.has_session && <div className="panel p-5 md:p-6"><div className="flex items-center justify-between gap-3"><div><p className="eyebrow">NOTIFICATIONS</p><h2 className="mt-1 text-xl font-semibold">互动通知</h2></div><Bell size={19} className="text-teal" /></div><p className="mt-4 text-sm text-ink-soft">{notifications.data ? `${notifications.data.items.filter((item) => !item.read).length} 条未读，最近载入 ${notifications.data.items.length} 条` : notifications.error ? '通知暂时不可用' : '正在读取通知…'}</p><Link className="button-secondary mt-4 w-full" to="/notifications">打开通知中心</Link></div>}<div className="panel p-5 md:p-6">
          <p className="eyebrow">HOT POSTS</p><h2 className="mt-1 text-xl font-semibold">最近 12 小时热榜</h2>
			<div className="mt-4 grid gap-2">{hotPosts.data?.length ? hotPosts.data.map((post) => <Link key={post.id} to={`/posts/${post.id}?source=live`} className="rounded-xl border border-line bg-white/45 p-3 text-sm hover:border-coral/40"><span className="font-mono font-semibold text-coral">#{post.id}</span><p className="mt-1 line-clamp-2 text-ink-soft">{post.text || '（无正文）'}</p><span className="mt-2 inline-flex items-center gap-1 text-xs text-ink-soft"><Flame size={12} />{post.follownum}</span></Link>) : <p className="rounded-xl border border-dashed border-line p-5 text-center text-xs text-ink-soft">{hotPosts.error ? '热榜暂时不可用' : '正在读取热榜…'}</p>}</div>
		</div><div className="panel p-5 md:p-6">
          <p className="eyebrow">QUICK START</p><h2 className="mt-1 text-xl font-semibold">常用入口</h2>
          <div className="mt-5 grid gap-3">
            <QuickLink to="/posts" icon={Archive} title="浏览资料库" note="按时间、来源和图片筛选" />
            <QuickLink to="/search" icon={Search} title="全文检索" note="洞正文与评论一起搜索" />
            <QuickLink to="/imports" icon={Import} title="导入归档" note="Studio 原生支持 JSON 与 .treehole.zip" />
          </div>
        </div></div>
      </section>
    </>
  )
}

function Metric({ icon: Icon, label, value, note, tone }: { icon: typeof Archive; label: string; value: string | number; note: string; tone?: 'coral' }) {
  return <div className="panel p-5"><div className={`grid size-10 place-items-center rounded-xl ${tone === 'coral' ? 'bg-coral-soft text-coral' : 'bg-teal-soft text-teal'}`}><Icon size={19} /></div><p className="mt-5 text-3xl font-semibold tracking-tight">{typeof value === 'number' ? value.toLocaleString('zh-CN') : value}</p><p className="mt-1 text-sm font-semibold">{label}</p><p className="mt-2 text-xs text-ink-soft">{note}</p></div>
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
