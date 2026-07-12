import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, CheckCheck, ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'

export function NotificationsPage() {
	const [type, setType] = useState<'interactive' | 'system'>('interactive')
	const client = useQueryClient()
	const session = useQuery({ queryKey: ['notification-session'], queryFn: api.probeSession, retry: false })
	const notifications = useQuery({ queryKey: ['notifications', type], queryFn: () => api.notifications(type), enabled: session.data?.can_read_online === true })
	const markRead = useMutation({ mutationFn: api.markNotificationRead, onSuccess: () => client.invalidateQueries({ queryKey: ['notifications'] }) })
	const markAll = useMutation({ mutationFn: () => api.markAllNotificationsRead(type), onSuccess: () => client.invalidateQueries({ queryKey: ['notifications'] }) })
	if (session.isLoading) return <LoadingState label="正在验证在线会话…" />
	if (!session.data?.can_read_online) return <ErrorState error={new Error(session.data?.message || '请先在同步中心登录树洞')} />
	return <>
		<PageHeader eyebrow="MESSAGES" title="通知" description="实时读取互动与系统通知。通知正文不会写入本地资料库或归档。" actions={<button className="button-secondary" disabled={markAll.isPending} onClick={() => markAll.mutate()}><CheckCheck size={16} />全部已读</button>} />
		<div className="mb-5 inline-flex rounded-xl border border-line bg-white/50 p-1"><button className={`rounded-lg px-4 py-2 text-sm font-medium ${type === 'interactive' ? 'bg-ink text-white' : 'text-ink-soft'}`} onClick={() => setType('interactive')}>互动通知</button><button className={`rounded-lg px-4 py-2 text-sm font-medium ${type === 'system' ? 'bg-ink text-white' : 'text-ink-soft'}`} onClick={() => setType('system')}>系统通知</button></div>
		{notifications.isLoading ? <LoadingState /> : notifications.error ? <ErrorState error={notifications.error} /> : <div className="grid gap-3">{notifications.data?.items.length ? notifications.data.items.map((item) => <article key={item.id} className={`panel p-5 ${item.read ? 'opacity-65' : 'border-teal/35'}`}><div className="flex flex-wrap items-start justify-between gap-3"><div className="flex items-start gap-3"><div className="grid size-9 place-items-center rounded-lg bg-teal-soft text-teal"><Bell size={16} /></div><div><p className="font-semibold">{item.title || (type === 'interactive' ? '互动通知' : '系统通知')}</p><p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-ink-soft">{item.content}</p><time className="mt-2 block text-xs text-ink-soft">{item.created_at || formatTime(item.timestamp)}</time></div></div><div className="flex gap-2">{item.pid ? <Link className="button-secondary !px-3 !py-1.5" to={`/posts/${item.pid}?source=live`}><ExternalLink size={13} />查看洞</Link> : null}{!item.read && <button className="button-secondary !px-3 !py-1.5" disabled={markRead.isPending} onClick={() => markRead.mutate(item.id)}>标为已读</button>}</div></div></article>) : <p className="panel p-10 text-center text-sm text-ink-soft">暂无通知</p>}</div>}
	</>
}
