import { CirclePause, CirclePlay, FolderOpen, RotateCcw, XCircle } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { Job } from '../lib/types'
import { handoffLocalSelection } from '../features/library/selection'

const labels: Record<string, string> = {
	sync_followed: '保存我的关注', sync_pids: '保存指定 PID', sync_latest: '保存最新时间线',
	repair_comments: '补全评论', repair_media: '补全图片', import_archive: '导入资料', rebuild_search_index: '修复搜索', rebuild_references: '修复引用关系',
	sync_pages: '按旧版页码保存', monitor_latest: '旧版持续监控', repair_thumbnails: '补全旧版缩略图', cleanup_staging: '清理临时文件',
	save_raw_json: '生成诊断数据', fetch_images: '补全旧版图片', export_archive: '打包资料',
}

const statusLabels: Record<string, string> = {
  queued: '等待中', running: '正在运行', paused: '已暂停', completed: '已完成', partial: '部分完成', failed: '失败', cancelled: '已取消',
}

export function jobTypeLabel(type: string) {
  return labels[type] ?? type
}

export function JobRow({ job, onAction, busy }: { job: Job; onAction?: (action: 'pause' | 'resume' | 'cancel' | 'retry') => void; busy?: boolean }) {
  const continuous = job.type === 'monitor_latest'
	const nonResumableImport = job.type === 'import_archive'
	const restrictedAction = continuous || nonResumableImport
	const active = ['queued', 'running', 'paused'].includes(job.status)
  const total = Math.max(job.total_items, 1)
	const processed = job.completed_items + job.failed_items
	const progress = job.status === 'completed' ? 100 : Math.min(100, Math.round((processed / total) * 100))
  const checkpoint = continuous && job.checkpoint && typeof job.checkpoint === 'object' ? job.checkpoint as { cycle?: number; page?: number; posts?: number; comments?: number } : undefined
  const needsResume = job.status === 'paused' && !restrictedAction && job.error?.includes('process restarted')
	const statusLabel = continuous && active ? '已停用' : nonResumableImport && job.status === 'paused' ? '需要重新上传' : needsResume ? '需要恢复' : statusLabels[job.status] ?? job.status
	const terminal = ['failed', 'partial', 'cancelled'].includes(job.status)
	const userMessage = continuous
		? active
			? '这个旧版持续保存任务已经停用。请取消它，并按需使用“保存最新时间线”。'
			: terminal ? '这个旧版持续保存任务已经结束，不再提供重试；需要时请使用“保存最新时间线”。' : jobStatusMessage(job, processed)
		: nonResumableImport
			? job.status === 'paused'
				? '导入中断后不能继续，请取消此任务并重新选择迁移包。'
				: terminal ? '这次导入没有全部完成。请重新选择迁移包再试。' : jobStatusMessage(job, processed)
			: needsResume ? '程序上次退出后，任务已暂停。点击“继续”即可恢复。' : jobStatusMessage(job, processed)
  const savedPIDs = job.type === 'sync_pids' && job.status === 'completed' ? job.scope?.pids?.filter((pid) => Number.isInteger(pid) && pid > 0) ?? [] : []
  return (
    <div className="rounded-xl border border-line/80 bg-white/55 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div><p className="text-sm font-semibold">{jobTypeLabel(job.type)}</p><p className="mt-1 text-[11px] text-ink-soft">{new Date(job.created_at).toLocaleString('zh-CN')}</p></div>
		<span className={`badge ${job.status === 'failed' || job.status === 'partial' || needsResume || (continuous && active) ? '!border-coral/30 !bg-coral-soft/60 !text-coral' : job.status === 'running' ? '!border-teal/30 !bg-teal-soft/60 !text-teal' : ''}`}>{statusLabel}</span>
	  </div>
	  {!continuous && <><div className="mt-4 h-1.5 overflow-hidden rounded-full bg-paper-deep" role="progressbar" aria-label={`${jobTypeLabel(job.type)}进度`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress}><div className="h-full rounded-full bg-teal transition-all" style={{ width: `${progress}%` }} /></div><div className="mt-2 flex items-center justify-between text-[11px] text-ink-soft"><span>{job.total_items > 0 ? `已处理 ${processed} / ${job.total_items}` : `已处理 ${processed}`}{job.failed_items > 0 ? ` · ${job.failed_items} 项未完成` : ''}</span><span>{progress}%</span></div></>}
	  {continuous && checkpoint && <p className="mt-3 text-[11px] text-ink-soft">最近记录：{checkpoint.cycle ? `${checkpoint.cycle} 轮` : '尚无完整检查'}{checkpoint.page ? ` · 第 ${checkpoint.page} 页` : ''}</p>}
	  <p className={`mt-3 text-xs leading-5 ${job.status === 'failed' || job.status === 'partial' || needsResume || (continuous && active) ? 'text-coral' : 'text-ink-soft'}`}>{userMessage}</p>
	  {nonResumableImport && (job.status === 'queued' || job.status === 'running') && <p className="mt-2 text-xs leading-5 text-ink-soft">导入期间可以离开本页；如果取消，之后需要重新选择文件。</p>}
      {onAction && <div className="mt-3 flex gap-2">
        {job.status === 'running' && !restrictedAction && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('pause')}><CirclePause size={14} />暂停</button>}
        {job.status === 'paused' && !restrictedAction && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('resume')}><CirclePlay size={14} />继续</button>}
        {['failed', 'partial', 'cancelled'].includes(job.status) && !restrictedAction && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('retry')}><RotateCcw size={14} />重试</button>}
        {!['completed', 'failed', 'partial', 'cancelled'].includes(job.status) && <button disabled={busy} className="button-secondary !min-h-8 !px-2.5 !py-1 text-xs" onClick={() => onAction('cancel')}><XCircle size={14} />取消</button>}
      </div>}
      {savedPIDs.length > 0 && <div className="mt-3 flex justify-end"><Link className="button-primary !min-h-8 !px-3 !py-1 text-xs" to="/posts" onClick={() => handoffLocalSelection('/posts', savedPIDs)}><FolderOpen size={14} />前往本地整理（{savedPIDs.length}）</Link></div>}
	  <details className="mt-3 text-[10px] text-ink-soft"><summary className="cursor-pointer hover:text-ink">技术详情</summary><dl className="mt-2 grid gap-1.5"><div><dt className="inline">任务 ID：</dt><dd className="inline break-all font-mono">{job.id}</dd></div><div><dt className="inline">任务类型：</dt><dd className="inline font-mono">{job.type}</dd></div><div><dt className="inline">尝试次数：</dt><dd className="inline">{job.attempts}</dd></div><div><dt className="inline">最近更新：</dt><dd className="inline">{new Date(job.updated_at).toLocaleString('zh-CN')}</dd></div></dl>{job.error && <div className="mt-2"><p className="font-semibold text-coral">原始错误</p><p className="mt-1 break-all font-mono">{job.error}</p></div>}{job.checkpoint != null && <details className="mt-2"><summary className="cursor-pointer">运行记录</summary><pre className="mt-1 max-h-48 overflow-auto whitespace-pre-wrap break-all rounded bg-paper/70 p-2 font-mono">{JSON.stringify(job.checkpoint, null, 2)}</pre></details>}</details>
    </div>
  )
}

function jobStatusMessage(job: Job, processed: number) {
	if (job.status === 'queued') return '任务正在等待开始。可以离开本页，稍后回来查看。'
	if (job.status === 'running') return job.total_items > 0 ? `正在处理，已处理 ${processed} / ${job.total_items} 项。` : '任务正在处理，可以离开本页。'
	if (job.status === 'paused') return '任务已暂停。点击“继续”即可从当前进度恢复。'
	if (job.status === 'completed') return `任务已完成${job.completed_items > 0 ? `，共处理 ${job.completed_items} 项` : ''}。`
	if (job.status === 'partial') return '部分内容已经处理，其余内容未完成。可以重试或查看技术详情。'
	if (job.status === 'failed') return '任务没有完成。可以重试；如果仍然失败，再查看技术详情。'
	if (job.status === 'cancelled') return '任务已取消，没有继续处理。需要时可以重新发起。'
	return '任务状态已更新。'
}
