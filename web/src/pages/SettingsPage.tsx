import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Activity, Archive, Check, CircleOff, Database, PanelsTopLeft, Pencil, Plus, Radio, RotateCcw, Server, Sparkles, Tags, Trash2, X } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { errorDescription, useFeedback } from '../components/Feedback'
import { restartOnboarding } from '../components/OnboardingGuide'
import { PageHeader } from '../components/PageHeader'
import { ObserverSettingsPanel } from '../components/ObserverSettingsPanel'
import { ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { AIProviderSetting, LocalTag, Settings, SettingsUpdate } from '../lib/types'
import { useUIStore, type LayoutPreset } from '../store/ui'

export function SettingsPage() {
	const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
	const providers = useQuery({ queryKey: ['ai-providers'], queryFn: api.aiProviders })
	const tags = useQuery({ queryKey: ['local-tags'], queryFn: api.localTags })
	const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings })
	if (capabilities.isLoading || providers.isLoading || tags.isLoading || settings.isLoading) return <LoadingState label="正在读取本机设置…" />
	if (capabilities.error || providers.error || tags.error || settings.error || !capabilities.data || !settings.data) return <ErrorState error={capabilities.error || providers.error || tags.error || settings.error} />
	const provider = providers.data?.find((item) => item.active)
	const refreshAI = () => Promise.all([settings.refetch(), providers.refetch()])
	return <>
		<PageHeader eyebrow="LOCAL CONFIG" title="设置" description="按使用方式、本机资料和可选服务管理 Studio。账户密钥只写入本机配置文件，网页不会回显。" />
		<nav className="panel sticky top-[72px] z-10 mb-6 flex items-center gap-1 overflow-x-auto p-1.5" aria-label="设置分区">
			<span className="shrink-0 px-2 font-mono text-[9px] font-semibold uppercase tracking-wider text-ink-soft/60">使用</span><a className="shrink-0 rounded-lg px-3.5 py-2 text-xs font-semibold text-ink-soft transition hover:bg-white hover:text-teal" href="#usage-and-interface">工作区与界面</a>
			<span className="ml-2 shrink-0 px-2 font-mono text-[9px] font-semibold uppercase tracking-wider text-ink-soft/60">本机资料</span>{[['#system-overview', '系统状态'], ['#local-tags', '本地标签']].map(([href, label]) => <a key={href} className="shrink-0 rounded-lg px-3.5 py-2 text-xs font-semibold text-ink-soft transition hover:bg-white hover:text-teal" href={href}>{label}</a>)}
			<span className="ml-2 shrink-0 px-2 font-mono text-[9px] font-semibold uppercase tracking-wider text-ink-soft/60">可选服务</span>{[['#observer-service', 'Observer'], ['#ai-providers', 'AI 服务'], ['#ai-policy', 'AI 策略']].map(([href, label]) => <a key={href} className="shrink-0 rounded-lg px-3.5 py-2 text-xs font-semibold text-ink-soft transition hover:bg-white hover:text-teal" href={href}>{label}</a>)}
		</nav>
		<div className="grid gap-5 lg:grid-cols-2">
			<ExperienceSettings />
			<section id="system-overview" className="grid scroll-mt-32 gap-5 lg:col-span-2 lg:grid-cols-2">
				<SettingCard icon={Database} title="资料库" items={[['运行资料目录', settings.data.data_dir || '未提供'], ['数据库文件', settings.data.database_file || settings.data.database_type], ['Schema', `v${capabilities.data.schema_version}`], ['全文搜索', capabilities.data.fts5 ? 'FTS5 trigram' : 'LIKE 兼容模式'], ['归档导入 / 导出', capabilities.data.archive_import && capabilities.data.archive_export ? '可用' : '部分不可用']]} />
				<SettingCard icon={Server} title="本机服务" items={[['API', capabilities.data.api_version], ['任务管理', capabilities.data.jobs ? '持久化可用' : '不可用'], ['原生同步', capabilities.data.online_sync ? '可用' : '不可用'], ['访问范围', '由启动参数决定']]} />
			</section>
			<ObserverSettingsPanel />
			<TagManager tags={tags.data ?? []} refresh={() => tags.refetch()} />
			<ProviderManager initial={settings.data} refresh={refreshAI} />
			<AISettingsForm initial={settings.data} runtimeConfigured={provider?.configured ?? false} refresh={refreshAI} />
		</div>
	</>
}

