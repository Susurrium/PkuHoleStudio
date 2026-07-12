import type { AIProvider, AISession, AISessionDetail, AuthStatus, BridgePairing, Capabilities, Comment, CommentPage, CourseScheduleRow, ExportDownload, Health, HotPost, ImportCreated, Job, LocalTag, LogLine, Note, NotificationPage, Post, PostDetail, PostPage, ScoreSummary, SearchHistory, Settings, SettingsUpdate, Tag, UploadedMedia } from './types'

interface Envelope<T> { data: T }
interface ErrorEnvelope { error?: { code?: string; message?: string; details?: unknown } }

export class APIError extends Error {
  constructor(public status: number, public code: string, message: string, public details?: unknown) {
    super(message)
    this.name = 'APIError'
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, init)
  const contentType = response.headers.get('content-type') ?? ''
  const body = contentType.includes('application/json') ? await response.json() : null
  if (!response.ok) {
    const failure = body as ErrorEnvelope | null
    throw new APIError(response.status, failure?.error?.code ?? 'request_failed', failure?.error?.message ?? `请求失败 (${response.status})`, failure?.error?.details)
  }
  return (body as Envelope<T>).data
}

function queryString(values: Record<string, string | number | boolean | undefined | null>) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  }
  const encoded = params.toString()
  return encoded ? `?${encoded}` : ''
}

