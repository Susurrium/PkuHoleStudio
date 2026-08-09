import { MessageCircle, Star, ThumbsUp } from 'lucide-react'
import { compactNumber } from '../lib/format'
import type { Post } from '../lib/types'

type MetricPost = Pick<Post, 'reply' | 'likenum' | 'praise_num'>

export function PostMetrics({ post, className = '' }: { post: MetricPost; className?: string }) {
  const metrics = [
    { label: '评论', value: post.reply ?? 0, Icon: MessageCircle },
    { label: '关注', value: post.likenum ?? 0, Icon: Star },
    { label: '点赞', value: post.praise_num ?? 0, Icon: ThumbsUp },
  ]

  return <span className={`inline-flex flex-wrap items-center gap-x-4 gap-y-2 ${className}`.trim()}>
    {metrics.map(({ label, value, Icon }) => <span key={label} className="inline-flex items-center gap-1.5" aria-label={`${label} ${value}`}><Icon aria-hidden="true" size={14} /><span>{label} {compactNumber(value)}</span></span>)}
  </span>
}
