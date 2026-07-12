import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Check, CircleOff, Database, Pencil, Server, Sparkles, Tags, Trash2 } from 'lucide-react'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'

export function SettingsPage() {
	const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
	const providers = useQuery({ queryKey: ['ai-providers'], queryFn: api.aiProviders })
	const tags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
	if (capabilities.isLoading || providers.isLoading || tags.isLoading) return <LoadingState />
	if (capabilities.error || providers.error || tags.error || !capabilities.data) return <ErrorState error={capabilities.error || providers.error || tags.error} />
	const provider = providers.data?.[0]
  return <>
    <PageHeader eyebrow="LOCAL CONFIG" title="设置与能力" description="首版展示当前运行能力。数据库连接、同步账号和 AI 密钥仍通过本机配置文件管理，不会由网页回显敏感值。" />
    <div className="grid gap-5 lg:grid-cols-2">
      <SettingCard icon={Database} title="资料库" items={[['Schema', `v${capabilities.data.schema_version}`], ['全文搜索', capabilities.data.fts5 ? 'FTS5 trigram' : 'LIKE 兼容模式'], ['归档导入', capabilities.data.archive_import ? '可用' : '不可用'], ['归档导出', capabilities.data.archive_export ? '可用' : '不可用']]} />
      <SettingCard icon={Server} title="本机服务" items={[['API', capabilities.data.api_version], ['任务管理', capabilities.data.jobs ? '持久化可用' : '不可用'], ['原生同步', capabilities.data.online_sync ? '可用' : '不可用'], ['访问范围', '由启动参数决定']]} />
		<section className="panel p-6 lg:col-span-2"><div className="flex items-start gap-4"><div className="grid size-11 place-items-center rounded-xl bg-coral-soft text-coral"><Sparkles size={20} /></div><div className="flex-1"><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-lg font-semibold">AI Provider</h2><p className="mt-1 text-sm text-ink-soft">{provider?.name ?? 'OpenAI-compatible'} · {provider?.model ?? '未配置模型'}</p></div><span className={`badge gap-1 ${provider?.configured ? '!border-teal/30 !bg-teal-soft !text-teal' : ''}`}>{provider?.configured ? <><Check size={11} />已启用</> : <><CircleOff size={11} />尚未启用</>}</span></div><p className="mt-5 rounded-xl border border-dashed border-line bg-paper/40 p-4 text-sm leading-6 text-ink-soft">Base URL：{provider?.base_url ?? '未配置'}。API key 不会由网页回显。AI 默认只使用本地 FTS；实时树洞搜索保持独立、默认关闭。</p></div></div></section>
		<TagManager tags={tags.data ?? []} refresh={() => tags.refetch()} />
    </div>
  </>
}

function TagManager({ tags, refresh }: { tags: import('../lib/types').LocalTag[]; refresh: () => unknown }) {
	const [editingID, setEditingID] = useState<number>()
	const [name, setName] = useState('')
	const [color, setColor] = useState('#0f766e')
	const editing = tags.find((tag) => tag.id === editingID)
	useEffect(() => {
		setName(editing?.name ?? '')
		setColor(editing?.color || '#0f766e')
	}, [editing?.id, editing?.name, editing?.color])
	const save = useMutation({ mutationFn: () => editingID ? api.updateLocalTag(editingID, name, color) : api.createLocalTag(name, color), onSuccess: () => { setEditingID(undefined); setName(''); refresh() } })
	const remove = useMutation({ mutationFn: api.deleteLocalTag, onSuccess: () => { setEditingID(undefined); refresh() } })
	return <section className="panel p-6 lg:col-span-2"><div className="flex items-center gap-3"><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Tags size={19} /></div><div><h2 className="text-lg font-semibold">本地标签管理</h2><p className="mt-1 text-xs text-ink-soft">标签和笔记只存储在本机；同步远端内容不会覆盖它们。</p></div></div><div className="mt-5 grid gap-5 lg:grid-cols-[1fr_360px]"><div className="flex flex-wrap content-start gap-2">{tags.length ? tags.map((tag) => <button key={tag.id} className={`badge gap-2 ${editingID === tag.id ? '!border-teal !bg-teal-soft !text-teal' : ''}`} onClick={() => setEditingID(tag.id)}><span className="size-2 rounded-full" style={{ backgroundColor: tag.color || '#94a3b8' }} />{tag.name}<Pencil size={11} /></button>) : <p className="text-sm text-ink-soft">还没有标签。请在右侧创建第一个标签。</p>}</div><div className="rounded-xl border border-line bg-paper/45 p-4"><p className="text-xs font-semibold">{editingID ? '编辑标签' : '创建标签'}</p><input className="field mt-3" value={name} maxLength={128} onChange={(event) => setName(event.target.value)} placeholder="标签名称" /><label className="mt-3 flex items-center gap-3 text-xs text-ink-soft">颜色<input type="color" value={color} onChange={(event) => setColor(event.target.value)} /></label><div className="mt-4 flex flex-wrap gap-2"><button className="button-primary" disabled={!name.trim() || save.isPending} onClick={() => save.mutate()}>{editingID ? '保存修改' : '创建标签'}</button>{editingID && <><button className="button-secondary" onClick={() => setEditingID(undefined)}>取消</button><button className="button-secondary !text-coral" disabled={remove.isPending} onClick={() => remove.mutate(editingID)}><Trash2 size={14} />删除</button></>}</div>{(save.error || remove.error) && <p className="mt-3 text-xs text-coral">{String(save.error || remove.error)}</p>}</div></div></section>
}

function SettingCard({ icon: Icon, title, items }: { icon: typeof Database; title: string; items: [string, string][] }) {
  return <section className="panel p-6"><div className="flex items-center gap-3"><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={19} /></div><h2 className="text-lg font-semibold">{title}</h2></div><dl className="mt-5 divide-y divide-line/70">{items.map(([label, value]) => <div key={label} className="flex items-center justify-between gap-4 py-3 text-sm"><dt className="text-ink-soft">{label}</dt><dd className="inline-flex items-center gap-1.5 font-medium"><Check size={13} className="text-teal" />{value}</dd></div>)}</dl></section>
}
