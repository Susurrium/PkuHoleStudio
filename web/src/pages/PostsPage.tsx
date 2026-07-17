import { FormEvent, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckSquare2, Download, FileOutput, Filter, FolderPlus, ImagePlus, Search, Send, SlidersHorizontal, Tags, X } from 'lucide-react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { LocalTagFilter, parseLocalTagIDs } from '../components/LocalTagFilter'
import { errorDescription, useFeedback } from '../components/Feedback'
import { OnlineFreshnessBar } from '../components/OnlineFreshnessBar'
import { postDetailTarget, updatePostExplorerParam, usePostsExplorer, usePublishPost } from '../features/posts/usePostsExplorer'
import { api } from '../lib/api'
import { clearLocalSelection, LOCAL_SELECTION_HANDOFF_EVENT, readLocalSelection, writeLocalSelection, type LocalSelectionSnapshot } from '../features/library/selection'

export function PostsPage() {
  const client = useQueryClient()
  const { notify } = useFeedback()
  const [params, setParams] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const explorer = usePostsExplorer(params, { history: true })
  const { source, q, sort, hasMedia, followed, label, online, liveTags: tags, localTags, history, canRead, browserOnline, query, posts: displayed } = explorer
  const localTagIDs = source === 'local' ? parseLocalTagIDs(explorer.localTag) : []
  const [searchDraft, setSearchDraft] = useState(q)
  const [filtersOpen, setFiltersOpen] = useState(false)
  const selectionContext = location.pathname + location.search
  const initialSelection = useRef(source === 'local' ? readLocalSelection(selectionContext)?.pids ?? [] : [])
  const [selectionMode, setSelectionMode] = useState(initialSelection.current.length > 0)
  const [selectedPIDs, setSelectedPIDs] = useState<Set<number>>(() => new Set(initialSelection.current))
  const [batchAction, setBatchAction] = useState<'project' | 'tag' | null>(null)
  const [batchTargetID, setBatchTargetID] = useState('')
  const searchInput = useRef<HTMLInputElement>(null)
  const previousContext = useRef(selectionContext)
  const skipSelectionPersistence = useRef(false)

  useEffect(() => setSearchDraft(q), [q])
  useEffect(() => { if (params.get('focus') === 'search') requestAnimationFrame(() => searchInput.current?.focus()) }, [params])
  useEffect(() => {
    if (previousContext.current === selectionContext) return
	clearLocalSelection(previousContext.current)
    previousContext.current = selectionContext
	skipSelectionPersistence.current = true
    const restored = source === 'local' ? readLocalSelection(selectionContext)?.pids ?? [] : []
    setSelectionMode(restored.length > 0)
    setSelectedPIDs(new Set(restored))
    setBatchAction(null)
    setBatchTargetID('')
  }, [selectionContext, source])
  useEffect(() => {
    if (source !== 'local') return
	if (skipSelectionPersistence.current) {
	  skipSelectionPersistence.current = false
	  return
	}
    if (selectionMode && selectedPIDs.size) writeLocalSelection(selectionContext, selectedPIDs)
    else clearLocalSelection(selectionContext)
  }, [selectedPIDs, selectionContext, selectionMode, source])
  useEffect(() => {
    const acceptHandoff = (event: Event) => {
      const snapshot = (event as CustomEvent<LocalSelectionSnapshot>).detail
      if (source !== 'local' || snapshot?.returnTo !== selectionContext || !snapshot.pids.length) return
      setSelectionMode(true)
      setSelectedPIDs(new Set(snapshot.pids))
      setBatchAction(null)
      setBatchTargetID('')
    }
    window.addEventListener(LOCAL_SELECTION_HANDOFF_EVENT, acceptHandoff)
    return () => window.removeEventListener(LOCAL_SELECTION_HANDOFF_EVENT, acceptHandoff)
  }, [selectionContext, source])
  useEffect(() => {
    if (!selectionMode) return
    const close = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setSelectionMode(false)
      setSelectedPIDs(new Set())
      setBatchAction(null)
      setBatchTargetID('')
    }
    window.addEventListener('keydown', close)
    return () => window.removeEventListener('keydown', close)
  }, [selectionMode])

  const setParam = (name: string, value: string, replace = true) => {
    setParams(updatePostExplorerParam(params, name, value), { replace })
  }

  function submitSearch(event: FormEvent) {
    event.preventDefault()
    const value = searchDraft.trim()
    const pid = value.match(/^#?(\d+)$/)?.[1]
    if (pid) {
      navigate(postDetailTarget(pid, source, location.pathname + location.search))
      return
    }
    setParam('q', value, false)
  }

  const [draft, setDraft] = useState(readStudioPublishDraft)
  const [files, setFiles] = useState<File[]>([])
  const [composerOpen, setComposerOpen] = useState(params.get('compose') === 'true' || Boolean(readStudioPublishDraft()))
  useEffect(() => { if (params.get('compose') === 'true') setComposerOpen(true) }, [params])
  useEffect(() => writeStudioPublishDraft(draft), [draft])
  const publish = usePublishPost({
    text: draft,
    files,
    onSuccess: () => {
      setDraft('')
      setFiles([])
      setComposerOpen(false)
      const next = new URLSearchParams(params)
      next.delete('compose')
      setParams(next, { replace: true })
      query.refetch()
    },
  })
  const selectedUnsaved = displayed.filter((post) => selectedPIDs.has(post.pid) && post.local_state !== 'saved').map((post) => post.pid)
  const selectedList = [...selectedPIDs]
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects, enabled: source === 'local' && selectionMode })
  const saveSelected = useMutation({
    mutationFn: () => api.createJob('sync_pids', { pids: selectedUnsaved, include_comments: true, include_media: true }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ['jobs'] })
      notify({ title: `已创建 ${selectedUnsaved.length} 个树洞的保存任务`, description: '任务会在后台读取完整评论和图片，可在任务中心查看进度。' })
      setSelectionMode(false)
      setSelectedPIDs(new Set())
    },
    onError: (error) => notify({ tone: 'error', title: '批量保存任务创建失败', description: errorDescription(error) }),
  })
  const addToProject = useMutation({
    mutationFn: ({ pids, projectID }: { pids: number[]; projectID: number }) => api.addPostsToProjects(pids, [projectID]),
    onSuccess: (result) => {
      client.invalidateQueries({ queryKey: ['projects'] })
      client.invalidateQueries({ queryKey: ['post-projects'] })
      notify({ title: `已将 ${result.updated} 个树洞加入项目`, description: '原有项目关系保持不变，可以继续为当前选择添加标签或导出。' })
      setBatchAction(null)
      setBatchTargetID('')
    },
    onError: (error) => notify({ tone: 'error', title: '加入项目失败', description: errorDescription(error) }),
  })
  const addTags = useMutation({
    mutationFn: ({ pids, tagID }: { pids: number[]; tagID: number }) => api.addPostTags(pids, [tagID]),
    onSuccess: (result) => {
      client.invalidateQueries({ queryKey: ['post-tags'] })
      client.invalidateQueries({ queryKey: ['posts', 'local'] })
      notify({ title: `已为 ${result.updated} 个树洞添加标签`, description: '原有标签保持不变，可以继续整理当前选择。' })
      setBatchAction(null)
      setBatchTargetID('')
    },
    onError: (error) => notify({ tone: 'error', title: '添加标签失败', description: errorDescription(error) }),
  })
  useRestoreListScroll(location.pathname + location.search, displayed.length > 0)

  function toggleComposer(open: boolean) {
    setComposerOpen(open)
    if (open || params.get('compose') !== 'true') return
    const next = new URLSearchParams(params)
    next.delete('compose')
    setParams(next, { replace: true })
  }

  return <>
    <PageHeader eyebrow="EXPLORE" title={q ? `搜索“${q}”` : source === 'live' ? '在线树洞' : '浏览资料'} description={q ? '搜索结果会同时匹配帖子正文和评论；返回列表时会保留当前位置。' : source === 'live' ? '直接浏览当前树洞内容；只有明确保存或同步时才会写入本地资料库。' : '浏览保存在本机的帖子、评论、图片、标签和笔记。'} />

    <form className="panel mb-5 flex flex-col gap-3 p-3 sm:flex-row" role="search" onSubmit={submitSearch}>
      <label className="relative flex-1"><span className="sr-only">搜索关键词</span><Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft" size={17} /><input ref={searchInput} className="field !pl-10" value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} placeholder="课程名、教师、关键词或 PID" /></label>
      {q && <button className="button-secondary sm:!px-3" type="button" onClick={() => { setSearchDraft(''); setParam('q', '', false) }}><X size={15} />清除</button>}
      <button className="button-primary sm:min-w-28" type="submit">搜索</button>
    </form>
    {!q && history.data?.length ? <div className="mb-5 flex flex-wrap items-center gap-2 text-xs text-ink-soft"><span>最近搜索</span>{history.data.slice(0, 8).map((item) => <button key={item.id} className="badge cursor-pointer hover:border-teal hover:text-teal" onClick={() => { setSearchDraft(item.query); setParam('q', item.query, false) }}>{item.query}</button>)}</div> : null}

    {source === 'live' && online.isLoading && <div className="mb-5"><LoadingState label="正在验证在线会话…" /></div>}
    {source === 'live' && !online.isLoading && !online.data?.can_read_online && <div className="panel mb-5 border-coral/30 bg-coral-soft/30 p-5 text-sm"><p className="font-semibold">需要先登录树洞</p><p className="mt-1 text-ink-soft">{online.data?.message || '当前本机会话不能读取在线内容。'}</p><Link className="button-primary mt-4" to="/sync">前往登录</Link></div>}
    {source === 'live' && online.data?.can_read_online && !online.data.can_write_online && <div className="panel mb-5 border-coral/25 p-4 text-sm text-ink-soft">当前会话可以浏览，但缺少发布所需凭据。请在<Link className="mx-1 font-semibold text-coral hover:underline" to="/sync">同步中心</Link>重新检测或登录。</div>}
	{source === 'live' && online.data?.can_read_online && <OnlineFreshnessBar browserOnline={browserOnline} updatedAt={query.dataUpdatedAt} isFetching={query.isFetching} error={query.error} hasData={displayed.length > 0} onRefresh={() => query.refetch()} />}
	{source === 'live' && online.data?.can_write_online && <details className="panel mb-6 p-5" open={composerOpen} onToggle={(event) => toggleComposer(event.currentTarget.open)}><summary className="cursor-pointer font-semibold">发布新洞</summary><div className="mt-4"><textarea className="field min-h-28" value={draft} maxLength={10000} onChange={(event) => setDraft(event.target.value)} placeholder="写下想发布的内容…" />{publish.error && <p className="mt-3 rounded-lg border border-coral/25 bg-coral-soft/25 px-3 py-2 text-xs text-coral">发布失败：{errorDescription(publish.error)}。正文和图片选择已保留。</p>}<div className="mt-3 flex flex-wrap items-center justify-between gap-3"><label className="button-secondary cursor-pointer"><ImagePlus size={16} />选择图片<input className="hidden" type="file" accept="image/*" multiple onChange={(event) => setFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><div className="flex items-center gap-3"><span className="text-xs text-ink-soft">{files.length ? `${files.length} 张图片 · ` : ''}{draft.length}/10000</span><button className="button-primary" disabled={publish.isPending || (!draft.trim() && !files.length)} onClick={() => publish.mutate()}><Send size={16} />{publish.isPending ? '正在发布…' : '确认发布'}</button></div></div></div></details>}

    {selectionMode && <section className="selection-action-bar panel mb-4 flex flex-wrap items-center gap-2 border-teal/35 bg-paper/95 p-3" aria-label={source === 'live' ? '在线树洞批量操作' : '本地资料批量操作'}><div className="mr-auto min-w-40"><p className="text-sm font-semibold">已选择 {selectedPIDs.size} 个树洞</p><p className={`text-[11px] ${selectedPIDs.size > 200 && source === 'local' ? 'text-coral' : 'text-ink-soft'}`}>{source === 'live' ? selectedUnsaved.length ? `${selectedUnsaved.length} 个尚未保存到本地` : selectedPIDs.size ? '所选内容均已保存' : '点击卡片选择要保存的内容' : selectedPIDs.size > 200 ? '项目和标签每次最多处理 200 个；导出最多 2000 个' : '可加入项目、添加标签或直接导出'}</p></div><button type="button" className="button-secondary !min-h-9 !px-3 text-xs" onClick={() => setSelectedPIDs(new Set(displayed.map((post) => post.pid)))}>全选已加载</button>{source === 'live' ? <button type="button" className="button-primary !min-h-9 !px-3 text-xs" disabled={!selectedUnsaved.length || saveSelected.isPending} onClick={() => saveSelected.mutate()}><Download size={14} />{saveSelected.isPending ? '正在创建…' : `保存到本地${selectedUnsaved.length ? `（${selectedUnsaved.length}）` : ''}`}</button> : <><button type="button" className="button-secondary !min-h-9 !px-3 text-xs" disabled={!selectedPIDs.size || selectedPIDs.size > 200} aria-expanded={batchAction === 'project'} onClick={() => { setBatchAction(batchAction === 'project' ? null : 'project'); setBatchTargetID('') }}><FolderPlus size={14} />加入项目</button><button type="button" className="button-secondary !min-h-9 !px-3 text-xs" disabled={!selectedPIDs.size || selectedPIDs.size > 200} aria-expanded={batchAction === 'tag'} onClick={() => { setBatchAction(batchAction === 'tag' ? null : 'tag'); setBatchTargetID('') }}><Tags size={14} />添加标签</button><button type="button" className="button-primary !min-h-9 !px-3 text-xs" disabled={!selectedPIDs.size || selectedPIDs.size > 2000} onClick={() => { writeLocalSelection(selectionContext, selectedPIDs); navigate('/imports?view=export&from=selection') }}><FileOutput size={14} />导出</button></>}<button type="button" className="button-secondary !min-h-9 !px-3 text-xs" onClick={() => { setSelectionMode(false); setSelectedPIDs(new Set()); setBatchAction(null); setBatchTargetID('') }}><X size={14} />退出选择</button>{source === 'local' && batchAction && <div className="basis-full border-t border-line pt-3"><div className="flex flex-col gap-2 sm:flex-row sm:items-end"><label className="min-w-0 flex-1 text-xs font-medium text-ink-soft">{batchAction === 'project' ? '选择项目' : '选择标签'}<select className="field mt-1.5" aria-label={batchAction === 'project' ? '选择要加入的项目' : '选择要添加的标签'} value={batchTargetID} onChange={(event) => setBatchTargetID(event.target.value)}><option value="">请选择…</option>{batchAction === 'project' ? projects.data?.map((project) => <option key={project.id} value={project.id}>{project.name}（{project.post_count}）</option>) : localTags.data?.map((tag) => <option key={tag.id} value={tag.id}>{tag.name}</option>)}</select></label><button type="button" className="button-primary shrink-0" disabled={!batchTargetID || addToProject.isPending || addTags.isPending} onClick={() => { const id = Number(batchTargetID); if (!id) return; if (batchAction === 'project') addToProject.mutate({ pids: selectedList, projectID: id }); else addTags.mutate({ pids: selectedList, tagID: id }) }}>{addToProject.isPending || addTags.isPending ? '正在应用…' : batchAction === 'project' ? `加入项目（${selectedPIDs.size}）` : `添加标签（${selectedPIDs.size}）`}</button>{batchAction === 'project' && !projects.isLoading && !projects.data?.length && <Link className="button-secondary shrink-0" to="/projects">先创建项目</Link>}{batchAction === 'tag' && !localTags.isLoading && !localTags.data?.length && <Link className="button-secondary shrink-0" to="/settings#local-tags">先创建标签</Link>}</div></div>}</section>}

    <div className="mb-4 flex gap-2"><button type="button" className="button-secondary flex-1 justify-between xl:hidden" aria-expanded={filtersOpen} onClick={() => setFiltersOpen((value) => !value)}><span className="inline-flex items-center gap-2"><SlidersHorizontal size={16} />筛选当前{source === 'live' ? '在线列表' : '本地资料'}</span><span className="badge">{[hasMedia, followed ? 'followed' : '', label, explorer.localTag, sort !== 'desc' ? sort : ''].filter(Boolean).length || '全部'}</span></button><button type="button" className="button-secondary shrink-0" aria-pressed={selectionMode} onClick={() => { setSelectionMode((value) => !value); setSelectedPIDs(new Set()); setBatchAction(null); setBatchTargetID('') }}><CheckSquare2 size={16} />{selectionMode ? '取消选择' : '选择多个'}</button></div>

    <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,860px)_minmax(260px,1fr)]">
      <section className="min-w-0">
        {q && !query.isLoading && <p className="mb-3 text-sm text-ink-soft">找到 {displayed.length} 个{query.hasNextPage ? '以上' : ''}帖子结果</p>}
		{canRead && (query.isLoading && !displayed.length ? <LoadingState label={q ? `正在搜索“${q}”…` : undefined} /> : query.error && !displayed.length ? <ErrorState error={query.error} /> : displayed.length ? <div className="grid gap-4">{displayed.map((post) => <PostCard key={post.pid} post={post} source={source} selectable={selectionMode} selected={selectedPIDs.has(post.pid)} onSelect={(pid) => setSelectedPIDs((current) => { const next = new Set(current); if (next.has(pid)) next.delete(pid); else next.add(pid); return next })} />)}</div> : <EmptyState title={q ? '没有找到匹配内容' : '没有符合条件的帖子'} description={q ? '减少关键词、检查 PID，或保存更多资料后重试。' : '尝试调整筛选，或先从归档导入、保存一些内容。'} action={<Filter size={18} />} />)}
        {query.hasNextPage && <div className="mt-6 flex justify-center"><button className="button-secondary" disabled={query.isFetchingNextPage} onClick={() => query.fetchNextPage()}>{query.isFetchingNextPage ? '读取中…' : '加载更多'}</button></div>}
      </section>
      <aside className={`${filtersOpen ? 'block' : 'hidden'} panel order-first p-4 xl:order-none xl:sticky xl:top-24 xl:block`}>
        <div className="flex items-center justify-between gap-3"><p className="text-xs font-semibold text-ink-soft">筛选当前{source === 'live' ? '在线列表' : '本地资料'}</p><button type="button" className="text-xs font-semibold text-teal xl:hidden" onClick={() => setFiltersOpen(false)}>完成</button></div>
        <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
          <label><span className="mb-1.5 block text-xs font-medium text-ink-soft">排序</span><select className="field" value={sort} disabled={source === 'live'} onChange={(event) => setParam('sort', event.target.value)}><option value="desc">最新优先</option><option value="asc">最早优先</option><option value="reply">评论最多</option><option value="praise_num">点赞最多</option></select></label>
          <label><span className="mb-1.5 block text-xs font-medium text-ink-soft">图片</span><select className="field" value={hasMedia} onChange={(event) => setParam('media', event.target.value)}><option value="">全部内容</option><option value="true">有图片</option><option value="false">无图片</option></select></label>
          {source === 'live' && <label><span className="mb-1.5 block text-xs font-medium text-ink-soft">在线范围</span><select className="field" value={followed ? 'followed' : label ? `label:${label}` : ''} onChange={(event) => { const value = event.target.value; const next = new URLSearchParams(params); next.delete('followed'); next.delete('label'); if (value === 'followed') next.set('followed', 'true'); else if (value.startsWith('label:')) next.set('label', value.slice(6)); setParams(next, { replace: true }) }}><option value="">公共时间线</option><option value="followed">我关注的洞</option>{tags.data?.filter((tag) => tag.label || tag.name).map((tag) => <option key={tag.id} value={`label:${tag.id}`}>{tag.label || tag.name}</option>)}</select></label>}
        </div>
        {source === 'local' && <div className="mt-5 border-t border-line pt-4"><p className="mb-2 text-xs font-medium text-ink-soft">本地标签</p><LocalTagFilter selected={localTagIDs} onChange={(ids) => setParam('tag', ids.join(','))} /></div>}
      </aside>
    </div>
  </>
}

const studioPublishDraftKey = 'pkustudio:studio:publish-draft'

function readStudioPublishDraft() {
  try { return window.sessionStorage.getItem(studioPublishDraftKey) ?? '' } catch { return '' }
}

function writeStudioPublishDraft(value: string) {
  try {
    if (value) window.sessionStorage.setItem(studioPublishDraftKey, value)
    else window.sessionStorage.removeItem(studioPublishDraftKey)
  } catch { /* best effort */ }
}

function useRestoreListScroll(key: string, ready: boolean) {
  const restored = useRef('')
  useEffect(() => {
    if (!ready || restored.current === key) return
    restored.current = key
    const value = sessionStorage.getItem(`studio:list-scroll:${key}`)
    if (!value) return
    sessionStorage.removeItem(`studio:list-scroll:${key}`)
    requestAnimationFrame(() => window.scrollTo({ top: Number(value) || 0 }))
  }, [key, ready])
}
