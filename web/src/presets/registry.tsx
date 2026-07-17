import { Fragment, lazy, type ComponentType, type PropsWithChildren } from 'react'
import { Shell } from '../components/Shell'
import { DashboardPage } from '../pages/DashboardPage'
import { PostsPage } from '../pages/PostsPage'
import { PostDetailPage } from '../pages/PostDetailPage'
import type { LayoutPreset } from '../store/ui'

type PresetRoot = ComponentType<PropsWithChildren>

export interface LayoutPresetDefinition {
  Root: PresetRoot
  Shell: ComponentType
  Dashboard: ComponentType
  Posts: ComponentType
  PostDetail: ComponentType
}

function PlainRoot({ children }: PropsWithChildren) {
  return <Fragment>{children}</Fragment>
}

const GithubPresetRoot = lazy(() => import('../github/GithubPreset').then((module) => ({ default: module.GithubPresetRoot })))
const GithubShell = lazy(() => import('../github/GithubPreset').then((module) => ({ default: module.GithubShell })))
const GithubDashboardPage = lazy(() => import('../github/GithubPreset').then((module) => ({ default: module.GithubDashboardPage })))
const GithubPostsPage = lazy(() => import('../github/GithubPreset').then((module) => ({ default: module.GithubPostsPage })))
const GithubPostDetailPage = lazy(() => import('../github/GithubPreset').then((module) => ({ default: module.GithubPostDetailPage })))
const ClassicShell = lazy(() => import('../classic/ClassicShell').then((module) => ({ default: module.ClassicShell })))
const ClassicPostsPage = lazy(() => import('../classic/ClassicPostsPage').then((module) => ({ default: module.ClassicPostsPage })))

export const layoutPresets: Record<LayoutPreset, LayoutPresetDefinition> = {
  studio: {
    Root: PlainRoot,
    Shell,
    Dashboard: DashboardPage,
    Posts: PostsPage,
    PostDetail: PostDetailPage,
  },
  classic: {
    Root: PlainRoot,
    Shell: ClassicShell,
    Dashboard: DashboardPage,
    Posts: ClassicPostsPage,
    PostDetail: ClassicPostsPage,
  },
  github: {
    Root: GithubPresetRoot,
    Shell: GithubShell,
    Dashboard: GithubDashboardPage,
    Posts: GithubPostsPage,
    PostDetail: GithubPostDetailPage,
  },
}
