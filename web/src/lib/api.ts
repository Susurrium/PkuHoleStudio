import type { AIProvider, AISession, AISessionDetail, Capabilities, CommentPage, Health, ImportCreated, Job, PostDetail, PostPage, SearchHistory } from './types'

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
  capabilities: () => request<Capabilities>('/capabilities'),
  posts: (params: Record<string, string | number | boolean | undefined | null>) => request<PostPage>(`/posts${queryString(params)}`),
	search: (params: Record<string, string | number | boolean | undefined | null>) => request<PostPage>(`/search${queryString(params)}`),
	searchHistory: () => request<SearchHistory[]>('/search/history?limit=12'),
  post: (pid: string | number) => request<PostDetail>(`/posts/${pid}`),
  comments: (pid: string | number, cursor = 0) => request<CommentPage>(`/posts/${pid}/comments${queryString({ cursor })}`),
  jobs: () => request<Job[]>('/jobs?limit=50'),
  job: (id: string) => request<Job>(`/jobs/${id}`),
  createJob: (type: string, payload: unknown = {}) => request<Job>('/jobs', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ type, payload }),
  }),
  jobAction: (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry') => request<Job>(`/jobs/${id}/${action}`, { method: 'POST' }),
	importArchive: (file: File) => {
    const body = new FormData()
    body.append('file', file)
    return request<ImportCreated>('/imports', { method: 'POST', body })
	},
	aiProviders: () => request<AIProvider[]>('/ai/providers'),
	aiSessions: () => request<AISession[]>('/ai/sessions?limit=50'),
	createAISession: (mode: AISession['mode'], title: string) => request<AISession>('/ai/sessions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mode, title }) }),
	aiSession: (id: string) => request<AISessionDetail>(`/ai/sessions/${id}`),
	startAIMessage: (id: string, body: { prompt: string; pids?: number[]; course?: string; teachers?: string[] }) => request<{ session_id: string; status: string }>(`/ai/sessions/${id}/messages`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
	cancelAI: (id: string) => request<{ status: string }>(`/ai/sessions/${id}/cancel`, { method: 'POST' }),
}
