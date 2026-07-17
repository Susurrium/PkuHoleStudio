import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { api, isOnlineSessionError } from '../../lib/api'
import { errorDescription, useFeedback } from '../../components/Feedback'
import { useBrowserOnline } from '../online/connectivity'
import { invalidateOnlineSession, useOnlineSession } from '../online/session'

export interface PostExplorerFilters {
  source: 'local' | 'live'
  q: string
  sort: string
  hasMedia: string
  followed: boolean
  label: string
  localTag: string
}

export function readPostExplorerFilters(params: URLSearchParams): PostExplorerFilters {
  const source = params.get('source') === 'live' ? 'live' : 'local'
  return {
    source,
    q: params.get('q')?.trim() ?? '',
    sort: params.get('sort') || 'desc',
    hasMedia: params.get('media') || '',
    followed: source === 'live' && params.get('followed') === 'true',
    label: source === 'live' ? params.get('label') || '' : '',
    localTag: source === 'local' ? params.get('tag') || '' : '',
  }
}

export function usePostsExplorer(params: URLSearchParams, options: { history?: boolean; enabled?: boolean } = {}) {
  const client = useQueryClient()
  const filters = readPostExplorerFilters(params)
  const { source, q, sort, hasMedia, followed, label, localTag } = filters
  const enabled = options.enabled ?? true
	const browserOnline = useBrowserOnline()
  const online = useOnlineSession(enabled && source === 'live')
  const liveTags = useQuery({ queryKey: ['live-tags'], queryFn: api.tags, enabled: enabled && source === 'live' && online.data?.can_read_online === true })
  const localTags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags, enabled: enabled && source === 'local' })
  const history = useQuery({ queryKey: ['search-history'], queryFn: api.searchHistory, enabled: enabled && options.history === true && source === 'local' && !q })
  const canRead = source === 'local' || online.data?.can_read_online === true
  const query = useInfiniteQuery({
    queryKey: ['posts', source, q, sort, hasMedia, followed, label, localTag],
    initialPageParam: 0,
    queryFn: ({ pageParam }) => {
      const values = { q: q || (followed ? ':follow' : undefined), cursor: pageParam, limit: 20, source, sort, has_media: hasMedia || undefined, label: label || undefined, tag: localTag || undefined }
      return q ? api.search(values) : api.posts(values)
    },
    getNextPageParam: (lastPage) => lastPage.has_more ? (lastPage.next_cursor ?? undefined) : undefined,
    enabled: enabled && canRead,
  })
  const posts = canRead ? query.data?.pages.flatMap((page) => page.items).filter((post, index, all) => all.findIndex((item) => item.pid === post.pid) === index) ?? [] : []

  useEffect(() => {
    if (!enabled || source !== 'live' || online.data?.can_read_online !== false) return
    client.removeQueries({ queryKey: ['posts', 'live'] })
  }, [client, enabled, online.data?.can_read_online, source])

  return { ...filters, online, liveTags, localTags, history, canRead, browserOnline, query, posts }
}

export function updatePostExplorerParam(params: URLSearchParams, name: string, value: string) {
  const next = new URLSearchParams(params)
  next.delete('focus')
  if (value) next.set(name, value); else next.delete(name)
  return next
}

export function updatePostExplorerSource(params: URLSearchParams, source: 'local' | 'live') {
  const next = new URLSearchParams(params)
  next.delete('focus')
  if (source === 'live') next.set('source', 'live'); else next.delete('source')
  next.delete('followed')
  next.delete('label')
  next.delete('tag')
  next.delete('sort')
  return next
}

export function postDetailTarget(pid: number | string, source: 'local' | 'live', returnTo: string) {
  const params = new URLSearchParams()
  if (source === 'live') params.set('source', 'live')
  params.set('return_to', returnTo)
  return `/posts/${pid}?${params}`
}

export function usePublishPost({ text, files, onSuccess }: { text: string; files: File[]; onSuccess: (post: Awaited<ReturnType<typeof api.createPost>>) => void }) {
  const client = useQueryClient()
  const { notify } = useFeedback()
  return useMutation({
    mutationFn: async () => api.createPost(text, await api.uploadMediaIDs(files)),
    onSuccess: (post) => {
      onSuccess(post)
      notify({ title: `树洞 #${post.pid} 已发布` })
    },
    onError: (error) => {
      if (isOnlineSessionError(error)) invalidateOnlineSession(client)
      notify({ tone: 'error', title: '发布失败，草稿已保留', description: errorDescription(error) })
    },
  })
}
