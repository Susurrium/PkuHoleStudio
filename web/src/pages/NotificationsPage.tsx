import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, CheckCheck, ExternalLink, LogIn, MailOpen, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { errorDescription, useFeedback } from '../components/Feedback'
import { PageHeader } from '../components/PageHeader'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api, isOnlineSessionError } from '../lib/api'
import { formatTime } from '../lib/format'
import type { Notification } from '../lib/types'
import { invalidateOnlineSession, useOnlineSession, useSlowLoading } from '../features/online/session'

type NotificationType = 'interactive' | 'system'

export function NotificationsPage() {
	const [type, setType] = useState<NotificationType>('interactive')
	const client = useQueryClient()
	const { notify } = useFeedback()
	const session = useOnlineSession()
	const slowSession = useSlowLoading(session.isLoading)
	const online = session.data?.can_read_online === true
	const notifications = useQuery({ queryKey: ['notifications', type], queryFn: () => api.notifications(type), enabled: online })
	const markRead = useMutation({
		mutationFn: api.markNotificationRead,
		onSuccess: () => client.invalidateQueries({ queryKey: ['notifications'] }),
		onError: (error) => { if (isOnlineSessionError(error)) invalidateOnlineSession(client); notify({ tone: 'error', title: '未能标记为已读', description: errorDescription(error) }) },
	})
	const markAll = useMutation({
		mutationFn: () => api.markAllNotificationsRead(type),
		onSuccess: () => {
			client.invalidateQueries({ queryKey: ['notifications'] })
			notify({ title: type === 'interactive' ? '互动通知已全部标记为已读' : '系统通知已全部标记为已读' })
		},
		onError: (error) => { if (isOnlineSessionError(error)) invalidateOnlineSession(client); notify({ tone: 'error', title: '批量操作失败', description: errorDescription(error) }) },
	})
	const items = notifications.data?.items ?? []
	const unread = items.filter((item) => !item.read).length

	return <>
		<PageHeader
			eyebrow="MESSAGES"
			title="通知"
			description="实时读取互动与系统消息；通知不会写入本地资料库或导出归档。"
			actions={online ? <>
				<button className="button-secondary" disabled={notifications.isFetching} onClick={() => notifications.refetch()}><RefreshCw size={16} />{notifications.isFetching ? '刷新中…' : '刷新'}</button>
				<button className="button-secondary" disabled={markAll.isPending || unread === 0} onClick={() => markAll.mutate()}><CheckCheck size={16} />{markAll.isPending ? '处理中…' : unread ? `全部已读（${unread}）` : '没有未读'}</button>
			</> : undefined}
		/>
		{session.isLoading ? <LoadingState label={slowSession ? '正在连接树洞，可能需要几秒…' : '正在验证在线会话…'} /> : session.error ? <ErrorState error={session.error} /> : !online ? <EmptyState
			title="登录后查看通知"
			description={session.data?.message || '通知直接从树洞读取，不会在未登录状态下反复请求。'}
			action={<Link className="button-primary" to="/sync"><LogIn size={16} />前往同步中心登录</Link>}
		/> : <>
			<nav className="mb-5 grid grid-cols-2 rounded-xl border border-line bg-white/45 p-1 sm:inline-grid sm:min-w-[320px]" aria-label="通知类型">
				<NotificationTab active={type === 'interactive'} label="互动通知" onClick={() => setType('interactive')} />
				<NotificationTab active={type === 'system'} label="系统通知" onClick={() => setType('system')} />
			</nav>
			{notifications.isLoading ? <LoadingState label="正在读取通知…" /> : notifications.error ? <ErrorState error={notifications.error} /> : items.length ? <div className="grid gap-3">
				{items.map((item) => <NotificationRow key={item.id} item={item} type={type} marking={markRead.isPending && markRead.variables === item.id} markRead={() => markRead.mutate(item.id)} />)}
			</div> : <EmptyState title={type === 'interactive' ? '还没有互动通知' : '还没有系统通知'} description={type === 'interactive' ? '收到回复、点赞或关注相关消息后，会显示在这里。' : '树洞发布的系统消息会显示在这里。'} action={<MailOpen size={20} />} />}
		</>}
	</>
}

function NotificationTab({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
	return <button className={`rounded-lg px-4 py-2.5 text-sm font-medium transition ${active ? 'bg-ink text-white shadow-sm' : 'text-ink-soft hover:bg-white/60 hover:text-ink'}`} aria-pressed={active} onClick={onClick}>{label}</button>
}

function NotificationRow({ item, type, marking, markRead }: { item: Notification; type: NotificationType; marking: boolean; markRead: () => void }) {
	const content = <>
		<div className={`grid size-10 shrink-0 place-items-center rounded-xl ${item.read ? 'bg-paper-deep text-ink-soft' : 'bg-teal-soft text-teal'}`}><Bell size={17} /></div>
		<div className="min-w-0 flex-1">
			<div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold">{item.title || (type === 'interactive' ? '互动通知' : '系统通知')}</h2>{!item.read && <span className="size-2 rounded-full bg-coral" aria-label="未读" />}</div>
			<p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-ink-soft">{item.content}</p>
			<time className="mt-2 block text-xs text-ink-soft">{item.created_at || formatTime(item.timestamp)}</time>
		</div>
	</>
	return <article className={`panel p-4 transition sm:p-5 ${item.read ? 'bg-white/45' : 'border-teal/35 hover:-translate-y-0.5 hover:shadow-md'}`}>
		<div className="flex flex-col gap-4 sm:flex-row sm:items-start">
			{item.pid ? <Link className="flex min-w-0 flex-1 items-start gap-3 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-teal" to={`/posts/${item.pid}?source=live&return_to=%2Fnotifications`} aria-label={`查看通知关联树洞 #${item.pid}`}>{content}</Link> : <div className="flex min-w-0 flex-1 items-start gap-3">{content}</div>}
			<div className="flex shrink-0 gap-2 sm:flex-col sm:items-stretch">
				{item.pid && <Link className="button-secondary !min-h-9 !px-3 !py-1.5 text-xs" to={`/posts/${item.pid}?source=live&return_to=%2Fnotifications`}><ExternalLink size={13} />查看树洞</Link>}
				{!item.read && <button className="button-secondary !min-h-9 !px-3 !py-1.5 text-xs" disabled={marking} onClick={markRead}>{marking ? '处理中…' : '标为已读'}</button>}
			</div>
		</div>
	</article>
}
