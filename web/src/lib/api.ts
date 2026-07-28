import type { AIProvider, AIProviderSettingUpdate, AIScope, AISession, AISessionDetail, AuthStatus, BridgeDevice, BridgeDeviceRequest, BridgePairing, BridgeTransfer, Capabilities, Comment, CommentPage, CourseScheduleRow, ExportDownload, Health, HotPostsResult, ImportCreated, Job, LocalTag, LogLine, Note, NotificationPage, ObserverConnectionProbe, ObserverSettings, ObserverSettingsUpdate, ObserverStatus, ObserverSyncResult, Post, PostDetail, PostPage, ReferenceGraph, RemovedPostDetail, RemovedPostPage, ScoreSummary, SearchHistory, Settings, SettingsUpdate, Tag, UploadedMedia } from './types'

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

export function isOnlineSessionError(error: unknown) {
	return error instanceof APIError && (error.status === 401 || error.code === 'online_session_expired' || error.code === 'online_login_required')
}

const uploadedMedia = new WeakMap<File, Promise<string>>()

async function uploadMediaIDs(files: File[]) {
	const ids: string[] = []
	for (const file of files) {
		let pending = uploadedMedia.get(file)
		if (!pending) {
			pending = api.uploadMedia(file).then((result) => result.id)
			uploadedMedia.set(file, pending)
			pending.catch(() => uploadedMedia.delete(file))
		}
		ids.push(await pending)
	}
	return ids
}

