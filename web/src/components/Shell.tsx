import { useState } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { Archive, Bell, Bot, ChevronDown, FileText, Flame, FolderKanban, Gauge, GraduationCap, Import, ListTodo, Menu, PanelsTopLeft, PenLine, Radio, RefreshCw, Search, Settings, ShieldAlert, Star, Wrench, X } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useUIStore } from '../store/ui'
import { GlobalSearch } from './GlobalSearch'
import { navigationTargetIsActive, WorkspaceSwitcher } from './WorkspaceSwitcher'

const onlineNavigation = [
  { to: '/online', label: '在线首页', icon: Radio, end: true },
  { to: '/posts?source=live', label: '最新树洞', icon: Archive },
  { to: '/online#hot', label: '近期热榜', icon: Flame },
  { to: '/posts?source=live&followed=true', label: '我的关注', icon: Star },
  { to: '/notifications', label: '互动消息', icon: Bell },
  { to: '/posts?source=live&compose=true', label: '发表新洞', icon: PenLine },
]

const libraryNavigation = [
  { to: '/', label: '资料库概览', icon: Gauge, end: true },
  { to: '/posts', label: '全部资料', icon: Archive },
  { to: '/removed', label: '删除归档', icon: ShieldAlert },
  { to: '/projects', label: '研究项目', icon: FolderKanban },
  { to: '/imports', label: '导入与导出', icon: Import },
  { to: '/ai', label: 'AI 研究', icon: Bot },
  { to: '/maintenance', label: '资料库维护', icon: Wrench },
]

const moreNavigation = [
	{ to: '/campus', label: '课表与成绩', icon: GraduationCap },
]

const systemNavigation = [
  { to: '/sync', label: '登录与保存', icon: RefreshCw },
	{ to: '/tasks', label: '任务中心', icon: ListTodo },
	{ to: '/logs', label: '运行日志', icon: FileText },
  { to: '/settings', label: '设置', icon: Settings },
]

