import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Image, MessageCircle } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import { ErrorState, LoadingState } from '../components/States'

export function PostDetailPage() {
  const { pid = '' } = useParams()
  const detail = useQuery({ queryKey: ['post', pid], queryFn: () => api.post(pid), enabled: /^\d+$/.test(pid) })
  if (detail.isLoading) return <LoadingState label={`正在读取 #${pid}…`} />
  if (detail.error || !detail.data) return <ErrorState error={detail.error ?? new Error('帖子不存在')} />
  const { post, comments, references } = detail.data
  return <>
    <Link to="/posts" className="mb-6 inline-flex items-center gap-2 text-sm font-medium text-ink-soft hover:text-teal"><ArrowLeft size={16} />返回资料库</Link>
    <article className="panel overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white/40 px-5 py-4 md:px-7"><span className="font-mono text-base font-bold text-coral">#{post.pid}</span><time className="text-xs text-ink-soft">{formatTime(post.timestamp)}</time></header>
      <div className="px-5 py-7 md:px-7"><p className="whitespace-pre-wrap text-base leading-8 md:text-[17px]">{post.text || '（无正文）'}</p>{post.media_ids && <div className="mt-6 flex flex-wrap gap-2">{post.media_ids.split(',').filter(Boolean).map((id) => <a key={id} href={`/api/v1/media/${id.trim()}`} target="_blank" rel="noreferrer" className="button-secondary"><Image size={16} />查看图片 {id.trim()}</a>)}</div>}</div>
      <footer className="flex gap-5 border-t border-line bg-paper/45 px-5 py-4 text-xs text-ink-soft md:px-7"><span className="inline-flex items-center gap-1.5"><MessageCircle size={14} />{post.reply ?? comments.length} 条评论</span>{post.praise_num !== undefined && <span>点赞 {post.praise_num}</span>}</footer>
    </article>
    <section className="mt-7 grid gap-6 xl:grid-cols-[1fr_300px]">
      <div><div className="mb-4 flex items-center justify-between"><h2 className="text-xl font-semibold">评论</h2><span className="badge">已载入 {comments.length}</span></div><div className="grid gap-3">{comments.length ? comments.map((comment) => <article key={comment.cid} id={`comment-${comment.cid}`} className="panel p-5"><div className="flex items-center justify-between gap-3"><div><span className="font-mono text-xs text-teal">C{comment.cid}</span><span className="ml-2 text-xs font-medium text-ink-soft">{comment.name_tag || '匿名'}</span>{comment.is_lz ? <span className="ml-2 badge !py-0.5">洞主</span> : null}</div><time className="text-[11px] text-ink-soft">{formatTime(comment.timestamp)}</time></div>{comment.quote && <blockquote className="mt-3 rounded-lg bg-paper px-3 py-2 text-xs leading-5 text-ink-soft">引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}<p className="mt-3 whitespace-pre-wrap text-sm leading-7">{comment.text}</p></article>) : <p className="panel p-7 text-center text-sm text-ink-soft">暂无本地评论</p>}</div></div>
      <aside><h2 className="mb-4 text-xl font-semibold">引用关系</h2><div className="panel p-4">{references.length ? <div className="grid gap-3">{references.map((reference, index) => <div key={`${reference.kind}-${index}`} className="rounded-lg border border-line bg-white/45 p-3 text-xs leading-5"><span className="badge mb-2">{reference.kind}</span><p>#{reference.source_pid}{reference.source_cid ? ` / C${reference.source_cid}` : ''} → <Link className="font-semibold text-coral hover:underline" to={`/posts/${reference.target_pid}`}>#{reference.target_pid}</Link>{reference.target_cid ? ` / C${reference.target_cid}` : ''}</p></div>)}</div> : <p className="py-5 text-center text-xs leading-5 text-ink-soft">尚未发现此帖的出站引用</p>}</div></aside>
    </section>
  </>
}
