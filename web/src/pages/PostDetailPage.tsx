import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, Check, Download, Heart, Image, ImagePlus, MessageCircle, Radio, Reply, Send, Sparkles, Star, StickyNote } from 'lucide-react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import { preferredScrollBehavior } from '../lib/motion'
import { ErrorState, LoadingState } from '../components/States'
import type { Comment, Media, ReferenceGraph } from '../lib/types'
import { errorDescription } from '../components/Feedback'
import { useLocalPostMetadata, usePostComments, usePostDetailResource, usePostInteraction, usePostReply, useSavePostToLocal } from '../features/posts/usePostDetail'

export function PostDetailPage() {
  const { pid = '' } = useParams()
	const [searchParams] = useSearchParams()
  const source = searchParams.get('source') === 'live' ? 'live' : 'local'
  const { detail, online } = usePostDetailResource(pid, source)
	const { comments, loadMoreComments, restoreStatus, cancelRestore, nextCommentCursor, hasMoreComments } = usePostComments(pid, source, detail.data, detail.dataUpdatedAt)
	const saveLocal = useSavePostToLocal(pid)
	const reply = usePostReply(pid, () => { detail.refetch() })
	const { text: replyText, setText: setReplyText, quoteCID, setQuoteCID, files: replyFiles, setFiles: setReplyFiles, mutation: submitReply } = reply
	const replyComposer = useRef<HTMLElement>(null)
	const replyInput = useRef<HTMLTextAreaElement>(null)
	const interact = usePostInteraction(pid, () => { detail.refetch() })
  if (detail.isLoading) return <LoadingState label={`正在读取 #${pid}…`} />
  if (detail.error || !detail.data) return <ErrorState error={detail.error ?? new Error('帖子不存在')} />
	const { post, references, media = [] } = detail.data
	const postMedia = media.filter((item) => item.owner_type === 'post' && item.owner_id === post.pid)
	const canWrite = source === 'live' && online.data?.can_write_online === true
	  const requestedReturn = searchParams.get('return_to')
	  const returnTo = requestedReturn && /^\/(online|posts|search|ai|notifications)([/?]|$)/.test(requestedReturn) ? requestedReturn : `/posts${source === 'live' ? '?source=live' : ''}`
	  const returnLabel = requestedReturn?.startsWith('/ai') ? 'AI 研究' : requestedReturn?.startsWith('/notifications') ? '通知' : source === 'live' ? '在线树洞' : requestedReturn?.startsWith('/search') || requestedReturn?.includes('q=') ? '搜索结果' : '浏览列表'
	  return <>
	    <div className="mb-5 flex flex-wrap items-center justify-between gap-3"><Link to={returnTo} className="inline-flex items-center gap-2 rounded-lg px-2 py-1 text-sm font-medium text-ink-soft hover:bg-white/60 hover:text-teal"><ArrowLeft size={16} />返回{returnLabel}</Link><div className="flex flex-wrap gap-2">{source === 'local' && <Link className="button-secondary" to={`/posts/${pid}?source=live&return_to=${encodeURIComponent(`/posts/${pid}`)}`}><Radio size={15} />查看线上版本</Link>}{source === 'live' && (!saveLocal.job && detail.data.local_state === 'saved' ? <span className="button-secondary cursor-default text-teal"><Check size={15} />已保存到本地</span> : <button className={`button-secondary ${saveLocal.retryable ? '!text-coral' : ''}`} disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start} title={saveLocal.failure || undefined}><Download size={15} />{saveLocal.label}</button>)}</div></div>
	{source === 'live' && saveLocal.failure && <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-coral/25 bg-coral-soft/25 px-4 py-3 text-xs text-coral" role="alert"><span>保存任务失败：{saveLocal.failure}</span><Link className="font-semibold underline" to="/tasks">查看任务详情</Link></div>}
	<div className="grid items-start gap-7 xl:grid-cols-[minmax(0,860px)_minmax(280px,1fr)]">
	  <section className="min-w-0">
    <article className="panel overflow-hidden" aria-labelledby="post-title">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white/40 px-5 py-4 md:px-7"><span className="font-mono text-base font-bold text-coral">#{post.pid}</span><time className="text-xs text-ink-soft">{formatTime(post.timestamp)}</time></header>
      <div className="px-5 py-7 md:px-7"><p id="post-title" className="whitespace-pre-wrap text-base leading-8 md:text-[17px]">{post.text || '（无正文）'}</p><MediaGallery items={postMedia} pid={post.pid} /></div>
	      <footer className="flex flex-wrap items-center gap-5 border-t border-line bg-paper/45 px-5 py-4 text-xs text-ink-soft md:px-7"><a href="#comments" className="inline-flex items-center gap-1.5 hover:text-teal"><MessageCircle size={14} />{post.reply ?? comments.length} 条评论</a>{post.praise_num !== undefined && <span>点赞 {post.praise_num}</span>}{(source === 'local' || detail.data.local_state === 'saved') && <Link className="inline-flex items-center gap-1.5 font-semibold text-teal hover:underline" to={`/ai?mode=selected&pids=${post.pid}`}><Sparkles size={14} />研究此洞</Link>}{canWrite && <div className="ml-auto flex gap-2"><button className="button-secondary !px-3 !py-1.5" disabled={interact.isPending} onClick={() => interact.mutate('praise')}><Heart size={14} fill={post.is_praise ? 'currentColor' : 'none'} />{post.is_praise ? '取消点赞' : '点赞'}</button><button className="button-secondary !px-3 !py-1.5" disabled={interact.isPending} onClick={() => interact.mutate('follow')}><Star size={14} fill={post.is_follow ? 'currentColor' : 'none'} />{post.is_follow ? '取消关注' : '关注'}</button></div>}{source === 'live' && !canWrite && !online.isLoading && <Link className="ml-auto font-semibold text-coral hover:underline" to="/sync">登录后互动</Link>}</footer>
    </article>
		{canWrite && <section ref={replyComposer} className="panel mt-6 scroll-mt-24 p-5"><div className="flex items-center justify-between gap-3"><div><p className="eyebrow">REPLY</p><h2 className="mt-1 text-lg font-semibold">回复此洞</h2></div>{quoteCID && <button type="button" className="badge cursor-pointer" onClick={() => setQuoteCID(undefined)}>引用 C{quoteCID} ×</button>}</div><textarea ref={replyInput} aria-label="回复内容" className="field mt-4 min-h-24" value={replyText} maxLength={10000} onChange={(event) => setReplyText(event.target.value)} placeholder={quoteCID ? `回复并引用 C${quoteCID}…` : '写下回复…'} />{submitReply.error && <p className="mt-3 rounded-lg border border-coral/25 bg-coral-soft/25 px-3 py-2 text-xs text-coral">回复失败：{errorDescription(submitReply.error)}。正文和图片选择已保留。</p>}<div className="mt-3 flex flex-wrap items-center justify-between gap-3"><label className="button-secondary cursor-pointer"><ImagePlus size={16} />选择图片<input className="hidden" type="file" accept="image/*" multiple onChange={(event) => setReplyFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><div className="flex items-center gap-3"><span className="text-xs text-ink-soft">{replyFiles.length ? `${replyFiles.length} 张图片 · ` : ''}{replyText.length}/10000</span><button className="button-primary" disabled={submitReply.isPending || (!replyText.trim() && !replyFiles.length)} onClick={() => submitReply.mutate()}><Send size={15} />{submitReply.isPending ? '正在发送…' : '发送回复'}</button></div></div></section>}
		{source === 'live' && !canWrite && !online.isLoading && <section className="panel mt-6 p-5 text-sm text-ink-soft">当前会话不能回复此洞。<Link className="ml-1 font-semibold text-coral hover:underline" to="/sync">前往登录或重新检测会话</Link></section>}
	    <section id="comments" className="mt-7 scroll-mt-24"><div className="mb-4 flex items-center justify-between"><h2 className="text-xl font-semibold">评论</h2><span className="badge">已载入 {comments.length}{post.reply ? ` / ${post.reply}` : ''}</span></div>{restoreStatus && <div className="panel mb-3 flex items-center justify-between gap-3 border-coral/25 px-4 py-3 text-xs text-ink-soft"><span>{restoreStatus}</span>{restoreStatus.startsWith('正在') && <button className="button-secondary !py-1" onClick={cancelRestore}>取消</button>}</div>}<div className="grid gap-3">{comments.length ? comments.map((comment) => <CommentCard key={comment.cid} comment={comment} source={source} media={media} pid={post.pid} canReply={canWrite} quote={() => { setQuoteCID(comment.cid); replyInput.current?.focus(); requestAnimationFrame(() => replyComposer.current?.scrollIntoView?.({ behavior: preferredScrollBehavior(), block: 'center' })) }} />) : <p className="panel p-7 text-center text-sm text-ink-soft">暂无{source === 'local' ? '本地' : '在线'}评论</p>}</div>{hasMoreComments && nextCommentCursor !== undefined && <button className="button-secondary mt-4 w-full" disabled={loadMoreComments.isPending} onClick={() => loadMoreComments.mutate(nextCommentCursor)}>{loadMoreComments.isPending ? '正在加载更多评论…' : '继续加载评论'}</button>}{loadMoreComments.error && <p className="mt-3 text-sm text-coral">加载评论失败：{loadMoreComments.error.message}</p>}</section>
	  </section>
	  <aside className="grid gap-5 xl:sticky xl:top-24">
		{(source === 'local' || detail.data.local_state === 'saved') && <LocalMetadata pid={post.pid} />}
		<section><h2 className="mb-3 text-lg font-semibold">引用关系</h2>{source === 'live' ? <div className="panel p-4"><p className="py-5 text-center text-xs leading-5 text-ink-soft">引用图谱保存在本地；保存此洞后即可建立关系。</p></div> : <ReferenceGraphPanel pid={post.pid} fallback={references} />}</section>
	  </aside>
	</div>
  </>
}

function CommentCard({ comment, source, media, pid, canReply, quote }: { comment: Comment; source: 'local' | 'live'; media: Media[]; pid: number; canReply: boolean; quote: () => void }) {
	const [showNote, setShowNote] = useState(false)
	const note = useQuery({ queryKey: ['comment-note', comment.cid], queryFn: () => api.commentNote(comment.cid), enabled: source === 'local' && showNote })
	const [content, setContent] = useState('')
	useEffect(() => setContent(note.data?.content ?? ''), [note.data])
	const save = useMutation({ mutationFn: () => api.saveCommentNote(comment.cid, content), onSuccess: () => note.refetch() })
	const items = media.filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)
	const commentMedia = items.length || source === 'local' ? items : remoteCommentMedia(comment)
	return <article id={`comment-${comment.cid}`} className="panel scroll-mt-24 p-5"><div className="flex flex-col items-start justify-between gap-2 sm:flex-row sm:items-center"><div><span className="font-mono text-xs text-teal">C{comment.cid}</span><span className="ml-2 text-xs font-medium text-ink-soft">{comment.name_tag || '匿名'}</span>{comment.is_lz ? <span className="ml-2 badge !py-0.5">洞主</span> : null}</div><div className="flex items-center gap-2"><time className="text-[11px] text-ink-soft">{formatTime(comment.timestamp)}</time>{source === 'local' && <button className="button-secondary !px-2 !py-1 text-xs" onClick={() => setShowNote((value) => !value)}><StickyNote size={12} />笔记</button>}{canReply && <button className="button-secondary !px-2 !py-1 text-xs" onClick={quote}><Reply size={12} />引用回复</button>}</div></div>{comment.quote && <blockquote className="mt-3 rounded-lg bg-paper px-3 py-2 text-xs leading-5 text-ink-soft">引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}<p className="mt-3 whitespace-pre-wrap text-sm leading-7">{comment.text}</p><MediaGallery items={commentMedia} pid={pid} />{showNote && <div className="mt-4 rounded-xl border border-line bg-paper/45 p-3"><textarea aria-label={`C${comment.cid} 的本地笔记`} className="field min-h-20" value={content} maxLength={100000} onChange={(event) => setContent(event.target.value)} placeholder="只保存在本机的评论笔记…" /><div className="mt-2 flex items-center justify-between gap-3"><span className="text-xs text-ink-soft">{save.error ? String(save.error) : save.isSuccess ? '已保存' : ''}</span><button className="button-secondary" disabled={save.isPending || note.isLoading} onClick={() => save.mutate()}>保存评论笔记</button></div></div>}</article>
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
	const metadata = useLocalPostMetadata(pid)
	return <section className="panel p-5"><p className="eyebrow">PERSONAL</p><h2 className="mt-1 text-lg font-semibold">项目、标签与笔记</h2><div className="mt-4 grid gap-5">
		<div><div className="flex items-center justify-between gap-3"><p className="text-xs font-medium text-ink-soft">研究项目</p><Link className="text-xs font-semibold text-teal hover:underline" to="/projects">管理项目</Link></div><div className="mt-2 grid gap-2">{metadata.projects.data?.length ? metadata.projects.data.map((project) => <label key={project.id} className="flex cursor-pointer items-center gap-2 rounded-lg border border-line bg-white/45 px-3 py-2 text-xs"><input type="checkbox" checked={metadata.selectedProjectIDs.includes(project.id)} onChange={() => metadata.setSelectedProjectIDs((value) => value.includes(project.id) ? value.filter((id) => id !== project.id) : [...value, project.id])} /><span className="size-2 rounded-full" style={{ backgroundColor: project.color || '#0f766e' }} />{project.name}</label>) : <span className="text-xs text-ink-soft">尚未创建研究项目</span>}</div><button className="button-secondary mt-3" disabled={metadata.projects.isLoading || metadata.assignedProjects.isLoading || metadata.saveProjects.isPending} onClick={() => metadata.saveProjects.mutate()}>保存项目归属</button></div>
		<div><p className="text-xs font-medium text-ink-soft">标签</p><div className="mt-2 flex flex-wrap gap-2">{metadata.tags.data?.length ? metadata.tags.data.map((tag) => <label key={tag.id} className="badge cursor-pointer" style={tag.color ? { borderColor: tag.color, color: tag.color } : undefined}><input className="hidden" type="checkbox" checked={metadata.selected.includes(tag.id)} onChange={() => metadata.setSelected((value) => value.includes(tag.id) ? value.filter((id) => id !== tag.id) : [...value, tag.id])} /><span className="size-2 rounded-full" style={{ backgroundColor: tag.color || '#94a3b8' }} />{tag.name}{metadata.selected.includes(tag.id) && <span aria-label="已选择">✓</span>}</label>) : <span className="text-xs text-ink-soft">尚未创建标签</span>}</div><div className="mt-3 flex gap-2"><input className="field" value={metadata.tagName} onChange={(event) => metadata.setTagName(event.target.value)} placeholder="新标签名称" /><button className="button-secondary shrink-0" disabled={!metadata.tagName.trim() || metadata.createTag.isPending} onClick={() => metadata.createTag.mutate()}>创建</button></div><button className="button-secondary mt-3" disabled={metadata.saveTags.isPending} onClick={() => metadata.saveTags.mutate()}>保存标签</button></div>
		<div><label className="text-xs font-medium text-ink-soft">笔记<textarea className="field mt-2 min-h-28" value={metadata.content} maxLength={100000} onChange={(event) => metadata.setContent(event.target.value)} placeholder="只保存在本机的笔记…" /></label><button className="button-secondary mt-3" disabled={metadata.saveNote.isPending} onClick={() => metadata.saveNote.mutate()}>保存笔记</button></div>
	</div></section>
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
		? <MediaImage key={`${item.owner_type}-${item.owner_id}-${item.remote_id ?? index}`} item={item} pid={pid} index={index} />
		: <div key={item.id} className="flex min-h-24 items-center justify-center gap-2 rounded-xl border border-dashed border-line bg-paper/60 px-4 text-sm text-ink-soft"><Image size={17} />图片尚未下载{item.remote_id ? ` · ${item.remote_id}` : ''}</div>)}</div>
}

function MediaImage({ item, pid, index }: { item: Media; pid?: number; index: number }) {
	const url = item.status === 'remote' ? `/api/v1/remote-media/${item.remote_id || '_'}?pid=${pid}` : `/api/v1/media/${item.id}`
	return <a href={url} target="_blank" rel="noreferrer" className="overflow-hidden rounded-xl border border-line bg-paper" aria-label={`在新窗口查看树洞 #${pid ?? item.owner_id} 的第 ${index + 1} 张图片`}><img src={url} alt={`树洞 #${pid ?? item.owner_id} 的图片 ${index + 1}`} loading="lazy" className="max-h-[32rem] w-full object-contain" /></a>
}
