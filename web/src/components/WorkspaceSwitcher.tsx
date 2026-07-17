import { useEffect } from 'react'
import { Archive, Radio } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useUIStore, type Workspace } from '../store/ui'
import { useFeedback } from './Feedback'

export function workspaceForLocation(pathname: string, search: string): Workspace | null {
  if (pathname === '/online' || pathname === '/notifications' || pathname === '/campus') return 'online'
  if (pathname === '/posts' || pathname.startsWith('/posts/')) {
    return new URLSearchParams(search).get('source') === 'live' ? 'online' : 'library'
  }
  if (pathname === '/' || pathname === '/search' || pathname === '/imports' || pathname === '/projects' || pathname.startsWith('/removed') || pathname === '/ai' || pathname === '/maintenance') return 'library'
  return null
}

export function navigationTargetIsActive(pathname: string, search: string, hash: string, target: string, end = false) {
  const parsed = new URL(target, 'http://pkustudio.local')
  if (pathname !== parsed.pathname && (end || !pathname.startsWith(`${parsed.pathname}/`))) return false
  if (parsed.hash) return hash === parsed.hash
  if (parsed.pathname === '/online' && hash) return false
  if (parsed.pathname !== '/posts') return true
  const current = new URLSearchParams(search)
  const targetSource = parsed.searchParams.get('source') === 'live' ? 'live' : 'local'
  const currentSource = current.get('source') === 'live' ? 'live' : 'local'
  if (targetSource !== currentSource) return false
  for (const key of ['followed', 'compose']) {
    const expected = parsed.searchParams.get(key)
    const actual = current.get(key)
    if (expected ? expected !== actual : actual !== null) return false
  }
  return true
}

export function WorkspaceTracker() {
  const location = useLocation()
  const navigate = useNavigate()
  const setActiveWorkspace = useUIStore((state) => state.setActiveWorkspace)
  const rememberWorkspaceLocation = useUIStore((state) => state.rememberWorkspaceLocation)
  const lastOnlineLocation = useUIStore((state) => state.lastOnlineLocation)
  const lastLibraryLocation = useUIStore((state) => state.lastLibraryLocation)

  useEffect(() => {
    const workspace = workspaceForLocation(location.pathname, location.search)
    if (!workspace) return
    const target = location.pathname + location.search + location.hash
    setActiveWorkspace(workspace)
    rememberWorkspaceLocation(workspace, target)
  }, [location.hash, location.pathname, location.search, rememberWorkspaceLocation, setActiveWorkspace])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || (event.key !== '1' && event.key !== '2')) return
      event.preventDefault()
      navigate(event.key === '1' ? lastOnlineLocation : lastLibraryLocation)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [lastLibraryLocation, lastOnlineLocation, navigate])

  return null
}

export function WorkspaceSwitcher({ compact = false, className = '', onSelect }: { compact?: boolean; className?: string; onSelect?: (workspace: Workspace) => void }) {
  const navigate = useNavigate()
  const location = useLocation()
  const activeWorkspace = useUIStore((state) => state.activeWorkspace)
  const layoutPreset = useUIStore((state) => state.layoutPreset)
  const lastOnlineLocation = useUIStore((state) => state.lastOnlineLocation)
  const lastLibraryLocation = useUIStore((state) => state.lastLibraryLocation)
  const { notify } = useFeedback()

  function open(workspace: Workspace) {
    try { window.sessionStorage.setItem(`pkustudio:workspace-scroll:${location.pathname}${location.search}${location.hash}`, String(window.scrollY)) } catch { /* best effort */ }
    navigate(workspace === 'online' ? lastOnlineLocation : lastLibraryLocation)
    if (workspace !== activeWorkspace && layoutPreset === 'studio') notify({ tone: 'info', title: `已切换到${workspace === 'online' ? '在线树洞' : '本地资料库'}`, description: workspace === 'online' ? '这里实时读取官方内容；明确保存后才会进入本地资料库。' : '这里使用保存在本机的资料，可以离线整理、搜索和导出。' })
    onSelect?.(workspace)
  }

  useEffect(() => {
    const key = `pkustudio:workspace-scroll:${location.pathname}${location.search}${location.hash}`
    try {
      const value = window.sessionStorage.getItem(key)
      if (value === null) return
      window.sessionStorage.removeItem(key)
      requestAnimationFrame(() => window.scrollTo({ top: Number(value) || 0 }))
    } catch { /* best effort */ }
  }, [location.hash, location.pathname, location.search])

  return (
    <div className={`workspace-switcher ${compact ? 'workspace-switcher--compact' : ''} ${className}`.trim()} aria-label="工作空间">
      <button type="button" className={activeWorkspace === 'online' ? 'is-active' : ''} aria-label="在线树洞" aria-pressed={activeWorkspace === 'online'} onClick={() => open('online')} title="在线树洞（Alt+1）"><Radio className="shrink-0" size={15} /><span className="min-w-0"><span className="block">在线树洞</span>{!compact && <span aria-hidden="true" className="mt-0.5 block text-[9px] font-normal opacity-75">实时浏览与互动</span>}</span></button>
      <button type="button" className={activeWorkspace === 'library' ? 'is-active' : ''} aria-label="本地资料库" aria-pressed={activeWorkspace === 'library'} onClick={() => open('library')} title="本地资料库（Alt+2）"><Archive className="shrink-0" size={15} /><span className="min-w-0"><span className="block">本地资料库</span>{!compact && <span aria-hidden="true" className="mt-0.5 block text-[9px] font-normal opacity-75">离线整理与导出</span>}</span></button>
    </div>
  )
}
