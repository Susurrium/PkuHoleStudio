import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, Download, Heart, Image, ImagePlus, MessageCircle, Reply, Send, Star, StickyNote } from 'lucide-react'
import { Link, useLocation, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import { ErrorState, LoadingState } from '../components/States'
import type { Comment, Media, ReferenceGraph } from '../lib/types'

export function PostDetailPage() {
  const { pid = '' } = useParams()
	const [searchParams, setSearchParams] = useSearchParams()
	const location = useLocation()
  const source = searchParams.get('source') === 'live' ? 'live' : 'local'
  const detail = useQuery({ queryKey: ['post', pid, source], queryFn: () => api.post(pid, source), enabled: /^\d+$/.test(pid) })
	const [additionalComments, setAdditionalComments] = useState<Comment[]>([])
	const [pagination, setPagination] = useState<{ cursor?: number; hasMore: boolean } | null>(null)
	const [restoreStatus, setRestoreStatus] = useState('')
	const [restoreCancelled, setRestoreCancelled] = useState(false)
	useEffect(() => { setAdditionalComments([]); setPagination(null); setRestoreStatus(''); setRestoreCancelled(false) }, [pid, source, detail.dataUpdatedAt])
	const loadMoreComments = useMutation({
		mutationFn: (cursor: number) => api.comments(pid, cursor, source, 50),
		onSuccess: (page) => {
			setAdditionalComments((current) => dedupeComments([...current, ...page.items]))
			setPagination({ cursor: page.next_cursor, hasMore: page.has_more })
			if (page.next_cursor) { const next = new URLSearchParams(searchParams); next.set('comment_cursor', String(page.next_cursor)); setSearchParams(next, { replace: true }) }
		},
	})
	const restoreCursor = Number(searchParams.get('comment_cursor') || 0)
	const restoreCID = Number(location.hash.match(/^#comment-(\d+)$/)?.[1] || 0)
	const comments = useMemo(() => dedupeComments([...(detail.data?.comments ?? []), ...additionalComments]), [detail.data?.comments, additionalComments])
	const hasRestoreCID = useMemo(() => restoreCID > 0 && comments.some((item) => item.cid === restoreCID), [comments, restoreCID])
	useEffect(() => {
		if (!detail.data || restoreCancelled || hasRestoreCID || (!restoreCursor && !restoreCID) || (!restoreCID && pagination?.cursor === restoreCursor)) return
		let cancelled = false
		const initial = detail.data.comments
		if (restoreCID && initial.some((item) => item.cid === restoreCID)) return
		const restore = async () => {
			let cursor = detail.data?.next_comment_cursor
			let hasMore = detail.data?.has_more_comments ?? false
			let loaded: Comment[] = []
			let pages = 0
			while (!cancelled && !restoreCancelled && hasMore && cursor !== undefined && pages < 40) {
				if (restoreCursor && cursor === restoreCursor && !restoreCID) break
				setRestoreStatus(`正在恢复评论位置…已读取 ${initial.length + loaded.length} 条`)
				const page = await api.comments(pid, cursor, source, 50)
				loaded = dedupeComments([...loaded, ...page.items])
				setAdditionalComments(loaded)
				setPagination({ cursor: page.next_cursor, hasMore: page.has_more })
				pages++
				if (restoreCID && loaded.some((item) => item.cid === restoreCID)) break
				if (restoreCursor && page.next_cursor === restoreCursor && !restoreCID) break
				cursor = page.next_cursor
				hasMore = page.has_more
			}
			if (cancelled) return
			const found = !restoreCID || initial.some((item) => item.cid === restoreCID) || loaded.some((item) => item.cid === restoreCID)
			setRestoreStatus(found ? '' : pages >= 40 ? '自动恢复已达到 2000 条安全上限，可继续手动加载。' : `未在此洞中找到 C${restoreCID}。`)
		}
		restore().catch((error) => { if (!cancelled) setRestoreStatus(`恢复评论位置失败：${String(error)}`) })
		return () => { cancelled = true }
	}, [detail.dataUpdatedAt, pid, source, restoreCursor, restoreCID, restoreCancelled, hasRestoreCID])
	useEffect(() => {
		if (!hasRestoreCID) return
		requestAnimationFrame(() => document.getElementById(`comment-${restoreCID}`)?.scrollIntoView?.({ behavior: 'smooth', block: 'center' }))
	}, [restoreCID, hasRestoreCID])
	const saveLocal = useMutation({ mutationFn: () => api.createJob('sync_pids', { pids: [Number(pid)] }) })
	const [replyText, setReplyText] = useState('')
	const [quoteCID, setQuoteCID] = useState<number | undefined>()
	const [replyFiles, setReplyFiles] = useState<File[]>([])
	const submitReply = useMutation({ mutationFn: async () => { const ids: string[] = []; for (const file of replyFiles) ids.push((await api.uploadMedia(file)).id); return api.createComment(Number(pid), replyText, quoteCID, ids) }, onSuccess: () => { setReplyText(''); setQuoteCID(undefined); setReplyFiles([]); detail.refetch() } })
	const interact = useMutation({ mutationFn: (action: 'praise' | 'follow') => api.togglePost(Number(pid), action), onSuccess: () => detail.refetch() })
  if (detail.isLoading) return <LoadingState label={`正在读取 #${pid}…`} />
  if (detail.error || !detail.data) return <ErrorState error={detail.error ?? new Error('帖子不存在')} />
	const { post, references, media = [] } = detail.data
	const nextCommentCursor = pagination?.cursor ?? detail.data.next_comment_cursor
	const hasMoreComments = pagination?.hasMore ?? detail.data.has_more_comments
	const postMedia = media.filter((item) => item.owner_type === 'post' && item.owner_id === post.pid)
	  const requestedReturn = searchParams.get('return_to')
	  const returnTo = requestedReturn && /^\/(posts|search)([/?]|$)/.test(requestedReturn) ? requestedReturn : `/posts${source === 'live' ? '?source=live' : ''}`
	  return <>
	    <div className="mb-6 flex flex-wrap items-center justify-between gap-3"><Link to={returnTo} className="inline-flex items-center gap-2 text-sm font-medium text-ink-soft hover:text-teal"><ArrowLeft size={16} />返回{source === 'live' ? '在线树洞' : requestedReturn?.startsWith('/search') ? '搜索结果' : '资料库'}</Link>{source === 'live' && <button className="button-secondary" disabled={saveLocal.isPending || saveLocal.isSuccess} onClick={() => saveLocal.mutate()}><Download size={15} />{saveLocal.isPending ? '正在创建任务…' : saveLocal.isSuccess ? '已加入同步队列' : '保存到本地资料库'}</button>}</div>
    <article className="panel overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white/40 px-5 py-4 md:px-7"><span className="font-mono text-base font-bold text-coral">#{post.pid}</span><time className="text-xs text-ink-soft">{formatTime(post.timestamp)}</time></header>
      <div className="px-5 py-7 md:px-7"><p className="whitespace-pre-wrap text-base leading-8 md:text-[17px]">{post.text || '（无正文）'}</p><MediaGallery items={postMedia} pid={post.pid} /></div>
      <footer className="flex flex-wrap items-center gap-5 border-t border-line bg-paper/45 px-5 py-4 text-xs text-ink-soft md:px-7"><span className="inline-flex items-center gap-1.5"><MessageCircle size={14} />{post.reply ?? comments.length} 条评论</span>{post.praise_num !== undefined && <span>点赞 {post.praise_num}</span>}{source === 'live' && <div className="ml-auto flex gap-2"><button className="button-secondary !px-3 !py-1.5" disabled={interact.isPending} onClick={() => interact.mutate('praise')}><Heart size={14} fill={post.is_praise ? 'currentColor' : 'none'} />{post.is_praise ? '取消点赞' : '点赞'}</button><button className="button-secondary !px-3 !py-1.5" disabled={interact.isPending} onClick={() => interact.mutate('follow')}><Star size={14} fill={post.is_follow ? 'currentColor' : 'none'} />{post.is_follow ? '取消关注' : '关注'}</button></div>}</footer>
    </article>
		{source === 'local' && <LocalMetadata pid={post.pid} />}
		{source === 'live' && <section className="panel mt-6 p-5"><div className="flex items-center justify-between"><div><p className="eyebrow">REPLY</p><h2 className="mt-1 text-lg font-semibold">回复此洞</h2></div>{quoteCID && <button className="badge" onClick={() => setQuoteCID(undefined)}>引用 C{quoteCID} ×</button>}</div><textarea className="field mt-4 min-h-24" value={replyText} maxLength={10000} onChange={(event) => setReplyText(event.target.value)} placeholder={quoteCID ? `回复并引用 C${quoteCID}…` : '写下回复…'} /><div className="mt-3 flex flex-wrap items-center justify-between gap-3"><label className="button-secondary cursor-pointer"><ImagePlus size={16} />选择图片<input className="hidden" type="file" accept="image/*" multiple onChange={(event) => setReplyFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><div className="flex items-center gap-3"><span className="text-xs text-ink-soft">{replyFiles.length ? `${replyFiles.length} 张图片` : submitReply.error ? String(submitReply.error) : ''}</span><button className="button-primary" disabled={submitReply.isPending || (!replyText.trim() && !replyFiles.length)} onClick={() => submitReply.mutate()}><Send size={15} />{submitReply.isPending ? '正在发送…' : '发送回复'}</button></div></div></section>}
    <section className="mt-7 grid gap-6 xl:grid-cols-[1fr_300px]">
	      <div><div className="mb-4 flex items-center justify-between"><h2 className="text-xl font-semibold">评论</h2><span className="badge">已载入 {comments.length}{post.reply ? ` / ${post.reply}` : ''}</span></div>{restoreStatus && <div className="panel mb-3 flex items-center justify-between gap-3 border-coral/25 px-4 py-3 text-xs text-ink-soft"><span>{restoreStatus}</span>{restoreStatus.startsWith('正在') && <button className="button-secondary !py-1" onClick={() => { setRestoreCancelled(true); setRestoreStatus('已取消自动恢复，可继续手动加载。') }}>取消</button>}</div>}<div className="grid gap-3">{comments.length ? comments.map((comment) => <CommentCard key={comment.cid} comment={comment} source={source} media={media} pid={post.pid} quote={() => { setQuoteCID(comment.cid); document.querySelector('textarea')?.focus(); window.scrollTo({ top: 0, behavior: 'smooth' }) }} />) : <p className="panel p-7 text-center text-sm text-ink-soft">暂无{source === 'local' ? '本地' : '在线'}评论</p>}</div>{hasMoreComments && nextCommentCursor !== undefined && <button className="button-secondary mt-4 w-full" disabled={loadMoreComments.isPending} onClick={() => loadMoreComments.mutate(nextCommentCursor)}>{loadMoreComments.isPending ? '正在加载更多评论…' : '加载更多评论'}</button>}{loadMoreComments.error && <p className="mt-3 text-sm text-coral">加载评论失败：{loadMoreComments.error.message}</p>}</div>
      <aside><h2 className="mb-4 text-xl font-semibold">引用关系</h2>{source === 'live' ? <div className="panel p-4"><p className="py-5 text-center text-xs leading-5 text-ink-soft">引用图谱来自本地资料库；同步此洞后即可建立关系。</p></div> : <ReferenceGraphPanel pid={post.pid} fallback={references} />}</aside>
    </section>
  </>
}

function CommentCard({ comment, source, media, pid, quote }: { comment: Comment; source: 'local' | 'live'; media: Media[]; pid: number; quote: () => void }) {
	const [showNote, setShowNote] = useState(false)
	const note = useQuery({ queryKey: ['comment-note', comment.cid], queryFn: () => api.commentNote(comment.cid), enabled: source === 'local' && showNote })
	const [content, setContent] = useState('')
	useEffect(() => setContent(note.data?.content ?? ''), [note.data])
	const save = useMutation({ mutationFn: () => api.saveCommentNote(comment.cid, content), onSuccess: () => note.refetch() })
	const items = media.filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)
	const commentMedia = items.length || source === 'local' ? items : remoteCommentMedia(comment)
	return <article id={`comment-${comment.cid}`} className="panel p-5"><div className="flex items-center justify-between gap-3"><div><span className="font-mono text-xs text-teal">C{comment.cid}</span><span className="ml-2 text-xs font-medium text-ink-soft">{comment.name_tag || '匿名'}</span>{comment.is_lz ? <span className="ml-2 badge !py-0.5">洞主</span> : null}</div><div className="flex items-center gap-2"><time className="text-[11px] text-ink-soft">{formatTime(comment.timestamp)}</time>{source === 'local' && <button className="button-secondary !px-2 !py-1 text-xs" onClick={() => setShowNote((value) => !value)}><StickyNote size={12} />笔记</button>}{source === 'live' && <button className="button-secondary !px-2 !py-1 text-xs" onClick={quote}><Reply size={12} />引用</button>}</div></div>{comment.quote && <blockquote className="mt-3 rounded-lg bg-paper px-3 py-2 text-xs leading-5 text-ink-soft">引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}<p className="mt-3 whitespace-pre-wrap text-sm leading-7">{comment.text}</p><MediaGallery items={commentMedia} pid={pid} />{showNote && <div className="mt-4 rounded-xl border border-line bg-paper/45 p-3"><textarea className="field min-h-20" value={content} maxLength={100000} onChange={(event) => setContent(event.target.value)} placeholder="只保存在本机的评论笔记…" /><div className="mt-2 flex items-center justify-between gap-3"><span className="text-xs text-ink-soft">{save.error ? String(save.error) : save.isSuccess ? '已保存' : ''}</span><button className="button-secondary" disabled={save.isPending || note.isLoading} onClick={() => save.mutate()}>保存评论笔记</button></div></div>}</article>
}

function dedupeComments(items: Comment[]) {
	const seen = new Set<number>()
	return items.filter((item) => item.cid > 0 && !seen.has(item.cid) && Boolean(seen.add(item.cid)))
}

function remoteCommentMedia(comment: Comment): Media[] {
	return (comment.media_ids ?? '').split(/[;,\s]+/).map((value) => value.trim()).filter(Boolean).map((remoteID, index) => ({ id: -(comment.cid * 100 + index + 1), owner_type: 'comment', owner_id: comment.cid, remote_id: remoteID, variant: 'original', status: 'remote' }))
}

function ReferenceGraphPanel({ pid, fallback }: { pid: number; fallback: import('../lib/types').Reference[] }) {
	const [depth, setDepth] = useState<1 | 2>(1)
	const query = useQuery({ queryKey: ['reference-graph', pid, depth], queryFn: () => api.referenceGraph(pid, depth) })
	const graph = query.data
	return <div className="panel overflow-hidden"><div className="flex items-center justify-between border-b border-line px-4 py-3"><span className="text-xs text-ink-soft">{graph ? `${graph.nodes.length} 个洞 · ${graph.edges.length} 条关系` : '局部关系网络'}</span><select className="field !w-auto !py-1 text-xs" value={depth} onChange={(event) => setDepth(Number(event.target.value) as 1 | 2)}><option value={1}>1 层</option><option value={2}>2 层</option></select></div>{graph && graph.nodes.length > 1 ? <ReferenceSVG graph={graph} /> : <div className="p-4">{fallback.length ? <div className="grid gap-3">{fallback.map((reference, index) => <ReferenceRow key={`${reference.kind}-${index}`} reference={reference} currentPID={pid} />)}</div> : <p className="py-5 text-center text-xs leading-5 text-ink-soft">{query.isLoading ? '正在展开引用关系…' : query.error ? `图谱读取失败：${String(query.error)}` : '尚未发现引用关系'}</p>}</div>}</div>
}

function ReferenceSVG({ graph }: { graph: ReferenceGraph }) {
	const nodes = graph.nodes.slice(0, 20)
	const positions = new Map<number, { x: number; y: number }>()
	positions.set(graph.root, { x: 140, y: 130 })
	const others = nodes.filter((node) => node.pid !== graph.root)
	others.forEach((node, index) => { const angle = (Math.PI * 2 * index) / others.length - Math.PI / 2; positions.set(node.pid, { x: 140 + Math.cos(angle) * 100, y: 130 + Math.sin(angle) * 100 }) })
	return <svg viewBox="0 0 280 260" role="img" aria-label={`#${graph.root} 的引用关系图`} className="w-full bg-paper/30">{graph.edges.map((edge, index) => { const from = positions.get(edge.source_pid); const to = positions.get(edge.target_pid); if (!from || !to) return null; return <line key={`${edge.kind}-${index}`} x1={from.x} y1={from.y} x2={to.x} y2={to.y} stroke={edge.kind === 'explicit' ? '#ef6548' : edge.kind === 'quoted_comment' ? '#0f766e' : '#94a3b8'} strokeWidth="1.5" strokeDasharray={edge.kind === 'inferred' ? '4 3' : undefined}><title>{edge.kind}</title></line>})}{nodes.map((node) => { const point = positions.get(node.pid); if (!point) return null; const root = node.pid === graph.root; return <Link key={node.pid} to={`/posts/${node.pid}`}><circle cx={point.x} cy={point.y} r={root ? 24 : 19} fill={root ? '#14333b' : '#fffaf2'} stroke={root ? '#14333b' : '#0f766e'} strokeWidth="2"><title>{node.text || `#${node.pid}`}</title></circle><text x={point.x} y={point.y + 3} textAnchor="middle" fontSize={root ? 10 : 8} fontWeight="700" fill={root ? 'white' : '#14333b'}>#{node.pid}</text></Link>})}</svg>
}

function LocalMetadata({ pid }: { pid: number }) {
	const tags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
	const assigned = useQuery({ queryKey: ['post-tags', pid], queryFn: () => api.postTags(pid) })
	const note = useQuery({ queryKey: ['post-note', pid], queryFn: () => api.postNote(pid) })
	const [selected, setSelected] = useState<number[]>([])
	const [content, setContent] = useState('')
	const [tagName, setTagName] = useState('')
	useEffect(() => setSelected(assigned.data?.map((tag) => tag.id) ?? []), [assigned.data])
	useEffect(() => setContent(note.data?.content ?? ''), [note.data])
	const saveTags = useMutation({ mutationFn: () => api.setPostTags(pid, selected), onSuccess: () => assigned.refetch() })
	const saveNote = useMutation({ mutationFn: () => api.savePostNote(pid, content), onSuccess: () => note.refetch() })
	const createTag = useMutation({ mutationFn: () => api.createLocalTag(tagName, ''), onSuccess: () => { setTagName(''); tags.refetch() } })
	return <section className="panel mt-6 p-5"><p className="eyebrow">LOCAL METADATA</p><h2 className="mt-1 text-lg font-semibold">本地标签与笔记</h2><div className="mt-4 grid gap-5 lg:grid-cols-2"><div><p className="text-xs font-medium text-ink-soft">标签</p><div className="mt-2 flex flex-wrap gap-2">{tags.data?.length ? tags.data.map((tag) => <label key={tag.id} className={`badge cursor-pointer ${selected.includes(tag.id) ? '!border-teal !bg-teal-soft !text-teal' : ''}`}><input className="hidden" type="checkbox" checked={selected.includes(tag.id)} onChange={() => setSelected((value) => value.includes(tag.id) ? value.filter((id) => id !== tag.id) : [...value, tag.id])} />{tag.name}</label>) : <span className="text-xs text-ink-soft">尚未创建标签</span>}</div><div className="mt-3 flex gap-2"><input className="field" value={tagName} onChange={(event) => setTagName(event.target.value)} placeholder="新标签名称" /><button className="button-secondary shrink-0" disabled={!tagName.trim() || createTag.isPending} onClick={() => createTag.mutate()}>创建</button></div><button className="button-secondary mt-3" disabled={saveTags.isPending} onClick={() => saveTags.mutate()}>保存标签</button></div><div><label className="text-xs font-medium text-ink-soft">笔记<textarea className="field mt-2 min-h-28" value={content} maxLength={100000} onChange={(event) => setContent(event.target.value)} placeholder="只保存在本机的笔记…" /></label><button className="button-secondary mt-3" disabled={saveNote.isPending} onClick={() => saveNote.mutate()}>保存笔记</button></div></div></section>
}

function ReferenceRow({ reference, currentPID }: { reference: import('../lib/types').Reference; currentPID: number }) {
	const outbound = reference.source_pid === currentPID
	const otherPID = outbound ? reference.target_pid : reference.source_pid
	const otherCID = outbound ? reference.target_cid : reference.source_cid
	const kind = reference.kind === 'explicit' ? '明确引用' : reference.kind === 'inferred' ? '推断引用' : reference.kind === 'quoted_comment' ? '评论引用' : reference.kind
	return <div className="rounded-lg border border-line bg-white/45 p-3 text-xs leading-5"><div className="mb-2 flex flex-wrap gap-2"><span className="badge">{outbound ? '引用了' : '被引用'}</span><span className="badge">{kind}</span></div><Link className="font-semibold text-coral hover:underline" to={`/posts/${otherPID}${otherCID ? `#comment-${otherCID}` : ''}`}>#{otherPID}{otherCID ? ` / C${otherCID}` : ''}</Link></div>
}

function MediaGallery({ items, pid }: { items: Media[]; pid?: number }) {
	if (!items.length) return null
	return <div className="mt-5 grid gap-3 sm:grid-cols-2">{items.map((item, index) => item.status === 'available' || item.status === 'remote'
		? <MediaImage key={`${item.owner_type}-${item.owner_id}-${item.remote_id ?? index}`} item={item} pid={pid} />
		: <div key={item.id} className="flex min-h-24 items-center justify-center gap-2 rounded-xl border border-dashed border-line bg-paper/60 px-4 text-sm text-ink-soft"><Image size={17} />图片尚未下载{item.remote_id ? ` · ${item.remote_id}` : ''}</div>)}</div>
}

function MediaImage({ item, pid }: { item: Media; pid?: number }) {
	const url = item.status === 'remote' ? `/api/v1/remote-media/${item.remote_id || '_'}?pid=${pid}` : `/api/v1/media/${item.id}`
	return <a href={url} target="_blank" rel="noreferrer" className="overflow-hidden rounded-xl border border-line bg-paper"><img src={url} alt="" loading="lazy" className="max-h-[32rem] w-full object-contain" /></a>
}
