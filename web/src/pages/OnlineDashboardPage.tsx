import { useQuery } from '@tanstack/react-query'
import { Bell, Flame, Info, MessageCircle, PenLine, Radio, RefreshCw, Star } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { ErrorState, LoadingState } from '../components/States'
import { OnlineFreshnessBar } from '../components/OnlineFreshnessBar'
import { useBrowserOnline } from '../features/online/connectivity'
import { useOnlineSession, useSlowLoading } from '../features/online/session'
import type { HotPost, HotPostsResult } from '../lib/types'

export function OnlineDashboardPage() {
  const location = useLocation()
  const hotFocused = location.hash === '#hot'
  const browserOnline = useBrowserOnline()
  const session = useOnlineSession()
  const slowSession = useSlowLoading(session.isLoading)
  const hotPosts = useQuery({ queryKey: ['hot-posts', 10], queryFn: () => api.hotPosts(10), retry: false, staleTime: 60_000, refetchInterval: 5 * 60_000 })
  const latest = useQuery({
    queryKey: ['online-home-latest', 5],
    queryFn: () => api.posts({ source: 'live', limit: 5 }),
    enabled: !hotFocused && session.data?.can_read_online === true,
    retry: false,
    staleTime: 30_000,
  })
  const latestItems = latest.data?.items ?? []
  const hotItems = hotPosts.data?.items ?? []

  if (hotFocused) {
    return <>
      <PageHeader
        eyebrow="在线树洞"
        title="近期热榜"
        description="集中查看最近一段时间最受关注、讨论最活跃的树洞。"
        actions={<>
          <Link className="button-secondary" to="/online">返回在线首页</Link>
          <button type="button" className="button-primary" disabled={hotPosts.isFetching} onClick={() => hotPosts.refetch()}><RefreshCw size={16} className={hotPosts.isFetching ? 'animate-spin' : ''} />刷新榜单</button>
        </>}
      />
      <HotRanking
        focused
        data={hotPosts.data}
        items={hotItems}
        isLoading={hotPosts.isLoading}
        isFetching={hotPosts.isFetching}
        error={hotPosts.error}
        onRefresh={() => hotPosts.refetch()}
      />
    </>
  }

  return <>
    <PageHeader eyebrow="在线浏览" title="在线树洞" description="浏览最新内容、我的关注与互动消息；只有主动保存的树洞才会进入本地资料库。" actions={
      <>
        <Link className="button-secondary" to="/posts?source=live&followed=true"><Star size={16} />我的关注</Link>
        {session.data?.can_write_online
          ? <Link className="button-primary" to="/posts?source=live&compose=true"><PenLine size={16} />发表新洞</Link>
          : <Link className="button-primary" to="/sync"><Radio size={16} />登录在线树洞</Link>}
      </>
    } />

    {!session.isLoading && !session.data?.can_read_online && <section className="panel mb-6 border-coral/30 bg-coral-soft/20 p-5"><p className="font-semibold">登录后即可浏览和互动</p><p className="mt-1 text-sm text-ink-soft">{session.data?.message || '本地资料库仍可离线使用。'}</p><Link className="button-primary mt-4" to="/sync">前往登录</Link></section>}
    {session.data?.can_read_online && <OnlineFreshnessBar browserOnline={browserOnline} updatedAt={latest.dataUpdatedAt} isFetching={latest.isFetching} error={latest.error} hasData={latestItems.length > 0} onRefresh={() => latest.refetch()} />}

    <section className="mb-6 grid grid-cols-2 gap-3 xl:hidden" aria-label="在线快捷入口">
      <OnlineShortcut to="/online#hot" icon={Flame} title="近期热榜" note="查看完整 Top 10" />
      <OnlineShortcut to="/posts?source=live&followed=true" icon={Star} title="我的关注" note="查看关注洞更新" />
      <OnlineShortcut to="/notifications" icon={Bell} title="互动消息" note="回复与系统通知" />
      <OnlineShortcut to="/posts?source=live&compose=true" icon={PenLine} title="发表新洞" note="支持文字和图片" />
    </section>

    <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.3fr)_minmax(320px,.7fr)]">
      <section>
        <div className="mb-4 flex items-center justify-between gap-3"><div><p className="eyebrow">在线时间线</p><h2 className="mt-1 text-xl font-semibold">最新树洞</h2></div><Link className="button-secondary" to="/posts?source=live">查看全部</Link></div>
        {session.isLoading ? <LoadingState label={slowSession ? '正在连接树洞，可能需要几秒…' : '正在验证在线会话…'} /> : latest.isLoading && !latestItems.length ? <LoadingState label="正在读取最新树洞…" /> : latest.error && !latestItems.length ? <ErrorState error={latest.error} /> : latestItems.length ? <div className="grid gap-4">{latestItems.map((post) => <PostCard key={post.pid} post={post} source="live" />)}</div> : session.data?.can_read_online ? <div className="panel p-8 text-center text-sm text-ink-soft">当前没有读取到最新内容</div> : <div className="panel p-8 text-center text-sm text-ink-soft">登录后显示最新树洞</div>}
      </section>

      <HotRanking
        data={hotPosts.data}
        items={hotItems.slice(0, 5)}
        isLoading={hotPosts.isLoading}
        isFetching={hotPosts.isFetching}
        error={hotPosts.error}
        onRefresh={() => hotPosts.refetch()}
      />
    </div>
  </>
}

