import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { ArrowLeft, Download, Heart, Image, ImagePlus, MessageCircle, Reply, Send, Star } from 'lucide-react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import { ErrorState, LoadingState } from '../components/States'
import type { Media } from '../lib/types'

export function PostDetailPage() {
  const { pid = '' } = useParams()
  const [searchParams] = useSearchParams()
  const source = searchParams.get('source') === 'live' ? 'live' : 'local'
  const detail = useQuery({ queryKey: ['post', pid, source], queryFn: () => api.post(pid, source), enabled: /^\d+$/.test(pid) })
	const saveLocal = useMutation({ mutationFn: () => api.createJob('sync_pids', { pids: [Number(pid)] }) })
	const [replyText, setReplyText] = useState('')
	const [quoteCID, setQuoteCID] = useState<number | undefined>()
	const [replyFiles, setReplyFiles] = useState<File[]>([])
	const submitReply = useMutation({ mutationFn: async () => { const ids: string[] = []; for (const file of replyFiles) ids.push((await api.uploadMedia(file)).id); return api.createComment(Number(pid), replyText, quoteCID, ids) }, onSuccess: () => { setReplyText(''); setQuoteCID(undefined); setReplyFiles([]); detail.refetch() } })
	const interact = useMutation({ mutationFn: (action: 'praise' | 'follow') => api.togglePost(Number(pid), action), onSuccess: () => detail.refetch() })
  if (detail.isLoading) return <LoadingState label={`正在读取 #${pid}…`} />
  if (detail.error || !detail.data) return <ErrorState error={detail.error ?? new Error('帖子不存在')} />
  const { post, comments, references, media = [] } = detail.data
	const postMedia = media.filter((item) => item.owner_type === 'post' && item.owner_id === post.pid)
  return <>
    <div className="mb-6 flex flex-wrap items-center justify-between gap-3"><Link to={`/posts${source === 'live' ? '?source=live' : ''}`} className="inline-flex items-center gap-2 text-sm font-medium text-ink-soft hover:text-teal"><ArrowLeft size={16} />返回{source === 'live' ? '在线树洞' : '资料库'}</Link>{source === 'live' && <button className="button-secondary" disabled={saveLocal.isPending || saveLocal.isSuccess} onClick={() => saveLocal.mutate()}><Download size={15} />{saveLocal.isPending ? '正在创建任务…' : saveLocal.isSuccess ? '已加入同步队列' : '保存到本地资料库'}</button>}</div>
    <article className="panel overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white/40 px-5 py-4 md:px-7"><span className="font-mono text-base font-bold text-coral">#{post.pid}</span><time className="text-xs text-ink-soft">{formatTime(post.timestamp)}</time></header>
      <div className="px-5 py-7 md:px-7"><p className="whitespace-pre-wrap text-base leading-8 md:text-[17px]">{post.text || '（无正文）'}</p><MediaGallery items={postMedia} pid={post.pid} /></div>
      <footer className="flex flex-wrap items-center gap-5 border-t border-line bg-paper/45 px-5 py-4 text-xs text-ink-soft md:px-7"><span className="inline-flex items-center gap-1.5"><MessageCircle size={14} />{post.reply ?? comments.length} 条评论</span>{post.praise_num !== undefined && <span>点赞 {post.praise_num}</span>}{source === 'live' && <div className="ml-auto flex gap-2"><button className="button-secondary !px-3 !py-1.5" disabled={interact.isPending} onClick={() => interact.mutate('praise')}><Heart size={14} fill={post.is_praise ? 'currentColor' : 'none'} />{post.is_praise ? '取消点赞' : '点赞'}</button><button className="button-secondary !px-3 !py-1.5" disabled={interact.isPending} onClick={() => interact.mutate('follow')}><Star size={14} fill={post.is_follow ? 'currentColor' : 'none'} />{post.is_follow ? '取消关注' : '关注'}</button></div>}</footer>
    </article>
		{source === 'local' && <LocalMetadata pid={post.pid} />}
		{source === 'live' && <section className="panel mt-6 p-5"><div className="flex items-center justify-between"><div><p className="eyebrow">REPLY</p><h2 className="mt-1 text-lg font-semibold">回复此洞</h2></div>{quoteCID && <button className="badge" onClick={() => setQuoteCID(undefined)}>引用 C{quoteCID} ×</button>}</div><textarea className="field mt-4 min-h-24" value={replyText} maxLength={10000} onChange={(event) => setReplyText(event.target.value)} placeholder={quoteCID ? `回复并引用 C${quoteCID}…` : '写下回复…'} /><div className="mt-3 flex flex-wrap items-center justify-between gap-3"><label className="button-secondary cursor-pointer"><ImagePlus size={16} />选择图片<input className="hidden" type="file" accept="image/*" multiple onChange={(event) => setReplyFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><div className="flex items-center gap-3"><span className="text-xs text-ink-soft">{replyFiles.length ? `${replyFiles.length} 张图片` : submitReply.error ? String(submitReply.error) : ''}</span><button className="button-primary" disabled={submitReply.isPending || (!replyText.trim() && !replyFiles.length)} onClick={() => submitReply.mutate()}><Send size={15} />{submitReply.isPending ? '正在发送…' : '发送回复'}</button></div></div></section>}
    <section className="mt-7 grid gap-6 xl:grid-cols-[1fr_300px]">
      <div><div className="mb-4 flex items-center justify-between"><h2 className="text-xl font-semibold">评论</h2><span className="badge">已载入 {comments.length}</span></div><div className="grid gap-3">{comments.length ? comments.map((comment) => <article key={comment.cid} id={`comment-${comment.cid}`} className="panel p-5"><div className="flex items-center justify-between gap-3"><div><span className="font-mono text-xs text-teal">C{comment.cid}</span><span className="ml-2 text-xs font-medium text-ink-soft">{comment.name_tag || '匿名'}</span>{comment.is_lz ? <span className="ml-2 badge !py-0.5">洞主</span> : null}</div><div className="flex items-center gap-2"><time className="text-[11px] text-ink-soft">{formatTime(comment.timestamp)}</time>{source === 'live' && <button className="button-secondary !px-2 !py-1 text-xs" onClick={() => { setQuoteCID(comment.cid); document.querySelector('textarea')?.focus(); window.scrollTo({ top: 0, behavior: 'smooth' }) }}><Reply size={12} />引用</button>}</div></div>{comment.quote && <blockquote className="mt-3 rounded-lg bg-paper px-3 py-2 text-xs leading-5 text-ink-soft">引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}<p className="mt-3 whitespace-pre-wrap text-sm leading-7">{comment.text}</p><MediaGallery items={media.filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)} pid={post.pid} /></article>) : <p className="panel p-7 text-center text-sm text-ink-soft">暂无本地评论</p>}</div></div>
      <aside><h2 className="mb-4 text-xl font-semibold">引用关系</h2><div className="panel p-4">{source === 'live' ? <p className="py-5 text-center text-xs leading-5 text-ink-soft">引用图谱来自本地资料库；同步此洞后即可建立关系。</p> : references.length ? <div className="grid gap-3">{references.map((reference, index) => <ReferenceRow key={`${reference.kind}-${index}`} reference={reference} currentPID={post.pid} />)}</div> : <p className="py-5 text-center text-xs leading-5 text-ink-soft">尚未发现引用关系</p>}</div></aside>
    </section>
  </>
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
