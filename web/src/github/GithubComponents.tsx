import { AlertIcon, CheckCircleIcon, InfoIcon } from '@primer/octicons-react'
import type { ReactNode } from 'react'

export function GithubPageHeader({ title, description, actions, eyebrow }: { title: ReactNode; description?: ReactNode; actions?: ReactNode; eyebrow?: string }) {
  return <header className="github-page-header"><div>{eyebrow && <p>{eyebrow}</p>}<h1>{title}</h1>{description && <div className="github-page-description">{description}</div>}</div>{actions && <div className="github-page-actions">{actions}</div>}</header>
}

export function GithubState({ title, description, tone = 'neutral', action }: { title: string; description?: ReactNode; tone?: 'neutral' | 'danger' | 'success'; action?: ReactNode }) {
  const Icon = tone === 'danger' ? AlertIcon : tone === 'success' ? CheckCircleIcon : InfoIcon
  return <div className={`github-state github-state--${tone}`} role={tone === 'danger' ? 'alert' : 'status'} aria-live={tone === 'danger' ? 'assertive' : 'polite'}><Icon size={24} /><strong>{title}</strong>{description && <p>{description}</p>}{action && <div>{action}</div>}</div>
}

export function GithubLoading({ label = '正在加载…' }: { label?: string }) {
  return <div className="github-loading" role="status"><span aria-hidden="true" /><span>{label}</span></div>
}
