import { useQuery } from '@tanstack/react-query'
import { Bell, Flame, MessageCircle, PenLine, Radio, RefreshCw, Star } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { ErrorState, LoadingState } from '../components/States'
import { OnlineFreshnessBar } from '../components/OnlineFreshnessBar'
import { useBrowserOnline } from '../features/online/connectivity'
import { useOnlineSession, useSlowLoading } from '../features/online/session'
import type { HotPostsResult } from '../lib/types'

export function OnlineDashboardPage() {
  const location = useLocation()
  const [hotExpanded, setHotExpanded] = useState(() => location.hash === '#hot')
	const browserOnline = useBrowserOnline()
  const session = useOnlineSession()
  const slowSession = useSlowLoading(session.isLoading)
  const hotPosts = useQuery({ queryKey: ['hot-posts', 10], queryFn: () => api.hotPosts(10), retry: false, staleTime: 60_000, refetchInterval: 5 * 60_000 })
  const latest = useQuery({
    queryKey: ['online-home-latest'],
    queryFn: () => api.posts({ source: 'live', limit: 8 }),
    enabled: session.data?.can_read_online === true,
    retry: false,
    staleTime: 30_000,
  })
	const latestItems = latest.data?.items ?? []
	const hotItems = hotPosts.data?.items ?? []
  const displayedHotItems = hotItems.slice(0, hotExpanded ? 10 : 5)

  useEffect(() => {
    if (location.hash === '#hot') setHotExpanded(true)
  }, [location.hash])

  return <>
    <PageHeader eyebrow="LIVE" title="在线树洞" description="查看最新、热榜、关注与消息；在线浏览不会自动写入本地资料库。" actions={
      <>
        <Link className="button-secondary" to="/posts?source=live&followed=true"><Star size={16} />我的关注</Link>
        {session.data?.can_write_online
          ? <Link className="button-primary" to="/posts?source=live&compose=true"><PenLine size={16} />发表新洞</Link>
          : <Link className="button-primary" to="/sync"><Radio size={16} />登录在线树洞</Link>}
      </>
    } />

    {!session.isLoading && !session.data?.can_read_online && <section className="panel mb-6 border-coral/30 bg-coral-soft/20 p-5"><p className="font-semibold">在线会话尚未就绪</p><p className="mt-1 text-sm text-ink-soft">{session.data?.message || '登录后可以浏览最新、关注、消息并发表内容；本地资料库仍可离线使用。'}</p><Link className="button-primary mt-4" to="/sync">前往登录</Link></section>}
	{session.data?.can_read_online && <OnlineFreshnessBar browserOnline={browserOnline} updatedAt={latest.dataUpdatedAt} isFetching={latest.isFetching} error={latest.error} hasData={latestItems.length > 0} onRefresh={() => latest.refetch()} />}

    <section className="mb-6 grid grid-cols-2 gap-3 xl:hidden" aria-label="在线快捷入口">
      <OnlineShortcut to="/online#hot" icon={Flame} title="近期热榜" note="查看完整 Top 10" />
      <OnlineShortcut to="/posts?source=live&followed=true" icon={Star} title="我的关注" note="查看关注洞更新" />
      <OnlineShortcut to="/notifications" icon={Bell} title="互动消息" note="回复与系统通知" />
      <OnlineShortcut to="/posts?source=live&compose=true" icon={PenLine} title="发表新洞" note="支持文字和图片" />
    </section>

    <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.3fr)_minmax(300px,.7fr)]">
      <section>
        <div className="mb-4 flex items-center justify-between gap-3"><div><p className="eyebrow">LATEST</p><h2 className="mt-1 text-xl font-semibold">最新树洞</h2></div><Link className="button-secondary" to="/posts?source=live">查看全部</Link></div>
		{session.isLoading ? <LoadingState label={slowSession ? '正在连接树洞，可能需要几秒…' : '正在验证在线会话…'} /> : latest.isLoading && !latestItems.length ? <LoadingState label="正在读取最新树洞…" /> : latest.error && !latestItems.length ? <ErrorState error={latest.error} /> : latestItems.length ? <div className="grid gap-4">{latestItems.map((post) => <PostCard key={post.pid} post={post} source="live" />)}</div> : session.data?.can_read_online ? <div className="panel p-8 text-center text-sm text-ink-soft">当前没有读取到最新内容</div> : <div className="panel p-8 text-center text-sm text-ink-soft">登录后显示最新树洞</div>}
      </section>

      <aside id="hot" className="panel scroll-mt-24 p-5 xl:sticky xl:top-24">
        <div className="flex items-start justify-between gap-3"><div><p className="eyebrow">HOT</p><h2 className="mt-1 text-xl font-semibold">最近 {hotPosts.data?.window_hours || 12} 小时热榜</h2></div><button type="button" className="button-secondary !min-h-9 !px-3 !py-1.5" disabled={hotPosts.isFetching} onClick={() => hotPosts.refetch()} aria-label="刷新热榜"><RefreshCw size={14} className={hotPosts.isFetching ? 'animate-spin' : ''} /></button></div>
		<div className="mt-4 grid gap-2">{hotPosts.isLoading && !hotItems.length ? <p className="rounded-xl border border-dashed border-line p-5 text-center text-xs text-ink-soft">正在读取热榜…</p> : hotPosts.error && !hotItems.length ? <div className="rounded-xl border border-coral/25 bg-coral-soft/20 p-4 text-xs text-ink-soft"><p>热榜暂时不可用：{hotPosts.error.message}</p><button type="button" className="mt-2 font-semibold text-coral hover:underline" onClick={() => hotPosts.refetch()}>重新加载</button></div> : hotItems.length ? displayedHotItems.map((post, index) => { const unavailable = post.availability_state === 'confirmed_unavailable'; return <Link key={post.id} to={unavailable ? `/removed/${post.id}` : `/posts/${post.id}?source=live&return_to=%2Fonline%23hot`} className="rounded-xl border border-line bg-white/45 p-3 text-sm transition hover:border-coral/40 hover:bg-white/75"><span className="mr-2 font-mono text-xs font-semibold text-ink-soft">#{index + 1}</span><span className="font-mono font-semibold text-coral">洞 {post.id}</span>{unavailable && <span className="badge ml-2 !border-coral/25 !py-0.5 !text-coral">已归档</span>}<p className="mt-1 line-clamp-3 text-ink-soft">{post.text || '（无正文）'}</p><span className="mt-2 inline-flex items-center gap-3 text-xs text-ink-soft"><span className="inline-flex items-center gap-1"><Flame size={12} />{post.score !== undefined ? post.score.toFixed(1) : post.follownum}</span>{post.reply !== undefined && <span className="inline-flex items-center gap-1"><MessageCircle size={12} />{post.reply}</span>}</span></Link> }) : <p className="rounded-xl border border-dashed border-line p-5 text-center text-xs text-ink-soft">当前时间范围内没有可用热榜数据</p>}</div>
        {hotItems.length > 5 && <button type="button" className="button-secondary mt-3 w-full !min-h-9 !py-1.5 text-xs" aria-expanded={hotExpanded} onClick={() => setHotExpanded((value) => !value)}>{hotExpanded ? '收起至前 5 条' : `展开第 6–${Math.min(hotItems.length, 10)} 条`}</button>}
		{hotPosts.data && <div className="mt-3 border-t border-line pt-3 text-[11px] leading-5 text-ink-soft"><p>{hotSourceLabel(hotPosts.data.source)} · {hotPosts.data.stale ? '数据范围已降级（已经过期）' : '自动刷新'} · 更新于 {formatUpdatedAt(hotPosts.data.updated_at)}</p>{hotPosts.data.message && <p className="mt-1">{hotPosts.data.message}</p>}{hotPosts.error && <p className="mt-1 text-coral">本次刷新失败，当前继续显示上次结果。</p>}</div>}
      </aside>
    </div>
  </>
}

function OnlineShortcut({ to, icon: Icon, title, note }: { to: string; icon: typeof Radio; title: string; note: string }) {
  return <Link to={to} className="panel flex min-w-0 items-center gap-2.5 p-3 transition hover:-translate-y-0.5 hover:border-teal/40"><div className="grid size-9 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={17} /></div><div className="min-w-0"><p className="truncate text-sm font-semibold">{title}</p><p className="mt-0.5 truncate text-[11px] text-ink-soft">{note}</p></div></Link>
}

function formatUpdatedAt(timestamp: number) {
  if (!timestamp) return '时间未知'
  return new Date(timestamp * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function hotSourceLabel(source: HotPostsResult['source']) {
	if (source === 'observer') return '自建 Observer 热榜'
	if (source === 'observer_cache') return 'Observer 本地缓存'
	return '在线时间线近似榜'
}
