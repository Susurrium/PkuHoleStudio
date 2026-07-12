import { Navigate, Route, Routes } from 'react-router-dom'
import { Shell } from './components/Shell'
import { DashboardPage } from './pages/DashboardPage'
import { PostsPage } from './pages/PostsPage'
import { PostDetailPage } from './pages/PostDetailPage'
import { SearchPage } from './pages/SearchPage'
import { ImportsPage } from './pages/ImportsPage'
import { SettingsPage } from './pages/SettingsPage'
import { AIPage } from './pages/AIPage'
import { SyncPage } from './pages/SyncPage'

export default function App() {
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route index element={<DashboardPage />} />
        <Route path="posts" element={<PostsPage />} />
        <Route path="posts/:pid" element={<PostDetailPage />} />
        <Route path="search" element={<SearchPage />} />
        <Route path="imports" element={<ImportsPage />} />
        <Route path="sync" element={<SyncPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="ai" element={<AIPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
