import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'

export function parseLocalTagIDs(value: string | null) {
	return [...new Set((value ?? '').split(',').map(Number).filter((item) => Number.isInteger(item) && item > 0))]
}

export function LocalTagFilter({ selected, onChange }: { selected: number[]; onChange: (ids: number[]) => void }) {
	const tags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
	if (tags.isLoading) return <p className="text-xs text-ink-soft">正在读取本地标签…</p>
	if (tags.error) return <p className="text-xs text-coral">标签读取失败：{tags.error.message}</p>
	if (!tags.data?.length) return <p className="text-xs text-ink-soft">尚未创建本地标签，可在设置或帖子详情中创建。</p>
	return <div className="flex flex-wrap gap-2"><button type="button" className={`badge cursor-pointer ${selected.length === 0 ? '!border-teal !bg-teal-soft !text-teal' : ''}`} onClick={() => onChange([])}>全部标签</button>{tags.data.map((tag) => { const active = selected.includes(tag.id); return <button type="button" key={tag.id} className="badge cursor-pointer" style={tag.color ? { borderColor: tag.color, color: tag.color } : undefined} onClick={() => onChange(active ? selected.filter((id) => id !== tag.id) : [...selected, tag.id])}><span className="size-2 rounded-full" style={{ backgroundColor: tag.color || '#94a3b8' }} />{tag.name}{active && <span aria-label="已选择">✓</span>}</button> })}</div>
}
