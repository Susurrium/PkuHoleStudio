import { Archive, Bell, Bot, FileText, FolderKanban, Gauge, GraduationCap, Import, ListTodo, PanelsTopLeft, RefreshCw, Settings, ShieldAlert, Wrench } from 'lucide-react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { useUIStore } from '../store/ui'
import { WorkspaceSwitcher } from '../components/WorkspaceSwitcher'

const tools = [
  { to: '/tasks', label: '后台任务', icon: ListTodo },
  { to: '/sync', label: '同步中心', icon: RefreshCw },
  { to: '/imports', label: '导入与导出', icon: Import },
  { to: '/projects', label: '研究项目', icon: FolderKanban },
  { to: '/removed', label: '删除归档', icon: ShieldAlert },
  { to: '/campus', label: '课表与成绩', icon: GraduationCap },
  { to: '/maintenance', label: '资料库维护', icon: Wrench },
  { to: '/logs', label: '运行日志', icon: FileText },
  { to: '/settings', label: '设置', icon: Settings },
]

export function ClassicShell() {
  const location = useLocation()
  const { setLayoutPreset, classicBackground, classicColorMode } = useUIStore()
  const isPosts = location.pathname.startsWith('/posts')

  return (
    <div className={`classic-shell classic-bg-${classicBackground} classic-mode-${classicColorMode}`}>
      <a className="classic-skip-link" href="#classic-main">跳到主要内容</a>
      <header className="classic-masthead">
        <nav className="classic-product-nav" aria-label="产品导航">
          <NavLink to="/" end><Gauge size={14} />Studio</NavLink>
          <WorkspaceSwitcher compact />
          <NavLink to="/ai"><Bot size={14} />AI 研究</NavLink>
          <NavLink to="/projects"><FolderKanban size={14} />研究项目</NavLink>
          <Link to="/notifications"><Bell size={14} />消息</Link>
          <details className="classic-tools-menu">
            <summary>工具</summary>
            <div className="classic-tools-popover">
              {tools.map(({ to, label, icon: Icon }) => <Link key={to} to={to}><Icon size={15} />{label}</Link>)}
              <button type="button" onClick={() => setLayoutPreset('studio')}><PanelsTopLeft size={15} />切回 Studio 界面</button>
              <button type="button" onClick={() => setLayoutPreset('github')}><PanelsTopLeft size={15} />切换到 GitHub 风格界面</button>
            </div>
          </details>
        </nav>
        <div className="classic-title"><span />PkuHoleStudio · 经典树洞<span /></div>
      </header>

      <main id="classic-main" className={isPosts ? 'classic-main classic-main--posts' : 'classic-main'}>
        <div className={isPosts ? undefined : 'classic-workbench'}><Outlet /></div>
      </main>

      <nav className="classic-mobile-nav" aria-label="移动端导航">
        <NavLink to="/" end><Gauge size={18} /><span>Studio</span></NavLink>
        <Link to="/online"><Archive size={18} /><span>在线</span></Link>
        <Link to="/"><FileText size={18} /><span>资料</span></Link>
        <NavLink to="/ai"><Bot size={18} /><span>AI</span></NavLink>
        <NavLink to="/settings"><Settings size={18} /><span>设置</span></NavLink>
      </nav>
    </div>
  )
}
