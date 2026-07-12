import { AlertTriangle, Inbox } from 'lucide-react'

export function LoadingState({ label = '正在读取本地资料…' }: { label?: string }) {
  return <div className="panel grid min-h-44 place-items-center p-8 text-sm text-ink-soft"><span className="animate-pulse">{label}</span></div>
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: React.ReactNode }) {
  return (
    <div className="panel flex min-h-56 flex-col items-center justify-center p-8 text-center">
      <div className="grid size-12 place-items-center rounded-2xl bg-teal-soft text-teal"><Inbox size={22} /></div>
      <h2 className="mt-4 text-lg font-semibold">{title}</h2>
      <p className="mt-2 max-w-md text-sm leading-6 text-ink-soft">{description}</p>
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}

export function ErrorState({ error }: { error: unknown }) {
  return (
    <div className="panel flex min-h-40 items-center gap-4 border-coral/30 bg-coral-soft/35 p-6">
      <AlertTriangle className="shrink-0 text-coral" />
      <div><p className="font-semibold">读取失败</p><p className="mt-1 text-sm text-ink-soft">{error instanceof Error ? error.message : '发生未知错误'}</p></div>
    </div>
  )
}
