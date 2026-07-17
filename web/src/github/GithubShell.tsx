import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  BellIcon,
  BookIcon,
  CommentDiscussionIcon,
  CopilotIcon,
  DatabaseIcon,
  GearIcon,
  HomeIcon,
  InboxIcon,
  PlusIcon,
  RepoIcon,
  SearchIcon,
  ShieldIcon,
  StarIcon,
  SyncIcon,
  TasklistIcon,
  ThreeBarsIcon,
  ToolsIcon,
  WorkflowIcon,
  XIcon,
} from '@primer/octicons-react'
import type { Icon } from '@primer/octicons-react'
import { useUIStore, type GithubColorMode, type LayoutPreset } from '../store/ui'
import { navigationTargetIsActive, WorkspaceSwitcher } from '../components/WorkspaceSwitcher'

const onlineNavigation = [
  { to: '/online', label: '在线首页', icon: HomeIcon, end: true },
  { to: '/posts?source=live', label: '最新', icon: CommentDiscussionIcon },
  { to: '/posts?source=live&followed=true', label: '关注', icon: StarIcon },
  { to: '/notifications', label: '消息', icon: InboxIcon },
]

const libraryNavigation = [
  { to: '/', label: '资料库概览', icon: HomeIcon, end: true },
  { to: '/posts', label: '全部资料', icon: CommentDiscussionIcon },
  { to: '/removed', label: '删除归档', icon: ShieldIcon },
  { to: '/projects', label: '研究项目', icon: RepoIcon },
  { to: '/ai', label: 'AI 研究', icon: CopilotIcon },
  { to: '/imports', label: '导入与导出', icon: DatabaseIcon },
]

const utilityNavigation = [
  { to: '/sync', label: '同步中心', icon: SyncIcon },
  { to: '/tasks', label: '任务中心', icon: WorkflowIcon },
  { to: '/campus', label: '课表与成绩', icon: BookIcon },
  { to: '/maintenance', label: '资料库维护', icon: ToolsIcon },
  { to: '/logs', label: '运行日志', icon: TasklistIcon },
  { to: '/settings', label: '设置', icon: GearIcon },
]

