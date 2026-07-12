import type { ReactNode } from 'react'

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow: string; title: string; description: string; actions?: ReactNode }) {
  return (
    <header className="mb-8 flex flex-col justify-between gap-5 lg:flex-row lg:items-end">
      <div className="max-w-3xl">
        <p className="eyebrow">{eyebrow}</p>
        <h1 className="page-title mt-2">{title}</h1>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-ink-soft md:text-base">{description}</p>
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  )
}
