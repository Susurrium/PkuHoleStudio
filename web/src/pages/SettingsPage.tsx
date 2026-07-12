import { useQuery } from '@tanstack/react-query'
import { Check, CircleOff, Database, Server, Sparkles } from 'lucide-react'
import { api } from '../lib/api'
import { PageHeader } from '../components/PageHeader'
import { ErrorState, LoadingState } from '../components/States'

export function SettingsPage() {
	const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
	const providers = useQuery({ queryKey: ['ai-providers'], queryFn: api.aiProviders })
	if (capabilities.isLoading || providers.isLoading) return <LoadingState />
	if (capabilities.error || providers.error || !capabilities.data) return <ErrorState error={capabilities.error || providers.error} />
	const provider = providers.data?.[0]
  return <>
    <PageHeader eyebrow="LOCAL CONFIG" title="设置与能力" description="首版展示当前运行能力。数据库连接、同步账号和 AI 密钥仍通过本机配置文件管理，不会由网页回显敏感值。" />
    <div className="grid gap-5 lg:grid-cols-2">
      <SettingCard icon={Database} title="资料库" items={[['Schema', `v${capabilities.data.schema_version}`], ['全文搜索', capabilities.data.fts5 ? 'FTS5 trigram' : 'LIKE 兼容模式'], ['归档导入', capabilities.data.archive_import ? '可用' : '不可用'], ['归档导出', capabilities.data.archive_export ? '可用' : '不可用']]} />
      <SettingCard icon={Server} title="本机服务" items={[['API', capabilities.data.api_version], ['任务管理', capabilities.data.jobs ? '持久化可用' : '不可用'], ['原生同步', capabilities.data.online_sync ? '可用' : '不可用'], ['访问范围', '由启动参数决定']]} />
		<section className="panel p-6 lg:col-span-2"><div className="flex items-start gap-4"><div className="grid size-11 place-items-center rounded-xl bg-coral-soft text-coral"><Sparkles size={20} /></div><div className="flex-1"><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-lg font-semibold">AI Provider</h2><p className="mt-1 text-sm text-ink-soft">{provider?.name ?? 'OpenAI-compatible'} · {provider?.model ?? '未配置模型'}</p></div><span className={`badge gap-1 ${provider?.configured ? '!border-teal/30 !bg-teal-soft !text-teal' : ''}`}>{provider?.configured ? <><Check size={11} />已启用</> : <><CircleOff size={11} />尚未启用</>}</span></div><p className="mt-5 rounded-xl border border-dashed border-line bg-paper/40 p-4 text-sm leading-6 text-ink-soft">Base URL：{provider?.base_url ?? '未配置'}。API key 不会由网页回显。AI 默认只使用本地 FTS；实时树洞搜索保持独立、默认关闭。</p></div></div></section>
    </div>
  </>
}

function SettingCard({ icon: Icon, title, items }: { icon: typeof Database; title: string; items: [string, string][] }) {
  return <section className="panel p-6"><div className="flex items-center gap-3"><div className="grid size-10 place-items-center rounded-xl bg-teal-soft text-teal"><Icon size={19} /></div><h2 className="text-lg font-semibold">{title}</h2></div><dl className="mt-5 divide-y divide-line/70">{items.map(([label, value]) => <div key={label} className="flex items-center justify-between gap-4 py-3 text-sm"><dt className="text-ink-soft">{label}</dt><dd className="inline-flex items-center gap-1.5 font-medium"><Check size={13} className="text-teal" />{value}</dd></div>)}</dl></section>
}