function ExperienceSettings() {
	const navigate = useNavigate()
	const { layoutPreset, setLayoutPreset } = useUIStore()
	const presets: { id: LayoutPreset; title: string; description: string }[] = [
		{ id: 'studio', title: 'Studio 工作台', description: '默认界面，适合在线浏览与本地整理的完整流程。' },
		{ id: 'classic', title: '经典树洞', description: '复刻官方原版树洞的排布与操作习惯。' },
		{ id: 'github', title: 'GitHub 风格', description: '适合偏好 Issues 与 Conversation 信息密度的用户。' },
	]
	return <section id="usage-and-interface" className="panel scroll-mt-32 p-5 lg:col-span-2 md:p-6">
		<div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><PanelsTopLeft size={20} /></div><div><p className="eyebrow">EXPERIENCE</p><h2 className="mt-1 text-lg font-semibold">工作区与界面</h2><p className="mt-1 text-xs leading-5 text-ink-soft">工作区决定当前使用在线内容还是本机资料；界面只改变呈现方式，不改变数据。</p></div></div>
		<div className="mt-5 grid gap-3 md:grid-cols-2"><button className="rounded-xl border border-line bg-white/45 p-4 text-left transition hover:border-teal/40 hover:bg-white/75" onClick={() => navigate('/online')}><Radio size={18} className="text-teal" /><p className="mt-3 font-semibold">打开在线树洞</p><p className="mt-1 text-xs leading-5 text-ink-soft">实时浏览和互动；只有明确保存的内容才进入本地。</p></button><button className="rounded-xl border border-line bg-white/45 p-4 text-left transition hover:border-teal/40 hover:bg-white/75" onClick={() => navigate('/')}><Archive size={18} className="text-teal" /><p className="mt-3 font-semibold">打开本地资料库</p><p className="mt-1 text-xs leading-5 text-ink-soft">离线搜索、整理项目、添加标签以及导入导出。</p></button></div>
		<div className="mt-6 border-t border-line pt-5"><p className="text-sm font-semibold">界面方案</p><div className="mt-3 grid gap-3 md:grid-cols-3">{presets.map((preset) => <button key={preset.id} type="button" aria-pressed={layoutPreset === preset.id} className={`rounded-xl border p-4 text-left transition ${layoutPreset === preset.id ? 'border-teal bg-teal-soft/30 ring-2 ring-teal/10' : 'border-line bg-white/40 hover:border-teal/35'}`} onClick={() => setLayoutPreset(preset.id)}><p className="font-semibold">{preset.title}</p><p className="mt-1.5 text-xs leading-5 text-ink-soft">{preset.description}</p>{layoutPreset === preset.id && <span className="badge mt-3 !border-teal/30 !text-teal"><Check size={11} />当前使用</span>}</button>)}</div></div>
		<div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-line bg-paper/45 p-4"><div><p className="text-sm font-semibold">需要重新了解使用流程？</p><p className="mt-1 text-xs text-ink-soft">重新打开两步入门引导，不会修改或删除任何资料。</p></div><button className="button-secondary" onClick={() => { if (layoutPreset !== 'studio') setLayoutPreset('studio'); window.setTimeout(restartOnboarding, 0) }}><RotateCcw size={14} />重新查看入门引导</button></div>
	</section>
}

