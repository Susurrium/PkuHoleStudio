import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, ChevronDown, Download, Heart, Image, ImagePlus, ListFilter, ListTodo, MessageCircle, RefreshCw, Reply, Search, Send, Settings, Sparkles, Star, X } from 'lucide-react'
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, isOnlineSessionError } from '../lib/api'
import { compactNumber, formatTime, HighlightedText } from '../lib/format'
import type { Comment, Media, PostSummary, Reference } from '../lib/types'
import { ClassicAccountDrawer, ClassicDrawerFrame, ClassicNotificationsDrawer, ClassicTasksDrawer } from './ClassicUtilityDrawers'
import { errorDescription, useFeedback } from '../components/Feedback'
import { invalidateOnlineSession } from '../features/online/session'
import { usePostsExplorer } from '../features/posts/usePostsExplorer'
import { useLocalPostMetadata, usePostDetailResource, usePostInteraction, useSavePostToLocal } from '../features/posts/usePostDetail'
import { freshnessTime } from '../features/online/connectivity'

type ClassicUtilityDrawer = 'notifications' | 'account' | 'tasks'

export function ClassicPostsPage() {
  const { pid: routePID } = useParams()
  const [routeParams, setRouteParams] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()
  const returnTo = routePID ? safeReturnTo(routeParams.get('return_to')) : location.pathname + location.search
  const listParams = useMemo(() => routePID ? paramsFromReturnTo(returnTo) : new URLSearchParams(routeParams), [routePID, returnTo, routeParams])
  const explorer = usePostsExplorer(listParams)
  const { source, q, sort, hasMedia, followed, label, localTag, online, liveTags, localTags, canRead, browserOnline, query, posts } = explorer
  const detailSource = routePID && routeParams.get('source') === 'live' ? 'live' : source
  const [searchDraft, setSearchDraft] = useState(q)
  const [publishOpen, setPublishOpen] = useState(false)
  const requestedPanel = !routePID ? routeParams.get('panel') : null
  const utilityDrawer = (['notifications', 'account', 'tasks'] as const).find((item) => item === requestedPanel) as ClassicUtilityDrawer | undefined
  const searchInput = useRef<HTMLInputElement>(null)

  useEffect(() => setSearchDraft(q), [q])

  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 5_000 })
  const notificationSummary = useQuery({ queryKey: ['notifications', 'interactive'], queryFn: () => api.notifications('interactive'), enabled: source === 'live' && online.data?.can_read_online === true, staleTime: 30_000 })
  const activeTasks = jobs.data?.filter((job) => ['queued', 'running', 'paused'].includes(job.status)).length ?? 0
  const attentionTasks = jobs.data?.filter((job) => ['failed', 'partial'].includes(job.status)).length ?? 0
  const unreadMessages = notificationSummary.data?.items.filter((item) => !item.read).length ?? 0

  useRestoreClassicListScroll(returnTo, posts.length > 0 && !routePID)

  function commit(next: URLSearchParams, replace = true) {
    if (routePID) return
    next.delete('focus')
    setRouteParams(next, { replace })
  }

  function setParam(name: string, value: string, replace = true) {
    const next = new URLSearchParams(listParams)
    if (value) next.set(name, value); else next.delete(name)
    commit(next, replace)
  }

  function setSource(nextSource: 'local' | 'live') {
    const next = new URLSearchParams(listParams)
    if (nextSource === 'live') next.set('source', 'live'); else next.delete('source')
    next.delete('followed'); next.delete('label'); next.delete('tag'); next.delete('sort')
    commit(next)
  }

  function showLatest() {
    const next = new URLSearchParams(listParams)
    next.delete('q'); next.delete('followed'); next.delete('label'); next.delete('tag')
    setSearchDraft('')
    commit(next, false)
  }

  function submitSearch(event: FormEvent) {
    event.preventDefault()
    const value = searchDraft.trim()
    const directPID = value.match(/^#?(\d+)$/)?.[1]
    if (directPID) {
      rememberClassicListPosition(returnTo)
      navigate(classicDetailTarget(Number(directPID), source, returnTo))
      return
    }
    setParam('q', value, false)
  }

  function openUtilityDrawer(panel: ClassicUtilityDrawer) {
    if (routePID) return
    const next = new URLSearchParams(routeParams)
    next.set('panel', panel)
    setRouteParams(next, { replace: false, preventScrollReset: true, state: { classicPanel: true } })
  }

  function closeUtilityDrawer() {
    if ((location.state as { classicPanel?: boolean } | null)?.classicPanel) {
      navigate(-1)
      return
    }
    const next = new URLSearchParams(routeParams)
    next.delete('panel')
    setRouteParams(next, { replace: true, preventScrollReset: true })
  }

  return (
    <div className="classic-posts-page">
      <form className="classic-toolbar" role="search" onSubmit={submitSearch} aria-label="经典树洞工具栏">
        <button type="button" onClick={showLatest}><RefreshCw size={15} />最新</button>
        <button type="button" className={followed ? 'is-active' : ''} disabled={source !== 'live'} onClick={() => setParam('followed', followed ? '' : 'true')}><Star size={14} fill={followed ? 'currentColor' : 'none'} />关注</button>
        <label className="classic-compact-field"><span className="sr-only">内容来源</span><select value={source} onChange={(event) => setSource(event.target.value as 'local' | 'live')}><option value="live">在线树洞</option><option value="local">本地资料</option></select><ChevronDown size={13} /></label>
        <label className="classic-search-field"><span className="sr-only">搜索内容或 PID</span><Search size={14} /><input ref={searchInput} value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} placeholder="搜索内容 或 #PID" /></label>
        <label className="classic-compact-field classic-filter-field"><ListFilter size={13} /><span className="sr-only">标签筛选</span><select value={source === 'live' ? label : localTag} onChange={(event) => setParam(source === 'live' ? 'label' : 'tag', event.target.value)}><option value="">全部标签</option>{source === 'live' ? liveTags.data?.map((tag) => <option key={tag.id} value={tag.id}>{tag.label || tag.name || `标签 ${tag.id}`}</option>) : localTags.data?.map((tag) => <option key={tag.id} value={tag.id}>{tag.name}</option>)}</select><ChevronDown size={13} /></label>
        <label className="classic-compact-field classic-filter-field"><span className="sr-only">图片筛选</span><select value={hasMedia} onChange={(event) => setParam('media', event.target.value)}><option value="">全部内容</option><option value="true">有图片</option><option value="false">无图片</option></select><ChevronDown size={13} /></label>
        <button type="submit" className="classic-search-button">搜索</button>
        <span className="classic-toolbar-spacer" />
        <button type="button" className="classic-toolbar-status" onClick={() => openUtilityDrawer('tasks')} aria-label={`后台任务${activeTasks ? `，${activeTasks} 项进行中` : ''}${attentionTasks ? `，${attentionTasks} 项需要处理` : ''}`}><ListTodo size={14} /><span>任务</span>{(activeTasks > 0 || attentionTasks > 0) && <i className={attentionTasks ? 'has-attention' : ''}>{attentionTasks || activeTasks}</i>}</button>
        <button type="button" className="classic-toolbar-status" onClick={() => openUtilityDrawer('notifications')}><Bell size={14} /><span>消息</span>{unreadMessages > 0 && <i>{unreadMessages > 99 ? '99+' : unreadMessages}</i>}</button>
        <button type="button" onClick={() => openUtilityDrawer('account')}><Settings size={14} />账户</button>
        {source === 'live' && online.data?.can_write_online ? <button type="button" className="classic-publish-button" onClick={() => setPublishOpen(true)}><span>＋</span>发表</button> : <Link to="/sync" className="classic-publish-button"><span>＋</span>{source === 'live' ? '登录' : '同步'}</Link>}
      </form>

      <section className="classic-thread-list" aria-label={q ? `“${q}”的搜索结果` : source === 'live' ? '在线树洞时间线' : '本地资料列表'}>
        {source === 'live' && online.isLoading && <ClassicNotice>正在验证在线会话…</ClassicNotice>}
        {source === 'live' && !online.isLoading && !online.data?.can_read_online && <ClassicNotice tone="warning"><strong>需要先登录树洞</strong><span>{online.data?.message || '当前本机会话不能读取在线内容。'}</span><Link to="/sync">前往登录</Link></ClassicNotice>}
		{source === 'live' && canRead && (query.dataUpdatedAt > 0 || query.error || !browserOnline) && <div className={`classic-freshness${!browserOnline || query.error ? ' is-degraded' : ''}`} role="status"><span className="classic-freshness-dot" /><span><strong>{!browserOnline ? '设备离线' : query.error ? posts.length ? '刷新失败，正在显示上次结果' : '在线内容读取失败' : query.isFetching ? '正在刷新在线内容' : '在线内容已连接'}</strong>{query.dataUpdatedAt ? ` · 最近读取 ${freshnessTime(query.dataUpdatedAt)}` : ' · 暂无可显示的在线结果'}</span><button type="button" disabled={!browserOnline || query.isFetching} onClick={() => query.refetch()}><RefreshCw size={12} />{query.isFetching ? '刷新中' : '刷新'}</button></div>}
        {canRead && query.isLoading && posts.length === 0 && <ClassicNotice>正在读取{q ? `“${q}”的搜索结果` : '帖子'}…</ClassicNotice>}
		{canRead && query.error && posts.length === 0 && <ClassicNotice tone="warning"><strong>读取失败</strong><span>{query.error.message}</span><button type="button" onClick={() => query.refetch()}>重试</button></ClassicNotice>}
        {canRead && !query.isLoading && !query.error && posts.length === 0 && <ClassicNotice><strong>{q ? '没有找到匹配内容' : '这里还没有帖子'}</strong><span>可以清除筛选、切换内容来源，或先同步/导入资料。</span><button type="button" onClick={showLatest}>清除筛选</button></ClassicNotice>}
        {posts.map((post) => <ClassicThread key={post.pid} post={post} source={source} returnTo={returnTo} />)}
        {query.hasNextPage && <div className="classic-load-more"><button type="button" disabled={query.isFetchingNextPage} onClick={() => query.fetchNextPage()}>{query.isFetchingNextPage ? '正在读取…' : '加载更多'}</button></div>}
      </section>

      {publishOpen && <ClassicPublishDrawer onClose={() => setPublishOpen(false)} onPublished={() => query.refetch()} />}
      {utilityDrawer === 'notifications' && <ClassicNotificationsDrawer returnTo={returnTo} onClose={closeUtilityDrawer} />}
      {utilityDrawer === 'account' && <ClassicAccountDrawer onClose={closeUtilityDrawer} />}
      {utilityDrawer === 'tasks' && <ClassicTasksDrawer onClose={closeUtilityDrawer} />}
      {routePID && <ClassicPostDetailDrawer pid={routePID} source={detailSource} returnTo={returnTo} />}
    </div>
  )
}

