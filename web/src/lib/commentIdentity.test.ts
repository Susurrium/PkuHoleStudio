import { describe, expect, it } from 'vitest'
import { resolveCommentIdentity } from './commentIdentity'

describe('resolveCommentIdentity', () => {
  it('uses a single owner label when the upstream name is already 洞主', () => {
    expect(resolveCommentIdentity({ name_tag: ' 洞主 ', is_lz: true })).toEqual({
      label: '洞主',
      isOwner: true,
      showOwnerBadge: false,
    })
  })

  it('falls back to 洞主 when an owner has no name tag', () => {
    expect(resolveCommentIdentity({ name_tag: '  ', is_lz: 1 })).toEqual({
      label: '洞主',
      isOwner: true,
      showOwnerBadge: false,
    })
  })

  it('keeps an owner alias and asks the renderer for one role badge', () => {
    expect(resolveCommentIdentity({ name_tag: 'Alice', is_lz: true })).toEqual({
      label: 'Alice',
      isOwner: true,
      showOwnerBadge: true,
    })
  })

  it('falls back to anonymous for a regular unnamed comment', () => {
    expect(resolveCommentIdentity({ is_lz: 0 })).toEqual({
      label: '匿名',
      isOwner: false,
      showOwnerBadge: false,
    })
  })
})
