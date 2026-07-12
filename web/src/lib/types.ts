export interface Post {
  pid: number
  text: string
  type?: string
  timestamp?: number
  reply?: number
  likenum?: number
  praise_num?: number
  media_ids?: string
  anonymous?: number | boolean
  is_follow?: number | boolean
  is_praise?: number | boolean
  comment_list?: Comment[]
}

export interface Comment {
  cid: number
  pid: number
  text: string
  name_tag?: string
  timestamp?: number
  media_ids?: string
  is_lz?: number | boolean
  quote?: Comment
}

export interface CommentMatch { cid: number; pid: number; snippet: string; score: number }
export interface PostSummary extends Post { snippet?: string; score?: number; comment_matches?: CommentMatch[] }
export interface PostPage { items: PostSummary[]; next_cursor?: number; has_more: boolean }
export interface CommentPage { items: Comment[]; next_cursor?: number; has_more: boolean }
export interface Reference { kind: string; source_pid: number; source_cid?: number; target_pid: number; target_cid?: number }
export interface ReferenceNode { pid: number; text?: string; timestamp?: number }
export interface ReferenceGraph { root: number; nodes: ReferenceNode[]; edges: Reference[] }
export interface Media { id: number; owner_type: 'post' | 'comment'; owner_id: number; remote_id?: string; variant: string; mime_type?: string; width?: number; height?: number; status: 'available' | 'missing' | 'failed' | 'remote'; last_error?: string }
export interface PostDetail { post: Post; comments: Comment[]; references: Reference[]; media: Media[]; next_comment_cursor?: number; has_more_comments: boolean }
export interface Tag { id: number; name?: string; label?: string; parent_id: number }

export type JobStatus = 'queued' | 'running' | 'paused' | 'completed' | 'partial' | 'failed' | 'cancelled'
export interface Job {
  id: string
  type: string
  status: JobStatus
  checkpoint?: unknown
  completed_items: number
  failed_items: number
  total_items: number
  attempts: number
  error?: string
  created_at: string
  updated_at: string
}

export interface Health { status: string; posts?: number; comments?: number }
export interface HotPost { id: number; text: string; follownum: number }
export interface UploadedMedia { id: string; filename: string; size: number }
export interface Notification { id: number; pid?: number; title?: string; content: string; read: boolean; created_at?: string; timestamp?: number; type: 'int_msg' | 'sys_msg' }
export interface NotificationPage { items: Notification[]; total: number; page: number }
export interface LogLine { module: 'crawler' | 'tui'; line: string }
export interface CourseDay { courseName?: string; parity?: string; sty?: string }
export interface CourseScheduleRow { time_num: string; mon: CourseDay; tue: CourseDay; wed: CourseDay; thu: CourseDay; fri: CourseDay; sat: CourseDay; sun: CourseDay }
export interface CourseScore { year_term: string; name: string; credit: string; score: string; category: string }
export interface ScoreSummary { gpa: string; total_credit: string; passed_credit: string; course_count: string; scores: CourseScore[]; gpa_terms: { year_term: string; gpa: string }[] }
export interface LocalTag { id: number; name: string; color?: string }
export interface Note { owner_type: string; owner_id: number; content: string; updated_at?: string }
export interface Settings {
  database_type: string
  database_file?: string
  ai_enabled: boolean
  ai_live_search: boolean
  ai_provider_name: string
  ai_base_url: string
  ai_model: string
  ai_temperature: number
  ai_max_output_tokens: number
  ai_request_timeout_seconds: number
  ai_max_search_rounds: number
  ai_api_key_configured: boolean
  restart_required: boolean
}
export interface SettingsUpdate extends Omit<Settings, 'database_type' | 'database_file' | 'ai_api_key_configured' | 'restart_required'> {
  ai_api_key?: string
  clear_ai_api_key?: boolean
}
export interface Capabilities {
  api_version: string
  schema_version: number
  fts5: boolean
  archive_import: boolean
  archive_export?: boolean
  jobs: boolean
  ai: boolean
  live_search: boolean
  online_sync?: boolean
}

export interface AuthStatus {
  checked: boolean
  has_session: boolean
  can_read_online: boolean
  can_write_online: boolean
  failure_kind?: string
  message?: string
  challenge?: 'sms' | 'otp' | 'username' | 'password' | ''
  challenge_stage?: 'iaaa' | 'treehole' | ''
  challenge_reason?: string
}

export interface ArchiveIssue { severity: string; code: string; message: string; path?: string; pid?: number; cid?: number }
export interface ArchivePreflight {
  format: string
  status: string
  hash: string
  run_id: string
  counts: Record<string, number>
  issues: ArchiveIssue[]
}

export interface ImportCreated { job?: Job; preflight: ArchivePreflight }
export interface ExportDownload { blob: Blob; filename: string }
export interface BridgePairing {
  token: string
  code?: string
  status: 'waiting_upload' | 'uploading' | 'awaiting_confirmation' | 'queued'
  expires_at: string
  filename?: string
  preflight?: ArchivePreflight
  job?: Job
}
export interface SearchHistory { id: number; query: string; filters?: string; created_at: string }

export interface AIProvider { name: string; base_url: string; model: string; configured: boolean }
export interface AISession { id: string; title: string; mode: 'selected' | 'local' | 'course'; provider: string; model: string; created_at: string; updated_at: string }
export interface AISource { message_id?: string; ordinal?: number; pid: number; cid?: number; snippet?: string }
export interface AIMessage { id: string; session_id: string; role: 'user' | 'assistant'; content: string; model?: string; mode?: string; trace?: string; created_at: string; sources: AISource[] }
export interface AISessionDetail { session: AISession; messages: AIMessage[] }
