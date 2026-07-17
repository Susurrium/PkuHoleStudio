import { AlertTriangle, CheckCircle2, Info, X } from 'lucide-react'
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'

type FeedbackTone = 'success' | 'error' | 'info'

interface ToastInput {
	title: string
	description?: string
	tone?: FeedbackTone
	action?: { label: string; to: string; onClick?: () => void }
}

interface Toast extends ToastInput {
	id: number
}

interface Confirmation {
	title: string
	description: string
	confirmLabel?: string
	tone?: 'danger' | 'default'
}

interface FeedbackContextValue {
	notify: (toast: ToastInput) => void
	confirm: (confirmation: Confirmation) => Promise<boolean>
}

const FeedbackContext = createContext<FeedbackContextValue | null>(null)
let nextToastID = 1

export function FeedbackProvider({ children }: { children: ReactNode }) {
	const [toasts, setToasts] = useState<Toast[]>([])
	const [confirmation, setConfirmation] = useState<Confirmation | null>(null)
	const resolver = useRef<((accepted: boolean) => void) | null>(null)
	const dialog = useRef<HTMLElement>(null)
	const cancelButton = useRef<HTMLButtonElement>(null)
	const timers = useRef<number[]>([])

	const dismissToast = useCallback((id: number) => setToasts((current) => current.filter((toast) => toast.id !== id)), [])
	const notify = useCallback((toast: ToastInput) => {
		const id = nextToastID++
		setToasts((current) => [...current.slice(-2), { ...toast, id }])
		timers.current.push(window.setTimeout(() => dismissToast(id), toast.tone === 'error' || toast.action ? 8_000 : 4_500))
	}, [dismissToast])
	const finishConfirmation = useCallback((accepted: boolean) => {
		resolver.current?.(accepted)
		resolver.current = null
		setConfirmation(null)
	}, [])
	const confirm = useCallback((next: Confirmation) => {
		resolver.current?.(false)
		return new Promise<boolean>((resolve) => {
			resolver.current = resolve
			setConfirmation(next)
		})
	}, [])

	useEffect(() => {
		if (!confirmation) return
		cancelButton.current?.focus()
		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Escape') finishConfirmation(false)
			if (event.key === 'Tab') {
				const buttons = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled)') ?? [])
				if (!buttons.length) return
				const first = buttons[0]
				const last = buttons[buttons.length - 1]
				if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
				else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
			}
		}
		window.addEventListener('keydown', handleKeyDown)
		return () => window.removeEventListener('keydown', handleKeyDown)
	}, [confirmation, finishConfirmation])

	useEffect(() => () => {
		resolver.current?.(false)
		for (const timer of timers.current) window.clearTimeout(timer)
	}, [])

	return <FeedbackContext.Provider value={{ notify, confirm }}>
		{children}
		<div className="pointer-events-none fixed inset-x-4 top-4 z-[70] flex flex-col items-end gap-2 md:left-auto md:w-[380px]" aria-live="polite" aria-atomic="true">
			{toasts.map((toast) => <ToastMessage key={toast.id} toast={toast} dismiss={() => dismissToast(toast.id)} />)}
		</div>
		{confirmation && <div className="fixed inset-0 z-[80] grid place-items-center bg-ink/35 p-4 backdrop-blur-sm" onMouseDown={(event) => { if (event.target === event.currentTarget) finishConfirmation(false) }}>
			<section ref={dialog} className="w-full max-w-md rounded-2xl border border-line bg-paper p-6 shadow-2xl" role="alertdialog" aria-modal="true" aria-labelledby="confirmation-title" aria-describedby="confirmation-description">
				<div className={`grid size-11 place-items-center rounded-xl ${confirmation.tone === 'danger' ? 'bg-coral-soft text-coral' : 'bg-teal-soft text-teal'}`}><AlertTriangle size={20} /></div>
				<h2 id="confirmation-title" className="mt-4 text-xl font-semibold">{confirmation.title}</h2>
				<p id="confirmation-description" className="mt-2 text-sm leading-6 text-ink-soft">{confirmation.description}</p>
				<div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
					<button ref={cancelButton} className="button-secondary" onClick={() => finishConfirmation(false)}>取消</button>
					<button className={confirmation.tone === 'danger' ? 'button-danger' : 'button-primary'} onClick={() => finishConfirmation(true)}>{confirmation.confirmLabel || '确认'}</button>
				</div>
			</section>
		</div>}
	</FeedbackContext.Provider>
}

function ToastMessage({ toast, dismiss }: { toast: Toast; dismiss: () => void }) {
	const Icon = toast.tone === 'error' ? AlertTriangle : toast.tone === 'info' ? Info : CheckCircle2
	return <div className={`pointer-events-auto flex w-full items-start gap-3 rounded-2xl border bg-paper/95 p-4 shadow-xl backdrop-blur ${toast.tone === 'error' ? 'border-coral/35' : 'border-teal/30'}`} role={toast.tone === 'error' ? 'alert' : 'status'}>
		<Icon size={19} className={`mt-0.5 shrink-0 ${toast.tone === 'error' ? 'text-coral' : 'text-teal'}`} />
		<div className="min-w-0 flex-1"><p className="text-sm font-semibold">{toast.title}</p>{toast.description && <p className="mt-1 text-xs leading-5 text-ink-soft">{toast.description}</p>}{toast.action && <Link className="mt-2 inline-flex text-xs font-semibold text-teal hover:underline" to={toast.action.to} onClick={() => { toast.action?.onClick?.(); dismiss() }}>{toast.action.label}</Link>}</div>
		<button className="-m-1 grid size-8 shrink-0 place-items-center rounded-lg text-ink-soft transition hover:bg-paper-deep hover:text-ink" aria-label="关闭提示" onClick={dismiss}><X size={15} /></button>
	</div>
}

export function useFeedback() {
	const value = useContext(FeedbackContext)
	if (!value) throw new Error('useFeedback must be used inside FeedbackProvider')
	return value
}

export function errorDescription(error: unknown) {
	return error instanceof Error ? error.message : String(error || '发生未知错误')
}
