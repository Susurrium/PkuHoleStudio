import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Check, CircleOff, Database, Pencil, Server, Sparkles, Tags, Trash2 } from 'lucide-react'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'
import type { Settings, SettingsUpdate } from '../lib/types'

export function SettingsPage() {
	const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
	const providers = useQuery({ queryKey: ['ai-providers'], queryFn: api.aiProviders })
	const tags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
	const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings })
	if (capabilities.isLoading || providers.isLoading || tags.isLoading || settings.isLoading) return <LoadingState />
	if (capabilities.error || providers.error || tags.error || settings.error || !capabilities.data || !settings.data) return <ErrorState error={capabilities.error || providers.error || tags.error || settings.error} />
	const provider = providers.data?.[0]
  return <>
    <PageHeader eyebrow="LOCAL CONFIG" title="设置与能力" description="查看本机资料库与服务能力，并安全配置 OpenAI-compatible 模型。API key 只写入本机配置文件，网页不会回显。" />
    <div className="grid gap-5 lg:grid-cols-2">
      <SettingCard icon={Database} title="资料库" items={[['Schema', `v${capabilities.data.schema_version}`], ['全文搜索', capabilities.data.fts5 ? 'FTS5 trigram' : 'LIKE 兼容模式'], ['归档导入', capabilities.data.archive_import ? '可用' : '不可用'], ['归档导出', capabilities.data.archive_export ? '可用' : '不可用']]} />
      <SettingCard icon={Server} title="本机服务" items={[['API', capabilities.data.api_version], ['任务管理', capabilities.data.jobs ? '持久化可用' : '不可用'], ['原生同步', capabilities.data.online_sync ? '可用' : '不可用'], ['访问范围', '由启动参数决定']]} />
		<AISettingsForm initial={settings.data} runtimeConfigured={provider?.configured ?? false} refresh={() => settings.refetch()} />
		<TagManager tags={tags.data ?? []} refresh={() => tags.refetch()} />
    </div>
  </>
}

function AISettingsForm({ initial, runtimeConfigured, refresh }: { initial: Settings; runtimeConfigured: boolean; refresh: () => unknown }) {
	const [draft, setDraft] = useState<SettingsUpdate>(() => settingsDraft(initial))
	const [apiKey, setAPIKey] = useState('')
	useEffect(() => setDraft(settingsDraft(initial)), [initial])
	const save = useMutation({ mutationFn: () => api.updateSettings({ ...draft, ai_api_key: apiKey || undefined }), onSuccess: () => { setAPIKey(''); refresh() } })
	const number = (key: keyof SettingsUpdate, value: string) => setDraft((current) => ({ ...current, [key]: Number(value) }))
	return <section className="panel p-6 lg:col-span-2"><div className="flex items-start gap-4"><div className="grid size-11 place-items-center rounded-xl bg-coral-soft text-coral"><Sparkles size={20} /></div><div className="flex-1"><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-lg font-semibold">AI Provider</h2><p className="mt-1 text-sm text-ink-soft">OpenAI-compatible 多模型配置</p></div><span className={`badge gap-1 ${runtimeConfigured ? '!border-teal/30 !bg-teal-soft !text-teal' : ''}`}>{runtimeConfigured ? <><Check size={11} />当前进程已启用</> : <><CircleOff size={11} />当前进程未启用</>}</span></div><div className="mt-5 grid gap-4 md:grid-cols-2"><label className="text-xs text-ink-soft">Provider 名称<input className="field mt-1" value={draft.ai_provider_name} onChange={(event) => setDraft({ ...draft, ai_provider_name: event.target.value })} /></label><label className="text-xs text-ink-soft">模型<input className="field mt-1" value={draft.ai_model} onChange={(event) => setDraft({ ...draft, ai_model: event.target.value })} /></label><label className="text-xs text-ink-soft md:col-span-2">Base URL<input className="field mt-1" value={draft.ai_base_url} onChange={(event) => setDraft({ ...draft, ai_base_url: event.target.value })} /></label><label className="text-xs text-ink-soft md:col-span-2">API key（留空即保留现有密钥）<input className="field mt-1" type="password" autoComplete="new-password" value={apiKey} onChange={(event) => setAPIKey(event.target.value)} placeholder={initial.ai_api_key_configured ? '已配置；不会回显' : '尚未配置'} /></label><label className="text-xs text-ink-soft">Temperature<input className="field mt-1" type="number" min="0" max="2" step="0.1" value={draft.ai_temperature} onChange={(event) => number('ai_temperature', event.target.value)} /></label><label className="text-xs text-ink-soft">最大输出 tokens<input className="field mt-1" type="number" min="1" max="1000000" value={draft.ai_max_output_tokens} onChange={(event) => number('ai_max_output_tokens', event.target.value)} /></label><label className="text-xs text-ink-soft">请求超时（秒）<input className="field mt-1" type="number" min="1" max="3600" value={draft.ai_request_timeout_seconds} onChange={(event) => number('ai_request_timeout_seconds', event.target.value)} /></label><label className="text-xs text-ink-soft">最大检索轮数<input className="field mt-1" type="number" min="1" max="20" value={draft.ai_max_search_rounds} onChange={(event) => number('ai_max_search_rounds', event.target.value)} /></label></div><div className="mt-4 flex flex-wrap gap-5 text-sm"><label className="inline-flex items-center gap-2"><input type="checkbox" checked={draft.ai_enabled} onChange={(event) => setDraft({ ...draft, ai_enabled: event.target.checked })} />启用 AI</label><label className="inline-flex items-center gap-2"><input type="checkbox" checked={draft.ai_live_search} onChange={(event) => setDraft({ ...draft, ai_live_search: event.target.checked })} />允许 AI 实时搜索树洞</label></div><div className="mt-5 flex flex-wrap items-center gap-3"><button className="button-primary" disabled={save.isPending} onClick={() => save.mutate()}>{save.isPending ? '正在保存…' : '保存 AI 设置'}</button><p className="text-xs text-ink-soft">保存后需重启 PkuHoleStudio 才会应用到 AI 会话。</p></div>{save.isSuccess && <p className="mt-3 text-sm text-teal">设置已安全写入；API key 未被回显。请重启程序。</p>}{save.error && <p className="mt-3 text-sm text-coral">{String(save.error)}</p>}</div></div></section>
}

function settingsDraft(settings: Settings): SettingsUpdate {
	return { ai_enabled: settings.ai_enabled, ai_live_search: settings.ai_live_search, ai_provider_name: settings.ai_provider_name, ai_base_url: settings.ai_base_url, ai_model: settings.ai_model, ai_temperature: settings.ai_temperature, ai_max_output_tokens: settings.ai_max_output_tokens, ai_request_timeout_seconds: settings.ai_request_timeout_seconds, ai_max_search_rounds: settings.ai_max_search_rounds }
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
