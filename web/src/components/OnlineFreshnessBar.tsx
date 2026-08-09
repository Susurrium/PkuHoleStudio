import { AlertTriangle, RefreshCw, Wifi, WifiOff } from 'lucide-react'
import { freshnessTime } from '../features/online/connectivity'

interface OnlineFreshnessBarProps {
	browserOnline: boolean
	updatedAt: number
	isFetching: boolean
	error?: Error | null
	hasData: boolean
	onRefresh: () => void
}

export function OnlineFreshnessBar({ browserOnline, updatedAt, isFetching, error, hasData, onRefresh }: OnlineFreshnessBarProps) {
	if (!updatedAt && !error && browserOnline) return null
	const degraded = !browserOnline || Boolean(error)
	const title = !browserOnline
		? '设备当前离线'
		: error && hasData
			? '刷新失败，继续显示上次结果'
			: error
				? '在线内容读取失败'
				: isFetching
					? '正在获取更新'
					: '已连接'
	const description = updatedAt
		? `${hasData ? '内容' : '连接状态'}更新于 ${freshnessTime(updatedAt)}。`
		: '暂时没有可显示的在线结果，本地资料库仍可正常使用。'
	return <div className={`mb-5 flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 text-xs ${degraded ? 'border-coral/25 bg-coral-soft/20' : 'border-teal/25 bg-teal-soft/20'}`} role="status">
		{!browserOnline ? <WifiOff className="shrink-0 text-coral" size={17} /> : error ? <AlertTriangle className="shrink-0 text-coral" size={17} /> : <Wifi className="shrink-0 text-teal" size={17} />}
		<div className="min-w-0 flex-1"><p className="font-semibold">{title}</p><p className="mt-0.5 leading-5 text-ink-soft">{description}</p></div>
		<button type="button" className="button-secondary !min-h-8 !px-3 !py-1" disabled={!browserOnline || isFetching} onClick={onRefresh}><RefreshCw size={13} className={isFetching ? 'animate-spin' : ''} />{isFetching ? '刷新中' : '刷新'}</button>
	</div>
}
