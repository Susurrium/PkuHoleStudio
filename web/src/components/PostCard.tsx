import { Check, CheckSquare2, Download, Image, MessageCircle, Square, ThumbsUp } from 'lucide-react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import type { PostSummary } from '../lib/types'
import { compactNumber, formatTime, HighlightedText } from '../lib/format'
import { useSavePostToLocal } from '../features/posts/usePostDetail'
import { handoffLocalSelection } from '../features/library/selection'

export function PostCard({ post, source = 'local', selectable = false, selected = false, onSelect }: { post: PostSummary; source?: 'local' | 'live'; selectable?: boolean; selected?: boolean; onSelect?: (pid: number) => void }) {
  const location = useLocation()
  const navigate = useNavigate()
  const text = post.snippet || post.text
  const saveLocal = useSavePostToLocal(post.pid)
  const targetParams = new URLSearchParams()
  if (source === 'live') targetParams.set('source', 'live')
  if (location.pathname === '/posts' || location.pathname === '/search' || location.pathname === '/online') targetParams.set('return_to', location.pathname + location.search)
  const target = `/posts/${post.pid}${targetParams.size ? `?${targetParams}` : ''}`
  const rememberPosition = () => sessionStorage.setItem(`studio:list-scroll:${location.pathname}${location.search}`, String(window.scrollY))
  const open = () => { rememberPosition(); navigate(target) }
  const choose = () => onSelect?.(post.pid)
  return (
    <article
      className={`panel group cursor-pointer p-5 transition hover:-translate-y-0.5 hover:border-teal/40 hover:shadow-[0_16px_40px_rgba(23,44,51,0.09)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal ${selected ? '!border-teal !bg-teal-soft/20 ring-2 ring-teal/15' : ''}`}
      role={selectable ? undefined : 'link'}
      tabIndex={0}
      aria-label={selectable ? `${selected ? '取消选择' : '选择'}树洞 #${post.pid}` : `打开树洞 #${post.pid}`}
      onClick={(event) => {
        if ((event.target as HTMLElement).closest('a, button, input, select, textarea')) return
        if (window.getSelection()?.toString()) return
        if (selectable) choose(); else open()
      }}
      onKeyDown={(event) => { if ((event.key === 'Enter' || (selectable && event.key === ' ')) && event.target === event.currentTarget) { event.preventDefault(); if (selectable) choose(); else open() } }}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-wrap items-center gap-2">
          {selectable && <button type="button" className="-m-1 grid size-8 place-items-center rounded-lg text-teal hover:bg-teal-soft" aria-pressed={selected} aria-label={`${selected ? '取消选择' : '选择'}树洞 #${post.pid}`} onClick={choose}>{selected ? <CheckSquare2 size={19} /> : <Square size={19} />}</button>}
          <Link to={target} onClick={(event) => { if (selectable) { event.preventDefault(); choose() } else rememberPosition() }} className="font-mono text-sm font-bold text-coral hover:underline">#{post.pid}</Link>
          {post.media_ids && <span className="badge gap-1"><Image size={11} /> 图片</span>}
        </div>
        <time className="shrink-0 text-xs text-ink-soft">{formatTime(post.timestamp)}</time>
      </div>
      <div className="mt-4 whitespace-pre-wrap text-[15px] leading-7 text-ink decoration-coral/40 group-hover:underline group-hover:decoration-2 group-hover:underline-offset-4">
        <HighlightedText value={text || '（无正文）'} />
      </div>
      {post.comment_matches?.slice(0, 2).map((match) => (
        <Link to={`${target}#comment-${match.cid}`} onClick={rememberPosition} key={match.cid} className="mt-3 block border-l-2 border-teal/35 pl-3 text-sm leading-6 text-ink-soft hover:text-teal">
          <span className="mr-2 font-mono text-[11px] text-teal">C{match.cid}</span><HighlightedText value={match.snippet} />
        </Link>
      ))}
      <footer className="mt-5 flex items-center gap-4 border-t border-line/70 pt-3 text-xs text-ink-soft">
        <span className="inline-flex items-center gap-1.5"><MessageCircle size={14} /> {compactNumber(post.reply)}</span>
        <span className="inline-flex items-center gap-1.5"><ThumbsUp size={14} /> {compactNumber(post.praise_num ?? post.likenum)}</span>
        {source === 'live' && saveLocal.isSuccess
          ? <Link className="ml-auto inline-flex items-center gap-1.5 rounded-lg px-2 py-1 font-semibold text-teal hover:bg-teal-soft/60" to="/posts" onClick={() => handoffLocalSelection('/posts', [post.pid])}><Check size={14} />前往本地整理</Link>
          : source === 'live' && (!saveLocal.job && post.local_state === 'saved'
          ? <span className="ml-auto inline-flex items-center gap-1.5 font-medium text-teal"><Check size={14} />已保存到本地</span>
          : <button type="button" className={`ml-auto inline-flex items-center gap-1.5 rounded-lg px-2 py-1 font-semibold hover:bg-teal-soft/60 disabled:opacity-50 ${saveLocal.retryable ? 'text-coral' : 'text-teal'}`} disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start} title={saveLocal.failure || undefined}><Download size={14} />{saveLocal.label}</button>)}
        {source === 'live' && saveLocal.failure && <span className="basis-full text-right text-[11px] text-coral" role="alert">{saveLocal.failure}</span>}
      </footer>
    </article>
  )
}
