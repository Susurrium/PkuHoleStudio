import { CirclePause, CirclePlay, FolderOpen, RotateCcw, XCircle } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { Job } from '../lib/types'
import { handoffLocalSelection } from '../features/library/selection'

const labels: Record<string, string> = {
  sync_followed: '同步关注', sync_pids: '同步指定 PID', sync_latest: '同步最新时间线',
  repair_comments: '补全评论', repair_media: '补全媒体', import_archive: '导入归档', rebuild_search_index: '重建搜索索引', rebuild_references: '重建引用关系',
  sync_pages: '顺序采集', monitor_latest: '持续监控', repair_thumbnails: '补全缩略图', cleanup_staging: '清理暂存文件',
	save_raw_json: '保存原始 JSON', fetch_images: '补全旧版图片', export_archive: '导出归档',
}

const statusLabels: Record<string, string> = {
  queued: '等待中', running: '正在运行', paused: '已暂停', completed: '已完成', partial: '部分完成', failed: '失败', cancelled: '已取消',
}

export function jobTypeLabel(type: string) {
  return labels[type] ?? type
}

export function JobRow({ job, onAction, busy }: { job: Job; onAction?: (action: 'pause' | 'resume' | 'cancel' | 'retry') => void; busy?: boolean }) {
  const continuous = job.type === 'monitor_latest'
  const total = Math.max(job.total_items, 1)
  const progress = Math.min(100, Math.round(((job.completed_items + job.failed_items) / total) * 100))
  const checkpoint = continuous && job.checkpoint && typeof job.checkpoint === 'object' ? job.checkpoint as { cycle?: number; page?: number; posts?: number; comments?: number } : undefined
  const needsResume = job.status === 'paused' && job.error?.includes('process restarted')
  const statusLabel = needsResume ? '需要恢复' : statusLabels[job.status] ?? job.status
  const visibleError = needsResume ? '程序上次退出后，监控任务已安全暂停。点击“继续”即可恢复。' : job.error
  const savedPIDs = job.type === 'sync_pids' && job.status === 'completed' ? job.scope?.pids?.filter((pid) => Number.isInteger(pid) && pid > 0) ?? [] : []
  return (
    <div className="rounded-xl border border-line/80 bg-white/55 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div><p className="text-sm font-semibold">{jobTypeLabel(job.type)}</p><p className="mt-1 text-[11px] text-ink-soft">{new Date(job.created_at).toLocaleString('zh-CN')}</p></div>
        <span className={`badge ${job.status === 'failed' || job.status === 'partial' || needsResume ? '!border-coral/30 !bg-coral-soft/60 !text-coral' : job.status === 'running' ? '!border-teal/30 !bg-teal-soft/60 !text-teal' : ''}`}>{statusLabel}</span>
      </div>
      {continuous ? <div className="mt-4 rounded-lg bg-paper/70 px-3 py-2 text-[11px] leading-5 text-ink-soft"><p>{job.status === 'running' ? '持续检查在线时间线' : needsResume ? '监控进度已保存，等待恢复' : '持续监控任务'}</p><p>{checkpoint?.cycle ? `已进行 ${checkpoint.cycle} 轮` : '尚未完成首轮检查'}{checkpoint?.page ? ` · 最近检查第 ${checkpoint.page} 页` : ''}</p><p>最近更新：{new Date(job.updated_at).toLocaleString('zh-CN')}</p></div> : <><div className="mt-4 h-1.5 overflow-hidden rounded-full bg-paper-deep"><div className="h-full rounded-full bg-teal transition-all" style={{ width: `${progress}%` }} /></div><div className="mt-2 flex items-center justify-between text-[11px] text-ink-soft"><span>{job.completed_items} 完成 · {job.failed_items} 失败</span><span>{progress}%</span></div></>}
      {job.status === 'queued' && <p className="mt-3 text-xs leading-5 text-ink-soft">任务已持久保存，正在等待本机执行器；刷新页面或重新启动程序不会丢失。</p>}
      {job.status === 'paused' && !needsResume && <p className="mt-3 text-xs leading-5 text-ink-soft">任务已暂停并保存进度，需要点击“继续”后才会重新排队。</p>}
      {visibleError && <p className="mt-3 text-xs leading-5 text-coral">{visibleError}</p>}
      {onAction && <div className="mt-3 flex gap-2">
        {job.status === 'running' && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('pause')}><CirclePause size={14} />暂停</button>}
        {job.status === 'paused' && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('resume')}><CirclePlay size={14} />继续</button>}
        {['failed', 'partial', 'cancelled'].includes(job.status) && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('retry')}><RotateCcw size={14} />重试</button>}
        {!['completed', 'failed', 'partial', 'cancelled'].includes(job.status) && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('cancel')}><XCircle size={14} />取消</button>}
      </div>}
      {savedPIDs.length > 0 && <div className="mt-3 flex justify-end"><Link className="button-primary !min-h-8 !px-3 !py-1 text-xs" to="/posts" onClick={() => handoffLocalSelection('/posts', savedPIDs)}><FolderOpen size={14} />前往本地整理（{savedPIDs.length}）</Link></div>}
      <details className="mt-3 text-[10px] text-ink-soft"><summary className="cursor-pointer hover:text-ink">技术详情</summary><p className="mt-2 break-all font-mono"><span>任务 ID：</span><span>{job.id}</span></p></details>
    </div>
  )
}
