import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Bell, CheckCheck, CircleCheck, ExternalLink, FileText, LogIn, PanelsTopLeft, RefreshCw, Settings, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { api, isOnlineSessionError } from '../lib/api'
import { formatTime } from '../lib/format'
import type { AuthStatus, Job, Notification } from '../lib/types'
import { useUIStore, type ClassicBackground, type ClassicColorMode } from '../store/ui'
import { errorDescription, useFeedback } from '../components/Feedback'
import { invalidateOnlineSession, setOnlineSession, useOnlineSession } from '../features/online/session'

type NotificationType = 'interactive' | 'system'

export function ClassicNotificationsDrawer({ returnTo, onClose }: { returnTo: string; onClose: () => void }) {
	const { notify } = useFeedback()
  const navigate = useNavigate()
  const client = useQueryClient()
  const [type, setType] = useState<NotificationType>('interactive')
  const session = useOnlineSession()
  const online = session.data?.can_read_online === true
  const notifications = useQuery({ queryKey: ['notifications', type], queryFn: () => api.notifications(type), enabled: online })
  const markRead = useMutation({ mutationFn: api.markNotificationRead, onSuccess: () => client.invalidateQueries({ queryKey: ['notifications'] }), onError: (error) => { if (isOnlineSessionError(error)) invalidateOnlineSession(client); notify({ tone: 'error', title: '未能标记为已读', description: errorDescription(error) }) } })
  const markAll = useMutation({ mutationFn: () => api.markAllNotificationsRead(type), onSuccess: () => { client.invalidateQueries({ queryKey: ['notifications'] }); notify({ title: '通知已全部标记为已读' }) }, onError: (error) => { if (isOnlineSessionError(error)) invalidateOnlineSession(client); notify({ tone: 'error', title: '批量操作失败', description: errorDescription(error) }) } })
  const items = notifications.data?.items ?? []
  const unread = items.filter((item) => !item.read).length

  function openPost(item: Notification) {
    if (!item.read) markRead.mutate(item.id)
    if (!item.pid) return
    const params = new URLSearchParams({ source: 'live', return_to: returnTo })
    navigate(`/posts/${item.pid}?${params}`)
  }

  return <ClassicDrawerFrame title="消息" onClose={onClose}>
    <div className="classic-utility-actions">
      <button type="button" disabled={!online || notifications.isFetching} onClick={() => notifications.refetch()}><RefreshCw size={14} />刷新</button>
      <button type="button" disabled={!online || markAll.isPending || unread === 0} onClick={() => markAll.mutate()}><CheckCheck size={14} />{unread ? `全部已读（${unread}）` : '没有未读'}</button>
      <Link to="/notifications"><ExternalLink size={14} />完整通知页</Link>
    </div>
      {session.isLoading ? <ClassicDrawerState>正在验证在线会话…</ClassicDrawerState> : session.error ? <ClassicDrawerState tone="error">{session.error.message}</ClassicDrawerState> : !online ? <ClassicDrawerState icon={<LogIn size={22} />}><strong>登录后查看消息</strong><span>{session.data?.message || '当前本机会话无法读取在线通知。'}</span><Link to="/sync">前往同步中心登录</Link></ClassicDrawerState> : <>
      <nav className="classic-utility-tabs" aria-label="通知类型"><button className={type === 'interactive' ? 'is-active' : ''} onClick={() => setType('interactive')}>互动通知</button><button className={type === 'system' ? 'is-active' : ''} onClick={() => setType('system')}>系统通知</button></nav>
      {notifications.isLoading ? <ClassicDrawerState>正在读取通知…</ClassicDrawerState> : notifications.error ? <ClassicDrawerState tone="error">{notifications.error.message}</ClassicDrawerState> : items.length ? <div className="classic-notification-list">{items.map((item) => <ClassicNotificationRow key={item.id} item={item} type={type} busy={markRead.isPending && markRead.variables === item.id} open={() => openPost(item)} mark={() => markRead.mutate(item.id)} />)}</div> : <ClassicDrawerState icon={<Bell size={22} />}><strong>{type === 'interactive' ? '还没有互动通知' : '还没有系统通知'}</strong><span>新消息会显示在这里。</span></ClassicDrawerState>}
    </>}
  </ClassicDrawerFrame>
}

