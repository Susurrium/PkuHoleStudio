import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, ArrowLeft, CheckCircle2, Clock3, Image, MessageCircle, RotateCcw, ShieldAlert, type LucideIcon } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import { formatTime } from '../lib/format'
import type { Comment, Media, ObserverAvailability } from '../lib/types'

export function RemovedPostDetailPage() {
	const { pid = '' } = useParams()
	const detail = useQuery({ queryKey: ['removed-post', pid], queryFn: () => api.removedPost(pid), retry: false })
	if (detail.isLoading) return <LoadingState label={`正在读取删除归档 #${pid}…`} />
	if (detail.error || !detail.data) return <ErrorState error={detail.error ?? new Error('删除归档不存在')} />
	const { availability, post, comments = [], media = [] } = detail.data
	if (!post) return <MissingPostEvidence availability={availability} />
	const postMedia = media.filter((item) => item.owner_type === 'post' && item.owner_id === post.pid)
	const partial = availability.completeness !== 'complete'
	return <>
		<div className="mb-5 flex flex-wrap items-center justify-between gap-3"><Link className="inline-flex items-center gap-2 rounded-lg px-2 py-1 text-sm font-medium text-ink-soft hover:bg-white/60 hover:text-teal" to="/removed"><ArrowLeft size={16} />返回删除归档</Link><div className="flex flex-wrap gap-2"><AvailabilityBadge availability={availability} /><span className={`badge ${partial ? '!border-coral/30 !text-coral' : '!border-teal/30 !text-teal'}`}>{partial ? '部分归档' : '完整归档'}</span></div></div>
		{partial && <div className="mb-5 flex items-start gap-3 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="status"><AlertTriangle className="mt-0.5 shrink-0 text-coral" size={20} /><div><p className="font-semibold">删除前只来得及保存部分内容</p><p className="mt-1 text-xs leading-5 text-ink-soft">下面只展示 Observer 实际取得并同步到本机的数据，不会用空占位冒充完整正文、评论或媒体。</p></div></div>}
		<div className="grid items-start gap-6 xl:grid-cols-[minmax(0,860px)_minmax(290px,1fr)]">
			<section className="min-w-0">
				<article className="panel overflow-hidden" aria-labelledby="removed-post-title"><header className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-white/40 px-5 py-4 md:px-7"><span className="font-mono text-base font-bold text-coral">#{post.pid}</span><time className="text-xs text-ink-soft">发布于 {formatTime(post.timestamp)}</time></header><div className="px-5 py-7 md:px-7"><p id="removed-post-title" className="whitespace-pre-wrap text-base leading-8 md:text-[17px]">{post.text || '（无正文）'}</p><ArchivedMedia items={postMedia} pid={post.pid} /></div><footer className="flex flex-wrap gap-5 border-t border-line bg-paper/45 px-5 py-4 text-xs text-ink-soft md:px-7"><span className="inline-flex items-center gap-1.5"><MessageCircle size={14} />{post.reply ?? comments.length} 条评论</span>{post.praise_num !== undefined && <span>点赞 {post.praise_num}</span>}<span>本机 Observer 归档</span></footer></article>
				<section className="mt-7"><div className="mb-4 flex items-center justify-between gap-3"><h2 className="text-xl font-semibold">已归档评论</h2><span className="badge">{comments.length} 条</span></div>{comments.length ? <div className="grid gap-3">{comments.map((comment) => <ArchivedComment key={comment.cid} comment={comment} media={media} pid={post.pid} />)}</div> : <div className="panel p-7 text-center text-sm text-ink-soft">没有归档到评论{partial ? '；这可能是删除发生得太快' : ''}</div>}</section>
			</section>
			<aside className="grid gap-5 xl:sticky xl:top-24"><AvailabilityTimeline availability={availability} /><section className="panel p-5"><p className="eyebrow">ARCHIVE EVIDENCE</p><h2 className="mt-1 text-lg font-semibold">归档依据</h2><dl className="mt-4 divide-y divide-line/70 text-sm"><InfoRow label="Observer" value={availability.observer_id || '未知实例'} /><InfoRow label="最后观察" value={formatISO(availability.observed_at)} /><InfoRow label="正文" value={post.text ? '已保存' : '未取得'} /><InfoRow label="评论" value={`${comments.length} 条`} /><InfoRow label="媒体" value={`${media.filter((item) => item.status === 'available').length} 个可用`} /></dl><p className="mt-4 text-xs leading-5 text-ink-soft">“不可访问”表示 Observer 在健康会话下完成确认；界面不会猜测是作者删除、管理员处理还是权限变化。</p></section></aside>
		</div>
	</>
}

function MissingPostEvidence({ availability }: { availability: ObserverAvailability }) {
	return <><div className="mb-5"><Link className="inline-flex items-center gap-2 rounded-lg px-2 py-1 text-sm font-medium text-ink-soft hover:bg-white/60 hover:text-teal" to="/removed"><ArrowLeft size={16} />返回删除归档</Link></div><div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_360px]"><section className="panel flex min-h-64 flex-col items-center justify-center border-coral/30 bg-coral-soft/20 p-8 text-center" role="status"><AlertTriangle className="text-coral" size={28} /><h1 className="mt-4 text-xl font-semibold">这条记录没有可展示的正文</h1><p className="mt-2 max-w-lg text-sm leading-6 text-ink-soft">Observer 保存了可访问状态证据，但旧版或未完成快照中没有真实正文。Studio 不会用 PID 占位内容冒充已归档树洞。</p><span className="badge mt-5 !border-coral/30 !text-coral">部分归档 · #{availability.pid}</span></section><AvailabilityTimeline availability={availability} /></div></>
}

