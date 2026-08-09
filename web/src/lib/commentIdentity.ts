import type { Comment } from './types'

export interface CommentIdentity {
  label: string
  isOwner: boolean
  showOwnerBadge: boolean
}

export function resolveCommentIdentity(comment: Pick<Comment, 'name_tag' | 'is_lz'>): CommentIdentity {
  const name = comment.name_tag?.trim()
  const isOwner = Boolean(comment.is_lz)
  const label = name || (isOwner ? '洞主' : '匿名')

  return {
    label,
    isOwner,
    showOwnerBadge: isOwner && label !== '洞主',
  }
}