function ClassicNotificationRow({ item, type, busy, open, mark }: { item: Notification; type: NotificationType; busy: boolean; open: () => void; mark: () => void }) {
  return <article className={`classic-notification-row${item.read ? ' is-read' : ''}`}>
    <button type="button" className="classic-notification-content" onClick={item.pid ? open : undefined} disabled={!item.pid}>
      <span className="classic-notification-icon"><Bell size={15} /></span>
      <span><span className="classic-notification-heading">{item.title || (type === 'interactive' ? '互动通知' : '系统通知')}{!item.read && <i aria-label="未读" />}</span><span className="classic-notification-text">{item.content}</span><time>{item.created_at || formatTime(item.timestamp)}</time></span>
    </button>
    <div>{item.pid && <button type="button" onClick={open}><ExternalLink size={13} />查看树洞</button>}{!item.read && <button type="button" disabled={busy} onClick={mark}>{busy ? '处理中…' : '标为已读'}</button>}</div>
  </article>
}

export function ClassicAccountDrawer({ onClose }: { onClose: () => void }) {
	const { notify } = useFeedback()
  const client = useQueryClient()
  const { setLayoutPreset, classicBackground, setClassicBackground, classicColorMode, setClassicColorMode } = useUIStore()
  const session = useQuery({ queryKey: ['session'], queryFn: api.session })
  const applySession = (value: AuthStatus) => {
    client.setQueryData(['session'], value)
    setOnlineSession(client, value)
  }
  const probe = useMutation({ mutationFn: api.probeSession, onSuccess: (value) => { applySession(value); notify({ title: value.can_write_online ? '在线会话可正常读写' : value.can_read_online ? '在线会话可以读取' : '会话检测完成', description: value.message }) }, onError: (error) => notify({ tone: 'error', title: '会话检测失败', description: errorDescription(error) }) })
  const logout = useMutation({ mutationFn: api.logoutSession, onSuccess: (value) => { applySession(value); notify({ title: '已退出树洞会话' }) }, onError: (error) => notify({ tone: 'error', title: '退出失败', description: errorDescription(error) }) })
  const online = session.data?.checked && session.data.can_read_online

  return <ClassicDrawerFrame title="账户与界面" onClose={onClose} compact>
    <section className="classic-account-section">
      <h2>树洞会话</h2>
      {session.isLoading ? <p>正在读取会话状态…</p> : session.error ? <p className="classic-error-text">{session.error.message}</p> : <div className={`classic-session-card${online ? ' is-online' : ''}`}><span>{online ? <CircleCheck size={20} /> : <AlertTriangle size={20} />}</span><div><strong>{online ? '在线读取已就绪' : session.data?.has_session ? '发现本机会话，尚未验证' : '尚未登录树洞'}</strong><p>{session.data?.message || '会话只保存在本机。'}</p></div></div>}
      <div className="classic-account-actions"><button disabled={probe.isPending} onClick={() => probe.mutate()}><RefreshCw size={14} />{probe.isPending ? '检测中…' : '检测会话'}</button>{session.data?.has_session && <button disabled={logout.isPending} onClick={() => logout.mutate()}>{logout.isPending ? '退出中…' : '退出会话'}</button>}<Link to="/sync">{online ? '同步中心' : '登录与同步'}</Link></div>
    </section>
    <section className="classic-account-section">
      <h2>经典界面</h2>
      <label>背景<select value={classicBackground} onChange={(event) => setClassicBackground(event.target.value as ClassicBackground)}><option value="stars">星空</option><option value="dusk">暮色</option><option value="plain">纯色</option></select></label>
      <label>明暗<select value={classicColorMode} onChange={(event) => setClassicColorMode(event.target.value as ClassicColorMode)}><option value="system">跟随系统</option><option value="light">浅色</option><option value="dark">夜间</option></select></label>
      <p className="classic-account-hint">这些偏好只保存在当前浏览器，不会修改资料和在线账户。</p>
    </section>
    <section className="classic-account-section">
      <h2>更多设置</h2>
      <div className="classic-account-links"><Link to="/settings"><Settings size={15} />本机与 AI 设置</Link><Link to="/imports"><FileText size={15} />资料迁移</Link><button type="button" onClick={() => { onClose(); setLayoutPreset('studio') }}><PanelsTopLeft size={15} />切回 Studio 界面</button><button type="button" onClick={() => { onClose(); setLayoutPreset('github') }}><PanelsTopLeft size={15} />切换到 GitHub 风格界面</button></div>
    </section>
  </ClassicDrawerFrame>
}

