import { lazy, Suspense, type ReactNode } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { FeedbackProvider } from './components/Feedback'
import { useUIStore } from './store/ui'
import { layoutPresets } from './presets/registry'
import { WorkspaceTracker } from './components/WorkspaceSwitcher'
import { OnboardingGuide } from './components/OnboardingGuide'
import { JobFeedbackMonitor } from './components/JobFeedbackMonitor'

const SearchPage = lazy(() => import('./pages/SearchPage').then((module) => ({ default: module.SearchPage })))
const ImportsPage = lazy(() => import('./pages/ImportsPage').then((module) => ({ default: module.ImportsPage })))
const SettingsPage = lazy(() => import('./pages/SettingsPage').then((module) => ({ default: module.SettingsPage })))
const AIPage = lazy(() => import('./pages/AIPage').then((module) => ({ default: module.AIPage })))
const NotificationsPage = lazy(() => import('./pages/NotificationsPage').then((module) => ({ default: module.NotificationsPage })))
const LogsPage = lazy(() => import('./pages/LogsPage').then((module) => ({ default: module.LogsPage })))
const CampusPage = lazy(() => import('./pages/CampusPage').then((module) => ({ default: module.CampusPage })))
const SyncPage = lazy(() => import('./pages/SyncPage').then((module) => ({ default: module.SyncPage })))
const MaintenancePage = lazy(() => import('./pages/MaintenancePage').then((module) => ({ default: module.MaintenancePage })))
const TasksPage = lazy(() => import('./pages/TasksPage').then((module) => ({ default: module.TasksPage })))
const OnlineDashboardPage = lazy(() => import('./pages/OnlineDashboardPage').then((module) => ({ default: module.OnlineDashboardPage })))
const ProjectsPage = lazy(() => import('./pages/ProjectsPage').then((module) => ({ default: module.ProjectsPage })))
const RemovedPostsPage = lazy(() => import('./pages/RemovedPostsPage').then((module) => ({ default: module.RemovedPostsPage })))
const RemovedPostDetailPage = lazy(() => import('./pages/RemovedPostDetailPage').then((module) => ({ default: module.RemovedPostDetailPage })))

export default function App() {
  const layoutPreset = useUIStore((state) => state.layoutPreset)
  const githubColorMode = useUIStore((state) => state.githubColorMode)
  const preset = layoutPresets[layoutPreset]
  const { Root, Shell, Dashboard, Posts, PostDetail } = preset
  const githubLoadingDark = layoutPreset === 'github' && (githubColorMode === 'dark' || (githubColorMode === 'system' && typeof window.matchMedia === 'function' && window.matchMedia('(prefers-color-scheme: dark)').matches))

  return (
    <Suspense fallback={<PresetLoading dark={githubLoadingDark} />}>
      <Root>
        <WorkspaceTracker />
        <FeedbackProvider>
          <OnboardingGuide />
          <JobFeedbackMonitor />
          <Routes>
            <Route element={<Shell />}>
              <Route index element={<Dashboard />} />
			  <Route path="online" element={lazyPage(<OnlineDashboardPage />)} />
              <Route path="posts" element={<Posts />} />
              <Route path="posts/:pid" element={<PostDetail />} />
			  <Route path="search" element={lazyPage(<SearchPage />)} />
			  <Route path="imports" element={lazyPage(<ImportsPage />)} />
			  <Route path="projects" element={lazyPage(<ProjectsPage />)} />
			  <Route path="removed" element={lazyPage(<RemovedPostsPage />)} />
			  <Route path="removed/:pid" element={lazyPage(<RemovedPostDetailPage />)} />
			  <Route path="sync" element={lazyPage(<SyncPage />)} />
			  <Route path="maintenance" element={lazyPage(<MaintenancePage />)} />
			  <Route path="tasks" element={lazyPage(<TasksPage />)} />
			  <Route path="settings" element={lazyPage(<SettingsPage />)} />
			  <Route path="ai" element={lazyPage(<AIPage />)} />
			  <Route path="notifications" element={lazyPage(<NotificationsPage />)} />
			  <Route path="logs" element={lazyPage(<LogsPage />)} />
			  <Route path="campus" element={lazyPage(<CampusPage />)} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Route>
          </Routes>
        </FeedbackProvider>
      </Root>
    </Suspense>
  )
}

function lazyPage(page: ReactNode) {
	return <Suspense fallback={<div className="panel p-8 text-center text-sm text-ink-soft" role="status">正在加载页面…</div>}>{page}</Suspense>
}

function PresetLoading({ dark }: { dark: boolean }) {
  return <div className="grid min-h-screen place-items-center text-sm" style={{ background: dark ? '#0d1117' : '#ffffff', color: dark ? '#8c959f' : '#59636e' }} role="status">正在切换界面…</div>
}