function ProviderManager({ initial, refresh }: { initial: Settings; refresh: () => unknown }) {
	const providers = initial.ai_providers ?? []
	const [editingID, setEditingID] = useState<string | 'new'>()
	const { confirm, notify } = useFeedback()
	const activate = useMutation({
		mutationFn: api.activateAIProviderSetting,
		onSuccess: (_, id) => { void refresh(); notify({ title: '活动 AI 服务已切换', description: `${providers.find((item) => item.id === id)?.name || '所选服务'} 将用于新会话。` }) },
		onError: (error) => notify({ tone: 'error', title: '切换失败', description: errorDescription(error) }),
	})
	const remove = useMutation({
		mutationFn: api.deleteAIProviderSetting,
		onSuccess: (_, id) => { setEditingID(undefined); void refresh(); notify({ title: 'AI 服务已删除', description: providers.find((item) => item.id === id)?.name }) },
		onError: (error) => notify({ tone: 'error', title: '删除失败', description: errorDescription(error) }),
	})
	const test = useMutation({
		mutationFn: api.testAIProviderSetting,
		onSuccess: (result) => notify({ title: '连接测试成功', description: `${result.model} · ${result.latency_ms} ms` }),
		onError: (error) => notify({ tone: 'error', title: '连接测试失败', description: errorDescription(error) }),
	})
	const requestDelete = async (provider: AIProviderSetting) => {
		const accepted = await confirm({ title: `删除“${provider.name}”？`, description: '将删除这个 Provider 的地址、模型参数和本机保存的 API key。已有 AI 研究记录不会被删除。', confirmLabel: '删除 Provider', tone: 'danger' })
		if (accepted) remove.mutate(provider.id)
	}
	return <section id="ai-providers" className="panel scroll-mt-32 p-5 lg:col-span-2 md:p-6">
		<div className="flex flex-wrap items-center justify-between gap-3"><div><p className="eyebrow">AI PROVIDERS</p><h2 className="mt-1 text-lg font-semibold">AI 服务</h2><p className="mt-1 text-xs leading-5 text-ink-soft">活动服务用于新研究；切换和编辑保存后立即生效。</p></div><button className="button-secondary" onClick={() => setEditingID('new')}><Plus size={14} />新增 AI 服务</button></div>
		<div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">{providers.map((item) => <article key={item.id} className={`rounded-xl border p-4 ${item.active ? 'border-teal bg-teal-soft/30' : 'border-line bg-white/40'}`}>
			<div className="flex items-start justify-between gap-2"><div className="min-w-0"><h3 className="truncate font-semibold">{item.name}</h3><p className="mt-1 truncate font-mono text-xs text-ink-soft">{item.model}</p></div>{item.active && <span className="badge shrink-0 !border-teal/30 !text-teal">运行中</span>}</div>
			<p className="mt-3 truncate text-xs text-ink-soft" title={item.base_url}>{item.base_url}</p><p className="mt-2 text-xs text-ink-soft">API key：{item.api_key_configured ? '已安全保存' : '未配置（可用于本地无鉴权服务）'}</p>
			<div className="mt-4 flex flex-wrap gap-2">{!item.active && <button className="button-secondary !min-h-9 !py-1.5 text-xs" disabled={activate.isPending} onClick={() => activate.mutate(item.id)}>{activate.isPending && activate.variables === item.id ? '切换中…' : '设为活动'}</button>}<button className="button-secondary !min-h-9 !px-3 !py-1.5 text-xs" onClick={() => setEditingID(item.id)}><Pencil size={13} />编辑</button><button className="button-secondary !min-h-9 !px-3 !py-1.5 text-xs" disabled={test.isPending} onClick={() => test.mutate(item.id)}><Activity size={13} />{test.isPending && test.variables === item.id ? '测试中…' : '测试'}</button><button className="button-secondary !min-h-9 !px-3 !py-1.5 text-xs !text-coral" aria-label={`删除 AI 服务 ${item.name}`} disabled={remove.isPending || providers.length <= 1} onClick={() => requestDelete(item)}><Trash2 size={13} />删除</button></div>
			{test.data?.provider_id === item.id && <p className="mt-3 text-xs text-teal">连接成功 · {test.data.latency_ms} ms</p>}
		</article>)}</div>
		{providers.length <= 1 && <p className="mt-3 text-xs text-ink-soft">至少需要保留一个 AI 服务，因此当前服务不能删除。</p>}
		{editingID && <ProviderEditor provider={editingID === 'new' ? undefined : providers.find((item) => item.id === editingID)} close={() => setEditingID(undefined)} refresh={refresh} />}
	</section>
}