function ClassicThread({ post, source, returnTo }: { post: PostSummary; source: 'local' | 'live'; returnTo: string }) {
  const navigate = useNavigate()
  const saveLocal = useSavePostToLocal(post.pid)
  const target = classicDetailTarget(post.pid, source, returnTo)
  const previewComments = post.comment_list?.slice(0, 8) ?? []
  const commentMatches = previewComments.length === 0 ? post.comment_matches?.slice(0, 4) ?? [] : []

  function open() {
    rememberClassicListPosition(returnTo)
    navigate(target)
  }

  return (
    <div className="classic-thread-row">
      <span className="classic-thread-arrow" aria-hidden="true">»</span>
      <div className="classic-thread-scroll">
        <article className="classic-root-card" role="link" tabIndex={0} aria-label={`打开树洞 #${post.pid}`} onClick={(event) => { if ((event.target as HTMLElement).closest('a, button, input, select, textarea') || window.getSelection()?.toString()) return; open() }} onKeyDown={(event) => { if (event.key === 'Enter' && event.target === event.currentTarget) open() }}>
          <span className="classic-new-node" aria-hidden="true" />
          <header><Link to={target} onClick={() => rememberClassicListPosition(returnTo)}>#{post.pid}</Link><time>{formatTime(post.timestamp)}</time>{source === 'live' && (!saveLocal.job && post.local_state === 'saved' ? <span className="classic-local-state">已保存</span> : <button type="button" className="classic-inline-save" disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start} title={saveLocal.failure || undefined}>{saveLocal.label}</button>)}<span className="classic-counts"><span>{compactNumber(post.reply)} <MessageCircle size={12} /></span><span>{compactNumber(post.praise_num ?? post.likenum)} <Star size={12} /></span></span></header>
          {source === 'live' && saveLocal.failure && <span className="classic-save-error" role="alert">{saveLocal.failure}</span>}
          <p><HighlightedText value={post.snippet || post.text || '（无正文）'} /></p>
          {post.media_ids && <span className="classic-media-hint"><Image size={13} />含图片</span>}
        </article>
        {previewComments.map((comment) => <ClassicPreviewComment key={comment.cid} comment={comment} target={target} returnTo={returnTo} />)}
        {commentMatches.map((match, index) => <Link className="classic-reply-card" style={{ '--classic-reply-color': classicIdentityColor(`match-${index}`) } as React.CSSProperties} key={match.cid} to={`${target}#comment-${match.cid}`} onClick={() => rememberClassicListPosition(returnTo)}><header><span>#C{match.cid}</span><span>命中评论</span></header><p><HighlightedText value={match.snippet} /></p></Link>)}
      </div>
    </div>
  )
}

function ClassicPreviewComment({ comment, target, returnTo }: { comment: Comment; target: string; returnTo: string }) {
  const name = comment.name_tag || (comment.is_lz ? '洞主' : '匿名')
  return <Link className="classic-reply-card" style={{ '--classic-reply-color': classicIdentityColor(name) } as React.CSSProperties} to={`${target}#comment-${comment.cid}`} onClick={() => rememberClassicListPosition(returnTo)}><header><span>#C{comment.cid}</span><time>{formatTime(comment.timestamp)}</time></header>{comment.quote && <blockquote>{comment.quote.name_tag || '匿名'}：{comment.quote.text}</blockquote>}<p>[{name}] {comment.text || '（无正文）'}</p></Link>
}

function ClassicPostDetailDrawer({ pid, source, returnTo }: { pid: string; source: 'local' | 'live'; returnTo: string }) {
	const { notify } = useFeedback()
	const queryClient = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const drawerPanel = useRef<HTMLElement>(null)
  const closeButton = useRef<HTMLButtonElement>(null)
  const replyInput = useRef<HTMLTextAreaElement>(null)
  const [reverse, setReverse] = useState(false)
  const [tab, setTab] = useState<'comments' | 'personal' | 'references' | 'ai'>('comments')
  const [extraComments, setExtraComments] = useState<Comment[]>([])
  const [page, setPage] = useState<{ cursor?: number; hasMore: boolean } | null>(null)
  const [replyText, setReplyText] = useState('')
  const [quote, setQuote] = useState<Comment>()
  const [replyFiles, setReplyFiles] = useState<File[]>([])
  const { detail, online } = usePostDetailResource(pid, source)
  const loadMore = useMutation({ mutationFn: (cursor: number) => api.comments(pid, cursor, source, 50), onSuccess: (nextPage) => { setExtraComments((current) => dedupeComments([...current, ...nextPage.items])); setPage({ cursor: nextPage.next_cursor, hasMore: nextPage.has_more }) } })
  const interact = usePostInteraction(pid, () => { detail.refetch() })
  const saveLocal = useSavePostToLocal(pid)
  const submitReply = useMutation({ mutationFn: async () => api.createComment(Number(pid), replyText, quote?.cid, await api.uploadMediaIDs(replyFiles)), onSuccess: () => { setReplyText(''); setQuote(undefined); setReplyFiles([]); detail.refetch(); notify({ title: '回复已发送' }) }, onError: (error) => { if (isOnlineSessionError(error)) invalidateOnlineSession(queryClient); notify({ tone: 'error', title: '回复失败，内容已保留', description: errorDescription(error) }) } })
  const closeDetail = useCallback(() => {
    navigate(returnTo)
    requestAnimationFrame(() => document.querySelector<HTMLElement>(`[aria-label="打开树洞 #${pid}"]`)?.focus())
  }, [navigate, pid, returnTo])

  useEffect(() => { setExtraComments([]); setPage(null); setReverse(false); setTab('comments') }, [pid, source])
  useEffect(() => {
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    closeButton.current?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { closeDetail(); return }
      if (event.key !== 'Tab') return
      const controls = Array.from(drawerPanel.current?.querySelectorAll<HTMLElement>('a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled)') ?? []).filter((item) => item.getClientRects().length > 0)
      if (!controls.length) return
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => { document.body.style.overflow = previous; window.removeEventListener('keydown', onKeyDown) }
  }, [closeDetail])

  const comments = useMemo(() => {
    const result = dedupeComments([...(detail.data?.comments ?? []), ...extraComments])
    return reverse ? [...result].reverse() : result
  }, [detail.data?.comments, extraComments, reverse])
  const nextCursor = page?.cursor ?? detail.data?.next_comment_cursor
  const hasMore = page?.hasMore ?? detail.data?.has_more_comments
  const highlightedCID = Number(location.hash.match(/^#comment-(\d+)$/)?.[1] || 0)
	const canWrite = source === 'live' && online.data?.can_write_online === true

  function chooseReply(comment: Comment) {
    setQuote(comment)
    setTab('comments')
    requestAnimationFrame(() => replyInput.current?.focus())
  }

  return (
    <div className="classic-drawer-layer" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) closeDetail() }}>
      <section ref={drawerPanel} className="classic-detail-drawer" role="dialog" aria-modal="true" aria-labelledby="classic-detail-title">
        <header className="classic-drawer-title"><button ref={closeButton} type="button" onClick={closeDetail} aria-label="关闭详情"><X size={17} /></button><h1 id="classic-detail-title">树洞 #{pid}</h1></header>
        <div className="classic-drawer-actions">
          <button type="button" disabled={detail.isFetching} onClick={() => detail.refetch()}><RefreshCw size={14} />刷新</button>
          <button type="button" onClick={() => setReverse((value) => !value)}>⇅ {reverse ? '正序' : '逆序'}</button>
          {source === 'local' && <Link to={`/posts/${pid}?source=live&return_to=${encodeURIComponent(`/posts/${pid}`)}`}>查看线上版本</Link>}
          {canWrite && <><button type="button" disabled={interact.isPending || !detail.data} onClick={() => interact.mutate('praise')}><Heart size={14} fill={detail.data?.post.is_praise ? 'currentColor' : 'none'} />{detail.data?.post.is_praise ? '已赞' : '点赞'}</button><button type="button" disabled={interact.isPending || !detail.data} onClick={() => interact.mutate('follow')}><Star size={14} fill={detail.data?.post.is_follow ? 'currentColor' : 'none'} />{detail.data?.post.is_follow ? '已关注' : '关注'}</button></>}{source === 'live' && (!saveLocal.job && detail.data?.local_state === 'saved' ? <span className="classic-local-state"><Download size={14} />已保存到本地</span> : <button type="button" disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start} title={saveLocal.failure || undefined}><Download size={14} />{saveLocal.label}</button>)}
        </div>
        <nav className="classic-detail-tabs" aria-label="详情区域">
          <button className={tab === 'comments' ? 'is-active' : ''} onClick={() => setTab('comments')}>评论</button>
          <button className={tab === 'personal' ? 'is-active' : ''} onClick={() => setTab('personal')}>个人资料</button>
          <button className={tab === 'references' ? 'is-active' : ''} onClick={() => setTab('references')}>引用</button>
          <button className={tab === 'ai' ? 'is-active' : ''} onClick={() => setTab('ai')}>AI</button>
        </nav>

        <div className="classic-detail-content">
          {detail.isLoading && <ClassicNotice>正在读取 #{pid}…</ClassicNotice>}
          {detail.error && <ClassicNotice tone="warning"><strong>详情读取失败</strong><span>{detail.error.message}</span><button onClick={() => detail.refetch()}>重试</button></ClassicNotice>}
          {detail.data && tab === 'comments' && <>
            <ClassicDetailRoot post={detail.data.post} media={(detail.data.media ?? []).filter((item) => item.owner_type === 'post' && item.owner_id === detail.data?.post.pid)} />
            <div id="classic-comments" className="classic-detail-comments">{comments.map((comment) => <ClassicDetailComment key={comment.cid} comment={comment} media={(detail.data!.media ?? []).filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)} highlighted={comment.cid === highlightedCID} canReply={canWrite} onReply={() => chooseReply(comment)} />)}</div>
            {hasMore && nextCursor !== undefined && <div className="classic-load-more"><button disabled={loadMore.isPending} onClick={() => loadMore.mutate(nextCursor)}>{loadMore.isPending ? '正在读取…' : '加载更多评论'}</button></div>}
          </>}
          {detail.data && tab === 'personal' && (source === 'local' || detail.data.local_state === 'saved' ? <ClassicPersonalPanel pid={Number(pid)} /> : <ClassicNotice><strong>个人资料保存在本机</strong><span>{saveLocal.failure ? `保存失败：${saveLocal.failure}` : '请先把这个在线洞保存到本地，再加入项目、添加标签与笔记。'}</span><button disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start}>{saveLocal.label}</button></ClassicNotice>)}
          {detail.data && tab === 'references' && <ClassicReferences references={detail.data.references} pid={Number(pid)} source={source} returnTo={returnTo} />}
          {detail.data && tab === 'ai' && <ClassicNotice><Sparkles size={22} /><strong>以这个洞为研究范围</strong><span>{source === 'local' || detail.data.local_state === 'saved' ? `使用本地资料中的 #${pid} 创建可持续的研究范围。` : saveLocal.failure ? `保存失败：${saveLocal.failure}` : '分析基于本地资料，请先保存这个在线洞。'}</span>{source === 'local' || detail.data.local_state === 'saved' ? <Link to={`/ai?mode=selected&pids=${pid}`}>研究这个洞</Link> : <button disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start}>{saveLocal.label}</button>}</ClassicNotice>}
        </div>

        {canWrite && tab === 'comments' && <form className="classic-reply-composer" onSubmit={(event) => { event.preventDefault(); if ((replyText.trim() || replyFiles.length) && !submitReply.isPending) submitReply.mutate() }}>
          {quote && <div className="classic-replying-to"><span>回复 {quote.name_tag || `C${quote.cid}`}</span><button type="button" onClick={() => setQuote(undefined)} aria-label="取消引用"><X size={13} /></button></div>}
          <textarea ref={replyInput} value={replyText} maxLength={10000} onChange={(event) => setReplyText(event.target.value)} placeholder={quote ? `回复 ${quote.name_tag || `C${quote.cid}`}…` : '写下回复…'} />
          <div><label><ImagePlus size={16} /><span>{replyFiles.length ? `${replyFiles.length} 张图片` : '图片'}</span><input type="file" accept="image/*" multiple onChange={(event) => setReplyFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><span>{submitReply.error ? String(submitReply.error) : `${replyText.length}/10000`}</span><button type="submit" disabled={submitReply.isPending || (!replyText.trim() && !replyFiles.length)}><Send size={15} />{submitReply.isPending ? '发送中…' : '回复'}</button></div>
        </form>}
		{source === 'live' && tab === 'comments' && !canWrite && !online.isLoading && <div className="classic-reply-composer"><span className="classic-error-text">当前会话不能回复。请前往同步中心重新检测或登录。</span><Link to="/sync">登录</Link></div>}
      </section>
    </div>
  )
}

