import { FormEvent, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { LocalTagFilter, parseLocalTagIDs } from '../components/LocalTagFilter'

export function SearchPage() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const cursor = Number(params.get('cursor') ?? 0)
	const tagIDs = parseLocalTagIDs(params.get('tag'))
	const tagKey = tagIDs.join(',')
	const [draft, setDraft] = useState(q)
	const results = useQuery({ queryKey: ['search', q, cursor, tagKey], queryFn: () => api.search({ q, cursor, limit: 20, tag: tagKey || undefined }), enabled: Boolean(q) })
	const history = useQuery({ queryKey: ['search-history'], queryFn: api.searchHistory, enabled: !q })
	function updateTags(ids: number[]) { const next = new URLSearchParams(params); next.delete('cursor'); if (ids.length) next.set('tag', ids.join(',')); else next.delete('tag'); setParams(next, { replace: true }) }
  function submit(event: FormEvent) { event.preventDefault(); const value = draft.trim(); const next = new URLSearchParams(params); next.delete('cursor'); if (value) next.set('q', value); else next.delete('q'); setParams(next) }
  return <>
    <PageHeader eyebrow="FULL TEXT" title="全文搜索" description="输入多个词时默认按 AND 匹配；直接输入 123456 或 #123456 都可精确定位 PID。评论命中会聚合回所属帖子。" />
    <form className="panel flex flex-col gap-3 p-4 sm:flex-row" onSubmit={submit}>
      <label className="relative flex-1"><span className="sr-only">搜索关键词</span><Search className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft" size={17} /><input className="field !pl-10" value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="课程名、教师、关键词或 PID" /></label>
      <button className="button-primary sm:min-w-28" type="submit">搜索资料库</button>
    </form>
		<section className="panel mt-4 p-4"><p className="mb-2 text-xs font-medium text-ink-soft">按本地标签缩小结果（选择多个时需同时拥有全部标签）</p><LocalTagFilter selected={tagIDs} onChange={updateTags} /></section>
    <div className="mt-6">
		{!q ? <><EmptyState title="从一个具体问题开始" description="例如“数据结构 作业量”或“#123456”。所有检索默认只在本地资料库中进行。" />{history.data?.length ? <div className="panel mt-5 p-5"><p className="eyebrow">RECENT QUERIES</p><div className="mt-3 flex flex-wrap gap-2">{history.data.map((item) => <button key={item.id} className="badge hover:border-teal hover:text-teal" onClick={() => { setDraft(item.query); const next = new URLSearchParams(params); next.set('q', item.query); next.delete('cursor'); setParams(next) }}>{item.query}</button>)}</div></div> : null}</> : results.isLoading ? <LoadingState label={`正在检索“${q}”…`} /> : results.error ? <ErrorState error={results.error} /> : results.data?.items.length ? <><p className="mb-4 text-sm text-ink-soft">当前页找到 {results.data.items.length} 个帖子结果</p><div className="grid gap-4 xl:grid-cols-2">{results.data.items.map((post) => <PostCard key={post.pid} post={post} />)}</div>{results.data.has_more && <div className="mt-6 flex justify-center"><button className="button-secondary" onClick={() => { const next = new URLSearchParams(params); next.set('cursor', String(results.data?.next_cursor ?? 0)); setParams(next) }}>下一页</button></div>}</> : <EmptyState title="没有找到匹配内容" description="减少关键词、检查 PID，或同步更多资料后重试。" />}
    </div>
  </>
}
