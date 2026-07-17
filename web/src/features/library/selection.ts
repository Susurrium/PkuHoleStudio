export interface LocalSelectionSnapshot {
  returnTo: string
  pids: number[]
  createdAt: number
}

const storageKey = 'pkustudio:local-selection'
const maxAge = 24 * 60 * 60 * 1_000
export const LOCAL_SELECTION_HANDOFF_EVENT = 'pkustudio:local-selection-handoff'

export function readLocalSelection(returnTo?: string): LocalSelectionSnapshot | undefined {
  try {
    const raw = window.sessionStorage.getItem(storageKey)
    if (!raw) return undefined
    const value = JSON.parse(raw) as Partial<LocalSelectionSnapshot>
    if (typeof value.returnTo !== 'string' || typeof value.createdAt !== 'number' || !Array.isArray(value.pids) || Date.now() - value.createdAt > maxAge) {
      window.sessionStorage.removeItem(storageKey)
      return undefined
    }
    if (returnTo && value.returnTo !== returnTo) return undefined
    const pids = [...new Set(value.pids.filter((pid): pid is number => Number.isInteger(pid) && pid > 0))].slice(0, 2_000)
    if (!pids.length) return undefined
    return { returnTo: value.returnTo, pids, createdAt: value.createdAt }
  } catch {
    return undefined
  }
}

export function writeLocalSelection(returnTo: string, pids: Iterable<number>) {
  try {
    const normalized = [...new Set(pids)].filter((pid) => Number.isInteger(pid) && pid > 0).slice(0, 2_000)
    if (!normalized.length) {
      clearLocalSelection(returnTo)
      return
    }
    window.sessionStorage.setItem(storageKey, JSON.stringify({ returnTo, pids: normalized, createdAt: Date.now() } satisfies LocalSelectionSnapshot))
  } catch { /* best effort */ }
}

export function handoffLocalSelection(returnTo: string, pids: Iterable<number>) {
  const normalized = [...new Set(pids)].filter((pid) => Number.isInteger(pid) && pid > 0).slice(0, 2_000)
  if (!normalized.length) return
  writeLocalSelection(returnTo, normalized)
  const snapshot = readLocalSelection(returnTo) ?? { returnTo, pids: normalized, createdAt: Date.now() }
  window.dispatchEvent(new CustomEvent<LocalSelectionSnapshot>(LOCAL_SELECTION_HANDOFF_EVENT, { detail: snapshot }))
}

export function clearLocalSelection(returnTo?: string) {
  try {
    if (!returnTo) {
      window.sessionStorage.removeItem(storageKey)
      return
    }
    const current = readLocalSelection()
    if (!current || current.returnTo === returnTo) window.sessionStorage.removeItem(storageKey)
  } catch { /* best effort */ }
}
