import { useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import {
  ArrowLeftIcon,
  CheckIcon,
  CommentIcon,
  CopilotIcon,
  DownloadIcon,
  HeartIcon,
  ImageIcon,
  IssueOpenedIcon,
  LinkExternalIcon,
  PaperclipIcon,
  ReplyIcon,
  StarIcon,
  TagIcon,
} from '@primer/octicons-react'
import type { Comment, Media, Reference } from '../lib/types'
import { formatTime } from '../lib/format'
import { preferredScrollBehavior } from '../lib/motion'
import { errorDescription } from '../components/Feedback'
import { useCommentNote, useLocalPostMetadata, usePostComments, usePostDetailResource, usePostInteraction, usePostReply, useSavePostToLocal } from '../features/posts/usePostDetail'
import { GithubLoading, GithubPageHeader, GithubState } from './GithubComponents'

export function GithubPostDetailPage() {
  const { pid = '' } = useParams()
  const [searchParams] = useSearchParams()
  const source = searchParams.get('source') === 'live' ? 'live' : 'local'
  const { detail, online } = usePostDetailResource(pid, source)
  const commentState = usePostComments(pid, source, detail.data, detail.dataUpdatedAt)
  const saveLocal = useSavePostToLocal(pid)
  const reply = usePostReply(pid, () => { detail.refetch() })
  const interact = usePostInteraction(pid, () => { detail.refetch() })
  const composer = useRef<HTMLElement>(null)
  const replyInput = useRef<HTMLTextAreaElement>(null)

  if (detail.isLoading) return <GithubLoading label={`正在读取 #${pid}…`} />
  if (detail.error || !detail.data) return <GithubState tone="danger" title="无法打开这个帖子" description={detail.error?.message || '帖子不存在'} />

  const { post, references, media = [] } = detail.data
  const canWrite = source === 'live' && online.data?.can_write_online === true
  const requestedReturn = searchParams.get('return_to')
  const returnTo = requestedReturn && /^\/(online|posts|search|ai|notifications)([/?]|$)/.test(requestedReturn) ? requestedReturn : `/posts${source === 'live' ? '?source=live' : ''}`
  const postMedia = media.filter((item) => item.owner_type === 'post' && item.owner_id === post.pid)

  function quoteComment(cid: number) {
    reply.setQuoteCID(cid)
    replyInput.current?.focus()
    requestAnimationFrame(() => composer.current?.scrollIntoView?.({ behavior: preferredScrollBehavior(), block: 'center' }))
  }

  return <>
    <Link className="github-back-link" to={returnTo}><ArrowLeftIcon size={16} />返回列表</Link>
    <GithubPageHeader
      eyebrow={source === 'live' ? 'ONLINE TREEHOLE' : 'LOCAL ARCHIVE'}
      title={<>树洞 #{post.pid}</>}
      description={<span className="github-detail-status"><IssueOpenedIcon size={16} />{source === 'live' ? '在线内容' : '本地资料'} · {post.reply ?? commentState.comments.length} 条评论</span>}
      actions={<>{source === 'local' && <Link className="github-button" to={`/posts/${pid}?source=live&return_to=${encodeURIComponent(`/posts/${pid}`)}`}><LinkExternalIcon size={15} />查看线上版本</Link>}{source === 'live' && (!saveLocal.job && detail.data.local_state === 'saved' ? <span className="github-button"><CheckIcon size={15} />已保存到本地</span> : <button className="github-button" disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start} title={saveLocal.failure || undefined}><DownloadIcon size={15} />{saveLocal.label}</button>)}{canWrite && <><button className="github-button" disabled={interact.isPending} onClick={() => interact.mutate('praise')}><HeartIcon size={15} />{post.is_praise ? '取消点赞' : '点赞'}</button><button className="github-button" disabled={interact.isPending} onClick={() => interact.mutate('follow')}><StarIcon size={15} />{post.is_follow ? '取消关注' : '关注'}</button></>}</>}
    />

    {source === 'live' && saveLocal.failure && <div className="github-inline-error github-save-detail-error" role="alert">保存任务失败：{saveLocal.failure} <Link to="/tasks">查看任务详情</Link></div>}
    <div className="github-conversation-layout">
      <section className="github-conversation" aria-label="帖子讨论">
        <GithubConversationPost text={post.text || '（无正文）'} timestamp={post.timestamp} media={postMedia} pid={post.pid} />

        <div className="github-timeline-break"><span><CommentIcon size={15} />{commentState.comments.length} 条评论</span></div>

        {commentState.restoreStatus && <div className="github-restore-notice"><span>{commentState.restoreStatus}</span>{commentState.restoreStatus.startsWith('正在') && <button className="github-link-button" onClick={commentState.cancelRestore}>取消</button>}</div>}

        {commentState.comments.length ? commentState.comments.map((comment) => <GithubCommentCard key={comment.cid} comment={comment} source={source} media={media} pid={post.pid} canReply={canWrite} quote={() => quoteComment(comment.cid)} />) : <GithubState title="还没有评论" description={source === 'local' ? '这份本地资料中没有保存评论。' : '成为第一个回复此洞的人。'} />}

        {commentState.hasMoreComments && commentState.nextCommentCursor !== undefined && <button className="github-button github-load-comments" disabled={commentState.loadMoreComments.isPending} onClick={() => commentState.loadMoreComments.mutate(commentState.nextCommentCursor!)}>{commentState.loadMoreComments.isPending ? '正在加载…' : '加载更多评论'}</button>}
        {commentState.loadMoreComments.error && <div className="github-inline-error" role="alert">加载评论失败：{errorDescription(commentState.loadMoreComments.error)}</div>}

        {canWrite ? <section ref={composer} className="github-reply-box">
          <div className="github-avatar">P</div>
          <div>
            <header><strong>发表回复</strong>{reply.quoteCID && <button className="github-link-button" onClick={() => reply.setQuoteCID(undefined)}>正在引用 C{reply.quoteCID} ×</button>}</header>
            <textarea ref={replyInput} value={reply.text} maxLength={10000} onChange={(event) => reply.setText(event.target.value)} placeholder={reply.quoteCID ? `回复并引用 C${reply.quoteCID}…` : '写下回复…'} aria-label="回复内容" />
            {reply.mutation.error && <div className="github-inline-error">回复失败：{errorDescription(reply.mutation.error)}。正文和图片选择已保留。</div>}
            <footer><label className="github-button"><PaperclipIcon size={15} />添加图片<input type="file" accept="image/*" multiple onChange={(event) => reply.setFiles(Array.from(event.target.files ?? []).slice(0, 9))} /></label><span>{reply.files.length ? `${reply.files.length} 张图片 · ` : ''}{reply.text.length}/10000</span><button className="github-button github-button--primary" disabled={reply.mutation.isPending || (!reply.text.trim() && !reply.files.length)} onClick={() => reply.mutation.mutate()}>{reply.mutation.isPending ? '正在发送…' : '回复'}</button></footer>
          </div>
        </section> : source === 'live' && !online.isLoading ? <GithubState title="登录后参与互动" description="当前会话不能回复、点赞或关注此洞。" action={<Link className="github-button github-button--primary" to="/sync">前往登录</Link>} /> : null}
      </section>

      <aside className="github-detail-sidebar">
        {(source === 'local' || detail.data.local_state === 'saved') && <GithubLocalMetadata pid={post.pid} />}
        <section><h2><TagIcon size={15} />来源</h2><p>{source === 'live' ? '在线树洞' : '本地资料库'}</p></section>
        {source === 'local' ? <GithubReferences references={references} currentPID={post.pid} /> : <section><h2>引用关系</h2><p>引用关系保存在本地资料库；先保存此洞，完成同步后即可建立关系。</p></section>}
        <section><h2><CopilotIcon size={15} />AI 研究</h2><p>{source === 'local' || detail.data.local_state === 'saved' ? '以这个洞为线索，在本地资料中查找相关证据。' : saveLocal.failure ? `保存失败：${saveLocal.failure}` : '分析基于本地资料，请先保存这个在线洞。'}</p>{source === 'local' || detail.data.local_state === 'saved' ? <Link className="github-button" to={`/ai?mode=selected&pids=${post.pid}`}>打开 AI 研究台</Link> : <button className="github-button" disabled={saveLocal.isPending || saveLocal.active || saveLocal.isSuccess} onClick={saveLocal.start}>{saveLocal.label}</button>}</section>
      </aside>
    </div>
  </>
}

function GithubConversationPost({ text, timestamp, media, pid }: { text: string; timestamp?: number; media: Media[]; pid: number }) {
  return <article className="github-comment github-comment--root"><div className="github-avatar github-avatar--root">P</div><div className="github-comment-box"><header><div><strong>洞主</strong><span>发布于 {formatTime(timestamp)}</span></div><span className="github-author-badge">Author</span></header><div className="github-comment-body"><p>{text}</p><GithubMedia items={media} pid={pid} /></div></div></article>
}

function GithubCommentCard({ comment, source, media, pid, canReply, quote }: { comment: Comment; source: 'local' | 'live'; media: Media[]; pid: number; canReply: boolean; quote: () => void }) {
  const [showNote, setShowNote] = useState(false)
  const note = useCommentNote(comment.cid, source === 'local' && showNote)
  const items = media.filter((item) => item.owner_type === 'comment' && item.owner_id === comment.cid)
  const commentMedia = items.length || source === 'local' ? items : remoteCommentMedia(comment)
  return <article id={`comment-${comment.cid}`} className="github-comment">
    <div className="github-avatar">{(comment.name_tag || 'C').slice(0, 1).toUpperCase()}</div>
    <div className="github-comment-box">
      <header><div><strong>{comment.name_tag || '匿名'}</strong><span>评论于 {formatTime(comment.timestamp)} · C{comment.cid}</span></div>{comment.is_lz && <span className="github-author-badge">Author</span>}</header>
      <div className="github-comment-body">
        {comment.quote && <blockquote>引用 C{comment.quote.cid}：{comment.quote.text}</blockquote>}
        <p>{comment.text || '（无正文）'}</p>
        <GithubMedia items={commentMedia} pid={pid} />
        {showNote && <div className="github-comment-note" aria-busy={note.note.isLoading}>
          <textarea value={note.content} disabled={note.note.isLoading} onChange={(event) => note.setContent(event.target.value)} aria-label={`C${comment.cid} 的本地笔记`} placeholder={note.note.isLoading ? '正在读取笔记…' : '只保存在本机的评论笔记…'} />
          {note.note.error && <p className="github-inline-error" role="alert">读取笔记失败：{errorDescription(note.note.error)}</p>}
          <button className="github-button" disabled={note.save.isPending || note.note.isLoading || Boolean(note.note.error)} onClick={() => note.save.mutate()}>{note.save.isSuccess ? '已保存' : note.save.isPending ? '正在保存…' : '保存笔记'}</button>
        </div>}
      </div>
      <footer>{source === 'local' && <button className="github-link-button" onClick={() => setShowNote((value) => !value)}>笔记</button>}{canReply && <button className="github-link-button" onClick={quote}><ReplyIcon size={13} />引用回复</button>}</footer>
    </div>
  </article>
}

function GithubLocalMetadata({ pid }: { pid: number }) {
  const metadata = useLocalPostMetadata(pid)
  const tagLoading = metadata.tags.isLoading || metadata.assigned.isLoading
  const tagError = metadata.tags.error || metadata.assigned.error
  return <section className="github-metadata" aria-busy={tagLoading || metadata.note.isLoading}>
    <h2><TagIcon size={15} />研究项目</h2>
    {(metadata.projects.error || metadata.assignedProjects.error) && <p className="github-inline-error" role="alert">读取研究项目失败：{errorDescription(metadata.projects.error || metadata.assignedProjects.error)}</p>}
    <div className="github-metadata-tags">
      {metadata.projects.isLoading || metadata.assignedProjects.isLoading ? <p>正在读取项目…</p> : metadata.projects.data?.length ? metadata.projects.data.map((project) => <label key={project.id}><input type="checkbox" checked={metadata.selectedProjectIDs.includes(project.id)} onChange={() => metadata.setSelectedProjectIDs((current) => current.includes(project.id) ? current.filter((id) => id !== project.id) : [...current, project.id])} /><span style={{ background: project.color || 'var(--bgColor-accent-emphasis)' }} />{project.name}</label>) : <p>尚未创建项目。<Link to="/projects">前往创建</Link></p>}
    </div>
    <button className="github-button" disabled={metadata.projects.isLoading || metadata.assignedProjects.isLoading || metadata.saveProjects.isPending} onClick={() => metadata.saveProjects.mutate()}>{metadata.saveProjects.isPending ? '正在保存…' : metadata.saveProjects.isSuccess ? '项目已更新' : '保存项目归属'}</button>
    <h2><TagIcon size={15} />本地标签</h2>
    {tagError && <p className="github-inline-error" role="alert">读取标签失败：{errorDescription(tagError)}</p>}
    <div className="github-metadata-tags">
      {tagLoading ? <p>正在读取标签…</p> : metadata.tags.data?.length ? metadata.tags.data.map((tag) => <label key={tag.id}><input type="checkbox" disabled={Boolean(tagError)} checked={metadata.selected.includes(tag.id)} onChange={() => metadata.setSelected((current) => current.includes(tag.id) ? current.filter((id) => id !== tag.id) : [...current, tag.id])} /><span style={{ background: tag.color || 'var(--bgColor-accent-emphasis)' }} />{tag.name}</label>) : <p>尚未创建标签。</p>}
    </div>
    <button className="github-button" disabled={tagLoading || Boolean(tagError) || metadata.saveTags.isPending} onClick={() => metadata.saveTags.mutate()}>{metadata.saveTags.isPending ? '正在保存…' : metadata.saveTags.isSuccess ? '标签已保存' : '保存标签'}</button>
    <h2>个人笔记</h2>
    {metadata.note.error && <p className="github-inline-error" role="alert">读取笔记失败：{errorDescription(metadata.note.error)}</p>}
    <textarea value={metadata.content} disabled={metadata.note.isLoading || Boolean(metadata.note.error)} onChange={(event) => metadata.setContent(event.target.value)} placeholder={metadata.note.isLoading ? '正在读取笔记…' : '只保存在本机的笔记…'} />
    <button className="github-button" disabled={metadata.note.isLoading || Boolean(metadata.note.error) || metadata.saveNote.isPending} onClick={() => metadata.saveNote.mutate()}>{metadata.saveNote.isPending ? '正在保存…' : metadata.saveNote.isSuccess ? '笔记已保存' : '保存笔记'}</button>
  </section>
}

function GithubReferences({ references, currentPID }: { references: Reference[]; currentPID: number }) {
  return <section><h2>引用关系</h2>{references.length ? <div className="github-reference-list">{references.slice(0, 12).map((reference, index) => { const outbound = reference.source_pid === currentPID; const pid = outbound ? reference.target_pid : reference.source_pid; const cid = outbound ? reference.target_cid : reference.source_cid; return <Link key={`${reference.source_pid}-${reference.target_pid}-${index}`} to={`/posts/${pid}${cid ? `#comment-${cid}` : ''}`}><span>{outbound ? '引用了' : '被引用'}</span>#{pid}{cid ? ` / C${cid}` : ''}</Link> })}</div> : <p>还没有发现引用关系。</p>}</section>
}

function GithubMedia({ items, pid }: { items: Media[]; pid: number }) {
  if (!items.length) return null
  return <div className="github-media-grid">{items.map((item, index) => item.status === 'available' || item.status === 'remote' ? <a key={`${item.id}-${index}`} href={mediaURL(item, pid)} target="_blank" rel="noreferrer" aria-label={`在新窗口查看树洞 #${pid} 的第 ${index + 1} 张图片`}><img loading="lazy" alt={`树洞 #${pid} 的图片 ${index + 1}`} src={mediaURL(item, pid)} /></a> : <div key={`${item.id}-${index}`}><ImageIcon size={17} />图片尚未下载</div>)}</div>
}

function mediaURL(item: Media, pid: number) {
  return item.status === 'remote' ? `/api/v1/remote-media/${item.remote_id || '_'}?pid=${pid}` : `/api/v1/media/${item.id}`
}

function remoteCommentMedia(comment: Comment): Media[] {
  return (comment.media_ids ?? '').split(/[;,\s]+/).map((value) => value.trim()).filter(Boolean).map((remoteID, index) => ({ id: -(comment.cid * 100 + index + 1), owner_type: 'comment', owner_id: comment.cid, remote_id: remoteID, variant: 'original', status: 'remote' }))
}
