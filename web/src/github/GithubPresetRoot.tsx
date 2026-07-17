import { useEffect, type PropsWithChildren } from 'react'
import { useUIStore } from '../store/ui'

export function GithubPresetRoot({ children }: PropsWithChildren) {
  const colorMode = useUIStore((state) => state.githubColorMode)
  useEffect(() => {
    const media = typeof window.matchMedia === 'function' ? window.matchMedia('(prefers-color-scheme: dark)') : null
    const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    const previousMeta = meta?.content
    const previousScheme = document.documentElement.style.colorScheme
    const apply = () => {
      const dark = colorMode === 'dark' || (colorMode === 'system' && media?.matches === true)
      if (meta) meta.content = dark ? '#0d1117' : '#f6f8fa'
      document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
    }
    apply()
    if (colorMode === 'system') media?.addEventListener('change', apply)
    return () => {
      if (colorMode === 'system') media?.removeEventListener('change', apply)
      if (meta && previousMeta !== undefined) meta.content = previousMeta
      document.documentElement.style.colorScheme = previousScheme
    }
  }, [colorMode])
  return <div className="github-preset-root" data-color-mode={colorMode === 'system' ? 'auto' : colorMode} data-light-theme="light" data-dark-theme="dark">{children}</div>
}
