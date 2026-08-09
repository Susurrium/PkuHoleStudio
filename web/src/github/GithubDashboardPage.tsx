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
import { jobTypeLabel } from '../components/JobRow'
import { GithubLoading, GithubPageHeader, GithubState } from './GithubComponents'

export function GithubDashboardPage() {
  const health = useQuery({ queryKey: ['health'], queryFn: api.health })
  const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10_000 })
  const hotPosts = useQuery({ queryKey: ['hot-posts', 10], queryFn: () => api.hotPosts(10), retry: false, staleTime: 60_000 })
  const session = useQuery({ queryKey: ['session'], queryFn: api.session })

  if (health.isLoading || jobs.isLoading) return <GithubLoading label="正在打开工作台…" />
  if (health.error || jobs.error) return <GithubState tone="danger" title="工作台暂时不可用" description={(health.error || jobs.error)?.message} action={<button className="github-button" onClick={() => { health.refetch(); jobs.refetch() }}>重新加载</button>} />

  const activeJobs = jobs.data?.filter((job) => ['queued', 'running', 'paused'].includes(job.status)) ?? []
  const recentJobs = jobs.data?.slice(0, 5) ?? []

  return <>
    <GithubPageHeader
      eyebrow="LOCAL WORKSPACE"
      title="PkuHoleStudio"
      description="在一处浏览本地资料、连接在线树洞，并管理保存和研究任务。"
      actions={<><Link className="github-button" to="/sync"><SyncIcon size={16} />保存在线内容</Link><Link className="github-button github-button--primary" to="/posts"><CommentDiscussionIcon size={16} />浏览树洞</Link></>}
    />

    <div className="github-dashboard-grid">
      <section className="github-readme-card">
        <header><RepoIcon size={16} /><strong>README</strong></header>
        <div className="github-readme-body">
          <h2>你的本地树洞工作区</h2>
          <p>PkuHoleStudio 把树洞资料、在线会话、后台任务和 AI 研究保存在同一个本地优先的工作台中。</p>
          <div className="github-readme-actions">
            <Link to="/posts"><CommentDiscussionIcon size={18} /><span><strong>浏览与搜索</strong><small>查看本地资料或在线时间线</small></span></Link>
            <Link to="/imports"><DatabaseIcon size={18} /><span><strong>导入资料</strong><small>从 Studio 或 Toolkit 文件迁入资料</small></span></Link>
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
          <div><dt>全文搜索</dt><dd>{capabilities.data?.fts5 ? '可用' : '基础搜索'}</dd></div>
          <div><dt>在线会话</dt><dd>{session.data?.can_read_online ? '已连接' : '未连接'}</dd></div>
        </dl>
        <div className="github-topic-list"><span>local-first</span><span>treehole</span><span>research</span></div>
      </aside>
    </div>

    {(health.data?.posts ?? 0) === 0 && <div className="github-dashboard-empty"><DatabaseIcon size={22} /><div><strong>资料库还是空的</strong><p>登录后保存需要的树洞，或导入已有文件，即可开始浏览和搜索。</p></div><Link className="github-button github-button--primary" to="/sync">登录并保存</Link></div>}

    <section className="github-dashboard-section">
      <header><div><WorkflowIcon size={17} /><h2>后台任务</h2>{activeJobs.length > 0 && <span className="github-counter">{activeJobs.length}</span>}</div><Link to="/tasks">查看全部</Link></header>
      <div className="github-activity-list">
        {recentJobs.length ? recentJobs.map((job) => <Link key={job.id} to="/tasks"><span className={`github-job-dot github-job-dot--${job.status}`} /><div><strong>{jobTypeLabel(job.type)}</strong><small>{jobStatusLabel(job.status)} · {job.completed_items}/{job.total_items || 0} 项</small></div><time>{formatTime(new Date(job.updated_at).getTime() / 1000)}</time></Link>) : <p>还没有后台任务记录。</p>}
      </div>
    </section>

    <section className="github-dashboard-section">
      <header><div><FlameIcon size={17} /><h2>近期热榜</h2></div><button className="github-link-button" disabled={hotPosts.isFetching} onClick={() => hotPosts.refetch()}>刷新</button></header>
      <div className="github-hot-list">
        {hotPosts.isLoading ? <GithubLoading label="正在读取热榜…" /> : hotPosts.error ? <GithubState tone="danger" title="热榜暂时不可用" description="请稍后重试，其他功能不受影响。" /> : hotPosts.data?.items.length ? hotPosts.data.items.slice(0, 6).map((post) => { const unavailable = post.availability_state === 'confirmed_unavailable'; return <Link key={post.id} to={unavailable ? `/removed/${post.id}` : `/posts/${post.id}?source=live`}><CommentDiscussionIcon size={16} /><div><strong>{post.text || '（无正文）'}</strong><small>#{post.id} · 热度 {post.score !== undefined ? post.score.toFixed(1) : post.follownum} · 关注 {post.follownum} · 评论 {post.reply ?? 0}{unavailable ? ' · 已归档' : ''}</small></div></Link> }) : <p>当前时间范围内没有热榜数据。</p>}
      </div>
      {(hotPosts.data?.items.length ?? 0) > 6 && <div className="mt-3 flex justify-end"><Link className="github-link-button" to="/online#hot">查看完整 Top 10 →</Link></div>}
    </section>

    {session.data?.has_session && <Link className="github-notification-strip" to="/notifications"><BellIcon size={17} /><span>在线会话已连接，打开通知中心查看最新互动。</span><strong>查看通知</strong></Link>}
  </>
}

const jobStatusLabels: Record<string, string> = { queued: '等待开始', running: '正在处理', paused: '已暂停', completed: '已完成', partial: '部分完成', failed: '失败', cancelled: '已取消' }
function jobStatusLabel(value: string) { return jobStatusLabels[value] || value }