export function Shell() {
  const { navOpen, setNavOpen, activeWorkspace } = useUIStore()
  const location = useLocation()
  const [workspaceOpen, setWorkspaceOpen] = useState(false)
  const utilityMode = ['/sync', '/tasks', '/logs', '/settings'].some((path) => location.pathname === path || location.pathname.startsWith(`${path}/`))
  const primaryNavigation = activeWorkspace === 'online' ? onlineNavigation : libraryNavigation
  const mobileNavigation = utilityMode ? systemNavigation : activeWorkspace === 'online'
    ? [onlineNavigation[0], onlineNavigation[1], onlineNavigation[3], onlineNavigation[4]]
    : [libraryNavigation[0], libraryNavigation[1], { to: '/posts?focus=search', label: '搜索', icon: Search }, libraryNavigation[2]]
  return (
    <div className="paper-grid min-h-screen bg-paper text-ink">
      <a className="studio-skip-link" href="#studio-main">跳到主要内容</a>
      <header className="sticky top-0 z-50 flex h-16 items-center gap-3 border-b border-line bg-paper/90 px-4 backdrop-blur lg:hidden">
        <Brand compact />
        <button type="button" className="flex min-h-10 min-w-0 flex-1 items-center justify-center gap-2 rounded-xl border border-line bg-white/65 px-3 text-sm font-semibold text-ink shadow-sm" aria-expanded={workspaceOpen} aria-controls="studio-mobile-workspaces" onClick={() => { setWorkspaceOpen((value) => !value); setNavOpen(false) }}>
          {activeWorkspace === 'online' ? <Radio className="shrink-0 text-teal" size={16} /> : <Archive className="shrink-0 text-teal" size={16} />}
          <span className="truncate">{activeWorkspace === 'online' ? '在线树洞' : '本地资料库'}</span>
          <ChevronDown className={`shrink-0 transition-transform ${workspaceOpen ? 'rotate-180' : ''}`} size={15} />
        </button>
        <div className="flex shrink-0 gap-2"><Link className="button-secondary !size-10 !p-0" to={activeWorkspace === 'online' ? '/posts?source=live&focus=search' : '/posts?focus=search'} aria-label="搜索"><Search size={18} /></Link><button className="button-secondary !size-10 !p-0" onClick={() => { setWorkspaceOpen(false); setNavOpen(!navOpen) }} aria-label={navOpen ? '关闭导航' : '打开导航'}>{navOpen ? <X size={19} /> : <Menu size={19} />}</button></div>
      </header>
      {workspaceOpen && <><button className="fixed inset-0 top-16 z-40 bg-ink/20 lg:hidden" aria-label="关闭工作区选择" onClick={() => setWorkspaceOpen(false)} /><section id="studio-mobile-workspaces" className="fixed inset-x-4 top-[4.5rem] z-50 rounded-2xl border border-line bg-paper p-3 shadow-2xl lg:hidden" aria-label="切换工作区"><p className="mb-2 px-1 text-xs font-semibold text-ink-soft">选择工作区</p><WorkspaceSwitcher onSelect={() => setWorkspaceOpen(false)} /></section></>}
      {navOpen && <button className="fixed inset-0 z-30 bg-ink/20 lg:hidden" aria-label="关闭导航遮罩" onClick={() => setNavOpen(false)} />}
      <aside className={`fixed inset-y-0 left-0 z-[60] flex w-72 flex-col border-r border-line bg-[#ede7dc] px-5 py-6 transition-transform lg:z-40 lg:translate-x-0 ${navOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        <Brand />
        <WorkspaceSwitcher className="mt-6" />
        <nav className="mt-5 flex min-h-0 flex-1 flex-col overflow-y-auto pr-1" aria-label="主导航">
          {utilityMode ? <NavigationGroup label="通用工具" items={systemNavigation} close={() => setNavOpen(false)} /> : <><NavigationGroup label={activeWorkspace === 'online' ? '在线树洞' : '本地资料库'} items={primaryNavigation} close={() => setNavOpen(false)} /><NavigationGroup label="更多" items={moreNavigation} close={() => setNavOpen(false)} className="mt-6" /><NavigationGroup label="系统" items={systemNavigation} close={() => setNavOpen(false)} className="mt-6" compact /></>}
          <div className="mt-6 grid gap-2" aria-label="界面设置">
            <Link className="flex min-h-10 items-center gap-3 rounded-xl border border-line bg-white/45 px-3.5 text-left text-xs font-semibold text-ink-soft transition hover:border-teal hover:bg-white/70 hover:text-teal" to="/settings#usage-and-interface" onClick={() => setNavOpen(false)}>
              <PanelsTopLeft size={16} />界面与工作区设置
            </Link>
          </div>
        </nav>
      </aside>
      <div className="sticky top-0 z-20 hidden h-16 items-center border-b border-line bg-paper/90 px-8 backdrop-blur lg:ml-72 lg:flex"><GlobalSearch compact /><div className="ml-auto flex items-center gap-2"><Link className="button-secondary !min-h-9 !px-3" to="/tasks"><ListTodo size={15} />任务</Link><Link className="button-secondary !size-9 !p-0" to="/notifications" aria-label="通知"><Bell size={16} /></Link></div></div>
      <main id="studio-main" className="studio-mobile-main min-h-screen lg:pl-72 lg:pb-0">
        <div className="mx-auto w-full max-w-[1500px] px-4 py-6 sm:px-7 md:py-8 lg:px-10">
          <Outlet />
        </div>
      </main>
      <nav className="studio-mobile-nav fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t border-line bg-paper/95 px-1 backdrop-blur lg:hidden" aria-label="移动端主导航">
        {mobileNavigation.map((item) => { const { to, label, icon: Icon } = item; const end = 'end' in item && typeof item.end === 'boolean' ? item.end : undefined; return <NavLink key={to} to={to} end={end} className={() => `flex flex-col items-center justify-center gap-1 text-[10px] font-medium ${navigationTargetIsActive(location.pathname, location.search, location.hash, to, end) ? 'text-teal' : 'text-ink-soft'}`}><Icon size={19} /><span>{label}</span></NavLink> })}
        <button className="flex flex-col items-center justify-center gap-1 text-[10px] font-medium text-ink-soft" onClick={() => setNavOpen(true)}><Menu size={19} /><span>更多</span></button>
      </nav>
    </div>
  )
}

function NavigationGroup({ label, items, close, className = '', compact = false }: { label: string; items: { to: string; label: string; icon: LucideIcon; end?: boolean }[]; close: () => void; className?: string; compact?: boolean }) {
  const location = useLocation()
  return <div className={className}><p className="mb-2 px-3 font-mono text-[10px] font-semibold uppercase tracking-[0.16em] text-ink-soft/70">{label}</p><div className="grid gap-1">{items.map(({ to, label: itemLabel, icon: Icon, end }) => <NavLink key={to} to={to} end={end} onClick={close} className={() => `group flex items-center gap-3 rounded-xl px-3.5 ${compact ? 'py-2 text-xs' : 'py-2.5 text-sm'} font-medium transition ${navigationTargetIsActive(location.pathname, location.search, location.hash, to, end) ? 'bg-ink text-white shadow-sm' : 'text-ink-soft hover:bg-white/60 hover:text-ink'}`}><Icon size={compact ? 16 : 18} strokeWidth={1.8} /><span>{itemLabel}</span></NavLink>)}</div></div>
}

function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <div className="grid size-10 place-items-center rounded-xl bg-coral text-lg font-black text-white shadow-[4px_4px_0_#172c33]">P</div>
      <div className={compact ? 'hidden' : ''}>
        <p className="text-[15px] font-bold tracking-[-0.02em]">PkuHoleStudio</p>
        <p className="font-mono text-[10px] uppercase tracking-[0.16em] text-ink-soft">Personal archive</p>
      </div>
    </div>
  )
}