export function GithubShell() {
  const navigate = useNavigate()
  const location = useLocation()
  const { navOpen, setNavOpen, setLayoutPreset, githubColorMode, setGithubColorMode, activeWorkspace } = useUIStore()
  const primaryNavigation = activeWorkspace === 'online' ? onlineNavigation : libraryNavigation
  const [search, setSearch] = useState('')
  const menuButton = useRef<HTMLButtonElement>(null)
  const drawer = useRef<HTMLElement>(null)
  const closeButton = useRef<HTMLButtonElement>(null)
  const searchInput = useRef<HTMLInputElement>(null)
  const accountMenu = useRef<HTMLDetailsElement>(null)
  const moreMenu = useRef<HTMLDetailsElement>(null)
  const utilityActive = utilityNavigation.some((item) => location.pathname === item.to || location.pathname.startsWith(`${item.to}/`))

  useEffect(() => {
    setNavOpen(false)
    if (accountMenu.current) accountMenu.current.open = false
    if (moreMenu.current) moreMenu.current.open = false
  }, [location.pathname, setNavOpen])
  useEffect(() => {
    const handleKeydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (accountMenu.current) accountMenu.current.open = false
        if (moreMenu.current) moreMenu.current.open = false
        return
      }
      if (event.key !== '/' || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return
      const target = event.target as HTMLElement | null
      if (target?.matches('input, textarea, select, [contenteditable="true"]')) return
      event.preventDefault()
      searchInput.current?.focus()
    }
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null
      if (accountMenu.current?.open && target && !accountMenu.current.contains(target)) accountMenu.current.open = false
      if (moreMenu.current?.open && target && !moreMenu.current.contains(target)) moreMenu.current.open = false
    }
    window.addEventListener('keydown', handleKeydown)
    document.addEventListener('pointerdown', handlePointerDown)
    return () => {
      window.removeEventListener('keydown', handleKeydown)
      document.removeEventListener('pointerdown', handlePointerDown)
    }
  }, [])
  useEffect(() => {
    if (!navOpen) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    closeButton.current?.focus()
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setNavOpen(false)
      if (event.key !== 'Tab') return
      const controls = Array.from(drawer.current?.querySelectorAll<HTMLElement>('a[href], button:not(:disabled), input:not(:disabled)') ?? []).filter((item) => item.getClientRects().length > 0)
      if (!controls.length) return
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', keydown)
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', keydown)
      requestAnimationFrame(() => menuButton.current?.focus())
    }
  }, [navOpen, setNavOpen])

  function submitSearch(event: FormEvent) {
    event.preventDefault()
    const value = search.trim()
    if (!value) return
    const pid = value.match(/^#?(\d+)$/)?.[1]
    const source = activeWorkspace === 'online' ? 'source=live&' : ''
    navigate(pid ? `/posts/${pid}${activeWorkspace === 'online' ? '?source=live' : ''}` : `/posts?${source}q=${encodeURIComponent(value)}`)
  }

  return (
    <div className="github-shell">
      <a className="github-skip-link" href="#github-main">跳到主要内容</a>
      <header className="github-app-header">
        <button ref={menuButton} className="github-icon-button" type="button" aria-label="打开导航" onClick={() => setNavOpen(true)}><ThreeBarsIcon size={18} /></button>
        <Link className="github-brand" to="/" aria-label="PkuHoleStudio 首页"><span>P</span><strong>PkuHoleStudio</strong></Link>
        <form className="github-global-search" role="search" aria-label="全局搜索" onSubmit={submitSearch}>
          <SearchIcon size={16} />
          <input ref={searchInput} value={search} onChange={(event) => setSearch(event.target.value)} aria-label="全局搜索内容或 PID" placeholder={activeWorkspace === 'online' ? '搜索在线树洞或跳转到 #PID' : '搜索本地资料或跳转到 #PID'} />
          <kbd>/</kbd>
        </form>
        <span className="github-header-spacer" />
        <Link className="github-icon-button github-header-create" to="/posts?source=live&compose=true" aria-label="发表新洞"><PlusIcon size={18} /></Link>
        <Link className="github-icon-button" to="/tasks" aria-label="后台任务"><WorkflowIcon size={18} /></Link>
        <Link className="github-icon-button" to="/notifications" aria-label="通知"><BellIcon size={18} /></Link>
        <details ref={accountMenu} className="github-account-menu">
          <summary aria-label="账户与界面"><span>P</span></summary>
          <div className="github-menu-popover">
            <strong>PkuHoleStudio</strong>
            <small>本地优先的树洞工作台</small>
            <hr />
            <Link to="/sync" onClick={() => { if (accountMenu.current) accountMenu.current.open = false }}><SyncIcon size={15} />会话与同步</Link>
            <Link to="/settings" onClick={() => { if (accountMenu.current) accountMenu.current.open = false }}><GearIcon size={15} />设置</Link>
            <hr />
            <fieldset>
              <legend>配色</legend>
              {(['system', 'light', 'dark'] as GithubColorMode[]).map((mode) => <button key={mode} type="button" className={githubColorMode === mode ? 'is-selected' : ''} onClick={() => setGithubColorMode(mode)}>{mode === 'system' ? '跟随系统' : mode === 'light' ? '浅色' : '深色'}</button>)}
            </fieldset>
            <fieldset>
              <legend>界面</legend>
              <PresetButton current="github" value="studio" label="默认 Studio" select={setLayoutPreset} />
              <PresetButton current="github" value="classic" label="经典树洞" select={setLayoutPreset} />
            </fieldset>
          </div>
        </details>
      </header>

      <section className="github-context" aria-label="当前位置">
        <RepoIcon size={17} />
        <Link to="/">PkuHoleStudio</Link><span>/</span><strong>{activeWorkspace === 'online' ? 'online' : 'library'}</strong>
        <WorkspaceSwitcher compact />
        <span className="github-context-label">local-first</span>
      </section>

      <nav className="github-underline-nav" aria-label="项目导航">
        {primaryNavigation.map(({ to, label, icon: Icon, end }) => <NavLink key={to} to={to} end={end} className={() => navigationTargetIsActive(location.pathname, location.search, location.hash, to, end) ? 'active' : ''}><Icon size={16} /><span>{label}</span></NavLink>)}
        <details ref={moreMenu} className={utilityActive ? 'is-active' : ''}><summary><span>更多</span></summary><div>{utilityNavigation.map(({ to, label, icon: Icon }) => <Link key={to} to={to} onClick={() => { if (moreMenu.current) moreMenu.current.open = false }}><Icon size={15} />{label}</Link>)}</div></details>
      </nav>

      <main id="github-main" className="github-main"><Outlet /></main>

      {navOpen && <div className="github-drawer-layer" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setNavOpen(false) }}>
        <aside ref={drawer} className="github-navigation-drawer" role="dialog" aria-modal="true" aria-label="全部导航">
          <header><strong>导航</strong><button ref={closeButton} className="github-icon-button" type="button" aria-label="关闭导航" onClick={() => setNavOpen(false)}><XIcon size={18} /></button></header>
          <nav>
            {primaryNavigation.map(({ to, label, icon: Icon, end }) => <DrawerLink key={to} to={to} label={label} icon={Icon} end={end} />)}
            <p>工具</p>
            {utilityNavigation.map(({ to, label, icon: Icon }) => <DrawerLink key={to} to={to} label={label} icon={Icon} />)}
          </nav>
          <footer><span className="github-mark">P</span><div><strong>PkuHoleStudio</strong><small>GitHub 风格界面</small></div></footer>
        </aside>
      </div>}
    </div>
  )
}

function DrawerLink({ to, label, icon: Icon, end }: { to: string; label: string; icon: Icon; end?: boolean }) {
  const location = useLocation()
  return <NavLink to={to} end={end} className={() => navigationTargetIsActive(location.pathname, location.search, location.hash, to, end) ? 'is-active' : ''}><Icon size={17} /><span>{label}</span></NavLink>
}

function PresetButton({ current, value, label, select }: { current: LayoutPreset; value: LayoutPreset; label: string; select: (preset: LayoutPreset) => void }) {
  return <button type="button" className={current === value ? 'is-selected' : ''} onClick={() => select(value)}>{label}</button>
}