function HotRanking({ focused = false, data, items, isLoading, isFetching, error, onRefresh }: { focused?: boolean; data?: HotPostsResult; items: HotPost[]; isLoading: boolean; isFetching: boolean; error: Error | null; onRefresh: () => void }) {
  const windowHours = data?.window_hours || 12
  return <section id="hot" className={`scroll-mt-24 ${focused ? 'mx-auto max-w-4xl' : 'panel p-5 xl:sticky xl:top-24'}`} aria-labelledby="hot-ranking-title">
    {focused
      ? <HotRule windowHours={windowHours} />
      : <div className="flex items-start justify-between gap-3"><div><p className="eyebrow">近期榜单</p><h2 id="hot-ranking-title" className="mt-1 text-xl font-semibold">最近 {windowHours} 小时热榜</h2></div><button type="button" className="button-secondary !min-h-9 !px-3 !py-1.5" disabled={isFetching} onClick={onRefresh} aria-label="刷新热榜"><RefreshCw size={14} className={isFetching ? 'animate-spin' : ''} /></button></div>}

    <div className={focused ? 'mt-5 grid gap-4' : 'mt-4 grid gap-2'}>
      {isLoading && !items.length
        ? <p className="panel border-dashed p-6 text-center text-sm text-ink-soft" role="status">正在读取热榜…</p>
        : error && !items.length
          ? <HotError error={error} onRefresh={onRefresh} />
          : items.length
            ? items.map((post, index) => <HotPostCard key={post.id} post={post} index={index} focused={focused} />)
            : <p className="panel border-dashed p-6 text-center text-sm text-ink-soft">当前时间范围内没有可用热榜数据</p>}
    </div>

    {!focused && <Link className="button-secondary mt-3 w-full !min-h-9 !py-1.5 text-xs" to="/online#hot">查看完整 Top 10</Link>}
    <HotStatus data={data} error={error} />
  </section>
}

function HotRule({ windowHours }: { windowHours: number }) {
  return <section className="panel border-teal/25 bg-teal-soft/15 p-5 sm:p-6" aria-labelledby="hot-ranking-title">
    <div className="flex items-start gap-3"><div className="grid size-10 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><Info size={19} /></div><div><h2 id="hot-ranking-title" className="text-lg font-semibold">榜单如何计算</h2><p className="mt-1 text-sm leading-6 text-ink-soft">统计最近 {windowHours} 小时的树洞，关注越多、评论越活跃，热度越高；点赞不参与热度计算。</p><p className="mt-1 text-xs text-ink-soft">榜单每 5 分钟自动更新。</p></div></div>
  </section>
}

