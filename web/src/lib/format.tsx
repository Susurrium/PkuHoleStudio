import { Fragment } from 'react'

export function formatTime(timestamp?: number) {
  if (!timestamp) return '时间未知'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(timestamp * 1000))
}

export function compactNumber(value = 0) {
  return new Intl.NumberFormat('zh-CN', { notation: value > 9999 ? 'compact' : 'standard' }).format(value)
}

export function HighlightedText({ value }: { value: string }) {
  const parts = value.split(/(<mark>|<\/mark>)/i)
  let highlighted = false
  return <>{parts.map((part, index) => {
    if (part.toLowerCase() === '<mark>') { highlighted = true; return null }
    if (part.toLowerCase() === '</mark>') { highlighted = false; return null }
    return <Fragment key={`${index}-${part}`}>{highlighted ? <mark>{part}</mark> : part}</Fragment>
  })}</>
}