export const api = {
  health: () => request<Health>('/health'),
	hotPosts: () => request<HotPost[]>('/posts/hot'),
  capabilities: () => request<Capabilities>('/capabilities'),
  posts: (params: Record<string, string | number | boolean | undefined | null>) => request<PostPage>(`/posts${queryString(params)}`),
	search: (params: Record<string, string | number | boolean | undefined | null>) => request<PostPage>(`/search${queryString(params)}`),
	searchHistory: () => request<SearchHistory[]>('/search/history?limit=12'),
  post: (pid: string | number, source: 'local' | 'live' = 'local') => request<PostDetail>(`/posts/${pid}${queryString({ source })}`),
  comments: (pid: string | number, cursor = 0, source: 'local' | 'live' = 'local') => request<CommentPage>(`/posts/${pid}/comments${queryString({ cursor, source })}`),
	tags: () => request<Tag[]>('/tags?source=live'),
	uploadMedia: (file: File) => { const body = new FormData(); body.append('file', file); return request<UploadedMedia>('/media/uploads', { method: 'POST', body }) },
	createPost: (text: string, mediaIDs: string[]) => request<Post>('/posts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text, media_ids: mediaIDs }) }),
	createComment: (pid: number, text: string, quoteCID: number | undefined, mediaIDs: string[]) => request<Comment>(`/posts/${pid}/comments`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text, quote_cid: quoteCID, media_ids: mediaIDs }) }),
	togglePost: (pid: number, action: 'praise' | 'follow') => request<Post | { pid: number; updated: boolean }>(`/posts/${pid}/${action}`, { method: 'POST' }),
	notifications: (type: 'interactive' | 'system') => request<NotificationPage>(`/notifications?type=${type}&limit=50`),
	markNotificationRead: (id: number) => request<{ id: number; read: boolean }>(`/notifications/${id}/read`, { method: 'POST' }),
	markAllNotificationsRead: (type: 'interactive' | 'system') => request<{ type: string; read: boolean }>(`/notifications/read-all?type=${type}`, { method: 'POST' }),
	logs: (module: string, q: string) => request<LogLine[]>(`/logs${queryString({ module, q, limit: 1000 })}`),
	clearLogs: (module: string) => request<{ cleared: boolean }>(`/logs/clear${queryString({ module })}`, { method: 'POST' }),
	campusSchedule: () => request<CourseScheduleRow[]>('/campus/schedule'),
	campusScores: () => request<ScoreSummary>('/campus/scores'),
	localTags: () => request<LocalTag[]>('/local-tags'),
	createLocalTag: (name: string, color: string) => request<LocalTag>('/local-tags', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, color }) }),
	updateLocalTag: (id: number, name: string, color: string) => request<LocalTag>(`/local-tags/${id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, color }) }),
	deleteLocalTag: (id: number) => request<{ deleted: boolean }>(`/local-tags/${id}`, { method: 'DELETE' }),
	settings: () => request<Settings>('/settings'),
	updateSettings: (update: SettingsUpdate) => request<Settings>('/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update) }),
	postTags: (pid: number) => request<LocalTag[]>(`/posts/${pid}/tags`),
	setPostTags: (pid: number, tagIDs: number[]) => request<LocalTag[]>(`/posts/${pid}/tags`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ tag_ids: tagIDs }) }),
	postNote: (pid: number) => request<Note>(`/posts/${pid}/note`),
	savePostNote: (pid: number, content: string) => request<Note>(`/posts/${pid}/note`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ content }) }),
  jobs: () => request<Job[]>('/jobs?limit=50'),
  job: (id: string) => request<Job>(`/jobs/${id}`),
  createJob: (type: string, payload: unknown = {}) => request<Job>('/jobs', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ type, payload }),
  }),
  jobAction: (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry') => request<Job>(`/jobs/${id}/${action}`, { method: 'POST' }),
	session: () => request<AuthStatus>('/session'),
	probeSession: () => request<AuthStatus>('/session/probe', { method: 'POST' }),
	loginSession: (username: string, password: string) => request<AuthStatus>('/session/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, password }) }),
	sendSessionSMS: (username: string) => request<AuthStatus>('/session/sms', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username }) }),
	continueSession: (stage: 'iaaa' | 'treehole' | '', challenge: 'sms' | 'otp', username: string, password: string, code: string) => request<AuthStatus>('/session/challenge', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stage, challenge, username: stage === 'iaaa' ? username : undefined, password: stage === 'iaaa' ? password : undefined, code }) }),
	logoutSession: () => request<AuthStatus>('/session/logout', { method: 'POST' }),
	importArchive: (file: File) => {
    const body = new FormData()
    body.append('file', file)
    return request<ImportCreated>('/imports', { method: 'POST', body })
	},
	createBridgePairing: () => request<BridgePairing>('/bridge/pairings', { method: 'POST' }),
	bridgePairing: (token: string) => request<BridgePairing>(`/bridge/pairings/${token}`),
	confirmBridgePairing: (token: string) => request<BridgePairing>(`/bridge/pairings/${token}/confirm`, { method: 'POST' }),
	cancelBridgePairing: (token: string) => request<{ status: string }>(`/bridge/pairings/${token}/cancel`, { method: 'POST' }),
	exportArchive: async (format: 'treehole-v2' | 'markdown', pids: number[], includeComments: boolean): Promise<ExportDownload> => {
		const response = await fetch('/api/v1/exports', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ format, pids: pids.length ? pids : undefined, include_comments: includeComments }) })
		if (!response.ok) {
			const failure = await response.json().catch(() => null) as ErrorEnvelope | null
			throw new APIError(response.status, failure?.error?.code ?? 'export_failed', failure?.error?.message ?? `导出失败 (${response.status})`, failure?.error?.details)
		}
		const disposition = response.headers.get('content-disposition') ?? ''
		const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? (format === 'markdown' ? 'pkuhole-studio-markdown.zip' : 'pkuhole-studio.treehole.zip')
		return { blob: await response.blob(), filename }
	},
	aiProviders: () => request<AIProvider[]>('/ai/providers'),
	aiSessions: () => request<AISession[]>('/ai/sessions?limit=50'),
	createAISession: (mode: AISession['mode'], title: string) => request<AISession>('/ai/sessions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mode, title }) }),
	aiSession: (id: string) => request<AISessionDetail>(`/ai/sessions/${id}`),
	startAIMessage: (id: string, body: { prompt: string; pids?: number[]; course?: string; teachers?: string[] }) => request<{ session_id: string; status: string }>(`/ai/sessions/${id}/messages`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
	cancelAI: (id: string) => request<{ status: string }>(`/ai/sessions/${id}/cancel`, { method: 'POST' }),
}