function ClassicDetailRoot({ post, media }: { post: PostSummary; media: Media[] }) {
  return <article className="classic-detail-root"><header><span>#{post.pid}</span><time>{formatTime(post.timestamp)}</time><span>{compactNumber(post.reply)} <MessageCircle size={12} />　{compactNumber(post.praise_num ?? post.likenum)} <Star size={12} /></span></header><p>{post.text || '（无正文）'}</p><ClassicMedia items={media} pid={post.pid} /></article>
}

function ClassicDetailComment({ comment, media, highlighted, canReply, onReply }: { comment: Comment; media: Media[]; highlighted: boolean; canReply: boolean; onReply: () => void }) {
  const name = comment.name_tag || (comment.is_lz ? '洞主' : '匿名')
  return <article id={`comment-${comment.cid}`} className={`classic-detail-comment${highlighted ? ' is-highlighted' : ''}`} style={{ '--classic-reply-color': classicIdentityColor(name) } as React.CSSProperties}><header><span>#C{comment.cid}</span><span>{name}</span><time>{formatTime(comment.timestamp)}</time>{canReply && <button type="button" onClick={onReply}><Reply size={13} />回复</button>}</header>{comment.quote && <blockquote><strong>{comment.quote.name_tag || `C${comment.quote.cid}`}</strong>　{comment.quote.text}</blockquote>}<p>{comment.text || '（无正文）'}</p><ClassicMedia items={media} pid={comment.pid} /></article>
}

