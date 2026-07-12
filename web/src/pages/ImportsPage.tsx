import { ChangeEvent, DragEvent, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, Copy, Download, FileArchive, FileText, Link2, UploadCloud, XCircle } from 'lucide-react'
import { APIError, api } from '../lib/api'
import type { ArchivePreflight, ImportCreated } from '../lib/types'
import { PageHeader } from '../components/PageHeader'
import { ErrorState } from '../components/States'
import { JobRow } from '../components/JobRow'

export function ImportsPage() {
  const client = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<ImportCreated | null>(null)
  const upload = useMutation({
    mutationFn: api.importArchive,
    onSuccess: (value) => { setResult(value); client.invalidateQueries({ queryKey: ['jobs'] }) },
    onError: (error) => {
      const preflight = preflightFromError(error)
      if (preflight) setResult({ preflight })
    },
  })
  function pick(files?: FileList | null) { const next = files?.[0] ?? null; setFile(next); setResult(null); upload.reset() }
  function drop(event: DragEvent) { event.preventDefault(); pick(event.dataTransfer.files) }
  return <>
    <PageHeader eyebrow="TOOLKIT BRIDGE" title="归档导入" description="导入旧版 {holes, comments} JSON 或 archive v2 .treehole.zip。预检会先验证结构、PID、数量和完整性，再创建可恢复的持久任务。" />
    <ToolkitBridgePanel />
    <div className="grid gap-6 xl:grid-cols-[.9fr_1.1fr]">
      <section className="panel p-5 md:p-7">
        <label onDrop={drop} onDragOver={(event) => event.preventDefault()} className="flex min-h-64 cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed border-line bg-paper/50 p-7 text-center transition hover:border-teal hover:bg-teal-soft/20">
          <input className="sr-only" type="file" accept=".json,.zip,.treehole.zip,application/json,application/zip" onChange={(event: ChangeEvent<HTMLInputElement>) => pick(event.target.files)} />
          <div className="grid size-14 place-items-center rounded-2xl bg-teal-soft text-teal"><UploadCloud size={25} /></div>
          <p className="mt-5 font-semibold">拖入归档，或点击选择文件</p><p className="mt-2 text-xs leading-5 text-ink-soft">单文件上限 200 MB · 解压内容上限 500 MB</p>
        </label>
        {file && <div className="mt-4 flex items-center gap-3 rounded-xl border border-line bg-white/55 p-4"><FileArchive className="text-coral" size={20} /><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold">{file.name}</p><p className="mt-0.5 text-xs text-ink-soft">{(file.size / 1024 / 1024).toFixed(2)} MB</p></div></div>}
        <button className="button-primary mt-4 w-full" disabled={!file || upload.isPending} onClick={() => file && upload.mutate(file)}>{upload.isPending ? '正在预检并排队…' : '预检并开始导入'}</button>
        {upload.error && <div className="mt-4"><ErrorState error={upload.error} /></div>}
      </section>
      <section className="panel p-5 md:p-7">
        <p className="eyebrow">IMPORT REPORT</p><h2 className="mt-1 text-xl font-semibold">预检与任务报告</h2>
        {!result ? <div className="mt-6 grid min-h-64 place-items-center rounded-2xl border border-dashed border-line text-center"><div><FileArchive className="mx-auto text-ink-soft/50" /><p className="mt-3 text-sm text-ink-soft">选择文件后，这里会显示格式、记录数量和异常项。</p></div></div> : <div className="mt-5">
          <PreflightBanner preflight={result.preflight} />
          <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">{Object.entries(result.preflight.counts).filter(([, value]) => typeof value === 'number').map(([key, value]) => <div key={key} className="rounded-xl border border-line bg-white/50 p-3"><p className="text-xl font-semibold">{value}</p><p className="mt-1 font-mono text-[10px] text-ink-soft">{key}</p></div>)}</div>
          {result.preflight.issues.length > 0 && <div className="mt-4 max-h-56 space-y-2 overflow-auto">{result.preflight.issues.map((issue, index) => <div key={`${issue.code}-${index}`} className="rounded-lg border border-coral/20 bg-coral-soft/30 p-3 text-xs"><p className="font-semibold text-coral">{issue.code}</p><p className="mt-1 leading-5 text-ink-soft">{issue.message}</p></div>)}</div>}
          {result.job && <div className="mt-5"><JobRow job={result.job} /></div>}
        </div>}
      </section>
    </div>
    <ExportPanel />
  </>
}

function ToolkitBridgePanel() {
  const client = useQueryClient()
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
      <div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-teal-soft text-teal"><Link2 size={20} /></div><div><p className="eyebrow">DIRECT BRIDGE</p><h2 className="mt-1 text-xl font-semibold">从已登录的 Toolkit 直接发送</h2><p className="mt-2 max-w-2xl text-sm leading-6 text-ink-soft">Studio 生成一个 5 分钟有效的一次性配对码。把它粘贴到树洞网页里的 Toolkit；Toolkit 只发送刚生成的 archive v2，不发送账号、密码、Cookie 或 token。发送后仍需在这里确认导入。</p></div></div>
      {!current && <button className="button-primary shrink-0" disabled={pairing.isPending} onClick={() => pairing.mutate()}>{pairing.isPending ? '正在生成…' : '生成一次性配对码'}</button>}
    </div>
    {current && <div className="mt-5 rounded-2xl border border-line bg-white/55 p-4">
      <p className="text-xs font-medium text-ink-soft">配对码（仅可使用一次）</p>
      <div className="mt-2 flex flex-col gap-2 sm:flex-row"><code className="min-w-0 flex-1 break-all rounded-xl bg-ink px-4 py-3 text-sm text-white">{current.code ?? `${location.port}:${current.token}`}</code><button className="button-secondary shrink-0" onClick={() => navigator.clipboard.writeText(current.code ?? `${location.port}:${current.token}`)}><Copy size={15} />复制</button></div>
      {waiting && <p className="mt-3 text-sm text-ink-soft">现在打开树洞网页 → Toolkit“归档/迁移” → 完成一次导出 → 粘贴配对码 → “发送到 Studio”。本页会自动显示预检结果。</p>}
      {current.status === 'awaiting_confirmation' && current.preflight && <div className="mt-4"><PreflightBanner preflight={current.preflight} /><p className="mt-3 text-sm text-ink-soft">已收到 <strong>{current.filename}</strong>。核对后确认，才会创建本地导入任务。</p><div className="mt-3 flex gap-2"><button className="button-primary" disabled={confirm.isPending} onClick={() => confirm.mutate()}>{confirm.isPending ? '正在创建任务…' : '确认导入'}</button><button className="button-secondary" disabled={cancel.isPending} onClick={() => cancel.mutate()}>取消并删除暂存文件</button></div></div>}
      {current.status === 'queued' && <div className="mt-4"><p className="text-sm font-semibold text-teal">归档已进入本地导入队列。</p>{current.job && <div className="mt-3"><JobRow job={importedJob.data ?? current.job} /></div>}</div>}
      {(pairing.error || status.error || confirm.error || cancel.error) && <div className="mt-4"><ErrorState error={pairing.error || status.error || confirm.error || cancel.error} /></div>}
    </div>}
  </section>
}

function ExportPanel() {
  const [pidsValue, setPidsValue] = useState('')
  const [includeComments, setIncludeComments] = useState(true)
  const selectedPIDs = [...new Set(pidsValue.split(/[\s,，]+/).map(Number).filter((value) => Number.isInteger(value) && value > 0))]
  const download = useMutation({
    mutationFn: (format: 'treehole-v2' | 'markdown') => api.exportArchive(format, selectedPIDs, includeComments),
    onSuccess: ({ blob, filename }) => {
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = filename
      anchor.click()
      URL.revokeObjectURL(url)
    },
  })
  return <section className="panel mt-7 p-5 md:p-7">
    <div className="flex items-start gap-4"><div className="grid size-11 shrink-0 place-items-center rounded-xl bg-coral-soft text-coral"><Download size={20} /></div><div><p className="eyebrow">STUDIO EXPORT</p><h2 className="mt-1 text-xl font-semibold">从本机资料库导出</h2><p className="mt-2 text-sm leading-6 text-ink-soft">留空 PID 会导出全部本地帖子；也可以只导出指定 PID。archive v2 可重新导入 Studio 或 Toolkit，Markdown 包适合阅读和整理。</p></div></div>
    <div className="mt-5 grid gap-4 lg:grid-cols-[1fr_auto]">
      <label className="text-xs font-medium text-ink-soft">指定 PID（可选，最多 2000 个）<textarea className="field mt-1.5 min-h-20 resize-y" value={pidsValue} onChange={(event) => setPidsValue(event.target.value)} placeholder="留空导出全部；或输入 1234567, 2345678" /></label>
      <div className="flex min-w-64 flex-col justify-end gap-2"><label className="mb-1 inline-flex items-center gap-2 text-sm text-ink-soft"><input type="checkbox" checked={includeComments} onChange={(event) => setIncludeComments(event.target.checked)} />包含评论</label><button className="button-primary" disabled={download.isPending || selectedPIDs.length > 2000} onClick={() => download.mutate('treehole-v2')}><FileArchive size={16} />导出 archive v2</button><button className="button-secondary" disabled={download.isPending || selectedPIDs.length > 2000} onClick={() => download.mutate('markdown')}><FileText size={16} />导出 Markdown 包</button></div>
    </div>
    {download.isPending && <p className="mt-4 text-sm text-ink-soft">正在生成导出文件，请保持页面打开…</p>}
    {download.error && <div className="mt-4"><ErrorState error={download.error} /></div>}
  </section>
}

function preflightFromError(error: unknown): ArchivePreflight | undefined {
  if (!(error instanceof APIError) || error.code !== 'archive_no_valid_items' || !error.details || typeof error.details !== 'object') return undefined
  const preflight = (error.details as { preflight?: ArchivePreflight }).preflight
  return preflight && typeof preflight === 'object' ? preflight : undefined
}

function PreflightBanner({ preflight }: { preflight: ArchivePreflight }) {
  const accepted = preflight.status === 'completed' && (preflight.counts.valid_items ?? 0) > 0
  return <div className={`flex items-start gap-3 rounded-xl p-4 ${accepted ? 'bg-teal-soft/55' : 'border border-coral/20 bg-coral-soft/35'}`}>
    {accepted ? <CheckCircle2 className="mt-0.5 shrink-0 text-teal" size={19} /> : <XCircle className="mt-0.5 shrink-0 text-coral" size={19} />}
    <div><p className={`text-sm font-semibold ${accepted ? '' : 'text-coral'}`}>{accepted ? '预检完成' : '预检未通过'} · {preflight.format}</p><p className="mt-1 font-mono text-[10px] break-all text-ink-soft">SHA-256 {preflight.hash}</p>{!accepted && <p className="mt-2 text-xs text-ink-soft">没有可导入的有效帖子，未创建导入任务。请查看下方错误详情。</p>}</div>
  </div>
}
