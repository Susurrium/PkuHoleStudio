import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import {
  AlertIcon,
  ChevronDownIcon,
  CommentIcon,
  FilterIcon,
  ImageIcon,
  IssueOpenedIcon,
  PaperclipIcon,
  PlusIcon,
  SearchIcon,
  StarIcon,
	SyncIcon,
  TagIcon,
  ThumbsupIcon,
  XIcon,
} from '@primer/octicons-react'
import type { PostSummary } from '../lib/types'
import { compactNumber, formatTime, HighlightedText } from '../lib/format'
import { errorDescription } from '../components/Feedback'
import { postDetailTarget, updatePostExplorerParam, updatePostExplorerSource, usePostsExplorer, usePublishPost } from '../features/posts/usePostsExplorer'
import { GithubLoading, GithubPageHeader, GithubState } from './GithubComponents'
import { useSavePostToLocal } from '../features/posts/usePostDetail'
import { freshnessTime } from '../features/online/connectivity'

export function GithubPostsPage() {
  const [params, setParams] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const explorer = usePostsExplorer(params)
  const { source, q, sort, hasMedia, followed, label, localTag, online, liveTags, localTags, canRead, browserOnline, query, posts } = explorer
  const [searchDraft, setSearchDraft] = useState(q)
  const [draft, setDraft] = useState(readGithubPublishDraft)
  const [files, setFiles] = useState<File[]>([])
  const composeRequested = params.get('compose') === 'true'
  const canCompose = source === 'live' && online.data?.can_write_online === true
  const composing = composeRequested && canCompose

  useEffect(() => setSearchDraft(q), [q])
  useEffect(() => writeGithubPublishDraft(draft), [draft])

  function setParam(name: string, value: string, replace = true) {
    setParams(updatePostExplorerParam(params, name, value), { replace })
  }

  function setComposing(open: boolean) {
    setParam('compose', open ? 'true' : '', false)
  }

  function submitSearch(event: FormEvent) {
    event.preventDefault()
    const value = searchDraft.trim()
    const pid = value.match(/^#?(\d+)$/)?.[1]
    if (pid) {
      rememberListPosition(location.pathname + location.search)
      navigate(postDetailTarget(pid, source, location.pathname + location.search))
      return
    }
    setParam('q', value, false)
  }

  const publish = usePublishPost({
    text: draft,
    files,
    onSuccess: () => {
      setDraft('')
      setFiles([])
      setComposing(false)
      query.refetch()
    },
  })

  useRestoreListPosition(location.pathname + location.search, posts.length > 0)

  return <>
    <GithubPageHeader
      eyebrow="TREEHOLE"
      title={q ? <>搜索 <span className="github-muted">“{q}”</span></> : source === 'live' ? '在线树洞' : '本地资料'}
      description={source === 'live' ? '浏览当前在线时间线；只有明确保存或同步时才写入本地资料库。' : '浏览和搜索保存在本机的帖子、评论、标签和笔记。'}
      actions={source === 'live'
        ? online.isLoading
          ? <button className="github-button github-button--primary" disabled><PlusIcon size={16} />正在检测会话…</button>
          : canCompose
            ? <button className="github-button github-button--primary" onClick={() => setComposing(true)}><PlusIcon size={16} />发表新洞</button>
            : <Link className="github-button github-button--primary" to="/sync"><PlusIcon size={16} />登录后发表</Link>
        : <Link className="github-button github-button--primary" to="/sync"><PlusIcon size={16} />同步资料</Link>}
    />

    {composing && <section className="github-compose-panel" aria-labelledby="github-compose-title">
      <header><IssueOpenedIcon size={19} /><div><h2 id="github-compose-title">发表新洞</h2><p>只有点击“确认发表”才会发送到在线树洞。</p></div><button className="github-icon-button" type="button" aria-label="关闭发表区" onClick={() => setComposing(false)}><XIcon size={17} /></button></header>
      <textarea value={draft} maxLength={10000} onChange={(event) => setDraft(event.target.value)} placeholder="写下想发表的内容…" aria-label="新洞正文" />
      {publish.error && <div className="github-inline-error"><AlertIcon size={15} />发表失败：{errorDescription(publish.error)}。草稿和图片选择已保留。</div>}
      <footer><label className="github-button"><PaperclipIcon size={15} />添加图片<input type="file" accept="image/*" multiple onChange={(event) => setFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><span>{files.length ? `${files.length} 张图片 · ` : ''}{draft.length}/10000</span><button className="github-button github-button--primary" disabled={!canCompose || publish.isPending || (!draft.trim() && !files.length)} onClick={() => { if (canCompose) publish.mutate() }}>{publish.isPending ? '正在发表…' : '确认发表'}</button></footer>
    </section>}

    <section className="github-issues-box">
      <div className="github-issues-toolbar">
        <div className="github-source-tabs" aria-label="内容来源">
          <button className={source === 'local' ? 'is-active' : ''} onClick={() => setParams(updatePostExplorerSource(params, 'local'), { replace: true })}>本地资料</button>
          <button className={source === 'live' ? 'is-active' : ''} onClick={() => setParams(updatePostExplorerSource(params, 'live'), { replace: true })}>在线树洞</button>
        </div>
        <form className="github-issue-search" role="search" onSubmit={submitSearch}>
          <SearchIcon size={16} />
          <input value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} aria-label="搜索内容或 PID" placeholder="搜索内容或 #PID" />
          {q && <button type="button" aria-label="清除搜索" onClick={() => { setSearchDraft(''); setParam('q', '', false) }}><XIcon size={14} /></button>}
          <button className="github-button" type="submit">搜索</button>
        </form>
      </div>

      <div className="github-filter-bar">
        <span><IssueOpenedIcon size={16} /><strong>{posts.length}{query.hasNextPage ? '+' : ''}</strong> 个帖子</span>
        <div>
          {source === 'live' && <label><StarIcon size={14} /><span>范围</span><select value={followed ? 'followed' : label ? `label:${label}` : ''} onChange={(event) => { const next = new URLSearchParams(params); next.delete('followed'); next.delete('label'); if (event.target.value === 'followed') next.set('followed', 'true'); else if (event.target.value.startsWith('label:')) next.set('label', event.target.value.slice(6)); setParams(next, { replace: true }) }}><option value="">公共时间线</option><option value="followed">我关注的洞</option>{liveTags.data?.map((tag) => <option key={tag.id} value={`label:${tag.id}`}>{tag.label || tag.name || `标签 ${tag.id}`}</option>)}</select><ChevronDownIcon size={12} /></label>}
          {source === 'local' && <label><TagIcon size={14} /><span>标签</span><select value={localTag} onChange={(event) => setParam('tag', event.target.value)}><option value="">全部标签</option>{localTags.data?.map((tag) => <option key={tag.id} value={tag.id}>{tag.name}</option>)}</select><ChevronDownIcon size={12} /></label>}
          <label><FilterIcon size={14} /><span>图片</span><select value={hasMedia} onChange={(event) => setParam('media', event.target.value)}><option value="">全部</option><option value="true">有图片</option><option value="false">无图片</option></select><ChevronDownIcon size={12} /></label>
          <label><span>排序</span><select value={sort} disabled={source === 'live'} onChange={(event) => setParam('sort', event.target.value)}><option value="desc">最新</option><option value="asc">最早</option><option value="reply">评论最多</option><option value="praise_num">点赞最多</option></select><ChevronDownIcon size={12} /></label>
        </div>
      </div>
	  {source === 'live' && canRead && (query.dataUpdatedAt > 0 || query.error || !browserOnline) && <div className={`github-freshness${!browserOnline || query.error ? ' is-degraded' : ''}`} role="status"><span className="github-freshness-dot" /><span><strong>{!browserOnline ? '设备离线' : query.error ? posts.length ? '刷新失败，正在显示上次结果' : '在线内容读取失败' : query.isFetching ? '正在刷新在线内容' : '在线内容已连接'}</strong>{query.dataUpdatedAt ? ` · 最近读取 ${freshnessTime(query.dataUpdatedAt)}` : ' · 暂无可显示的在线结果'}</span><button className="github-button" type="button" disabled={!browserOnline || query.isFetching} onClick={() => query.refetch()}><SyncIcon size={13} />{query.isFetching ? '刷新中' : '刷新'}</button></div>}

      <div className="github-issue-list">
        {source === 'live' && online.isLoading && <GithubLoading label="正在验证在线会话…" />}
        {source === 'live' && !online.isLoading && !online.data?.can_read_online && <GithubState tone="danger" title="需要先登录树洞" description={online.data?.message || '当前本机会话不能读取在线内容。'} action={<Link className="github-button github-button--primary" to="/sync">前往登录</Link>} />}
        {canRead && query.isLoading && posts.length === 0 && <GithubLoading label={q ? `正在搜索“${q}”…` : '正在读取帖子…'} />}
		{canRead && query.error && posts.length === 0 && <GithubState tone="danger" title="读取失败" description={query.error.message} action={<button className="github-button" onClick={() => query.refetch()}>重试</button>} />}
        {canRead && !query.isLoading && !query.error && posts.length === 0 && <GithubState title={q ? '没有找到匹配内容' : '这里还没有帖子'} description="可以调整筛选、切换来源，或先同步和导入资料。" action={<button className="github-button" onClick={() => { setSearchDraft(''); setParams(source === 'live' ? { source: 'live' } : {}, { replace: false }) }}>清除筛选</button>} />}
        {posts.map((post) => <GithubIssueRow key={post.pid} post={post} source={source} returnTo={location.pathname + location.search} />)}
      </div>
      {query.hasNextPage && <div className="github-pagination"><button className="github-button" disabled={query.isFetchingNextPage} onClick={() => query.fetchNextPage()}>{query.isFetchingNextPage ? '正在加载…' : '加载更多'}</button></div>}
    </section>
  </>
}

const githubPublishDraftKey = 'pkustudio:github:publish-draft'

function readGithubPublishDraft() {
  try { return window.sessionStorage.getItem(githubPublishDraftKey) ?? '' } catch { return '' }
}

function writeGithubPublishDraft(value: string) {
  try {
    if (value) window.sessionStorage.setItem(githubPublishDraftKey, value)
    else window.sessionStorage.removeItem(githubPublishDraftKey)
  } catch { /* best effort */ }
}

function GithubIssueRow({ post, source, returnTo }: { post: PostSummary; source: 'local' | 'live'; returnTo: string }) {
  const target = postDetailTarget(post.pid, source, returnTo)
  const saveLocal = useSavePostToLocal(post.pid)
  const remember = () => rememberListPosition(returnTo)
  return <article className="github-issue-row">
    <IssueOpenedIcon className="github-open-icon" size={18} />
    <div className="github-issue-content">
      <Link to={target} onClick={remember} className="github-issue-title"><HighlightedText value={post.snippet || post.text || '（无正文）'} /></Link>
      <div className="github-issue-labels">{post.media_ids && <span><ImageIcon size={12} />图片</span>}{source === 'live' ? <span className="github-label--live">在线</span> : <span className="github-label--local">本地</span>}{source === 'live' && !saveLocal.job && post.local_state === 'saved' && <span className="github-label--local">已保存</span>}{source === 'live' && (saveLocal.job || post.local_state !== 'saved') && <button type="button" className={`github-inline-save ${saveLocal.retryable ? 'github-inline-save--failed' : ''}`} disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start} title={saveLocal.failure || undefined}>{saveLocal.label}</button>}{source === 'live' && saveLocal.failure && <span className="github-save-error" role="alert">{saveLocal.failure}</span>}</div>
      {post.comment_matches?.slice(0, 1).map((match) => <Link key={match.cid} to={`${target}#comment-${match.cid}`} onClick={remember} className="github-comment-match">C{match.cid}：<HighlightedText value={match.snippet} /></Link>)}
      <p>#{post.pid} · {formatTime(post.timestamp)}</p>
    </div>
    <div className="github-issue-metrics"><span title="点赞"><ThumbsupIcon size={14} />{compactNumber(post.praise_num ?? post.likenum)}</span><span title="评论"><CommentIcon size={14} />{compactNumber(post.reply)}</span></div>
  </article>
}

function rememberListPosition(key: string) {
  try { sessionStorage.setItem(`studio:list-scroll:${key}`, String(window.scrollY)) } catch { /* best effort */ }
}

function useRestoreListPosition(key: string, ready: boolean) {
  useEffect(() => {
    if (!ready) return
    try {
      const value = sessionStorage.getItem(`studio:list-scroll:${key}`)
      if (!value) return
      sessionStorage.removeItem(`studio:list-scroll:${key}`)
      requestAnimationFrame(() => window.scrollTo({ top: Number(value) || 0 }))
    } catch { /* best effort */ }
  }, [key, ready])
}