function ProviderEditor({ provider, close, refresh }: { provider?: AIProviderSetting; close: () => void; refresh: () => unknown }) {
	const { notify } = useFeedback()
	const [draft, setDraft] = useState(() => ({ name: provider?.name ?? '', base_url: provider?.base_url ?? 'https://api.deepseek.com', model: provider?.model ?? 'deepseek-chat', temperature: provider?.temperature ?? 0.2, max_output_tokens: provider?.max_output_tokens ?? 4096, request_timeout_seconds: provider?.request_timeout_seconds ?? 120, api_key: '' }))
	const save = useMutation({
		mutationFn: () => provider ? api.updateAIProviderSetting(provider.id, { ...draft, api_key: draft.api_key || undefined }) : api.createAIProviderSetting({ ...draft, api_key: draft.api_key || undefined }),
		onSuccess: () => { void refresh(); notify({ title: provider ? 'AI 服务设置已保存' : 'AI 服务已创建', description: `${draft.name} · ${draft.model}` }); close() },
		onError: (error) => notify({ tone: 'error', title: '未能保存 AI 服务', description: errorDescription(error) }),
	})
	return <form className="mt-5 rounded-xl border border-line bg-paper/45 p-4" onSubmit={(event) => { event.preventDefault(); save.mutate() }}>
		<div className="flex items-center justify-between"><div><p className="eyebrow">{provider ? 'EDIT PROVIDER' : 'NEW PROVIDER'}</p><h3 className="mt-1 font-semibold">{provider ? `编辑 ${provider.name}` : '新增 OpenAI-compatible 服务'}</h3></div><button type="button" className="button-secondary !p-2" aria-label="关闭 Provider 编辑" onClick={close}><X size={14} /></button></div>
		<div className="mt-4 grid gap-3 md:grid-cols-2"><label className="text-xs font-medium text-ink-soft">显示名称<input className="field mt-1.5" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label><label className="text-xs font-medium text-ink-soft">模型<input className="field mt-1.5" value={draft.model} onChange={(event) => setDraft({ ...draft, model: event.target.value })} /></label><label className="text-xs font-medium text-ink-soft md:col-span-2">Base URL<input className="field mt-1.5" type="url" value={draft.base_url} onChange={(event) => setDraft({ ...draft, base_url: event.target.value })} /></label><label className="text-xs font-medium text-ink-soft md:col-span-2">API key（留空保留已有密钥；本地服务可为空）<input className="field mt-1.5" type="password" autoComplete="new-password" value={draft.api_key} placeholder={provider?.api_key_configured ? '已配置；不会回显' : '未配置；本地无鉴权服务可留空'} onChange={(event) => setDraft({ ...draft, api_key: event.target.value })} /></label></div>
		<details className="mt-4 rounded-xl border border-line bg-white/40 p-3"><summary className="cursor-pointer text-sm font-semibold">高级生成参数</summary><div className="mt-3 grid gap-3 md:grid-cols-3"><label className="text-xs text-ink-soft">Temperature<input className="field mt-1" type="number" min="0" max="2" step="0.1" value={draft.temperature} onChange={(event) => setDraft({ ...draft, temperature: Number(event.target.value) })} /></label><label className="text-xs text-ink-soft">最大输出 tokens<input className="field mt-1" type="number" min="1" max="1000000" value={draft.max_output_tokens} onChange={(event) => setDraft({ ...draft, max_output_tokens: Number(event.target.value) })} /></label><label className="text-xs text-ink-soft">请求超时（秒）<input className="field mt-1" type="number" min="1" max="3600" value={draft.request_timeout_seconds} onChange={(event) => setDraft({ ...draft, request_timeout_seconds: Number(event.target.value) })} /></label></div></details>
		<div className="mt-4 flex flex-wrap gap-2"><button className="button-primary" disabled={save.isPending || !draft.name.trim() || !draft.model.trim() || !draft.base_url.trim()}>{save.isPending ? '正在保存…' : '保存并立即应用'}</button><button type="button" className="button-secondary" onClick={close}>取消</button></div>
	</form>
}