export const api = {
  health: () => request<Health>('/health'),
	hotPosts: (limit = 5) => request<HotPostsResult>(`/posts/hot${queryString({ limit })}`),
  capabilities: () => request<Capabilities>('/capabilities'),
  posts: (params: Record<string, string | number | boolean | undefined | null>) => request<PostPage>(`/posts${queryString(params)}`),
	search: (params: Record<string, string | number | boolean | undefined | null>) => request<PostPage>(`/search${queryString(params)}`),
	searchHistory: () => request<SearchHistory[]>('/search/history?limit=12'),
  post: (pid: string | number, source: 'local' | 'live' = 'local') => request<PostDetail>(`/posts/${pid}${queryString({ source })}`),
  comments: (pid: string | number, cursor = 0, source: 'local' | 'live' = 'local', limit = 50) => request<CommentPage>(`/posts/${pid}/comments${queryString({ cursor, source, limit })}`),
	tags: () => request<Tag[]>('/tags?source=live'),
	uploadMedia: (file: File) => { const body = new FormData(); body.append('file', file); return request<UploadedMedia>('/media/uploads', { method: 'POST', body }) },
	uploadMediaIDs,
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
	projects: () => request<import('./types').ResearchProject[]>('/projects'),
	createProject: (name: string, description: string, color: string) => request<import('./types').ResearchProject>('/projects', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, description, color }) }),
	updateProject: (id: number, name: string, description: string, color: string) => request<import('./types').ResearchProject>(`/projects/${id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, description, color }) }),
	deleteProject: (id: number) => request<{ deleted: boolean }>(`/projects/${id}`, { method: 'DELETE' }),
	projectPosts: (id: number) => request<Post[]>(`/projects/${id}/posts`),
	removeProjectPosts: (id: number, pids: number[]) => request<{ updated: number }>(`/projects/${id}/posts/remove`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pids }) }),
	postProjects: (pid: number) => request<import('./types').ResearchProject[]>(`/posts/${pid}/projects`),
	setPostProjects: (pid: number, projectIDs: number[]) => request<import('./types').ResearchProject[]>(`/posts/${pid}/projects`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ project_ids: projectIDs }) }),
	settings: () => request<Settings>('/settings'),
	updateSettings: (update: SettingsUpdate) => request<Settings>('/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update) }),
	observerSettings: () => request<ObserverSettings>('/settings/observer'),
	updateObserverSettings: (update: ObserverSettingsUpdate) => request<ObserverSettings>('/settings/observer', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update) }),
	testObserverConnection: () => request<ObserverConnectionProbe>('/settings/observer/test', { method: 'POST' }),
	observerStatus: () => request<ObserverStatus>('/observer/status'),
	syncObserver: () => request<ObserverSyncResult>('/observer/sync', { method: 'POST' }),
	submitObserverChallenge: (code: string) => request<ObserverStatus>('/observer/auth/challenge/submit', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code }) }),
	resendObserverChallenge: () => request<ObserverStatus>('/observer/auth/challenge/resend', { method: 'POST' }),
	retryObserverLogin: () => request<ObserverStatus>('/observer/auth/retry', { method: 'POST' }),
	removedPosts: (params: { state?: string; query?: string; cursor?: number; limit?: number } = {}) => request<RemovedPostPage>(`/removed${queryString(params)}`),
	removedPost: (pid: string | number) => request<RemovedPostDetail>(`/removed/${pid}`),
	createAIProviderSetting: (update: AIProviderSettingUpdate) => request<Settings>('/settings/ai/providers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update) }),
	updateAIProviderSetting: (id: string, update: AIProviderSettingUpdate) => request<Settings>(`/settings/ai/providers/${id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update) }),
	deleteAIProviderSetting: (id: string) => request<Settings>(`/settings/ai/providers/${id}`, { method: 'DELETE' }),
	activateAIProviderSetting: (id: string) => request<Settings>(`/settings/ai/providers/${id}/activate`, { method: 'POST' }),
	testAIProviderSetting: (id: string) => request<import('./types').AIProviderProbe>(`/settings/ai/providers/${id}/test`, { method: 'POST' }),
	postTags: (pid: number) => request<LocalTag[]>(`/posts/${pid}/tags`),
	setPostTags: (pid: number, tagIDs: number[]) => request<LocalTag[]>(`/posts/${pid}/tags`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ tag_ids: tagIDs }) }),
	addPostTags: (pids: number[], tagIDs: number[]) => request<{ updated: number }>('/posts/batch/tags', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pids, tag_ids: tagIDs }) }),
	addPostsToProjects: (pids: number[], projectIDs: number[]) => request<{ updated: number }>('/posts/batch/projects', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pids, project_ids: projectIDs }) }),
	postNote: (pid: number) => request<Note>(`/posts/${pid}/note`),
	savePostNote: (pid: number, content: string) => request<Note>(`/posts/${pid}/note`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ content }) }),
	commentNote: (cid: number) => request<Note>(`/comments/${cid}/note`),
	saveCommentNote: (cid: number, content: string) => request<Note>(`/comments/${cid}/note`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ content }) }),
	referenceGraph: (pid: number, depth: 1 | 2) => request<ReferenceGraph>(`/posts/${pid}/references?depth=${depth}`),
  jobs: () => request<Job[]>('/jobs?limit=50'),
  job: (id: string) => request<Job>(`/jobs/${id}`),
  createJob: (type: string, payload: unknown = {}) => request<Job>('/jobs', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ type, payload }),
  }),
  jobAction: (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry') => request<Job>(`/jobs/${id}/${action}`, { method: 'POST' }),
	session: () => request<AuthStatus>('/session'),
	probeSession: () => request<AuthStatus>('/session/probe', { method: 'POST' }),
	reloadSession: () => request<AuthStatus>('/session/reload', { method: 'POST' }),
	loginSession: (username: string, password: string) => request<AuthStatus>('/session/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, password }) }),
	sendSessionSMS: (stage: 'iaaa' | 'treehole', username: string) => request<AuthStatus>('/session/sms', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stage, username: stage === 'iaaa' ? username : undefined }) }),
	continueSession: (stage: 'iaaa' | 'treehole' | '', challenge: 'sms' | 'otp', username: string, password: string, code: string) => request<AuthStatus>('/session/challenge', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stage, challenge, username: stage === 'iaaa' ? username : undefined, password: stage === 'iaaa' ? password : undefined, code }) }),
	logoutSession: () => request<AuthStatus>('/session/logout', { method: 'POST' }),
	importArchive: (file: File) => {
    const body = new FormData()
    body.append('file', file)
		return request<ImportCreated>('/imports', { method: 'POST', body })
	},
	preflightImport: (file: File) => {
		const body = new FormData()
		body.append('file', file)
		return request<BridgePairing>('/imports/preflight', { method: 'POST', body })
	},
	confirmImportPreflight: (token: string) => request<BridgePairing>(`/imports/preflight/${token}/confirm`, { method: 'POST' }),
	cancelImportPreflight: (token: string) => request<{ status: string }>(`/imports/preflight/${token}/cancel`, { method: 'POST' }),
	importJobs: () => request<Job[]>('/imports?limit=50'),
	createBridgePairing: () => request<BridgePairing>('/bridge/pairings', { method: 'POST' }),
	bridgePairing: (token: string) => request<BridgePairing>(`/bridge/pairings/${token}`),
	confirmBridgePairing: (token: string) => request<BridgePairing>(`/bridge/pairings/${token}/confirm`, { method: 'POST' }),
	cancelBridgePairing: (token: string) => request<{ status: string }>(`/bridge/pairings/${token}/cancel`, { method: 'POST' }),
	bridgeDeviceRequests: () => request<BridgeDeviceRequest[]>('/bridge/device-requests'),
	approveBridgeDeviceRequest: (token: string) => request<BridgeDeviceRequest>(`/bridge/device-requests/${token}/approve`, { method: 'POST' }),
	rejectBridgeDeviceRequest: (token: string) => request<{ status: string }>(`/bridge/device-requests/${token}/reject`, { method: 'POST' }),
	bridgeDevices: () => request<BridgeDevice[]>('/bridge/devices'),
	revokeBridgeDevice: (id: string) => request<{ status: string }>(`/bridge/devices/${id}`, { method: 'DELETE' }),
	bridgeTransfers: () => request<BridgeTransfer[]>('/bridge/transfers'),
	confirmBridgeTransfer: (id: string) => request<BridgeTransfer>(`/bridge/transfers/${id}/confirm`, { method: 'POST' }),
	cancelBridgeTransfer: (id: string) => request<{ status: string }>(`/bridge/transfers/${id}/cancel`, { method: 'POST' }),
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
	createExportJob: (format: 'treehole-v2' | 'markdown', pids: number[], includeComments: boolean, captureLive = false, includeMedia = false, syncObserverBeforeExport?: boolean) => request<Job>('/exports/jobs', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ format, pids: pids.length ? pids : undefined, include_comments: includeComments, capture_live: captureLive, include_media: captureLive && includeMedia, sync_observer_before_export: syncObserverBeforeExport }) }),
	exportPreview: (pids: number[], includeComments: boolean) => request<import('./types').ArchiveExportPreview>('/exports/preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ format: 'treehole-v2', pids: pids.length ? pids : undefined, include_comments: includeComments }) }),
	exportJobs: () => request<Job[]>('/exports/jobs'),
	regenerateExportJob: (id: string) => request<Job>(`/exports/${id}/regenerate`, { method: 'POST' }),
	downloadExportJob: async (id: string): Promise<ExportDownload> => {
		const response = await fetch(`/api/v1/exports/${id}/download`)
		if (!response.ok) {
			const failure = await response.json().catch(() => null) as ErrorEnvelope | null
			throw new APIError(response.status, failure?.error?.code ?? 'export_download_failed', failure?.error?.message ?? `下载失败 (${response.status})`, failure?.error?.details)
		}
		const disposition = response.headers.get('content-disposition') ?? ''
		return { blob: await response.blob(), filename: disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? `${id}.zip` }
	},
	rawJSONJobs: () => request<Job[]>('/raw-json/jobs'),
	downloadRawJSONJob: async (id: string): Promise<ExportDownload> => {
		const response = await fetch(`/api/v1/raw-json/${id}/download`)
		if (!response.ok) {
			const failure = await response.json().catch(() => null) as ErrorEnvelope | null
			throw new APIError(response.status, failure?.error?.code ?? 'raw_json_download_failed', failure?.error?.message ?? `下载失败 (${response.status})`, failure?.error?.details)
		}
		const disposition = response.headers.get('content-disposition') ?? ''
		return { blob: await response.blob(), filename: disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? `${id}.json` }
	},
	aiProviders: () => request<AIProvider[]>('/ai/providers'),
	aiSessions: () => request<AISession[]>('/ai/sessions?limit=50'),
	createAISession: (mode: AISession['mode'], title: string, scope: AIScope = {}) => request<AISession>('/ai/sessions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mode, title, ...scope }) }),
	aiSession: (id: string) => request<AISessionDetail>(`/ai/sessions/${id}`),
	startAIMessage: (id: string, body: { prompt: string; replace_scope?: boolean; pids?: number[]; course?: string; teachers?: string[]; from?: number; to?: number; tag_ids?: number[]; origins?: string[]; has_media?: boolean }) => request<{ session_id: string; run_id?: string; status: string }>(`/ai/sessions/${id}/messages`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
	cancelAI: (id: string) => request<{ status: string }>(`/ai/sessions/${id}/cancel`, { method: 'POST' }),
}