function ClassicPersonalPanel({ pid }: { pid: number }) {
  const metadata = useLocalPostMetadata(pid)
  return <section className="classic-personal-panel"><h2>研究项目</h2><div className="classic-personal-tags">{metadata.projects.data?.length ? metadata.projects.data.map((project) => <label key={project.id}><input type="checkbox" checked={metadata.selectedProjectIDs.includes(project.id)} onChange={() => metadata.setSelectedProjectIDs((value) => value.includes(project.id) ? value.filter((id) => id !== project.id) : [...value, project.id])} /><span style={{ backgroundColor: project.color || '#287f78' }} />{project.name}</label>) : <p>尚未创建研究项目。<Link to="/projects">前往创建</Link></p>}</div><button disabled={metadata.projects.isLoading || metadata.assignedProjects.isLoading || metadata.saveProjects.isPending} onClick={() => metadata.saveProjects.mutate()}>保存项目归属</button><h2>本地标签与笔记</h2><div className="classic-personal-tags">{metadata.tags.data?.length ? metadata.tags.data.map((tag) => <label key={tag.id}><input type="checkbox" checked={metadata.selected.includes(tag.id)} onChange={() => metadata.setSelected((value) => value.includes(tag.id) ? value.filter((id) => id !== tag.id) : [...value, tag.id])} /><span style={{ backgroundColor: tag.color || '#8a929a' }} />{tag.name}</label>) : <p>尚未创建本地标签。</p>}</div><button disabled={metadata.saveTags.isPending} onClick={() => metadata.saveTags.mutate()}>保存标签</button><label className="classic-note-field">个人笔记<textarea value={metadata.content} onChange={(event) => metadata.setContent(event.target.value)} placeholder="只保存在本机的笔记…" /></label><button disabled={metadata.saveNote.isPending} onClick={() => metadata.saveNote.mutate()}>{metadata.saveNote.isSuccess ? '笔记已保存' : '保存笔记'}</button></section>
}

