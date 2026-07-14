import { NavLink, Outlet } from 'react-router-dom'
import { Archive, Bell, Bot, FileText, FolderSearch, Gauge, GraduationCap, Import, Menu, RefreshCw, Settings, Wrench, X } from 'lucide-react'
import { useUIStore } from '../store/ui'

const navigation = [
  { to: '/', label: '总览', icon: Gauge, end: true },
  { to: '/posts', label: '资料库', icon: Archive },
  { to: '/search', label: '全文搜索', icon: FolderSearch },
  { to: '/sync', label: '同步中心', icon: RefreshCw },
	{ to: '/maintenance', label: '资料库维护', icon: Wrench },
	{ to: '/notifications', label: '通知', icon: Bell },
	{ to: '/logs', label: '运行日志', icon: FileText },
	{ to: '/campus', label: '课表与成绩', icon: GraduationCap },
  { to: '/imports', label: '归档导入', icon: Import },
  { to: '/ai', label: 'AI 研究', icon: Bot },
  { to: '/settings', label: '设置', icon: Settings },
]

export function Shell() {
  const { navOpen, setNavOpen } = useUIStore()
  return (
    <div className="paper-grid min-h-screen bg-paper text-ink">
      <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-line bg-paper/90 px-4 backdrop-blur md:hidden">
        <Brand compact />
        <button className="button-secondary !size-10 !p-0" onClick={() => setNavOpen(!navOpen)} aria-label={navOpen ? '关闭导航' : '打开导航'}>
          {navOpen ? <X size={19} /> : <Menu size={19} />}
        </button>
      </header>
      {navOpen && <button className="fixed inset-0 z-30 bg-ink/20 md:hidden" aria-label="关闭导航遮罩" onClick={() => setNavOpen(false)} />}
      <aside className={`fixed inset-y-0 left-0 z-40 flex w-72 flex-col border-r border-line bg-[#ede7dc] px-5 py-6 transition-transform md:translate-x-0 ${navOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        <Brand />
        <nav className="mt-10 flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto pr-1" aria-label="主导航">
          {navigation.map(({ to, label, icon: Icon, end }) => (
            <NavLink key={to} to={to} end={end} onClick={() => setNavOpen(false)} className={({ isActive }) => `group flex items-center gap-3 rounded-xl px-3.5 py-3 text-sm font-medium transition ${isActive ? 'bg-ink text-white shadow-sm' : 'text-ink-soft hover:bg-white/60 hover:text-ink'}`}>
              <Icon size={18} strokeWidth={1.8} />
              <span>{label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="rounded-2xl border border-line bg-white/45 p-4">
          <p className="eyebrow">LOCAL FIRST</p>
          <p className="mt-2 text-xs leading-5 text-ink-soft">内容保存在本机资料库。在线检索与 AI 只有在你明确启用时才会访问外部服务。</p>
        </div>
      </aside>
      <main className="min-h-screen md:pl-72">
        <div className="mx-auto w-full max-w-[1500px] px-4 py-7 sm:px-7 md:py-10 lg:px-10">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <div className="grid size-10 place-items-center rounded-xl bg-coral text-lg font-black text-white shadow-[4px_4px_0_#172c33]">P</div>
      <div className={compact ? 'hidden min-[390px]:block' : ''}>
        <p className="text-[15px] font-bold tracking-[-0.02em]">PkuHoleStudio</p>
        <p className="font-mono text-[10px] uppercase tracking-[0.16em] text-ink-soft">Personal archive</p>
      </div>
    </div>
  )
}
