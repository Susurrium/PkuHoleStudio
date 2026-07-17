import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Bot, CheckSquare2, FolderKanban, Pencil, Plus, Save, Search, Trash2, X } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { errorDescription, useFeedback } from '../components/Feedback'
import { PageHeader } from '../components/PageHeader'
import { PostCard } from '../components/PostCard'
import { EmptyState, ErrorState, LoadingState } from '../components/States'
import { api } from '../lib/api'
import type { ResearchProject } from '../lib/types'

export function ProjectsPage() {
  const client = useQueryClient()
  const { notify, confirm } = useFeedback()
  const [searchParams, setSearchParams] = useSearchParams()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const requestedID = Number(searchParams.get('project') || 0)
  const selected = projects.data?.find((project) => project.id === requestedID)
  const selectedID = selected?.id ?? 0
  const posts = useQuery({ queryKey: ['project-posts', selectedID], queryFn: () => api.projectPosts(selectedID), enabled: selectedID > 0 })
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [color, setColor] = useState('#0f766e')
  const [pid, setPID] = useState('')
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editColor, setEditColor] = useState('#0f766e')
  const [postSearch, setPostSearch] = useState('')
  const [postSort, setPostSort] = useState<'added' | 'newest' | 'oldest' | 'comments'>('added')
  const [selectingPosts, setSelectingPosts] = useState(false)
  const [selectedPostIDs, setSelectedPostIDs] = useState<Set<number>>(() => new Set())

  useEffect(() => {
    if (!projects.data?.length || selected) return
    setSearchParams({ project: String(projects.data[0].id) }, { replace: true })
  }, [projects.data, selected, setSearchParams])
  useEffect(() => {
    if (!selected) return
    setEditing(false)
    setEditName(selected.name)
    setEditDescription(selected.description || '')
    setEditColor(selected.color || '#0f766e')
    setPostSearch('')
    setPostSort('added')
    setSelectingPosts(false)
    setSelectedPostIDs(new Set())
  }, [selected?.id])

  const visiblePosts = useMemo(() => {
    const normalized = postSearch.trim().toLocaleLowerCase()
    const filtered = (posts.data ?? []).filter((post) => !normalized || String(post.pid).includes(normalized) || post.text.toLocaleLowerCase().includes(normalized))
    if (postSort === 'added') return filtered
    return [...filtered].sort((left, right) => postSort === 'newest'
      ? (right.timestamp ?? 0) - (left.timestamp ?? 0) || right.pid - left.pid
      : postSort === 'oldest'
        ? (left.timestamp ?? 0) - (right.timestamp ?? 0) || left.pid - right.pid
        : (right.reply ?? 0) - (left.reply ?? 0) || right.pid - left.pid)
  }, [postSearch, postSort, posts.data])

  const create = useMutation({
    mutationFn: () => api.createProject(name, description, color),
    onSuccess: async (project) => {
      setName(''); setDescription(''); setColor('#0f766e')
      await client.invalidateQueries({ queryKey: ['projects'] })
      setSearchParams({ project: String(project.id) })
      notify({ title: `研究项目“${project.name}”已创建` })
    },
    onError: (error) => notify({ tone: 'error', title: '创建研究项目失败', description: errorDescription(error) }),
  })
  const removeProject = useMutation({
    mutationFn: (id: number) => api.deleteProject(id),
    onSuccess: async () => {
      setSearchParams({}, { replace: true })
      await client.invalidateQueries({ queryKey: ['projects'] })
      notify({ title: '研究项目已删除', description: '项目中的树洞仍保留在本地资料库。' })
    },
    onError: (error) => notify({ tone: 'error', title: '删除研究项目失败', description: errorDescription(error) }),
  })
  const updateProject = useMutation({
    mutationFn: () => api.updateProject(selectedID, editName.trim(), editDescription.trim(), editColor),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['projects'] })
      setEditing(false)
      notify({ title: '研究项目信息已更新' })
    },
    onError: (error) => notify({ tone: 'error', title: '更新研究项目失败', description: errorDescription(error) }),
  })
  const addPost = useMutation({
    mutationFn: (value: number) => api.addPostsToProjects([value], [selectedID]),
    onSuccess: async (_, value) => {
      setPID('')
      await Promise.all([
        client.invalidateQueries({ queryKey: ['project-posts', selectedID] }),
        client.invalidateQueries({ queryKey: ['projects'] }),
        client.invalidateQueries({ queryKey: ['post-projects', value] }),
      ])
      notify({ title: `树洞 #${value} 已加入项目` })
    },
    onError: (error) => notify({ tone: 'error', title: '无法加入项目', description: `${errorDescription(error)}。请确认该树洞已经保存到本地。` }),
  })
  const removePosts = useMutation({
    mutationFn: (values: number[]) => api.removeProjectPosts(selectedID, values),
    onSuccess: async (result, values) => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ['project-posts', selectedID] }),
        client.invalidateQueries({ queryKey: ['projects'] }),
        client.invalidateQueries({ queryKey: ['post-projects'] }),
      ])
      setSelectingPosts(false)
      setSelectedPostIDs(new Set())
      notify({ title: `已从项目移出 ${result.updated} 个树洞`, description: '树洞仍保留在本地资料库中。' })
    },
    onError: (error) => notify({ tone: 'error', title: '移出项目失败', description: errorDescription(error) }),
  })

  if (projects.isLoading) return <LoadingState label="正在读取研究项目…" />
  if (projects.error) return <ErrorState error={projects.error} />

  return <>
    <PageHeader eyebrow="PROJECTS" title="研究项目" description="把本地树洞整理成持续维护的主题集合，再以固定范围进行 AI 研究、复查或导出。" />
    <div className="grid items-start gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
      <aside className="grid gap-5 xl:sticky xl:top-24">
        <form className="panel p-5" onSubmit={(event: FormEvent) => { event.preventDefault(); if (name.trim()) create.mutate() }}>
          <h2 className="flex items-center gap-2 font-semibold"><Plus size={17} />新建项目</h2>
          <input className="field mt-4" value={name} maxLength={160} onChange={(event) => setName(event.target.value)} placeholder="例如：选课与教师评价" aria-label="项目名称" />
          <textarea className="field mt-3 min-h-20" value={description} maxLength={10000} onChange={(event) => setDescription(event.target.value)} placeholder="研究问题、收集标准或后续计划" aria-label="项目说明" />
          <label className="mt-3 flex items-center gap-3 text-xs text-ink-soft">项目颜色<input type="color" value={color} onChange={(event) => setColor(event.target.value)} /></label>
          <button className="button-primary mt-4 w-full" disabled={create.isPending || !name.trim()}>创建研究项目</button>
        </form>
        <section className="panel overflow-hidden">
          <header className="border-b border-line px-5 py-4"><h2 className="font-semibold">全部项目</h2><p className="mt-1 text-xs text-ink-soft">{projects.data?.length ?? 0} 个本地项目</p></header>
          <div className="grid p-2">{projects.data?.length ? projects.data.map((project) => <ProjectButton key={project.id} project={project} active={project.id === selectedID} select={() => setSearchParams({ project: String(project.id) })} />) : <p className="p-5 text-center text-sm text-ink-soft">先创建第一个研究项目。</p>}</div>
        </section>
      </aside>

      {!selected ? <EmptyState title="选择或创建一个研究项目" description="项目会保存树洞成员关系，但不会复制或删除原始资料。" action={<FolderKanban size={20} />} /> : <section className="min-w-0">
        <section className="panel p-5 md:p-6">
          <div className="flex flex-wrap items-start justify-between gap-4"><div className="flex gap-3"><span className="mt-1 size-3 shrink-0 rounded-full" style={{ background: selected.color || '#0f766e' }} /><div><p className="eyebrow">RESEARCH PROJECT</p><h2 className="mt-1 text-2xl font-semibold">{selected.name}</h2><p className="mt-2 max-w-3xl text-sm leading-6 text-ink-soft">{selected.description || '尚未填写项目说明。'}</p></div></div><div className="flex flex-wrap gap-2">{posts.data?.length ? <Link className="button-primary" to={`/ai?mode=selected&pids=${posts.data.slice(0, 200).map((post) => post.pid).join(',')}`}><Bot size={15} />研究项目内容</Link> : null}<button className="button-secondary" onClick={() => setEditing((value) => !value)}><Pencil size={15} />{editing ? '取消编辑' : '编辑项目'}</button><button className="button-secondary !text-coral" disabled={removeProject.isPending} onClick={async () => { const accepted = await confirm({ title: `删除研究项目“${selected.name}”？`, description: '项目中的树洞仍会保留在本地资料库，只删除项目及成员关系。', confirmLabel: '删除项目', tone: 'danger' }); if (accepted) removeProject.mutate(selected.id) }}><Trash2 size={15} />删除项目</button></div></div>
          {editing && <form className="mt-5 grid gap-3 rounded-xl border border-line bg-paper/45 p-4 sm:grid-cols-[1fr_auto]" onSubmit={(event) => { event.preventDefault(); if (editName.trim()) updateProject.mutate() }}><div className="grid gap-3"><label className="text-xs font-medium text-ink-soft">项目名称<input className="field mt-1.5" value={editName} maxLength={160} onChange={(event) => setEditName(event.target.value)} /></label><label className="text-xs font-medium text-ink-soft">项目说明<textarea className="field mt-1.5 min-h-20" value={editDescription} maxLength={10000} onChange={(event) => setEditDescription(event.target.value)} /></label></div><div className="flex items-end gap-2 sm:flex-col sm:items-stretch sm:justify-end"><label className="flex items-center gap-2 text-xs font-medium text-ink-soft">颜色<input type="color" value={editColor} onChange={(event) => setEditColor(event.target.value)} /></label><button className="button-primary" disabled={!editName.trim() || updateProject.isPending}><Save size={15} />{updateProject.isPending ? '保存中…' : '保存修改'}</button></div></form>}
          <form className="mt-5 flex flex-wrap gap-2 border-t border-line pt-5" onSubmit={(event) => { event.preventDefault(); const value = Number(pid); if (Number.isInteger(value) && value > 0) addPost.mutate(value) }}><input className="field min-w-[220px] flex-1" value={pid} onChange={(event) => setPID(event.target.value.replace(/\D/g, ''))} placeholder="输入已保存到本地的 PID" aria-label="加入项目的 PID" /><button className="button-secondary" disabled={addPost.isPending || !Number(pid)}><Plus size={15} />加入项目</button></form>
        </section>
        <section className="mt-6"><div className="mb-4 flex flex-wrap items-end justify-between gap-3"><div><p className="eyebrow">MATERIALS</p><h2 className="mt-1 text-xl font-semibold">项目资料</h2></div><span className="badge">{postSearch ? `显示 ${visiblePosts.length} / ${posts.data?.length ?? 0}` : `${posts.data?.length ?? selected.post_count} 个树洞`}</span></div>{posts.isLoading ? <LoadingState label="正在读取项目资料…" /> : posts.error ? <ErrorState error={posts.error} /> : posts.data?.length ? <><div className="panel mb-4 flex flex-col gap-3 p-3 sm:flex-row"><label className="relative min-w-0 flex-1"><span className="sr-only">搜索项目资料</span><Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft" size={16} /><input className="field !pl-9" value={postSearch} onChange={(event) => { setPostSearch(event.target.value); setSelectedPostIDs(new Set()) }} placeholder="搜索项目内正文或 PID" /></label><label><span className="sr-only">项目资料排序</span><select className="field sm:w-40" value={postSort} onChange={(event) => setPostSort(event.target.value as typeof postSort)}><option value="added">最近加入</option><option value="newest">发布时间最新</option><option value="oldest">发布时间最早</option><option value="comments">评论最多</option></select></label><button type="button" className="button-secondary shrink-0" aria-pressed={selectingPosts} onClick={() => { setSelectingPosts((value) => !value); setSelectedPostIDs(new Set()) }}><CheckSquare2 size={16} />{selectingPosts ? '取消选择' : '选择多个'}</button></div>{selectingPosts && <div className="selection-action-bar panel mb-4 flex flex-wrap items-center gap-2 border-teal/35 bg-paper/95 p-3" aria-label="项目资料批量操作"><div className="mr-auto"><p className="text-sm font-semibold">已选择 {selectedPostIDs.size} 个树洞</p><p className={`text-[11px] ${selectedPostIDs.size > 200 ? 'text-coral' : 'text-ink-soft'}`}>{selectedPostIDs.size > 200 ? '每次最多移出 200 个树洞' : '移出项目不会删除本地资料'}</p></div><button type="button" className="button-secondary !min-h-9 !px-3 text-xs" onClick={() => setSelectedPostIDs(new Set(visiblePosts.map((post) => post.pid)))}>全选当前结果</button><button type="button" className="button-secondary !min-h-9 !px-3 text-xs !text-coral" disabled={!selectedPostIDs.size || selectedPostIDs.size > 200 || removePosts.isPending} onClick={async () => { const accepted = await confirm({ title: `从“${selected.name}”移出 ${selectedPostIDs.size} 个树洞？`, description: '只移除项目成员关系，树洞、评论、图片、标签和笔记仍保留在本地资料库。', confirmLabel: '移出项目', tone: 'danger' }); if (accepted) removePosts.mutate([...selectedPostIDs]) }}><X size={14} />{removePosts.isPending ? '正在移出…' : '移出项目'}</button><button type="button" className="button-secondary !min-h-9 !px-3 text-xs" onClick={() => { setSelectingPosts(false); setSelectedPostIDs(new Set()) }}><X size={14} />退出选择</button></div>}{visiblePosts.length ? <div className="grid gap-4">{visiblePosts.map((post) => <div key={post.pid} className="relative"><PostCard post={post} selectable={selectingPosts} selected={selectedPostIDs.has(post.pid)} onSelect={(value) => setSelectedPostIDs((current) => { const next = new Set(current); if (next.has(value)) next.delete(value); else next.add(value); return next })} />{!selectingPosts && <button type="button" className="absolute right-3 top-3 z-10 grid size-8 place-items-center rounded-lg border border-line bg-paper/90 text-ink-soft shadow-sm hover:border-coral/40 hover:text-coral" aria-label={`将树洞 #${post.pid} 移出项目`} disabled={removePosts.isPending} onClick={() => removePosts.mutate([post.pid])}><X size={14} /></button>}</div>)}</div> : <EmptyState title="项目内没有匹配结果" description="尝试减少关键词，或者输入树洞 PID。" action={<Search size={19} />} />}</> : <EmptyState title="项目中还没有树洞" description="可输入本地 PID，或在树洞详情的个人资料区域选择此项目。" action={<FolderKanban size={19} />} />}</section>
      </section>}
    </div>
  </>
}

function ProjectButton({ project, active, select }: { project: ResearchProject; active: boolean; select: () => void }) {
  return <button type="button" className={`flex items-center gap-3 rounded-xl px-3 py-3 text-left transition ${active ? 'bg-ink text-white' : 'hover:bg-white/60'}`} onClick={select}><span className="size-2.5 shrink-0 rounded-full" style={{ background: project.color || '#0f766e' }} /><span className="min-w-0 flex-1"><strong className="block truncate text-sm">{project.name}</strong><small className={`mt-0.5 block truncate ${active ? 'text-white/65' : 'text-ink-soft'}`}>{project.description || '无项目说明'}</small></span><span className={`text-xs ${active ? 'text-white/70' : 'text-ink-soft'}`}>{project.post_count}</span></button>
}
