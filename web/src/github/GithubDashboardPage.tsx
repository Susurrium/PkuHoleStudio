import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  BellIcon,
  CommentDiscussionIcon,
  DatabaseIcon,
  FlameIcon,
  RepoIcon,
  SearchIcon,
  SyncIcon,
  WorkflowIcon,
} from '@primer/octicons-react'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import { GithubLoading, GithubPageHeader, GithubState } from './GithubComponents'

export function GithubDashboardPage() {
  const health = useQuery({ queryKey: ['health'], queryFn: api.health })
  const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
  const hotPosts = useQuery({ queryKey: ['hot-posts'], queryFn: api.hotPosts, retry: false, staleTime: 60_000 })
  const session = useQuery({ queryKey: ['session'], queryFn: api.session })

  if (health.isLoading || jobs.isLoading) return <GithubLoading label="正在打开工作台…" />
  if (health.error || jobs.error) return <GithubState tone="danger" title="工作台暂时不可用" description={(health.error || jobs.error)?.message} action={<button className="github-button" onClick={() => { health.refetch(); jobs.refetch() }}>重新加载</button>} />

  const activeJobs = jobs.data?.filter((job) => ['queued', 'running', 'paused'].includes(job.status)) ?? []
  const recentJobs = jobs.data?.slice(0, 5) ?? []

  return <>
    <GithubPageHeader
      eyebrow="LOCAL WORKSPACE"
      title="PkuHoleStudio"
      description="在一处浏览本地资料、连接在线树洞，并管理同步和研究任务。"
      actions={<><Link className="github-button" to="/sync"><SyncIcon size={16} />同步资料</Link><Link className="github-button github-button--primary" to="/posts"><CommentDiscussionIcon size={16} />浏览树洞</Link></>}
    />

    <div className="github-dashboard-grid">
      <section className="github-readme-card">
        <header><RepoIcon size={16} /><strong>README</strong></header>
        <div className="github-readme-body">
          <h2>你的本地树洞工作区</h2>
          <p>PkuHoleStudio 把树洞资料、在线会话、后台任务和 AI 研究保存在同一个本地优先的工作台中。</p>
          <div className="github-readme-actions">
            <Link to="/posts"><CommentDiscussionIcon size={18} /><span><strong>浏览与搜索</strong><small>查看本地资料或在线时间线</small></span></Link>
            <Link to="/imports"><DatabaseIcon size={18} /><span><strong>导入资料</strong><small>从兼容归档建立本地资料库</small></span></Link>
            <Link to="/ai"><SearchIcon size={18} /><span><strong>AI 研究</strong><small>基于本地证据整理问题</small></span></Link>
          </div>
        </div>
      </section>

      <aside className="github-about-panel">
        <h2>About</h2>
        <p>本地优先的树洞资料与研究工作台。</p>
        <dl>
          <div><dt>帖子</dt><dd>{(health.data?.posts ?? 0).toLocaleString('zh-CN')}</dd></div>
          <div><dt>评论</dt><dd>{(health.data?.comments ?? 0).toLocaleString('zh-CN')}</dd></div>
          <div><dt>全文搜索</dt><dd>{capabilities.data?.fts5 ? 'FTS5' : '兼容模式'}</dd></div>
          <div><dt>在线会话</dt><dd>{session.data?.can_read_online ? '已连接' : '未连接'}</dd></div>
        </dl>
        <div className="github-topic-list"><span>local-first</span><span>treehole</span><span>research</span></div>
      </aside>
    </div>

    {(health.data?.posts ?? 0) === 0 && <div className="github-dashboard-empty"><DatabaseIcon size={22} /><div><strong>资料库还是空的</strong><p>导入归档或登录后同步一些内容，即可开始浏览和搜索。</p></div><Link className="github-button github-button--primary" to="/imports">导入归档</Link></div>}

    <section className="github-dashboard-section">
      <header><div><WorkflowIcon size={17} /><h2>后台任务</h2>{activeJobs.length > 0 && <span className="github-counter">{activeJobs.length}</span>}</div><Link to="/tasks">查看全部</Link></header>
      <div className="github-activity-list">
        {recentJobs.length ? recentJobs.map((job) => <Link key={job.id} to="/tasks"><span className={`github-job-dot github-job-dot--${job.status}`} /><div><strong>{jobTypeLabel(job.type)}</strong><small>{job.status} · {job.completed_items}/{job.total_items || 0} 项</small></div><time>{formatTime(new Date(job.updated_at).getTime() / 1000)}</time></Link>) : <p>还没有后台任务记录。</p>}
      </div>
    </section>

    <section className="github-dashboard-section">
      <header><div><FlameIcon size={17} /><h2>近期热榜</h2></div><button className="github-link-button" disabled={hotPosts.isFetching} onClick={() => hotPosts.refetch()}>刷新</button></header>
      <div className="github-hot-list">
        {hotPosts.isLoading ? <GithubLoading label="正在读取热榜…" /> : hotPosts.error ? <GithubState tone="danger" title="热榜暂时不可用" description={hotPosts.error.message} /> : hotPosts.data?.items.length ? hotPosts.data.items.slice(0, 6).map((post) => { const unavailable = post.availability_state === 'confirmed_unavailable'; return <Link key={post.id} to={unavailable ? `/removed/${post.id}` : `/posts/${post.id}?source=live`}><CommentDiscussionIcon size={16} /><div><strong>{post.text || '（无正文）'}</strong><small>#{post.id} · {post.reply ?? 0} 条评论 · {unavailable ? '删除前归档' : `热度 ${post.score !== undefined ? post.score.toFixed(1) : post.follownum}`}</small></div></Link> }) : <p>当前时间范围内没有热榜数据。</p>}
      </div>
    </section>

    {session.data?.has_session && <Link className="github-notification-strip" to="/notifications"><BellIcon size={17} /><span>在线会话已连接，打开通知中心查看最新互动。</span><strong>查看通知</strong></Link>}
  </>
}

const jobLabels: Record<string, string> = { sync_followed: '同步关注树洞', sync_pids: '同步指定 PID', sync_latest: '同步最新时间线', import_archive: '导入归档', export_archive: '导出归档', repair_comments: '补全评论', repair_media: '补全媒体', rebuild_search_index: '重建搜索索引', rebuild_references: '重建引用关系' }
function jobTypeLabel(value: string) { return jobLabels[value] || value }