function ClassicReferences({ references, pid, source, returnTo }: { references: Reference[]; pid: number; source: 'local' | 'live'; returnTo: string }) {
  if (!references.length) return <ClassicNotice><strong>尚未发现引用关系</strong><span>明确 PID 引用和评论引用会显示在这里。</span></ClassicNotice>
  return <section className="classic-reference-list"><h2>与 #{pid} 有关的引用</h2>{references.map((reference, index) => { const outbound = reference.source_pid === pid; const otherPID = outbound ? reference.target_pid : reference.source_pid; const otherCID = outbound ? reference.target_cid : reference.source_cid; return <Link key={`${reference.kind}-${index}`} to={`${classicDetailTarget(otherPID, source, returnTo)}${otherCID ? `#comment-${otherCID}` : ''}`}><span>{outbound ? '引用了' : '被引用'}</span><strong>#{otherPID}{otherCID ? ` / C${otherCID}` : ''}</strong><small>{reference.kind}</small></Link>})}</section>
}

function ClassicPublishDrawer({ onClose, onPublished }: { onClose: () => void; onPublished: () => void }) {
	const { notify } = useFeedback()
	const queryClient = useQueryClient()
  const [text, setText] = useState(readPublishDraft)
  const [files, setFiles] = useState<File[]>([])
  const [previews, setPreviews] = useState<string[]>([])
  useEffect(() => writePublishDraft(text), [text])
  useEffect(() => {
    if (typeof URL.createObjectURL !== 'function') return
    const urls = files.map((file) => URL.createObjectURL(file))
    setPreviews(urls)
    return () => urls.forEach((url) => URL.revokeObjectURL(url))
  }, [files])
  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => { if (text.trim() || files.length) { event.preventDefault(); event.returnValue = '' } }
    window.addEventListener('beforeunload', beforeUnload)
    return () => window.removeEventListener('beforeunload', beforeUnload)
  }, [text, files.length])
  const publish = useMutation({ mutationFn: async () => api.createPost(text, await api.uploadMediaIDs(files)), onSuccess: (post) => { writePublishDraft(''); onPublished(); onClose(); notify({ title: `树洞 #${post.pid} 已发表` }) }, onError: (error) => { if (isOnlineSessionError(error)) invalidateOnlineSession(queryClient); notify({ tone: 'error', title: '发表失败，草稿已保留', description: errorDescription(error) }) } })
  const clearDraft = () => { setText(''); setFiles([]); writePublishDraft('') }
  return <ClassicDrawerFrame title="发表新洞" onClose={onClose} compact><div className="classic-publish-form"><p>内容会发送到当前登录的在线树洞。关闭后正文会保留在本次浏览器会话中，只有点击“确认发表”才会执行。</p><textarea value={text} maxLength={10000} onChange={(event) => setText(event.target.value)} placeholder="写下想发布的内容…" />{files.length > 0 && <div className="classic-publish-previews">{files.map((file, index) => <figure key={`${file.name}-${file.lastModified}`}><div>{previews[index] ? <img src={previews[index]} alt={`待发布图片 ${file.name}`} /> : <Image size={20} />}</div><figcaption title={file.name}>{file.name}</figcaption><button type="button" onClick={() => setFiles((current) => current.filter((_, fileIndex) => fileIndex !== index))} aria-label={`移除图片 ${file.name}`}><X size={13} /></button></figure>)}</div>}<label><ImagePlus size={17} />选择图片（最多 9 张）<input type="file" accept="image/*" multiple onChange={(event) => setFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><span>{files.length ? `已选择 ${files.length} 张图片 · ` : ''}{text.length}/10000{text && ' · 草稿已保存'}</span>{publish.error && <p className="classic-error-text">发表失败：{errorDescription(publish.error)}。正文和图片选择已保留。</p>}<div className="classic-publish-actions"><button type="button" disabled={publish.isPending || (!text && !files.length)} onClick={clearDraft}>清空草稿</button><button disabled={publish.isPending || (!text.trim() && !files.length)} onClick={() => publish.mutate()}><Send size={16} />{publish.isPending ? '正在发表…' : '确认发表'}</button></div></div></ClassicDrawerFrame>
}