function ArchivedComment({ comment, media, pid }: { comment: Comment; media: Media[]; pid: number }) {
	const items = media.filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)
	return <article id={`comment-${comment.cid}`} className="panel scroll-mt-24 p-5"><div className="flex flex-wrap items-center justify-between gap-2"><div><span className="font-mono text-xs text-teal">C{comment.cid}</span><span className="ml-2 text-xs font-medium text-ink-soft">{comment.name_tag || '匿名'}</span>{comment.is_lz ? <span className="badge ml-2 !py-0.5">洞主</span> : null}</div><time className="text-[11px] text-ink-soft">{formatTime(comment.timestamp)}</time></div>{comment.quote && <blockquote className="mt-3 rounded-lg bg-paper px-3 py-2 text-xs leading-5 text-ink-soft">引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}<p className="mt-3 whitespace-pre-wrap text-sm leading-7">{comment.text || '（无正文）'}</p><ArchivedMedia items={items} pid={pid} /></article>
}

function ArchivedMedia({ items, pid }: { items: Media[]; pid: number }) {
	if (!items.length) return null
	return <div className="mt-5 grid gap-3 sm:grid-cols-2">{items.map((item, index) => item.status === 'available' ? <a key={`${item.id}-${index}`} href={`/api/v1/media/${item.id}`} target="_blank" rel="noreferrer" className="overflow-hidden rounded-xl border border-line bg-paper" aria-label={`在新窗口查看树洞 #${pid} 的第 ${index + 1} 张归档图片`}><img loading="lazy" className="max-h-[32rem] w-full object-contain" src={`/api/v1/media/${item.id}`} alt={`树洞 #${pid} 的归档图片 ${index + 1}`} /></a> : <div key={`${item.id}-${index}`} className="flex min-h-24 items-center justify-center gap-2 rounded-xl border border-dashed border-line bg-paper/60 px-4 text-sm text-ink-soft"><Image size={17} />媒体未完整保存</div>)}</div>
}

function AvailabilityTimeline({ availability }: { availability: ObserverAvailability }) {
	const restored = availability.state === 'restored'
	return <section className="panel p-5"><p className="eyebrow">AVAILABILITY</p><h2 className="mt-1 text-lg font-semibold">可访问状态时间线</h2><ol className="mt-5 grid gap-4"><TimelineItem icon={Clock3} title="首次确认不可访问" time={availability.first_unavailable_at || availability.observed_at} description="Observer 在健康会话下完成两次明确探测。" /><TimelineItem icon={ShieldAlert} title="最近确认不可访问" time={availability.last_unavailable_at} description="删除前内容保持为不可变本地归档。" />{restored && <TimelineItem icon={RotateCcw} title="后来恢复访问" time={availability.restored_at} description="恢复不会覆盖原来的删除前快照。" tone="success" />}</ol></section>
}

function TimelineItem({ icon: Icon, title, time, description, tone = 'default' }: { icon: LucideIcon; title: string; time?: string; description: string; tone?: 'default' | 'success' }) {
	return <li className="flex gap-3"><div className={`grid size-8 shrink-0 place-items-center rounded-full ${tone === 'success' ? 'bg-teal-soft text-teal' : 'bg-coral-soft text-coral'}`}><Icon size={15} /></div><div><p className="text-sm font-semibold">{title}</p><time className="mt-0.5 block text-xs text-ink-soft">{formatISO(time)}</time><p className="mt-1 text-xs leading-5 text-ink-soft">{description}</p></div></li>
}

function AvailabilityBadge({ availability }: { availability: ObserverAvailability }) {
	return availability.state === 'restored' ? <span className="badge !border-teal/30 !bg-teal-soft/25 !text-teal"><CheckCircle2 size={12} />已恢复</span> : <span className="badge !border-coral/30 !bg-coral-soft/25 !text-coral"><ShieldAlert size={12} />已删除或不可访问</span>
}

function InfoRow({ label, value }: { label: string; value: string }) {
	return <div className="flex items-start justify-between gap-3 py-3"><dt className="text-ink-soft">{label}</dt><dd className="min-w-0 break-all text-right font-medium">{value}</dd></div>
}

function formatISO(value?: string) {
	if (!value) return '时间未知'
	const date = new Date(value)
	return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