export function ClassicTasksDrawer({ onClose }: { onClose: () => void }) {
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 5_000 })
  const rows = jobs.data ?? []
  const active = rows.filter((job) => ['queued', 'running', 'paused'].includes(job.status))
  const attention = rows.filter((job) => ['failed', 'partial'].includes(job.status))
  const shown = [...active, ...attention, ...rows.filter((job) => job.status === 'completed')].filter((job, index, all) => all.findIndex((item) => item.id === job.id) === index).slice(0, 8)

  return <ClassicDrawerFrame title="后台任务" onClose={onClose} compact>
    <div className="classic-task-summary"><span><strong>{active.length}</strong> 正在进行</span><span className={attention.length ? 'has-attention' : ''}><strong>{attention.length}</strong> 需要处理</span><button disabled={jobs.isFetching} onClick={() => jobs.refetch()}><RefreshCw size={14} />刷新</button></div>
    {jobs.isLoading ? <ClassicDrawerState>正在读取任务…</ClassicDrawerState> : jobs.error ? <ClassicDrawerState tone="error">{jobs.error.message}</ClassicDrawerState> : shown.length ? <div className="classic-task-list">{shown.map((job) => <ClassicTaskRow key={job.id} job={job} />)}</div> : <ClassicDrawerState icon={<CircleCheck size={22} />}><strong>还没有后台任务</strong><span>同步、导入、导出和资料维护任务会显示在这里。</span></ClassicDrawerState>}
    <Link className="classic-utility-footer-link" to="/tasks">打开完整任务中心 <ExternalLink size={13} /></Link>
  </ClassicDrawerFrame>
}

function ClassicTaskRow({ job }: { job: Job }) {
  const total = Math.max(job.total_items, 1)
  const progress = Math.min(100, Math.round(((job.completed_items + job.failed_items) / total) * 100))
  return <article className={`classic-task-row classic-task-row--${job.status}`}><header><strong>{jobTypeLabel(job.type)}</strong><span>{jobStatusLabel(job.status)}</span></header><div><i style={{ width: `${progress}%` }} /></div><footer><span>{job.completed_items} 完成 · {job.failed_items} 失败</span><span>{progress}%</span></footer>{job.error && <p>{job.error}</p>}</article>
}

export function ClassicDrawerFrame({ title, onClose, children, compact = false }: { title: string; onClose: () => void; children: ReactNode; compact?: boolean }) {
  const panel = useRef<HTMLElement>(null)
  const close = useRef<HTMLButtonElement>(null)
  const restoreFocus = useRef<HTMLElement | null>(null)
  const closeHandler = useRef(onClose)
  useEffect(() => { closeHandler.current = onClose }, [onClose])
  useEffect(() => {
    const previous = document.body.style.overflow
    restoreFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    document.body.style.overflow = 'hidden'
    close.current?.focus()
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeHandler.current()
      if (event.key !== 'Tab') return
      const controls = Array.from(panel.current?.querySelectorAll<HTMLElement>('a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled)') ?? []).filter((item) => item.getClientRects().length > 0)
      if (!controls.length) return
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', keydown)
    return () => {
      document.body.style.overflow = previous
      window.removeEventListener('keydown', keydown)
      requestAnimationFrame(() => restoreFocus.current?.focus())
    }
  }, [])
  return <div className="classic-drawer-layer" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}><section ref={panel} className={`classic-utility-drawer${compact ? ' is-compact' : ''}`} role="dialog" aria-modal="true" aria-label={title}><header className="classic-drawer-title"><button ref={close} type="button" onClick={onClose} aria-label={`关闭${title}`}><X size={17} /></button><h1>{title}</h1></header><div className="classic-utility-content">{children}</div></section></div>
}

function ClassicDrawerState({ children, icon, tone = 'neutral' }: { children: ReactNode; icon?: ReactNode; tone?: 'neutral' | 'error' }) {
  return <div className={`classic-drawer-state classic-drawer-state--${tone}`}>{icon}{children}</div>
}

const jobTypes: Record<string, string> = { sync_followed: '同步关注', sync_pids: '同步指定 PID', sync_latest: '同步最新时间线', import_archive: '导入归档', export_archive: '导出归档', repair_comments: '补全评论', repair_media: '补全媒体', rebuild_search_index: '重建搜索索引', rebuild_references: '重建引用关系', sync_pages: '顺序采集', monitor_latest: '持续监控' }
const jobStatuses: Record<string, string> = { queued: '等待中', running: '运行中', paused: '已暂停', completed: '已完成', partial: '部分完成', failed: '失败', cancelled: '已取消' }
function jobTypeLabel(value: string) { return jobTypes[value] || value }
function jobStatusLabel(value: string) { return jobStatuses[value] || value }