function ClassicMedia({ items, pid }: { items: Media[]; pid: number }) {
  if (!items.length) return null
  return <div className="classic-media-grid">{items.map((item, index) => item.status === 'available' || item.status === 'remote' ? <a key={`${item.id}-${index}`} href={item.status === 'remote' ? `/api/v1/remote-media/${item.remote_id || '_'}?pid=${pid}` : `/api/v1/media/${item.id}`} target="_blank" rel="noreferrer" aria-label={`在新窗口查看树洞 #${pid} 的第 ${index + 1} 张图片`}><img loading="lazy" alt={`树洞 #${pid} 的图片 ${index + 1}`} src={item.status === 'remote' ? `/api/v1/remote-media/${item.remote_id || '_'}?pid=${pid}` : `/api/v1/media/${item.id}`} /></a> : <div key={`${item.id}-${index}`}><Image size={17} />图片尚未下载</div>)}</div>
}

function ClassicNotice({ children, tone = 'neutral' }: { children: React.ReactNode; tone?: 'neutral' | 'warning' }) {
  return <div className={`classic-notice classic-notice--${tone}`}>{children}</div>
}

function paramsFromReturnTo(returnTo: string) {
  const question = returnTo.indexOf('?')
  return new URLSearchParams(question >= 0 ? returnTo.slice(question + 1) : '')
}

