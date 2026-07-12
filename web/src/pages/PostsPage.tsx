import { useState } from 'react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { Filter, Search } from 'lucide-react'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { EmptyState, ErrorState, LoadingState } from '../components/States'

export function PostsPage() {
	const [sort, setSort] = useState('desc')
	const [hasMedia, setHasMedia] = useState('')
	const query = useInfiniteQuery({
		queryKey: ['posts', sort, hasMedia],
		initialPageParam: 0,
		queryFn: ({ pageParam }) => api.posts({ cursor: pageParam, limit: 20, sort, has_media: hasMedia || undefined }),
		getNextPageParam: (lastPage) => lastPage.has_more ? (lastPage.next_cursor ?? undefined) : undefined,
	})
	const displayed = query.data?.pages.flatMap((page) => page.items).filter((post, index, all) => all.findIndex((current) => current.pid === post.pid) === index) ?? []

  return <>
    <PageHeader eyebrow="ARCHIVE" title="资料库" description="按游标逐页读取，不会一次把整张表搬进浏览器。打开任意帖子可继续查看评论、媒体和引用关系。" />
    <div className="panel mb-6 grid gap-3 p-4 sm:grid-cols-[1fr_1fr_auto]">
		<label><span className="mb-1.5 block text-xs font-medium text-ink-soft">排序</span><select className="field" value={sort} onChange={(event) => setSort(event.target.value)}><option value="desc">最新 PID 优先</option><option value="asc">最早 PID 优先</option><option value="reply">评论数优先</option><option value="praise_num">点赞数优先</option></select></label>
		<label><span className="mb-1.5 block text-xs font-medium text-ink-soft">媒体</span><select className="field" value={hasMedia} onChange={(event) => setHasMedia(event.target.value)}><option value="">全部内容</option><option value="true">有图片</option><option value="false">无图片</option></select></label>
      <div className="flex items-end"><a className="button-secondary w-full" href="/search"><Search size={16} />转到搜索</a></div>
    </div>
    {query.isLoading && !displayed.length ? <LoadingState /> : query.error ? <ErrorState error={query.error} /> : displayed.length ? <div className="grid gap-4 xl:grid-cols-2">{displayed.map((post) => <PostCard key={post.pid} post={post} />)}</div> : <EmptyState title="没有符合条件的帖子" description="尝试调整筛选，或先从归档导入、同步一些内容。" action={<Filter size={18} />} />}
	{query.hasNextPage && <div className="mt-6 flex justify-center"><button className="button-secondary" disabled={query.isFetchingNextPage} onClick={() => query.fetchNextPage()}>{query.isFetchingNextPage ? '读取中…' : '加载下一页'}</button></div>}
  </>
}
