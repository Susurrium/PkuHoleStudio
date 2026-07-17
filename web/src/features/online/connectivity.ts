import { useSyncExternalStore } from 'react'

function subscribe(callback: () => void) {
	window.addEventListener('online', callback)
	window.addEventListener('offline', callback)
	return () => {
		window.removeEventListener('online', callback)
		window.removeEventListener('offline', callback)
	}
}

function snapshot() {
	return typeof navigator === 'undefined' || navigator.onLine !== false
}

export function useBrowserOnline() {
	return useSyncExternalStore(subscribe, snapshot, () => true)
}

export function freshnessTime(updatedAt: number) {
	if (!updatedAt) return '尚未成功读取'
	const date = new Date(updatedAt)
	const now = new Date()
	const sameDay = date.getFullYear() === now.getFullYear() && date.getMonth() === now.getMonth() && date.getDate() === now.getDate()
	return date.toLocaleString('zh-CN', sameDay
		? { hour: '2-digit', minute: '2-digit', second: '2-digit' }
		: { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
