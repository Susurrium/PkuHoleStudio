import { ChangeEvent, DragEvent, useDeferredValue, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, CheckCircle2, Copy, Download, ExternalLink, FileArchive, FileText, ImageDown, Link2, UploadCloud, XCircle } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { APIError, api } from '../lib/api'
import type { ArchivePreflight, BridgePairing, ImportCreated, Job } from '../lib/types'
import { PageHeader } from '../components/PageHeader'
import { JobRow } from '../components/JobRow'
import { preferredScrollBehavior } from '../lib/motion'
import { readLocalSelection, type LocalSelectionSnapshot } from '../features/library/selection'
import { errorDescription, useFeedback } from '../components/Feedback'

const TOOLKIT_RELEASE_URL = 'https://github.com/Susurrium/PkuHoleToolkit/releases/latest'
const TREEHOLE_WEB_URL = 'https://treehole.pku.edu.cn/web/'

export function ImportsPage() {
  const client = useQueryClient()
	const [params, setParams] = useSearchParams()
	const view = params.get('view') === 'bridge' ? 'bridge' : params.get('view') === 'export' ? 'export' : 'import'
	const exportSelection = view === 'export' && params.get('from') === 'selection' ? readLocalSelection() : undefined
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<ImportCreated | null>(null)
	const reportRef = useRef<HTMLElement>(null)
	const [preview, setPreview] = useState<BridgePairing | null>(null)
	const imports = useQuery({ queryKey: ['import-jobs'], queryFn: api.importJobs, refetchInterval: (query) => Array.isArray(query.state.data) && query.state.data.some((job) => job.status === 'queued' || job.status === 'running' || job.status === 'paused') ? 1_000 : 5_000 })
  const upload = useMutation({
    mutationFn: api.preflightImport,
    onSuccess: (value) => {
		setPreview(value)
		if (value.preflight) setResult({ preflight: value.preflight })
	},
    onError: (error) => {
      const preflight = preflightFromError(error)
      if (preflight) setResult({ preflight })
    },
  })
	const confirm = useMutation({
		mutationFn: () => api.confirmImportPreflight(preview!.token),
		onSuccess: (value) => {
			setPreview(value)
			if (value.preflight) setResult({ preflight: value.preflight, job: value.job })
			client.invalidateQueries({ queryKey: ['jobs'] })
			client.invalidateQueries({ queryKey: ['import-jobs'] })
		},
	})
	const cancel = useMutation({
		mutationFn: () => api.cancelImportPreflight(preview!.token),
		onSuccess: () => {
			setPreview(null)
			setResult(null)
			setFile(null)
			upload.reset()
		},
	})
	const activeImport = useQuery({
		queryKey: ['import-job', result?.job?.id],
		queryFn: () => api.job(result!.job!.id),
		enabled: Boolean(result?.job?.id),
		refetchInterval: (query) => query.state.data && ['completed', 'partial', 'failed', 'cancelled'].includes(query.state.data.status) ? false : 1_000,
	})
	useEffect(() => {
		if (!result) return
		requestAnimationFrame(() => {
			const report = reportRef.current
			if (report && typeof report.scrollIntoView === 'function') {
				report.scrollIntoView({ behavior: preferredScrollBehavior(), block: 'start' })
			}
		})
	}, [result])
  function pick(files?: FileList | null) {
		if (preview?.status === 'awaiting_confirmation') api.cancelImportPreflight(preview.token).catch(() => undefined)
		const next = files?.[0] ?? null
		setFile(next)
		setPreview(null)
		setResult(null)
		upload.reset()
		confirm.reset()
		cancel.reset()
	}
  function drop(event: DragEvent) { event.preventDefault(); pick(event.dataTransfer.files) }
  return <>
	<PageHeader eyebrow="资料迁移" title="导入与导出" description="迁入已有资料，或把本地资料打包迁移和阅读。" />
	<nav className="mb-6 grid grid-cols-3 rounded-xl border border-line bg-white/45 p-1" aria-label="导入与导出方式">
		<TransferTab active={view === 'import'} icon={UploadCloud} label="导入文件" shortLabel="文件" onClick={() => setParams({ view: 'import' })} />
		<TransferTab active={view === 'bridge'} icon={Link2} label="Toolkit 传输" shortLabel="Toolkit" onClick={() => setParams({ view: 'bridge' })} />
		<TransferTab active={view === 'export'} icon={Download} label="导出资料" shortLabel="导出" onClick={() => setParams({ view: 'export' })} />
	</nav>
	{view === 'import' && <><div className="grid gap-6 xl:grid-cols-[.9fr_1.1fr]">
      <section ref={reportRef} className="panel scroll-mt-24 p-5 md:p-7">
        <label onDrop={drop} onDragOver={(event) => event.preventDefault()} className="flex min-h-64 cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed border-line bg-paper/50 p-7 text-center transition hover:border-teal hover:bg-teal-soft/20">
          <input className="sr-only" type="file" accept=".json,.zip,.treehole.zip,application/json,application/zip" onChange={(event: ChangeEvent<HTMLInputElement>) => pick(event.target.files)} />
          <div className="grid size-14 place-items-center rounded-2xl bg-teal-soft text-teal"><UploadCloud size={25} /></div>
		  <p className="mt-5 font-semibold">拖入 Studio 迁移包，或点击选择文件</p><p className="mt-2 text-xs leading-5 text-ink-soft">也兼容旧版 Studio 和 Toolkit 导出的文件。</p>
        </label>
		<details className="mt-3 text-[11px] text-ink-soft"><summary className="cursor-pointer">支持的文件与大小限制</summary><p className="mt-1">JSON、ZIP 或 .treehole.zip；单文件上限 200 MB，解压后上限 500 MB。</p></details>
        {file && <div className="mt-4 flex items-center gap-3 rounded-xl border border-line bg-white/55 p-4"><FileArchive className="text-coral" size={20} /><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold">{file.name}</p><p className="mt-0.5 text-xs text-ink-soft">{(file.size / 1024 / 1024).toFixed(2)} MB</p></div></div>}
		<button className="button-primary mt-4 w-full" disabled={!file || upload.isPending || preview?.status === 'awaiting_confirmation'} onClick={() => file && upload.mutate(file)}>{upload.isPending ? '正在检查文件…' : '检查文件'}</button>
		{upload.error && <div className="mt-4"><FriendlyError title="无法检查这个文件" description="请确认它来自 Studio 或 Toolkit，且文件没有损坏。" error={upload.error} /></div>}
      </section>
      <section className="panel p-5 md:p-7">
		<h2 className="text-xl font-semibold">文件检查与导入</h2>
        {!result ? <div className="mt-6 grid min-h-64 place-items-center rounded-2xl border border-dashed border-line text-center"><div><FileArchive className="mx-auto text-ink-soft/50" /><p className="mt-3 text-sm text-ink-soft">选择文件后，这里会显示格式、记录数量和异常项。</p></div></div> : <div className="mt-5">
		  <PreflightBanner preflight={result.preflight} queued={Boolean(result.job)} />
		  <PreflightCounts preflight={result.preflight} />
		  {result.preflight.issues.length > 0 && <div className="mt-4 max-h-56 space-y-2 overflow-auto">{result.preflight.issues.map((issue, index) => <div key={`${issue.code}-${index}`} className="rounded-lg border border-coral/20 bg-coral-soft/30 p-3 text-xs"><p className="leading-5 text-ink">{issue.message}</p><details className="mt-1 text-[10px] text-coral"><summary className="cursor-pointer">问题代码</summary><p className="mt-1 font-mono">{issue.code}</p></details></div>)}</div>}
		  {preview?.status === 'awaiting_confirmation' && <div className="mt-5 rounded-xl border border-line bg-white/55 p-4">
			<p className="text-sm font-semibold">{result.preflight.duplicate ? '这个迁移包已经成功导入过，无需重复导入。' : '文件检查完成，请确认是否写入本地资料库。'}</p>
			<p className="mt-1 text-xs leading-5 text-ink-soft">确认前不会写入帖子、评论或图片；关闭后会删除暂存文件。</p>
			<div className="mt-3 flex flex-wrap gap-2">{!result.preflight.duplicate && result.preflight.status === 'completed' && (result.preflight.counts.valid_items ?? 0) > 0 && <button className="button-primary" disabled={confirm.isPending || cancel.isPending} onClick={() => confirm.mutate()}>{confirm.isPending ? '正在创建任务…' : '确认导入'}</button>}<button className="button-secondary" disabled={confirm.isPending || cancel.isPending} onClick={() => cancel.mutate()}>{cancel.isPending ? '正在删除…' : '关闭并删除暂存文件'}</button></div>
			{(confirm.error || cancel.error) && <div className="mt-3"><FriendlyError title="未能完成操作" description="请稍后重试；确认前不会写入本地资料。" error={confirm.error || cancel.error} /></div>}
		  </div>}
          {result.job && <div className="mt-5"><JobRow job={activeImport.data ?? result.job} /></div>}
        </div>}
      </section>
    </div>
	<ImportHistoryPanel jobs={imports.data ?? []} loading={imports.isLoading} error={imports.error} /></>}
	{view === 'bridge' && <ToolkitBridgePanel />}
	{view === 'export' && <ExportPanel initialSelection={exportSelection} />}
  </>
}

function TransferTab({ active, icon: Icon, label, shortLabel, onClick }: { active: boolean; icon: typeof UploadCloud; label: string; shortLabel: string; onClick: () => void }) {
	return <button type="button" className={`flex min-h-11 items-center justify-center gap-2 rounded-lg px-2 text-xs font-semibold sm:text-sm ${active ? 'bg-ink text-white shadow-sm' : 'text-ink-soft hover:bg-white/70 hover:text-ink'}`} aria-label={label} aria-current={active ? 'page' : undefined} onClick={onClick}><Icon size={16} /><span className="sm:hidden">{shortLabel}</span><span className="hidden sm:inline">{label}</span></button>
}

function PreflightCounts({ preflight }: { preflight: ArchivePreflight }) {
	return <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">{Object.entries(preflight.counts).filter(([, value]) => typeof value === 'number').map(([key, value]) => <div key={key} className="rounded-xl border border-line bg-white/50 p-3"><p className="text-xl font-semibold">{value}</p><p className="mt-1 text-[11px] text-ink-soft">{countLabel(key)}</p></div>)}</div>
}

function ImportHistoryPanel({ jobs, loading, error }: { jobs: Job[]; loading: boolean; error: unknown }) {
	return <section className="panel mt-7 p-5 md:p-7"><div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-xl font-semibold">导入历史</h2><span className="badge">{jobs.length} 条</span></div>{loading ? <p className="mt-5 text-sm text-ink-soft">正在读取导入历史…</p> : error ? <div className="mt-5"><FriendlyError title="暂时无法读取导入历史" description="稍后刷新页面即可重试。" error={error} /></div> : <div className="mt-5 grid gap-3">{jobs.length ? jobs.map((job) => { const report = preflightFromCheckpoint(job.checkpoint); return <div key={job.id} className="rounded-xl border border-line bg-white/45 p-3"><JobRow job={job} />{report && <details className="mt-3 rounded-xl border border-line bg-paper/40 p-3"><summary className="cursor-pointer text-sm font-semibold">查看最终导入报告{report.status === 'duplicate' ? ' · 重复迁移包' : ''}</summary><PreflightCounts preflight={report} />{report.issues.length > 0 && <div className="mt-3 space-y-2">{report.issues.map((issue, index) => <div key={`${issue.code}-${index}`} className="text-xs text-coral"><p>{issue.message}</p><details className="mt-1 text-[10px] text-ink-soft"><summary className="cursor-pointer">问题代码</summary><p className="mt-1 font-mono">{issue.code}</p></details></div>)}</div>}</details>}</div> }) : <p className="rounded-xl border border-dashed border-line p-6 text-center text-sm text-ink-soft">还没有导入任务。</p>}</div>}</section>
}

function countLabel(key: string) {
	const labels: Record<string, string> = { items: '记录总数', valid_items: '可导入记录', skipped_items: '跳过记录', holes: '帖子', posts: '帖子', comments: '评论', media: '图片', tags: '标签', notes: '笔记' }
	return labels[key] ?? key.replaceAll('_', ' ')
}

function preflightFromCheckpoint(checkpoint: unknown): ArchivePreflight | undefined {
	if (!checkpoint || typeof checkpoint !== 'object') return undefined
	const value = checkpoint as Partial<ArchivePreflight>
	if (typeof value.format !== 'string' || typeof value.status !== 'string' || !value.counts || !Array.isArray(value.issues)) return undefined
	return value as ArchivePreflight
}

function ToolkitBridgePanel() {
  const client = useQueryClient()
  const deviceRequests = useQuery({ queryKey: ['bridge-device-requests'], queryFn: api.bridgeDeviceRequests, refetchInterval: 1_500 })
  const devices = useQuery({ queryKey: ['bridge-devices'], queryFn: api.bridgeDevices, refetchInterval: 5_000 })
  const transfers = useQuery({ queryKey: ['bridge-transfers'], queryFn: api.bridgeTransfers, refetchInterval: 1_500 })
  const approveDevice = useMutation({ mutationFn: api.approveBridgeDeviceRequest, onSuccess: () => { deviceRequests.refetch(); devices.refetch() } })
  const rejectDevice = useMutation({ mutationFn: api.rejectBridgeDeviceRequest, onSuccess: () => deviceRequests.refetch() })
  const revokeDevice = useMutation({ mutationFn: api.revokeBridgeDevice, onSuccess: () => { devices.refetch(); transfers.refetch() } })
  const confirmTransfer = useMutation({ mutationFn: api.confirmBridgeTransfer, onSuccess: () => { transfers.refetch(); client.invalidateQueries({ queryKey: ['jobs'] }); client.invalidateQueries({ queryKey: ['import-jobs'] }) } })
  const cancelTransfer = useMutation({ mutationFn: api.cancelBridgeTransfer, onSuccess: () => transfers.refetch() })
  const [token, setToken] = useState('')
  const pairing = useMutation({ mutationFn: api.createBridgePairing, onSuccess: (value) => setToken(value.token) })
  const status = useQuery({
    queryKey: ['bridge-pairing', token],
    queryFn: () => api.bridgePairing(token),
    enabled: Boolean(token),
    refetchInterval: (query) => query.state.data?.status === 'awaiting_confirmation' || query.state.data?.status === 'queued' ? false : 1_500,
    retry: false,
  })
  const confirm = useMutation({ mutationFn: () => api.confirmBridgePairing(token), onSuccess: (value) => { client.setQueryData(['bridge-pairing', token], value); client.invalidateQueries({ queryKey: ['jobs'] }) } })
  const cancel = useMutation({ mutationFn: () => api.cancelBridgePairing(token), onSuccess: () => { setToken(''); pairing.reset(); client.removeQueries({ queryKey: ['bridge-pairing', token] }) } })
  const current = status.data ?? pairing.data
  const importedJob = useQuery({
    queryKey: ['job', current?.job?.id],
    queryFn: () => api.job(current!.job!.id),
    enabled: Boolean(current?.job?.id),
    refetchInterval: (query) => query.state.data && ['completed', 'partial', 'failed', 'cancelled'].includes(query.state.data.status) ? false : 1_000,
  })
  const waiting = current?.status === 'waiting_upload' || current?.status === 'uploading'
  return <section className="panel mb-6 p-5 md:p-7">
    <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
	  <div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><Link2 size={20} /></div><div><h2 className="text-xl font-semibold">从 Toolkit 传输</h2><p className="mt-2 max-w-2xl text-sm leading-6 text-ink-soft">如果你还在使用独立 Toolkit，可以先在官方树洞中点击“关联本机 Studio”，核对六位码后直接发送资料。</p><details className="mt-2 max-w-2xl text-xs leading-5 text-ink-soft"><summary className="cursor-pointer font-medium">传输与安全说明</summary><p className="mt-1">Toolkit 可以独立导出文件，Studio 不依赖它运行。关联只保存设备公钥，每次传输使用短期凭证，不会传输树洞登录信息。</p></details></div></div>
      <div className="flex shrink-0 flex-wrap gap-2"><a className="button-secondary" href={TOOLKIT_RELEASE_URL} target="_blank" rel="noreferrer"><ExternalLink size={15} />安装 Toolkit</a><a className="button-primary" href={TREEHOLE_WEB_URL} target="_blank" rel="noreferrer"><ExternalLink size={15} />前往官方树洞</a></div>
    </div>
    <div className="mt-5 grid gap-4 lg:grid-cols-2">
      <div className="rounded-2xl border border-line bg-white/55 p-4"><div className="flex items-center justify-between gap-3"><div><p className="text-xs font-medium text-ink-soft">等待确认的关联</p><p className="mt-1 text-sm text-ink-soft">只确认你刚在 Toolkit 发起、且核对码一致的请求。</p></div><span className="badge">{deviceRequests.data?.length ?? 0}</span></div><div className="mt-4 grid gap-3">{deviceRequests.data?.length ? deviceRequests.data.map((item) => <div key={item.token} className="rounded-xl border border-line bg-paper/55 p-3"><div className="flex flex-wrap items-center justify-between gap-2"><div><p className="font-semibold">{item.name}</p><p className="mt-1 text-xs text-ink-soft">有效至 {new Date(item.expires_at).toLocaleTimeString()}</p></div><code className="rounded-lg bg-ink px-3 py-2 text-base tracking-[0.2em] text-white">{item.verification_code}</code></div><div className="mt-3 flex gap-2"><button className="button-primary !py-1.5" disabled={approveDevice.isPending} onClick={() => approveDevice.mutate(item.token)}>码一致，确认关联</button><button className="button-secondary !py-1.5" disabled={rejectDevice.isPending} onClick={() => rejectDevice.mutate(item.token)}>拒绝</button></div></div>) : <p className="rounded-xl border border-dashed border-line p-4 text-center text-sm text-ink-soft">请先从 Toolkit 发起关联，本页会自动出现请求。</p>}</div></div>
      <div className="rounded-2xl border border-line bg-white/55 p-4"><div className="flex items-center justify-between gap-3"><div><p className="text-xs font-medium text-ink-soft">已关联 Toolkit</p><p className="mt-1 text-sm text-ink-soft">撤销后，该 Toolkit 必须重新关联才能发送。</p></div><span className="badge">{devices.data?.length ?? 0}</span></div><div className="mt-4 grid gap-3">{devices.data?.length ? devices.data.map((device) => <div key={device.id} className="flex items-center justify-between gap-3 rounded-xl border border-line bg-paper/55 p-3"><div className="min-w-0"><p className="truncate font-semibold">{device.name}</p><p className="mt-1 text-xs text-ink-soft">关联于 {new Date(device.created_at).toLocaleString()}{device.last_used_at ? ` · 最近使用 ${new Date(device.last_used_at).toLocaleString()}` : ''}</p></div><button className="button-secondary shrink-0 !py-1.5" disabled={revokeDevice.isPending} onClick={() => revokeDevice.mutate(device.id)}>撤销</button></div>) : <p className="rounded-xl border border-dashed border-line p-4 text-center text-sm text-ink-soft">还没有已关联设备。</p>}</div></div>
    </div>
	<div className="mt-5 rounded-2xl border border-line bg-white/55 p-4"><div className="flex items-center justify-between gap-3"><div><p className="text-xs font-medium text-ink-soft">最近传输</p><p className="mt-1 text-sm text-ink-soft">收到文件后会先检查，只有经你确认才会导入。</p></div><span className="badge">{transfers.data?.length ?? 0}</span></div><div className="mt-4 grid gap-3">{transfers.data?.length ? transfers.data.map((transfer) => <div key={transfer.id} className="rounded-xl border border-line bg-paper/55 p-4"><div className="flex flex-wrap items-start justify-between gap-3"><div className="min-w-0"><p className="truncate font-semibold">{transfer.filename}</p><p className="mt-1 text-xs text-ink-soft">{transfer.device_name} · {(transfer.size / 1024 / 1024).toFixed(2)} MB · {bridgeStatusLabel(transfer.status)}</p></div><details className="max-w-40 text-[10px] text-ink-soft"><summary className="cursor-pointer">文件校验</summary><code className="mt-1 block truncate" title={transfer.sha256}>{transfer.sha256}</code></details></div>{transfer.status === 'awaiting_confirmation' && transfer.preflight && <div className="mt-4"><p className="text-sm font-semibold text-teal">文件检查通过，等待确认导入。</p><PreflightCounts preflight={transfer.preflight} /><div className="mt-3 flex gap-2"><button className="button-primary" disabled={confirmTransfer.isPending} onClick={() => confirmTransfer.mutate(transfer.id)}>确认导入</button><button className="button-secondary" disabled={cancelTransfer.isPending} onClick={() => cancelTransfer.mutate(transfer.id)}>取消并删除</button></div></div>}{transfer.status === 'queued' && transfer.job && <div className="mt-3"><JobRow job={transfer.job} /></div>}</div>) : <p className="rounded-xl border border-dashed border-line p-5 text-center text-sm text-ink-soft">还没有来自已关联 Toolkit 的传输。</p>}</div></div>
	<details className="mt-5 rounded-2xl border border-line bg-white/45 p-4"><summary className="cursor-pointer text-sm font-semibold">兼容旧版 Toolkit：一次性接收码</summary><p className="mt-3 text-sm leading-6 text-ink-soft">请先在 Toolkit 完成导出，再生成接收码。接收码等待上传 15 分钟；文件收到后另有 30 分钟用于核对确认。</p>{!current && <button className="button-secondary mt-3" disabled={pairing.isPending} onClick={() => pairing.mutate()}>{pairing.isPending ? '正在生成…' : '生成一次性接收码'}</button>}{current && <div className="mt-4"><p className="text-xs font-medium text-ink-soft">一次性接收码</p><div className="mt-2 flex flex-col gap-2 sm:flex-row"><code className="min-w-0 flex-1 break-all rounded-xl bg-ink px-4 py-3 text-sm text-white">{current.code ?? `${location.port}:${current.token}`}</code><button className="button-secondary shrink-0" onClick={() => navigator.clipboard.writeText(current.code ?? `${location.port}:${current.token}`)}><Copy size={15} />复制</button></div>{waiting && <p className="mt-3 text-sm text-ink-soft">回到已完成导出的 Toolkit，粘贴接收码并发送。本页会自动显示检查结果。</p>}{current.status === 'awaiting_confirmation' && current.preflight && <div className="mt-4"><PreflightBanner preflight={current.preflight} queued={false} /><p className="mt-3 text-sm text-ink-soft">已收到 <strong>{current.filename}</strong>。核对后确认，才会创建本地导入任务。</p><div className="mt-3 flex gap-2"><button className="button-primary" disabled={confirm.isPending} onClick={() => confirm.mutate()}>{confirm.isPending ? '正在创建任务…' : '确认导入'}</button><button className="button-secondary" disabled={cancel.isPending} onClick={() => cancel.mutate()}>取消并删除暂存文件</button></div></div>}{current.status === 'queued' && <div className="mt-4"><p className="text-sm font-semibold text-teal">资料已进入本地导入队列。</p>{current.job && <div className="mt-3"><JobRow job={importedJob.data ?? current.job} /></div>}</div>}</div>}</details>
	{(deviceRequests.error || devices.error || transfers.error || approveDevice.error || rejectDevice.error || revokeDevice.error || confirmTransfer.error || cancelTransfer.error || pairing.error || status.error || confirm.error || cancel.error) && <div className="mt-4"><FriendlyError title="Toolkit 传输暂时不可用" description="请检查 Toolkit 是否仍在运行和关联，然后重试。" error={deviceRequests.error || devices.error || transfers.error || approveDevice.error || rejectDevice.error || revokeDevice.error || confirmTransfer.error || cancelTransfer.error || pairing.error || status.error || confirm.error || cancel.error} /></div>}
  </section>
}

function bridgeStatusLabel(status: string) {
  return status === 'waiting_upload' ? '等待上传' : status === 'uploading' ? '正在接收' : status === 'awaiting_confirmation' ? '等待确认' : status === 'queued' ? '已进入导入队列' : status
}

function ExportPanel({ initialSelection }: { initialSelection?: LocalSelectionSnapshot }) {
  const client = useQueryClient()
  const { notify } = useFeedback()
  const [pidsValue, setPidsValue] = useState(() => initialSelection?.pids.join(', ') ?? '')
  const [includeComments, setIncludeComments] = useState(true)
  const [captureLive, setCaptureLive] = useState(false)
  const [includeMedia, setIncludeMedia] = useState(true)
  const selectedPIDs = parseExportPIDs(pidsValue)
  const deferredPIDsValue = useDeferredValue(pidsValue)
  const previewPIDs = parseExportPIDs(deferredPIDsValue)
  const previewCurrent = deferredPIDsValue === pidsValue
  const preview = useQuery({
    queryKey: ['export-preview', previewPIDs.join(','), includeComments],
    queryFn: () => api.exportPreview(previewPIDs, includeComments),
    enabled: previewPIDs.length <= 2_000,
    staleTime: 5_000,
  })
  const exports = useQuery({ queryKey: ['export-jobs'], queryFn: api.exportJobs, refetchInterval: (query) => Array.isArray(query.state.data) && query.state.data.some((job) => job.status === 'queued' || job.status === 'running') ? 1_000 : 5_000 })
  const exportRows = Array.isArray(exports.data) ? exports.data : []
	const create = useMutation({ mutationFn: ({ format, syncObserverBeforeExport }: { format: 'treehole-v2' | 'markdown'; syncObserverBeforeExport?: boolean }) => api.createExportJob(format, selectedPIDs, includeComments, captureLive, includeMedia, syncObserverBeforeExport), onSuccess: () => exports.refetch() })
	const regenerate = useMutation({ mutationFn: api.regenerateExportJob, onSuccess: () => exports.refetch() })
	const retry = useMutation({ mutationFn: (id: string) => api.jobAction(id, 'retry'), onSuccess: () => exports.refetch() })
  const repair = useMutation({
    mutationFn: () => api.createJob('repair_media', {}),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['jobs'] })
      notify({ title: '图片补全任务已创建', description: '任务完成后回到导出历史重新生成文件，即可带入已补全图片。' })
    },
    onError: (error) => notify({ tone: 'error', title: '无法创建图片补全任务', description: errorDescription(error) }),
  })
  const download = useMutation({
    mutationFn: api.downloadExportJob,
    onSuccess: ({ blob, filename }) => {
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = filename
      anchor.click()
      URL.revokeObjectURL(url)
    },
  })
  const noLocalMatches = !captureLive && previewCurrent && preview.data?.posts === 0
  const previewRequested = preview.data?.requested ?? selectedPIDs.length
  const previewFound = preview.data?.found ?? preview.data?.posts ?? 0
  const previewMissing = preview.data?.missing ?? Math.max(0, previewRequested - previewFound)
  const previewMissingPIDs = preview.data?.missing_pids ?? []
  const createDisabled = create.isPending || selectedPIDs.length > 2_000 || (captureLive && selectedPIDs.length === 0) || noLocalMatches

  return <section className="panel mt-7 p-5 md:p-7">
	<div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-coral-soft text-coral"><Download size={20} /></div><div><h2 className="text-xl font-semibold">打包本地资料</h2><p className="mt-2 text-sm leading-6 text-ink-soft">留空 PID 会导出全部本地资料；填写 PID 则只导出选中的帖子。任务完成后再下载文件。</p></div></div>
    {initialSelection && <div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-teal/25 bg-teal-soft/35 px-4 py-3"><div><p className="text-sm font-semibold text-teal">已带入 {initialSelection.pids.length} 个树洞</p><p className="mt-1 text-xs text-ink-soft">可以在下方继续调整范围；返回来源页面时仍会保留当前选择。</p></div><Link className="button-secondary !min-h-9 !px-3 text-xs" to={initialSelection.returnTo}><ArrowLeft size={14} />返回来源页面</Link></div>}
    <div className="mt-6 grid gap-5 lg:grid-cols-[1fr_340px]">
      <div><label className="text-xs font-medium text-ink-soft">导出范围<textarea className="field mt-1.5 min-h-24 resize-y" value={pidsValue} onChange={(event) => setPidsValue(event.target.value)} placeholder="留空导出全部；或输入 1234567, 2345678" /></label><p className={`mt-2 text-xs ${selectedPIDs.length > 2000 ? 'text-coral' : 'text-ink-soft'}`}>{selectedPIDs.length ? `已选择 ${selectedPIDs.length} 个 PID` : '当前将导出全部本地帖子'}{selectedPIDs.length > 2000 ? '，超过 2000 个上限' : ''}</p><label className="mt-4 inline-flex items-center gap-2 text-sm text-ink-soft"><input type="checkbox" checked={includeComments} onChange={(event) => setIncludeComments(event.target.checked)} />包含评论</label><details className="mt-4 rounded-xl border border-line bg-white/45 p-4"><summary className="cursor-pointer text-sm font-semibold">导出前需要在线更新？</summary><p className="mt-2 text-xs leading-5 text-ink-soft">仅适用于指定 PID。Studio 会先从树洞获取最新正文、评论和图片，再生成文件。</p><label className="mt-3 flex items-center gap-2 text-sm text-ink-soft"><input type="checkbox" checked={captureLive} onChange={(event) => setCaptureLive(event.target.checked)} aria-label="导出前在线更新指定 PID" />先在线更新指定 PID</label><label className="mt-3 flex items-center gap-2 text-sm text-ink-soft"><input type="checkbox" disabled={!captureLive} checked={includeMedia} onChange={(event) => setIncludeMedia(event.target.checked)} />下载并打包图片</label>{captureLive && selectedPIDs.length === 0 && <p className="mt-2 text-xs text-coral">在线更新时必须填写至少一个 PID。</p>}</details></div>
      <div className="rounded-2xl border border-line bg-paper/55 p-5">
        <p className="text-xs font-medium text-ink-soft">预计导出范围</p>
        {preview.isLoading ? <p className="mt-3 text-sm text-ink-soft">正在统计本地资料…</p> : preview.data ? <><div className="mt-3 grid grid-cols-2 gap-2">{[['帖子', preview.data.posts], ['评论', preview.data.comments], ['图片', preview.data.media], ['缺失图片', preview.data.missing_media]].map(([label, value]) => <div key={String(label)} className={`rounded-xl border p-3 ${label === '缺失图片' && Number(value) > 0 ? 'border-coral/25 bg-coral-soft/35' : 'border-line bg-white/60'}`}><p className="text-lg font-semibold">{value}</p><p className="mt-0.5 text-[11px] text-ink-soft">{label}</p></div>)}</div>{previewCurrent && selectedPIDs.length > 0 && <div className={`mt-3 rounded-lg px-3 py-2 text-xs ${previewMissing > 0 ? 'bg-coral-soft/45 text-coral' : 'bg-teal-soft/45 text-teal'}`}><p>指定范围：请求 {previewRequested} · 找到 {previewFound} · 缺失 {previewMissing}</p>{previewMissingPIDs.length > 0 && <p className="mt-1 break-words text-[11px] leading-5">未找到 PID：{previewMissingPIDs.map((pid) => `#${pid}`).join('、')}</p>}</div>}{!previewCurrent && <p className="mt-3 text-xs text-ink-soft">正在按新的 PID 范围更新预估…</p>}{captureLive && <p className="mt-3 text-xs leading-5 text-ink-soft">这是当前本地资料的预估；在线更新完成后数量可能增加。</p>}{previewCurrent && preview.data.posts === 0 && <p className="mt-3 text-xs text-coral">当前范围没有匹配到本地帖子。</p>}{previewCurrent && preview.data.missing_media > 0 && <button className="button-secondary mt-3 w-full !text-coral" disabled={repair.isPending} onClick={() => repair.mutate()}><ImageDown size={15} />{repair.isPending ? '正在创建任务…' : '先补全缺失图片'}</button>}{repair.data && <Link className="button-secondary mt-2 w-full" to="/tasks">查看补全进度</Link>}</> : <p className="mt-3 text-xs leading-5 text-coral">暂时无法预估：{errorDescription(preview.error)}</p>}
        <div className="my-5 border-t border-line" />
		<p className="text-xs font-medium text-ink-soft">选择用途</p><div className="mt-3 grid gap-3"><div className="rounded-xl bg-white/60 p-3"><p className="font-semibold">Studio 迁移包</p><p className="mt-1 text-xs leading-5 text-ink-soft">用于迁移到另一台 Studio，之后也可以再次导入。</p></div><button className="button-primary" disabled={createDisabled} onClick={() => create.mutate({ format: 'treehole-v2' })}><FileArchive size={16} />创建 Studio 迁移包</button><div className="mt-1 rounded-xl bg-white/60 p-3"><p className="font-semibold">Markdown 阅读文件</p><p className="mt-1 text-xs leading-5 text-ink-soft">用于阅读、整理或导入其他笔记工具。</p></div><button className="button-secondary" disabled={createDisabled} onClick={() => create.mutate({ format: 'markdown' })}><FileText size={16} />导出为 Markdown</button></div><details className="mt-4 text-[11px] leading-5 text-ink-soft"><summary className="cursor-pointer font-medium">格式与同步说明</summary><p className="mt-1">Studio 迁移包使用 Archive v2 格式。如启用了“导出前先同步”，系统会先更新删除记录；更新失败时会让你选择重试或使用当前本地资料。</p></details>
      </div>
    </div>
	{create.isPending && <p className="mt-4 text-sm text-ink-soft">正在准备导出任务…</p>}
    {create.data && <p className="mt-4 rounded-xl bg-teal-soft/45 px-4 py-3 text-sm font-medium text-teal">导出任务已创建，完成后可在下方下载。</p>}
	{observerExportFailure(create.error) && <div className="mt-4 rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="alert"><p className="font-semibold text-coral">无法更新最新删除记录</p><p className="mt-1 text-sm leading-6 text-ink-soft">导出尚未开始。你可以重试更新，或明确使用当前本地资料继续导出。</p><details className="mt-2 text-xs text-ink-soft"><summary className="cursor-pointer">查看技术详情</summary><p className="mt-1 break-all font-mono">{errorDescription(create.error)}</p></details><div className="mt-3 flex flex-wrap gap-2"><button className="button-primary" disabled={create.isPending} onClick={() => create.mutate({ format: create.variables?.format ?? 'treehole-v2' })}>重试更新并导出</button><button className="button-secondary" disabled={create.isPending} onClick={() => create.mutate({ format: create.variables?.format ?? 'treehole-v2', syncObserverBeforeExport: false })}>使用当前本地资料继续</button></div></div>}
	{((create.error && !observerExportFailure(create.error)) || regenerate.error || retry.error || download.error || exports.error) && <div className="mt-4"><FriendlyError title="导出操作没有完成" description="请稍后重试；已经生成的文件和本地资料不会丢失。" error={(create.error && !observerExportFailure(create.error) ? create.error : undefined) || regenerate.error || retry.error || download.error || exports.error} /></div>}
	<div className="mt-6 border-t border-line pt-5"><div className="mb-3 flex items-center justify-between"><h3 className="font-semibold">导出历史</h3><span className="text-xs text-ink-soft">完成文件保留 30 天</span></div><div className="grid gap-3">{exportRows.length ? exportRows.map((job) => { const report = exportReportFromCheckpoint(job.checkpoint); return <div key={job.id} className="rounded-xl border border-line bg-white/45 p-3"><JobRow job={job} />{report && <ExportReportBanner report={report} />}<div className="mt-3 flex flex-wrap justify-end gap-2">{report && report.missing_media > 0 && <button className="button-secondary !py-1.5 !text-coral" disabled={repair.isPending} onClick={() => repair.mutate()}><ImageDown size={14} />补全缺失图片</button>}{job.status === 'completed' && <><button className="button-secondary !py-1.5" disabled={regenerate.isPending} onClick={() => regenerate.mutate(job.id)}>重新生成</button><button className="button-primary !py-1.5" disabled={download.isPending} onClick={() => download.mutate(job.id)}><Download size={14} />下载</button></>}{(job.status === 'failed' || job.status === 'cancelled' || job.status === 'partial') && <button className="button-secondary !py-1.5" disabled={retry.isPending} onClick={() => retry.mutate(job.id)}>重试</button>}</div></div> }) : <p className="rounded-xl border border-dashed border-line p-5 text-center text-sm text-ink-soft">还没有导出任务。</p>}</div></div>
  </section>
}

function parseExportPIDs(value: string) {
  return [...new Set(value.split(/[\s,，]+/).map(Number).filter((item) => Number.isInteger(item) && item > 0))]
}

type ExportReportSummary = { posts: number; requested: number; found: number; missing: number; missing_pids: number[]; comments: number; media: number; missing_media: number }

function exportReportFromCheckpoint(checkpoint: unknown): ExportReportSummary | undefined {
	if (!checkpoint || typeof checkpoint !== 'object') return undefined
	const report = (checkpoint as { report?: unknown }).report
	if (!report || typeof report !== 'object') return undefined
	const value = report as Record<string, unknown>
	if (typeof value.posts !== 'number' || typeof value.comments !== 'number' || typeof value.media !== 'number' || typeof value.missing_media !== 'number') return undefined
	const missingPIDs = Array.isArray(value.missing_pids) ? value.missing_pids.filter((pid): pid is number => typeof pid === 'number' && Number.isInteger(pid) && pid > 0) : []
	return {
		posts: value.posts,
		requested: typeof value.requested === 'number' ? value.requested : value.posts,
		found: typeof value.found === 'number' ? value.found : value.posts,
		missing: typeof value.missing === 'number' ? value.missing : missingPIDs.length,
		missing_pids: missingPIDs,
		comments: value.comments,
		media: value.media,
		missing_media: value.missing_media,
	}
}

function ExportReportBanner({ report }: { report: ExportReportSummary }) {
	const warning = report.missing > 0 || report.missing_media > 0
	return <div className={`mt-3 rounded-lg px-3 py-2 text-xs ${warning ? 'bg-coral-soft/45 text-coral' : 'bg-teal-soft/45 text-teal'}`}><p>导出结果：{report.posts} 帖 · {report.comments} 评论 · {report.media} 图片 · {report.missing_media} 图片缺失</p>{report.missing > 0 && <><p className="mt-1 text-[11px] leading-5">指定范围：请求 {report.requested} · 找到 {report.found} · 缺失 {report.missing}。文件已按实际结果生成，并标记为不完整。</p>{report.missing_pids.length > 0 && <p className="mt-1 break-words text-[11px] leading-5">未找到 PID：{report.missing_pids.map((pid) => `#${pid}`).join('、')}</p>}</>}{report.missing_media > 0 && <p className="mt-1 text-[11px] leading-5">文件仍可下载；如需完整图片，请先创建补全任务，完成后再重新生成。</p>}</div>
}

function preflightFromError(error: unknown): ArchivePreflight | undefined {
  if (!(error instanceof APIError) || error.code !== 'archive_no_valid_items' || !error.details || typeof error.details !== 'object') return undefined
  const preflight = (error.details as { preflight?: ArchivePreflight }).preflight
  return preflight && typeof preflight === 'object' ? preflight : undefined
}

function PreflightBanner({ preflight, queued = true }: { preflight: ArchivePreflight; queued?: boolean }) {
	const duplicate = preflight.duplicate === true
	const accepted = !duplicate && preflight.status === 'completed' && (preflight.counts.valid_items ?? 0) > 0
	return <div className={`flex items-start gap-3 rounded-xl p-4 ${accepted ? 'bg-teal-soft/55' : duplicate ? 'border border-line bg-white/55' : 'border border-coral/20 bg-coral-soft/35'}`}>
		{accepted ? <CheckCircle2 className="mt-0.5 shrink-0 text-teal" size={19} /> : duplicate ? <FileArchive className="mt-0.5 shrink-0 text-ink-soft" size={19} /> : <XCircle className="mt-0.5 shrink-0 text-coral" size={19} />}
		<div><p className={`text-sm font-semibold ${!accepted && !duplicate ? 'text-coral' : ''}`}>{duplicate ? '这个迁移包已经导入过' : accepted ? '文件检查通过' : '文件检查未通过'}</p>{accepted && <p className="mt-1 text-xs text-ink-soft">{queued ? '已创建后台导入任务；中断后请重新选择文件。' : '等待你确认后开始导入。'}</p>}{duplicate && <p className="mt-2 text-xs text-ink-soft">内容与之前成功导入的文件相同，因此不会重复写入。</p>}{!accepted && !duplicate && <p className="mt-2 text-xs text-ink-soft">没有找到可导入的有效帖子，因此没有创建任务。请查看下方问题。</p>}<details className="mt-2 text-[10px] text-ink-soft"><summary className="cursor-pointer">文件格式与校验信息</summary><p className="mt-1">识别格式：{formatLabel(preflight.format)}</p><p className="mt-1 break-all font-mono">SHA-256 {preflight.hash}</p></details></div>
  </div>
}

function observerExportFailure(error: unknown) {
	if (!(error instanceof APIError) || error.code !== 'observer_sync_failed' || !error.details || typeof error.details !== 'object') return false
	return (error.details as { can_export_local_snapshot?: unknown }).can_export_local_snapshot === true
}

function formatLabel(value: string) {
	return value === 'v2' || value === 'archive-v2' || value === 'treehole-v2' ? 'Studio 迁移包（Archive v2）' : value === 'legacy-v1' ? '旧版兼容文件（JSON）' : value
}

function FriendlyError({ title, description, error }: { title: string; description: string; error: unknown }) {
	return <div className="rounded-xl border border-coral/30 bg-coral-soft/25 p-4" role="alert"><p className="font-semibold text-coral">{title}</p><p className="mt-1 text-sm leading-6 text-ink-soft">{description}</p><details className="mt-2 text-xs text-ink-soft"><summary className="cursor-pointer">查看技术详情</summary><p className="mt-1 break-all font-mono">{errorDescription(error)}</p></details></div>
}