function HotPostCard({ post, index, focused }: { post: HotPost; index: number; focused: boolean }) {
  const unavailable = post.availability_state === 'confirmed_unavailable'
  const returnTo = focused ? '%2Fonline%23hot' : '%2Fonline'
  return <Link to={unavailable ? `/removed/${post.id}` : `/posts/${post.id}?source=live&return_to=${returnTo}`} className={`${focused ? 'panel p-5 sm:p-6' : 'rounded-xl border border-line bg-white/45 p-3'} group block transition hover:-translate-y-0.5 hover:border-coral/40 hover:bg-white/75`}>
    <div className="flex items-start gap-3"><span className={`grid shrink-0 place-items-center rounded-xl font-mono font-bold ${focused ? 'size-10 bg-coral-soft text-coral' : 'size-7 bg-paper text-xs text-ink-soft'}`}>{index + 1}</span><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><span className="font-mono text-sm font-semibold text-coral">洞 {post.id}</span>{unavailable ? <span className="badge !border-coral/25 !py-0.5 !text-coral">已归档</span> : null}</div><p className={`${focused ? 'mt-3 line-clamp-5 text-[15px] leading-7 text-ink' : 'mt-1 line-clamp-3 text-sm text-ink-soft'}`}>{post.text || '（无正文）'}</p><div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-ink-soft"><span className="inline-flex items-center gap-1.5" aria-label={`热度 ${formatHeat(post)}`}><Flame size={13} />热度 {formatHeat(post)}</span><span className="inline-flex items-center gap-1.5" aria-label={`关注 ${post.follownum ?? 0}`}><Star size={13} />关注 {post.follownum ?? 0}</span><span className="inline-flex items-center gap-1.5" aria-label={`评论 ${post.reply ?? 0}`}><MessageCircle size={13} />评论 {post.reply ?? 0}</span></div></div></div>
  </Link>
}

function HotError({ error, onRefresh }: { error: Error; onRefresh: () => void }) {
  return <div className="panel border-coral/25 bg-coral-soft/20 p-5 text-sm text-ink-soft" role="alert"><p className="font-semibold text-ink">热榜暂时无法加载</p><p className="mt-1">请稍后重试，其他在线功能不受影响。</p><button type="button" className="button-secondary mt-3" onClick={onRefresh}>重新加载</button><details className="mt-3 text-xs"><summary className="cursor-pointer font-semibold">查看技术详情</summary><p className="mt-2 break-words">{error.message}</p></details></div>
}

function HotStatus({ data, error }: { data?: HotPostsResult; error: Error | null }) {
  if (!data) return null
  return <div className="mt-4 border-t border-line pt-3 text-xs leading-5 text-ink-soft"><p>{data.stale ? '当前显示最近一次可用结果' : error ? '更新失败，继续显示上次结果' : `更新于 ${formatUpdatedAt(data.updated_at)}`}</p><details className="mt-1"><summary className="cursor-pointer font-semibold hover:text-teal">数据说明</summary><div className="mt-2 rounded-lg bg-paper/65 p-3"><p>数据来源：{hotSourceLabel(data.source)}</p>{data.message ? <p className="mt-1">{data.message}</p> : null}{error ? <p className="mt-1 break-words text-coral">{error.message}</p> : null}</div></details></div>
}

function OnlineShortcut({ to, icon: Icon, title, note }: { to: string; icon: typeof Radio; title: string; note: string }) {
  return <Link to={to} className="panel flex min-w-0 items-center gap-2.5 p-3 transition hover:-translate-y-0.5 hover:border-teal/40"><div className="grid size-9 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={17} /></div><div className="min-w-0"><p className="truncate text-sm font-semibold">{title}</p><p className="mt-0.5 truncate text-[11px] text-ink-soft">{note}</p></div></Link>
}

function formatHeat(post: HotPost) {
  return post.score !== undefined ? post.score.toFixed(1) : String(post.follownum ?? 0)
}

function formatUpdatedAt(timestamp: number) {
  if (!timestamp) return '时间未知'
  return new Date(timestamp * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function hotSourceLabel(source: HotPostsResult['source']) {
  if (source === 'observer') return '删除记录服务'
  if (source === 'observer_cache') return '最近缓存'
  return '在线时间线'
}
