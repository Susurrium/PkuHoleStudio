import { CirclePause, CirclePlay, RotateCcw, XCircle } from 'lucide-react'
import type { Job } from '../lib/types'

const labels: Record<string, string> = {
  sync_followed: '同步关注', sync_pids: '同步指定 PID', sync_latest: '同步最新时间线',
  repair_comments: '补全评论', repair_media: '补全媒体', import_archive: '导入归档', rebuild_search_index: '重建搜索索引', rebuild_references: '重建引用关系',
  sync_pages: '顺序采集', monitor_latest: '持续监控', repair_thumbnails: '补全缩略图', cleanup_staging: '清理暂存文件',
}

export function JobRow({ job, onAction, busy }: { job: Job; onAction?: (action: 'pause' | 'resume' | 'cancel' | 'retry') => void; busy?: boolean }) {
  const total = Math.max(job.total_items, 1)
  const progress = Math.min(100, Math.round(((job.completed_items + job.failed_items) / total) * 100))
  return (
    <div className="rounded-xl border border-line/80 bg-white/55 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div><p className="text-sm font-semibold">{labels[job.type] ?? job.type}</p><p className="mt-1 font-mono text-[10px] text-ink-soft">{job.id}</p></div>
        <span className={`badge ${job.status === 'failed' || job.status === 'partial' ? '!border-coral/30 !bg-coral-soft/60 !text-coral' : job.status === 'running' ? '!border-teal/30 !bg-teal-soft/60 !text-teal' : ''}`}>{job.status}</span>
      </div>
      <div className="mt-4 h-1.5 overflow-hidden rounded-full bg-paper-deep"><div className="h-full rounded-full bg-teal transition-all" style={{ width: `${progress}%` }} /></div>
      <div className="mt-2 flex items-center justify-between text-[11px] text-ink-soft"><span>{job.completed_items} 完成 · {job.failed_items} 失败</span><span>{progress}%</span></div>
      {job.error && <p className="mt-3 text-xs leading-5 text-coral">{job.error}</p>}
      {onAction && <div className="mt-3 flex gap-2">
        {job.status === 'running' && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('pause')}><CirclePause size={14} />暂停</button>}
        {job.status === 'paused' && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('resume')}><CirclePlay size={14} />继续</button>}
        {['failed', 'partial', 'cancelled'].includes(job.status) && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('retry')}><RotateCcw size={14} />重试</button>}
        {!['completed', 'failed', 'partial', 'cancelled'].includes(job.status) && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('cancel')}><XCircle size={14} />取消</button>}
      </div>}
    </div>
  )
}