function AISettingsForm({ initial, runtimeConfigured, refresh }: { initial: Settings; runtimeConfigured: boolean; refresh: () => unknown }) {
	const [draft, setDraft] = useState<SettingsUpdate>(() => settingsDraft(initial))
	const { notify } = useFeedback()
	useEffect(() => setDraft(settingsDraft(initial)), [initial])
	const save = useMutation({
		mutationFn: () => api.updateSettings(draft),
		onSuccess: () => { void refresh(); notify({ title: 'AI 策略已保存', description: '新会话会立即使用最新设置。' }) },
		onError: (error) => notify({ tone: 'error', title: '设置保存失败', description: errorDescription(error) }),
	})
	const number = (key: keyof SettingsUpdate, value: string) => setDraft((current) => ({ ...current, [key]: Number(value) }))
	return <section id="ai-policy" className="panel scroll-mt-32 p-5 lg:col-span-2 md:p-6"><div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-coral-soft text-coral"><Sparkles size={20} /></div><div className="min-w-0 flex-1">
		<div className="flex flex-wrap items-center justify-between gap-3"><div><p className="eyebrow">AI POLICY</p><h2 className="mt-1 text-lg font-semibold">AI 使用策略</h2><p className="mt-1 text-sm text-ink-soft">运行时：{initial.ai_runtime_provider || initial.ai_provider_name} · {initial.ai_runtime_model || initial.ai_model}</p></div><span className={`badge gap-1 ${runtimeConfigured ? '!border-teal/30 !bg-teal-soft !text-teal' : ''}`}>{runtimeConfigured ? <><Check size={11} />当前进程已启用</> : <><CircleOff size={11} />当前进程未启用</>}</span></div>
		<div className="mt-5 grid gap-4 md:grid-cols-2"><label className="text-xs font-medium text-ink-soft">每个问题最大检索轮数<input className="field mt-1.5" type="number" min="1" max="5" value={draft.ai_max_search_rounds} onChange={(event) => number('ai_max_search_rounds', event.target.value)} /><span className="mt-1.5 block font-normal leading-5">轮数越高，证据覆盖可能更完整，但响应会更慢。</span></label></div>
		<div className="mt-5 grid gap-3 sm:grid-cols-2"><ToggleCard checked={draft.ai_enabled} title="启用 AI 研究" description="关闭后保留现有研究记录，但不能发起新问题。" onChange={(checked) => setDraft({ ...draft, ai_enabled: checked })} /><ToggleCard checked={draft.ai_live_search} title="允许实时搜索树洞" description="AI 可在本地资料不足时使用当前在线会话补充检索。" onChange={(checked) => setDraft({ ...draft, ai_live_search: checked })} /></div>
		<div className="mt-5 flex flex-wrap items-center gap-3"><button className="button-primary" disabled={save.isPending} onClick={() => save.mutate()}>{save.isPending ? '正在保存…' : '保存策略'}</button><p className="text-xs text-ink-soft">正在生成的回答使用原快照；新会话使用最新策略。</p></div>{save.isSuccess && <p className="mt-3 text-sm text-teal">设置已安全写入并应用，无需重启。</p>}
	</div></div></section>
}

function ToggleCard({ checked, title, description, onChange }: { checked: boolean; title: string; description: string; onChange: (checked: boolean) => void }) {
	return <label className={`flex cursor-pointer items-start gap-3 rounded-xl border p-4 transition ${checked ? 'border-teal/40 bg-teal-soft/25' : 'border-line bg-white/40'}`}><input className="mt-1" type="checkbox" aria-label={title} checked={checked} onChange={(event) => onChange(event.target.checked)} /><span><span className="block text-sm font-semibold">{title}</span><span className="mt-1 block text-xs leading-5 text-ink-soft">{description}</span></span></label>
}

function settingsDraft(settings: Settings): SettingsUpdate {
	return { ai_enabled: settings.ai_enabled, ai_live_search: settings.ai_live_search, ai_provider_name: settings.ai_provider_name, ai_base_url: settings.ai_base_url, ai_model: settings.ai_model, ai_temperature: settings.ai_temperature, ai_max_output_tokens: settings.ai_max_output_tokens, ai_request_timeout_seconds: settings.ai_request_timeout_seconds, ai_max_search_rounds: settings.ai_max_search_rounds }
}

