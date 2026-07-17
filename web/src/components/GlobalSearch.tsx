import { FormEvent, useState } from 'react'
import { Search } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useUIStore } from '../store/ui'

export function GlobalSearch({ compact = false }: { compact?: boolean }) {
  const navigate = useNavigate()
  const activeWorkspace = useUIStore((state) => state.activeWorkspace)
  const [value, setValue] = useState('')

  function submit(event: FormEvent) {
    event.preventDefault()
    const query = value.trim()
    if (!query) {
      navigate(`/posts?${activeWorkspace === 'online' ? 'source=live&' : ''}focus=search`)
      return
    }
    const pid = query.match(/^#?(\d+)$/)?.[1]
    const source = activeWorkspace === 'online' ? 'source=live&' : ''
    navigate(pid ? `/posts/${pid}${activeWorkspace === 'online' ? '?source=live' : ''}` : `/posts?${source}q=${encodeURIComponent(query)}`)
    setValue('')
  }

  return (
    <form className={`relative w-full ${compact ? 'max-w-lg' : 'max-w-2xl'}`} role="search" onSubmit={submit}>
      <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft" size={17} />
      <input
        className="field !min-h-10 !pl-10 !pr-24"
        value={value}
        onChange={(event) => setValue(event.target.value)}
        placeholder={activeWorkspace === 'online' ? '搜索在线树洞，或输入 #PID 直接打开' : '搜索本地资料，或输入 #PID 直接打开'}
        aria-label="全局搜索"
      />
      <button className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-lg px-3 py-1.5 text-xs font-semibold text-teal hover:bg-teal-soft" type="submit" aria-label="提交全局搜索">
        搜索
      </button>
    </form>
  )
}