function safeReturnTo(value: string | null) {
  return value && /^\/(online|posts)(?:[/?]|$)/.test(value) ? value : '/posts'
}

function classicDetailTarget(pid: number, source: 'local' | 'live', returnTo: string) {
  const params = new URLSearchParams()
  if (source === 'live') params.set('source', 'live')
  params.set('return_to', returnTo)
  return `/posts/${pid}?${params}`
}

function rememberClassicListPosition(key: string) {
  try { sessionStorage.setItem(`studio:list-scroll:${key}`, String(window.scrollY)) } catch { /* best effort */ }
}

function useRestoreClassicListScroll(key: string, ready: boolean) {
  const restored = useRef('')
  useEffect(() => {
    if (!ready || restored.current === key) return
    restored.current = key
    try {
      const stored = sessionStorage.getItem(`studio:list-scroll:${key}`)
      if (!stored) return
      sessionStorage.removeItem(`studio:list-scroll:${key}`)
      requestAnimationFrame(() => window.scrollTo({ top: Number(stored) || 0 }))
    } catch { /* best effort */ }
  }, [key, ready])
}

function dedupeComments(comments: Comment[]) {
  return comments.filter((comment, index, all) => all.findIndex((item) => item.cid === comment.cid) === index)
}

const CLASSIC_PUBLISH_DRAFT_KEY = 'pkustudio:classic:publish-draft'
function readPublishDraft() {
  try { return sessionStorage.getItem(CLASSIC_PUBLISH_DRAFT_KEY) || '' } catch { return '' }
}
function writePublishDraft(value: string) {
  try { if (value) sessionStorage.setItem(CLASSIC_PUBLISH_DRAFT_KEY, value); else sessionStorage.removeItem(CLASSIC_PUBLISH_DRAFT_KEY) } catch { /* best effort */ }
}

const identityColors = ['#e9f5dd', '#f5deec', '#dce9f6', '#f4f0d7', '#eadcf4', '#d8f0ef']
function classicIdentityColor(value: string) {
  let hash = 0
  for (let index = 0; index < value.length; index++) hash = (hash * 31 + value.charCodeAt(index)) | 0
  return identityColors[Math.abs(hash) % identityColors.length]
}