function TagManager({ tags, refresh }: { tags: LocalTag[]; refresh: () => unknown }) {
	const [editingID, setEditingID] = useState<number>()
	const [name, setName] = useState('')
	const [color, setColor] = useState('#0f766e')
	const { confirm, notify } = useFeedback()
	const editing = tags.find((tag) => tag.id === editingID)
	useEffect(() => { setName(editing?.name ?? ''); setColor(editing?.color || '#0f766e') }, [editing?.id, editing?.name, editing?.color])
	const save = useMutation({
		mutationFn: () => editingID ? api.updateLocalTag(editingID, name.trim(), color) : api.createLocalTag(name.trim(), color),
		onSuccess: () => { notify({ title: editingID ? '标签已更新' : '标签已创建', description: name.trim() }); setEditingID(undefined); setName(''); setColor('#0f766e'); void refresh() },
		onError: (error) => notify({ tone: 'error', title: '标签保存失败', description: errorDescription(error) }),
	})
	const remove = useMutation({
		mutationFn: api.deleteLocalTag,
		onSuccess: () => { notify({ title: '标签已删除', description: editing?.name }); setEditingID(undefined); void refresh() },
		onError: (error) => notify({ tone: 'error', title: '标签删除失败', description: errorDescription(error) }),
	})
	const requestDelete = async () => {
		if (!editingID || !editing) return
		const accepted = await confirm({ title: `删除标签“${editing.name}”？`, description: '该标签会从所有本地树洞中移除。树洞正文、评论和本地笔记不会被删除。', confirmLabel: '删除标签', tone: 'danger' })
		if (accepted) remove.mutate(editingID)
	}
	return <section id="local-tags" className="panel scroll-mt-32 p-5 lg:col-span-2 md:p-6">
		<div className="flex items-center gap-3"><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Tags size={19} /></div><div><p className="eyebrow">LOCAL METADATA</p><h2 className="mt-1 text-lg font-semibold">本地标签</h2><p className="mt-1 text-xs leading-5 text-ink-soft">标签和笔记只存储在本机，同步远端内容不会覆盖它们。</p></div></div>
		<div className="mt-5 grid gap-5 lg:grid-cols-[1fr_360px]"><div className="flex flex-wrap content-start gap-2">{tags.length ? tags.map((tag) => <button key={tag.id} className={`badge min-h-9 cursor-pointer gap-2 transition hover:border-teal ${editingID === tag.id ? '!border-teal !bg-teal-soft !text-teal' : ''}`} aria-pressed={editingID === tag.id} onClick={() => setEditingID(tag.id)}><span className="size-2 rounded-full" style={{ backgroundColor: tag.color || '#94a3b8' }} />{tag.name}<Pencil size={11} /></button>) : <p className="rounded-xl border border-dashed border-line p-6 text-sm leading-6 text-ink-soft">还没有标签。使用右侧表单创建后，即可在树洞详情中标记资料。</p>}</div>
			<form className="rounded-xl border border-line bg-paper/45 p-4" onSubmit={(event) => { event.preventDefault(); save.mutate() }}><p className="text-sm font-semibold">{editingID ? `编辑“${editing?.name || ''}”` : '创建标签'}</p><label className="mt-3 block text-xs font-medium text-ink-soft">标签名称<input className="field mt-1.5" value={name} maxLength={128} onChange={(event) => setName(event.target.value)} placeholder="例如：课程、收藏、待整理" /></label><label className="mt-3 flex items-center justify-between gap-3 text-xs font-medium text-ink-soft">标签颜色<input className="size-10 cursor-pointer rounded-lg border border-line bg-white p-1" type="color" value={color} aria-label="标签颜色" onChange={(event) => setColor(event.target.value)} /></label><div className="mt-4 flex flex-wrap gap-2"><button className="button-primary" disabled={!name.trim() || save.isPending}>{save.isPending ? '保存中…' : editingID ? '保存修改' : '创建标签'}</button>{editingID && <><button type="button" className="button-secondary" onClick={() => setEditingID(undefined)}>取消</button><button type="button" className="button-secondary !text-coral" disabled={remove.isPending} onClick={requestDelete}><Trash2 size={14} />删除标签</button></>}</div></form>
		</div>
	</section>
}

function SettingCard({ icon: Icon, title, items }: { icon: typeof Database; title: string; items: [string, string][] }) {
	return <section className="panel p-5 md:p-6"><div className="flex items-center gap-3"><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={19} /></div><h2 className="text-lg font-semibold">{title}</h2></div><dl className="mt-5 divide-y divide-line/70">{items.map(([label, value]) => <div key={label} className="flex items-start justify-between gap-4 py-3 text-sm"><dt className="shrink-0 text-ink-soft">{label}</dt><dd className="inline-flex min-w-0 items-start gap-1.5 break-all text-right font-medium"><Check size={13} className="mt-0.5 shrink-0 text-teal" />{value}</dd></div>)}</dl></section>
}
