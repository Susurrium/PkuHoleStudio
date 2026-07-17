import { create } from 'zustand'

export type LayoutPreset = 'studio' | 'classic' | 'github'
export type ClassicBackground = 'stars' | 'dusk' | 'plain'
export type ClassicColorMode = 'system' | 'light' | 'dark'
export type GithubColorMode = 'system' | 'light' | 'dark'
export type Workspace = 'online' | 'library'

export const LAYOUT_PRESET_STORAGE_KEY = 'pkustudio:layout-preset'
export const CLASSIC_BACKGROUND_STORAGE_KEY = 'pkustudio:classic:background'
export const CLASSIC_COLOR_MODE_STORAGE_KEY = 'pkustudio:classic:color-mode'
export const GITHUB_COLOR_MODE_STORAGE_KEY = 'pkustudio:github:color-mode'
export const WORKSPACE_STORAGE_KEY = 'pkustudio:workspace'
export const ONLINE_LOCATION_STORAGE_KEY = 'pkustudio:workspace:online-location'
export const LIBRARY_LOCATION_STORAGE_KEY = 'pkustudio:workspace:library-location'

function readStoredValue<T extends string>(key: string, allowed: readonly T[], fallback: T): T {
  if (typeof window === 'undefined') return fallback
  try {
    const value = window.localStorage.getItem(key) as T | null
    return value && allowed.includes(value) ? value : fallback
  } catch {
    return fallback
  }
}

function readLayoutPreset(): LayoutPreset {
  return readStoredValue(LAYOUT_PRESET_STORAGE_KEY, ['studio', 'classic', 'github'], 'studio')
}

function writeStoredValue(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value)
  } catch {
    // Browsers may deny storage in privacy modes; in-memory preferences still work.
  }
}

function readStoredLocation(key: string, fallback: string) {
  if (typeof window === 'undefined') return fallback
  try {
    const value = window.localStorage.getItem(key)
    return value?.startsWith('/') && !value.startsWith('//') ? value : fallback
  } catch {
    return fallback
  }
}

interface UIState {
  navOpen: boolean
  setNavOpen: (open: boolean) => void
  layoutPreset: LayoutPreset
  setLayoutPreset: (preset: LayoutPreset) => void
  classicBackground: ClassicBackground
  setClassicBackground: (background: ClassicBackground) => void
  classicColorMode: ClassicColorMode
  setClassicColorMode: (mode: ClassicColorMode) => void
  githubColorMode: GithubColorMode
  setGithubColorMode: (mode: GithubColorMode) => void
  activeWorkspace: Workspace
  lastOnlineLocation: string
  lastLibraryLocation: string
  setActiveWorkspace: (workspace: Workspace) => void
  rememberWorkspaceLocation: (workspace: Workspace, location: string) => void
}

export const useUIStore = create<UIState>((set) => ({
  navOpen: false,
  setNavOpen: (navOpen) => set({ navOpen }),
  layoutPreset: readLayoutPreset(),
  setLayoutPreset: (layoutPreset) => {
    writeStoredValue(LAYOUT_PRESET_STORAGE_KEY, layoutPreset)
    set({ layoutPreset, navOpen: false })
  },
  classicBackground: readStoredValue(CLASSIC_BACKGROUND_STORAGE_KEY, ['stars', 'dusk', 'plain'], 'stars'),
  setClassicBackground: (classicBackground) => {
    writeStoredValue(CLASSIC_BACKGROUND_STORAGE_KEY, classicBackground)
    set({ classicBackground })
  },
  classicColorMode: readStoredValue(CLASSIC_COLOR_MODE_STORAGE_KEY, ['system', 'light', 'dark'], 'system'),
  setClassicColorMode: (classicColorMode) => {
    writeStoredValue(CLASSIC_COLOR_MODE_STORAGE_KEY, classicColorMode)
    set({ classicColorMode })
  },
  githubColorMode: readStoredValue(GITHUB_COLOR_MODE_STORAGE_KEY, ['system', 'light', 'dark'], 'system'),
  setGithubColorMode: (githubColorMode) => {
    writeStoredValue(GITHUB_COLOR_MODE_STORAGE_KEY, githubColorMode)
    set({ githubColorMode })
  },
  activeWorkspace: readStoredValue(WORKSPACE_STORAGE_KEY, ['online', 'library'], 'library'),
  lastOnlineLocation: readStoredLocation(ONLINE_LOCATION_STORAGE_KEY, '/online'),
  lastLibraryLocation: readStoredLocation(LIBRARY_LOCATION_STORAGE_KEY, '/'),
  setActiveWorkspace: (activeWorkspace) => {
    writeStoredValue(WORKSPACE_STORAGE_KEY, activeWorkspace)
    set({ activeWorkspace })
  },
  rememberWorkspaceLocation: (workspace, location) => {
    const key = workspace === 'online' ? ONLINE_LOCATION_STORAGE_KEY : LIBRARY_LOCATION_STORAGE_KEY
    writeStoredValue(key, location)
    set(workspace === 'online' ? { lastOnlineLocation: location } : { lastLibraryLocation: location })
  },
}))
