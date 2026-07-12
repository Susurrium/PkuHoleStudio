import { Image, MessageCircle, ThumbsUp } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { PostSummary } from '../lib/types'
import { compactNumber, formatTime, HighlightedText } from '../lib/format'

export function PostCard({ post }: { post: PostSummary }) {
  const text = post.snippet || post.text
  return (
    <article className="panel group p-5 transition hover:-translate-y-0.5 hover:border-teal/40 hover:shadow-[0_16px_40px_rgba(23,44,51,0.09)]">
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Link to={`/posts/${post.pid}`} className="font-mono text-sm font-bold text-coral hover:underline">#{post.pid}</Link>
          {post.media_ids && <span className="badge gap-1"><Image size={11} /> 图片</span>}
        </div>
        <time className="shrink-0 text-xs text-ink-soft">{formatTime(post.timestamp)}</time>
      </div>
      <Link to={`/posts/${post.pid}`} className="mt-4 block whitespace-pre-wrap text-[15px] leading-7 text-ink decoration-coral/40 group-hover:underline group-hover:decoration-2 group-hover:underline-offset-4">
        <HighlightedText value={text || '（无正文）'} />
      </Link>
      {post.comment_matches?.slice(0, 2).map((match) => (
        <div key={match.cid} className="mt-3 border-l-2 border-teal/35 pl-3 text-sm leading-6 text-ink-soft">
          <span className="mr-2 font-mono text-[11px] text-teal">C{match.cid}</span><HighlightedText value={match.snippet} />
        </div>
      ))}
      <footer className="mt-5 flex items-center gap-4 border-t border-line/70 pt-3 text-xs text-ink-soft">
        <span className="inline-flex items-center gap-1.5"><MessageCircle size={14} /> {compactNumber(post.reply)}</span>
        <span className="inline-flex items-center gap-1.5"><ThumbsUp size={14} /> {compactNumber(post.praise_num ?? post.likenum)}</span>
      </footer>
    </article>
  )
}
