import { useInfiniteQuery } from '@tanstack/react-query'
import { useQuery } from '@tanstack/react-query'
import { Filter, Radio, Search } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

export function PostsPage() {
	const [params, setParams] = useSearchParams()
	const source = params.get('source') === 'live' ? 'live' : 'local'
	const sort = params.get('sort') || 'desc'
	const hasMedia = params.get('media') || ''
	const followed = source === 'live' && params.get('followed') === 'true'
	const label = source === 'live' ? params.get('label') || '' : ''
	const setParam = (name: string, value: string) => {
		const next = new URLSearchParams(params)
		if (value) next.set(name, value); else next.delete(name)
		setParams(next, { replace: true })
	}
	const online = useQuery({ queryKey: ['online-posts-session'], queryFn: api.probeSession, enabled: source === 'live', retry: false })
	const tags = useQuery({ queryKey: ['live-tags'], queryFn: api.tags, enabled: source === 'live' && online.data?.can_read_online === true })
	const canRead = source === 'local' || online.data?.can_read_online === true
	const query = useInfiniteQuery({
		queryKey: ['posts', source, sort, hasMedia, followed, label],
		initialPageParam: 0,
		queryFn: ({ pageParam }) => api.posts({ cursor: pageParam, limit: 20, source, sort, has_media: hasMedia || undefined, q: followed ? ':follow' : undefined, label: label || undefined }),
		getNextPageParam: (lastPage) => lastPage.has_more ? (lastPage.next_cursor ?? undefined) : undefined,
		enabled: canRead,
	})
	const displayed = query.data?.pages.flatMap((page) => page.items).filter((post, index, all) => all.findIndex((current) => current.pid === post.pid) === index) ?? []

  return <>
    <PageHeader eyebrow={source === 'live' ? 'LIVE TREEHOLE' : 'ARCHIVE'} title={source === 'live' ? '在线树洞' : '资料库'} description={source === 'live' ? '直接读取当前树洞内容；浏览本身不会自动写入本地资料库。' : '按游标逐页读取，不会一次把整张表搬进浏览器。打开任意帖子可继续查看评论、媒体和引用关系。'} />
		<div className="mb-5 inline-flex rounded-xl border border-line bg-white/50 p-1"><button className={`rounded-lg px-4 py-2 text-sm font-medium ${source === 'local' ? 'bg-ink text-white' : 'text-ink-soft'}`} onClick={() => setParam('source', '')}>本地资料库</button><button className={`rounded-lg px-4 py-2 text-sm font-medium ${source === 'live' ? 'bg-ink text-white' : 'text-ink-soft'}`} onClick={() => setParam('source', 'live')}><Radio size={14} className="mr-1 inline" />在线树洞</button></div>
		{source === 'live' && online.isLoading && <div className="mb-5"><LoadingState label="正在验证在线会话…" /></div>}
		{source === 'live' && !online.isLoading && !online.data?.can_read_online && <div className="panel mb-5 border-coral/30 bg-coral-soft/30 p-5 text-sm"><p className="font-semibold">需要先登录树洞</p><p className="mt-1 text-ink-soft">{online.data?.message || '当前本机会话不能读取在线内容。'}</p><Link className="button-primary mt-4" to="/sync">前往同步中心登录</Link></div>}
    <div className="panel mb-6 grid gap-3 p-4 sm:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_auto]">
		<label><span className="mb-1.5 block text-xs font-medium text-ink-soft">排序</span><select className="field" value={sort} disabled={source === 'live'} onChange={(event) => setParam('sort', event.target.value)}><option value="desc">最新 PID 优先</option><option value="asc">最早 PID 优先</option><option value="reply">评论数优先</option><option value="praise_num">点赞数优先</option></select></label>
		<label><span className="mb-1.5 block text-xs font-medium text-ink-soft">媒体</span><select className="field" value={hasMedia} onChange={(event) => setParam('media', event.target.value)}><option value="">全部内容</option><option value="true">有图片</option><option value="false">无图片</option></select></label>
		{source === 'live' ? <label><span className="mb-1.5 block text-xs font-medium text-ink-soft">在线筛选</span><select className="field" value={followed ? 'followed' : label ? `label:${label}` : ''} onChange={(event) => { const value = event.target.value; const next = new URLSearchParams(params); next.delete('followed'); next.delete('label'); if (value === 'followed') next.set('followed', 'true'); else if (value.startsWith('label:')) next.set('label', value.slice(6)); setParams(next, { replace: true }) }}><option value="">公共时间线</option><option value="followed">我关注的洞</option>{tags.data?.filter((tag) => tag.label || tag.name).map((tag) => <option key={tag.id} value={`label:${tag.id}`}>{tag.label || tag.name}</option>)}</select></label> : <div />}
      <div className="flex items-end"><a className="button-secondary w-full" href="/search"><Search size={16} />转到搜索</a></div>
    </div>
		{canRead && (query.isLoading && !displayed.length ? <LoadingState /> : query.error ? <ErrorState error={query.error} /> : displayed.length ? <div className="grid gap-4 xl:grid-cols-2">{displayed.map((post) => <PostCard key={post.pid} post={post} source={source} />)}</div> : <EmptyState title="没有符合条件的帖子" description="尝试调整筛选，或先从归档导入、同步一些内容。" action={<Filter size={18} />} />)}
	{query.hasNextPage && <div className="mt-6 flex justify-center"><button className="button-secondary" disabled={query.isFetchingNextPage} onClick={() => query.fetchNextPage()}>{query.isFetchingNextPage ? '读取中…' : '加载下一页'}</button></div>}
  </>
}
