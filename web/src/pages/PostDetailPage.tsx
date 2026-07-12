import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Download, Image, MessageCircle } from 'lucide-react'
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
  if (detail.isLoading) return <LoadingState label={`正在读取 #${pid}…`} />
  if (detail.error || !detail.data) return <ErrorState error={detail.error ?? new Error('帖子不存在')} />
  const { post, comments, references, media = [] } = detail.data
	const postMedia = media.filter((item) => item.owner_type === 'post' && item.owner_id === post.pid)
  return <>
    <div className="mb-6 flex flex-wrap items-center justify-between gap-3"><Link to={`/posts${source === 'live' ? '?source=live' : ''}`} className="inline-flex items-center gap-2 text-sm font-medium text-ink-soft hover:text-teal"><ArrowLeft size={16} />返回{source === 'live' ? '在线树洞' : '资料库'}</Link>{source === 'live' && <button className="button-secondary" disabled={saveLocal.isPending || saveLocal.isSuccess} onClick={() => saveLocal.mutate()}><Download size={15} />{saveLocal.isPending ? '正在创建任务…' : saveLocal.isSuccess ? '已加入同步队列' : '保存到本地资料库'}</button>}</div>
    <article className="panel overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white/40 px-5 py-4 md:px-7"><span className="font-mono text-base font-bold text-coral">#{post.pid}</span><time className="text-xs text-ink-soft">{formatTime(post.timestamp)}</time></header>
      <div className="px-5 py-7 md:px-7"><p className="whitespace-pre-wrap text-base leading-8 md:text-[17px]">{post.text || '（无正文）'}</p><MediaGallery items={postMedia} pid={post.pid} /></div>
      <footer className="flex gap-5 border-t border-line bg-paper/45 px-5 py-4 text-xs text-ink-soft md:px-7"><span className="inline-flex items-center gap-1.5"><MessageCircle size={14} />{post.reply ?? comments.length} 条评论</span>{post.praise_num !== undefined && <span>点赞 {post.praise_num}</span>}</footer>
    </article>
    <section className="mt-7 grid gap-6 xl:grid-cols-[1fr_300px]">
      <div><div className="mb-4 flex items-center justify-between"><h2 className="text-xl font-semibold">评论</h2><span className="badge">已载入 {comments.length}</span></div><div className="grid gap-3">{comments.length ? comments.map((comment) => <article key={comment.cid} id={`comment-${comment.cid}`} className="panel p-5"><div className="flex items-center justify-between gap-3"><div><span className="font-mono text-xs text-teal">C{comment.cid}</span><span className="ml-2 text-xs font-medium text-ink-soft">{comment.name_tag || '匿名'}</span>{comment.is_lz ? <span className="ml-2 badge !py-0.5">洞主</span> : null}</div><time className="text-[11px] text-ink-soft">{formatTime(comment.timestamp)}</time></div>{comment.quote && <blockquote className="mt-3 rounded-lg bg-paper px-3 py-2 text-xs leading-5 text-ink-soft">引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}<p className="mt-3 whitespace-pre-wrap text-sm leading-7">{comment.text}</p><MediaGallery items={media.filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)} pid={post.pid} /></article>) : <p className="panel p-7 text-center text-sm text-ink-soft">暂无本地评论</p>}</div></div>
      <aside><h2 className="mb-4 text-xl font-semibold">引用关系</h2><div className="panel p-4">{source === 'live' ? <p className="py-5 text-center text-xs leading-5 text-ink-soft">引用图谱来自本地资料库；同步此洞后即可建立关系。</p> : references.length ? <div className="grid gap-3">{references.map((reference, index) => <ReferenceRow key={`${reference.kind}-${index}`} reference={reference} currentPID={post.pid} />)}</div> : <p className="py-5 text-center text-xs leading-5 text-ink-soft">尚未发现引用关系</p>}</div></aside>
    </section>
  </>
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
