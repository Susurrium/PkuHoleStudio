import { create } from 'zustand'

interface UIState {
  navOpen: boolean
  setNavOpen: (open: boolean) => void
}

export const useUIStore = create<UIState>((set) => ({
  navOpen: false,
  setNavOpen: (navOpen) => set({ navOpen }),
}))
