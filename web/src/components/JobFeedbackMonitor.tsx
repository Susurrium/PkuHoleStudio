import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { api } from '../lib/api'
import type { Job } from '../lib/types'
import { useUIStore } from '../store/ui'
import { handoffLocalSelection } from '../features/library/selection'
import { useFeedback } from './Feedback'
import { jobTypeLabel } from './JobRow'

const notifiedStorageKey = 'pkustudio:notified-job-terminals'
const feedbackStatuses = new Set(['completed', 'partial', 'failed'])

export function JobFeedbackMonitor() {
  const layoutPreset = useUIStore((state) => state.layoutPreset)
  const { notify } = useFeedback()
  const previous = useRef(new Map<string, string>())
  const initialized = useRef(false)
  const jobs = useQuery({
    queryKey: ['jobs'],
    queryFn: api.jobs,
    enabled: layoutPreset !== 'classic',
    staleTime: 1_000,
    refetchInterval: (query) => Array.isArray(query.state.data) && query.state.data.some((job) => job.status === 'queued' || job.status === 'running') ? 2_000 : 10_000,
  })

  useEffect(() => {
    if (!jobs.data) return
    if (layoutPreset === 'classic') {
      for (const job of jobs.data) previous.current.set(job.id, job.status)
      initialized.current = true
      return
    }
    if (!initialized.current) {
      for (const job of jobs.data) previous.current.set(job.id, job.status)
      initialized.current = true
      return
    }
    for (const job of jobs.data) {
      const before = previous.current.get(job.id)
      previous.current.set(job.id, job.status)
      if (!feedbackStatuses.has(job.status) || before === job.status || alreadyNotified(job)) continue
      rememberNotified(job)
      const feedback = jobFeedback(job)
      notify(feedback)
    }
  }, [jobs.data, layoutPreset, notify])

  return null
}

export function jobFeedback(job: Job) {
  const label = jobTypeLabel(job.type)
  if (job.status === 'completed') {
    const savedPIDs = job.type === 'sync_pids' ? job.scope?.pids?.filter((pid) => Number.isInteger(pid) && pid > 0) ?? [] : []
    if (savedPIDs.length) return { title: `${label} 已完成`, description: `已保存 ${savedPIDs.length} 个树洞，可以继续添加标签、加入项目或导出。`, action: { label: '整理这些树洞', to: '/posts', onClick: () => handoffLocalSelection('/posts', savedPIDs) } }
    if (job.type === 'export_archive') return { title: '导出文件已生成', description: '文件已经准备好，可以前往导出历史下载。', action: { label: '查看导出', to: '/imports?view=export' } }
    if (job.type === 'import_archive') return { title: '归档导入已完成', description: `已处理 ${job.completed_items} 项内容，本地资料库已经更新。`, action: { label: '浏览本地资料', to: '/posts' } }
    return { title: `${label} 已完成`, description: job.completed_items ? `已完成 ${job.completed_items} 项。` : undefined, action: { label: '查看任务', to: '/tasks' } }
  }
  return { tone: 'error' as const, title: job.status === 'partial' ? `${label}部分完成` : `${label}失败`, description: job.error || `已完成 ${job.completed_items} 项，失败 ${job.failed_items} 项。`, action: { label: '检查并处理', to: '/tasks' } }
}

function notificationKey(job: Job) {
  return `${job.id}:${job.status}:${job.attempts}`
}

function readNotified() {
  try {
    const value = JSON.parse(window.sessionStorage.getItem(notifiedStorageKey) || '[]')
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
  } catch { return [] }
}

function alreadyNotified(job: Job) {
  return readNotified().includes(notificationKey(job))
}

function rememberNotified(job: Job) {
  try {
    const values = [...readNotified().filter((item) => !item.startsWith(`${job.id}:`)), notificationKey(job)].slice(-100)
    window.sessionStorage.setItem(notifiedStorageKey, JSON.stringify(values))
  } catch { /* best effort */ }
}
