import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useLocation, useSearchParams } from 'react-router-dom'
import { api, isOnlineSessionError } from '../../lib/api'
import { errorDescription, useFeedback } from '../../components/Feedback'
import type { Comment, Job, JobStatus } from '../../lib/types'
import { preferredScrollBehavior } from '../../lib/motion'
import { invalidateOnlineSession, useOnlineSession } from '../online/session'

export function usePostDetailResource(pid: string, source: 'local' | 'live') {
  const detail = useQuery({ queryKey: ['post', pid, source], queryFn: () => api.post(pid, source), enabled: /^\d+$/.test(pid) })
  const online = useOnlineSession(source === 'live')
  return { detail, online }
}

export function usePostComments(pid: string, source: 'local' | 'live', detailData: Awaited<ReturnType<typeof api.post>> | undefined, detailUpdatedAt: number) {
  const [searchParams, setSearchParams] = useSearchParams()
  const location = useLocation()
  const [additionalComments, setAdditionalComments] = useState<Comment[]>([])
  const [pagination, setPagination] = useState<{ cursor?: number; hasMore: boolean } | null>(null)
  const [restoreStatus, setRestoreStatus] = useState('')
  const [restoreCancelled, setRestoreCancelled] = useState(false)

  useEffect(() => {
    setAdditionalComments([])
    setPagination(null)
    setRestoreStatus('')
    setRestoreCancelled(false)
  }, [pid, source, detailUpdatedAt])

  const loadMoreComments = useMutation({
    mutationFn: (cursor: number) => api.comments(pid, cursor, source, 50),
    onSuccess: (page) => {
      setAdditionalComments((current) => dedupeComments([...current, ...page.items]))
      setPagination({ cursor: page.next_cursor, hasMore: page.has_more })
      if (page.next_cursor) {
        const next = new URLSearchParams(searchParams)
        next.set('comment_cursor', String(page.next_cursor))
        setSearchParams(next, { replace: true })
      }
    },
  })
  const restoreCursor = Number(searchParams.get('comment_cursor') || 0)
  const restoreCID = Number(location.hash.match(/^#comment-(\d+)$/)?.[1] || 0)
  const comments = useMemo(() => dedupeComments([...(detailData?.comments ?? []), ...additionalComments]), [detailData?.comments, additionalComments])
  const hasRestoreCID = useMemo(() => restoreCID > 0 && comments.some((item) => item.cid === restoreCID), [comments, restoreCID])

  useEffect(() => {
    if (!detailData || restoreCancelled || hasRestoreCID || (!restoreCursor && !restoreCID) || (!restoreCID && pagination?.cursor === restoreCursor)) return
    let cancelled = false
    const initial = detailData.comments
    if (restoreCID && initial.some((item) => item.cid === restoreCID)) return
    const restore = async () => {
      let cursor = detailData.next_comment_cursor
      let hasMore = detailData.has_more_comments ?? false
      let loaded: Comment[] = []
      let pages = 0
      while (!cancelled && !restoreCancelled && hasMore && cursor !== undefined && pages < 40) {
        if (restoreCursor && cursor === restoreCursor && !restoreCID) break
        setRestoreStatus(`正在恢复评论位置…已读取 ${initial.length + loaded.length} 条`)
        const page = await api.comments(pid, cursor, source, 50)
        loaded = dedupeComments([...loaded, ...page.items])
        setAdditionalComments(loaded)
        setPagination({ cursor: page.next_cursor, hasMore: page.has_more })
        pages++
        if (restoreCID && loaded.some((item) => item.cid === restoreCID)) break
        if (restoreCursor && page.next_cursor === restoreCursor && !restoreCID) break
        cursor = page.next_cursor
        hasMore = page.has_more
      }
      if (cancelled) return
      const found = !restoreCID || initial.some((item) => item.cid === restoreCID) || loaded.some((item) => item.cid === restoreCID)
      setRestoreStatus(found ? '' : pages >= 40 ? '自动恢复已达到 2000 条安全上限，可继续手动加载。' : `未在此洞中找到 C${restoreCID}。`)
    }
    restore().catch((error) => { if (!cancelled) setRestoreStatus(`恢复评论位置失败：${String(error)}`) })
    return () => { cancelled = true }
  }, [detailUpdatedAt, pid, source, restoreCursor, restoreCID, restoreCancelled, hasRestoreCID])

  useEffect(() => {
    if (!hasRestoreCID) return
    requestAnimationFrame(() => document.getElementById(`comment-${restoreCID}`)?.scrollIntoView?.({ behavior: preferredScrollBehavior(), block: 'center' }))
  }, [restoreCID, hasRestoreCID])

  return {
    comments,
    loadMoreComments,
    restoreStatus,
    cancelRestore: () => { setRestoreCancelled(true); setRestoreStatus('已取消自动恢复，可继续手动加载。') },
    nextCommentCursor: pagination?.cursor ?? detailData?.next_comment_cursor,
    hasMoreComments: pagination?.hasMore ?? detailData?.has_more_comments,
  }
}

export function useSavePostToLocal(pid: string | number) {
  const client = useQueryClient()
  const { notify } = useFeedback()
  const numericPID = Number(pid)
  const [jobID, setJobID] = useState(() => readTrackedSaveJob(numericPID))

  useEffect(() => setJobID(readTrackedSaveJob(numericPID)), [numericPID])

  const create = useMutation({
    mutationFn: () => api.createJob('sync_pids', { pids: [numericPID], include_comments: true, include_media: true }),
    onSuccess: (job) => {
      setJobID(job.id)
      rememberTrackedSaveJob(numericPID, job.id)
      client.invalidateQueries({ queryKey: ['jobs'] })
      notify({ title: `树洞 #${pid} 已加入本地保存队列`, description: '正在保存正文、评论与图片；你可以离开当前页面。' })
    },
    onError: (error) => notify({ tone: 'error', title: '保存到本地失败', description: errorDescription(error) }),
  })

  const tracked = useQuery({
    queryKey: ['post-save-job', numericPID, jobID],
    queryFn: () => api.job(jobID),
    enabled: numericPID > 0 && Boolean(jobID),
    retry: 1,
    refetchInterval: (query) => terminalSaveStatuses.has(query.state.data?.status as JobStatus) ? false : 1_200,
  })
  const restart = useMutation({
    mutationFn: (action: 'resume' | 'retry') => api.jobAction(jobID, action),
    onSuccess: (job) => {
      client.setQueryData(['post-save-job', numericPID, jobID], job)
      client.invalidateQueries({ queryKey: ['jobs'] })
      notify({ title: `正在继续保存树洞 #${pid}` })
    },
    onError: (error) => notify({ tone: 'error', title: '无法重试保存任务', description: errorDescription(error) }),
  })
  const job = tracked.data ?? create.data

  useEffect(() => {
    if (!job || !terminalSaveStatuses.has(job.status)) return
    client.invalidateQueries({ queryKey: ['post', String(pid)] })
    client.invalidateQueries({ queryKey: ['posts', 'live'] })
    client.invalidateQueries({ queryKey: ['online-home-latest'] })
    client.invalidateQueries({ queryKey: ['jobs'] })
  }, [client, job?.status, pid])

  const retryable = job ? retryableSaveStatuses.has(job.status) : false
  const active = job ? activeSaveStatuses.has(job.status) : false
  const progress = job ? Math.min(100, Math.round(((job.completed_items + job.failed_items) / Math.max(job.total_items, 1)) * 100)) : 0
  const label = savePostLabel(job, create.isPending, progress)
  const failure = job?.error || (tracked.error ? errorDescription(tracked.error) : create.error ? errorDescription(create.error) : restart.error ? errorDescription(restart.error) : '')

  return {
    job,
    jobID,
    label,
    failure,
    progress,
    active,
    retryable,
    isPending: create.isPending || restart.isPending,
    isSuccess: job?.status === 'completed',
    start: () => {
      if (retryable) restart.mutate(job?.status === 'paused' ? 'resume' : 'retry')
      else if (!active && job?.status !== 'completed') create.mutate()
    },
  }
}

const activeSaveStatuses = new Set<JobStatus>(['queued', 'running'])
const retryableSaveStatuses = new Set<JobStatus>(['paused', 'failed', 'partial', 'cancelled'])
const terminalSaveStatuses = new Set<JobStatus>(['completed', 'partial', 'failed', 'cancelled'])
const trackedSaveJobsKey = 'pkustudio:post-save-jobs'

function savePostLabel(job: Job | undefined, creating: boolean, progress: number) {
  if (creating) return '正在创建保存任务…'
  switch (job?.status) {
    case 'queued': return '等待保存…'
    case 'running': return progress > 0 ? `正在保存 ${progress}%…` : '正在保存…'
    case 'paused': return '保存已暂停，点击继续'
    case 'completed': return '已保存到本地'
    case 'partial': return '部分内容未保存，点击重试'
    case 'failed': return '保存失败，点击重试'
    case 'cancelled': return '保存已取消，点击重试'
    default: return '保存到本地'
  }
}

function readTrackedSaveJob(pid: number) {
  if (!Number.isInteger(pid) || pid <= 0) return ''
  try {
    const value = JSON.parse(localStorage.getItem(trackedSaveJobsKey) || '{}') as Record<string, unknown>
    return typeof value[String(pid)] === 'string' ? value[String(pid)] as string : ''
  } catch {
    return ''
  }
}

function rememberTrackedSaveJob(pid: number, jobID: string) {
  if (!Number.isInteger(pid) || pid <= 0 || !jobID) return
  try {
    const value = JSON.parse(localStorage.getItem(trackedSaveJobsKey) || '{}') as Record<string, unknown>
    value[String(pid)] = jobID
    localStorage.setItem(trackedSaveJobsKey, JSON.stringify(value))
  } catch {
    // Local persistence is best effort; the task itself is durable on the backend.
  }
}

export function usePostInteraction(pid: string | number, onChanged: () => void) {
  const client = useQueryClient()
  const { notify } = useFeedback()
  return useMutation({
    mutationFn: (action: 'praise' | 'follow') => api.togglePost(Number(pid), action),
    onSuccess: (_, action) => {
      onChanged()
      notify({ title: action === 'praise' ? '点赞状态已更新' : '关注状态已更新' })
    },
    onError: (error, action) => {
      if (isOnlineSessionError(error)) invalidateOnlineSession(client)
      notify({ tone: 'error', title: action === 'praise' ? '点赞失败' : '关注失败', description: errorDescription(error) })
    },
  })
}

export function usePostReply(pid: string | number, onChanged: () => void) {
  const client = useQueryClient()
  const { notify } = useFeedback()
  const [text, setText] = useState('')
  const [quoteCID, setQuoteCID] = useState<number | undefined>()
  const [files, setFiles] = useState<File[]>([])
  const mutation = useMutation({
    mutationFn: async () => api.createComment(Number(pid), text, quoteCID, await api.uploadMediaIDs(files)),
    onSuccess: () => {
      setText('')
      setQuoteCID(undefined)
      setFiles([])
      onChanged()
      notify({ title: '回复已发送' })
    },
    onError: (error) => {
      if (isOnlineSessionError(error)) invalidateOnlineSession(client)
      notify({ tone: 'error', title: '回复失败，内容已保留', description: errorDescription(error) })
    },
  })
  return { text, setText, quoteCID, setQuoteCID, files, setFiles, mutation }
}

export function useLocalPostMetadata(pid: number) {
  const { notify } = useFeedback()
  const tags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
  const assigned = useQuery({ queryKey: ['post-tags', pid], queryFn: () => api.postTags(pid) })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const assignedProjects = useQuery({ queryKey: ['post-projects', pid], queryFn: () => api.postProjects(pid) })
  const note = useQuery({ queryKey: ['post-note', pid], queryFn: () => api.postNote(pid) })
  const [selected, setSelected] = useState<number[]>([])
  const [selectedProjectIDs, setSelectedProjectIDs] = useState<number[]>([])
  const [content, setContent] = useState('')
  const [tagName, setTagName] = useState('')
  useEffect(() => setSelected(assigned.data?.map((tag) => tag.id) ?? []), [assigned.data])
  useEffect(() => setSelectedProjectIDs(assignedProjects.data?.map((project) => project.id) ?? []), [assignedProjects.data])
  useEffect(() => setContent(note.data?.content ?? ''), [note.data])
  const saveTags = useMutation({
    mutationFn: () => api.setPostTags(pid, selected),
    onSuccess: () => { assigned.refetch(); notify({ title: `树洞 #${pid} 的标签已保存` }) },
    onError: (error) => notify({ tone: 'error', title: '保存标签失败', description: errorDescription(error) }),
  })
  const saveNote = useMutation({
    mutationFn: () => api.savePostNote(pid, content),
    onSuccess: () => { note.refetch(); notify({ title: `树洞 #${pid} 的笔记已保存` }) },
    onError: (error) => notify({ tone: 'error', title: '保存笔记失败', description: errorDescription(error) }),
  })
  const saveProjects = useMutation({
    mutationFn: () => api.setPostProjects(pid, selectedProjectIDs),
    onSuccess: () => { assignedProjects.refetch(); projects.refetch(); notify({ title: `树洞 #${pid} 的研究项目已更新` }) },
    onError: (error) => notify({ tone: 'error', title: '保存研究项目失败', description: errorDescription(error) }),
  })
  const createTag = useMutation({
    mutationFn: () => api.createLocalTag(tagName, ''),
    onSuccess: () => { setTagName(''); tags.refetch(); notify({ title: '本地标签已创建' }) },
    onError: (error) => notify({ tone: 'error', title: '创建标签失败', description: errorDescription(error) }),
  })
  return { tags, assigned, projects, assignedProjects, note, selected, setSelected, selectedProjectIDs, setSelectedProjectIDs, content, setContent, tagName, setTagName, saveTags, saveProjects, saveNote, createTag }
}

export function useCommentNote(cid: number, enabled: boolean) {
  const { notify } = useFeedback()
  const note = useQuery({ queryKey: ['comment-note', cid], queryFn: () => api.commentNote(cid), enabled })
  const [content, setContent] = useState('')
  useEffect(() => setContent(note.data?.content ?? ''), [note.data])
  const save = useMutation({
    mutationFn: () => api.saveCommentNote(cid, content),
    onSuccess: () => { note.refetch(); notify({ title: `C${cid} 的笔记已保存` }) },
    onError: (error) => notify({ tone: 'error', title: `C${cid} 的笔记保存失败`, description: errorDescription(error) }),
  })
  return { note, content, setContent, save }
}

function dedupeComments(comments: Comment[]) {
  return comments.filter((comment, index, all) => all.findIndex((item) => item.cid === comment.cid) === index)
}
