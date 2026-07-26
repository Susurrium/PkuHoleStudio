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
export interface PostSummary extends Post { snippet?: string; score?: number; comment_matches?: CommentMatch[]; local_state?: 'saved' | 'not_saved' }
export interface PostPage { items: PostSummary[]; next_cursor?: number; has_more: boolean }
export interface CommentPage { items: Comment[]; next_cursor?: number; has_more: boolean }
export interface Reference { kind: string; source_pid: number; source_cid?: number; target_pid: number; target_cid?: number }
export interface ReferenceNode { pid: number; text?: string; timestamp?: number }
export interface ReferenceGraph { root: number; nodes: ReferenceNode[]; edges: Reference[] }
export interface Media { id: number; owner_type: 'post' | 'comment'; owner_id: number; remote_id?: string; variant: string; mime_type?: string; width?: number; height?: number; status: 'available' | 'missing' | 'failed' | 'remote'; last_error?: string }
export interface PostDetail { post: Post; comments: Comment[]; references: Reference[]; media: Media[]; next_comment_cursor?: number; has_more_comments: boolean; local_state?: 'saved' | 'not_saved' }
export interface Tag { id: number; name?: string; label?: string; parent_id: number }
export interface ResearchProject { id: number; name: string; description?: string; color?: string; post_count: number; created_at: string; updated_at: string }

export type JobStatus = 'queued' | 'running' | 'paused' | 'completed' | 'partial' | 'failed' | 'cancelled'
export interface Job {
  id: string
  type: string
  status: JobStatus
  checkpoint?: unknown
  scope?: { pids?: number[] }
  completed_items: number
  failed_items: number
  total_items: number
  attempts: number
  error?: string
  created_at: string
  updated_at: string
}

export interface Health { status: string; posts?: number; comments?: number }
export interface HotPost { id: number; text: string; follownum: number; reply?: number; timestamp?: number; score?: number; availability_state?: ObserverAvailabilityState }
export interface HotPostsResult {
  items: HotPost[]
  source: 'observer' | 'observer_cache' | 'live_recent'
  window_hours: number
  updated_at: number
  latest_timestamp?: number
  stale: boolean
  approximate: boolean
  message?: string
}
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
  data_dir?: string
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
  ai_active_provider: string
  ai_providers: AIProviderSetting[]
  ai_runtime_provider?: string
  ai_runtime_model?: string
}
export interface AIProviderSetting {
  id: string
  name: string
  base_url: string
  model: string
  temperature: number
  max_output_tokens: number
  request_timeout_seconds: number
  api_key_configured: boolean
  active: boolean
}
export interface AIProviderSettingUpdate extends Omit<AIProviderSetting, 'id' | 'api_key_configured' | 'active'> {
  api_key?: string
  clear_api_key?: boolean
}
export interface SettingsUpdate extends Omit<Settings, 'data_dir' | 'database_type' | 'database_file' | 'ai_api_key_configured' | 'restart_required' | 'ai_active_provider' | 'ai_providers' | 'ai_runtime_provider' | 'ai_runtime_model'> {
  ai_api_key?: string
  clear_ai_api_key?: boolean
}

export interface ObserverSettings {
  enabled: boolean
  base_url: string
  api_token_configured: boolean
  request_timeout_seconds: number
  auto_sync_on_start: boolean
  sync_interval_minutes: number
  sync_before_export: boolean
}

export interface ObserverSettingsUpdate extends Omit<ObserverSettings, 'api_token_configured'> {
  api_token?: string
  clear_api_token?: boolean
}

export type ObserverAuthState = 'starting' | 'authenticated' | 'reauthenticating' | 'network_backoff' | 'challenge_required' | 'credentials_invalid' | 'stopped' | string

export interface ObserverConnectionProbe {
  ok: boolean
  instance_id?: string
  api_version?: string
  service_version?: string
  commit?: string
  build_date?: string
  auth_state?: ObserverAuthState
  message?: string
}

export type ObserverTrafficState = 'normal' | 'backoff' | 'circuit_open' | string

export interface ObserverTrafficStatus {
  state: ObserverTrafficState
  blocked_until?: string
  reason?: string
  consecutive_rate_limits?: number
  consecutive_service_failures?: number
}

export interface ObserverStatus {
  configured: boolean
  enabled: boolean
  connected: boolean
  stale: boolean
  instance_id?: string
  api_version?: string
  auth_state?: ObserverAuthState
  challenge_required: boolean
  challenge?: 'sms' | 'otp' | string
  challenge_stage?: 'iaaa' | 'treehole' | string
  masked_target?: string
  auth_reason?: string
  auth_warning?: string
  auth_failure_kind?: string
  next_retry_at?: string
  sms_can_resend_at?: string
  last_successful_scan_at?: string
  latest_post_at?: string
  coverage_degraded: boolean
  baseline_completed: boolean
  queue_depth: number
  traffic?: ObserverTrafficStatus
  last_error?: string
  remote?: Record<string, unknown>
}

export interface ObserverSyncResult {
  instance_id?: string
  events_received: number
  events_applied: number
  snapshots_imported: number
  last_event_id: number
  completed_at: string
}

export type ObserverAvailabilityState = 'confirmed_unavailable' | 'restored' | string
export type ObserverCaptureCompleteness = 'complete' | 'partial' | 'unknown' | string

export interface ObserverAvailability {
  pid: number
  state: ObserverAvailabilityState
  observed_at: string
  first_unavailable_at?: string
  last_unavailable_at?: string
  restored_at?: string
  observer_id?: string
  completeness?: ObserverCaptureCompleteness
}

export interface RemovedPostSummary extends ObserverAvailability {
  post?: Post
}

export interface RemovedPostPage {
  items: RemovedPostSummary[]
  next_cursor?: number
  has_more: boolean
}

export interface RemovedPostDetail {
  availability: ObserverAvailability
  post?: Post
  comments: Comment[]
  media: Media[]
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
  duplicate?: boolean
}

export interface ImportCreated { job?: Job; preflight: ArchivePreflight }
export interface ExportDownload { blob: Blob; filename: string }
export interface ArchiveExportPreview { format: 'treehole-v2' | 'markdown'; posts: number; comments: number; media: number; missing_media: number; run_id?: string }
export interface BridgePairing {
  token: string
  code?: string
  status: 'waiting_upload' | 'uploading' | 'awaiting_confirmation' | 'queued'
  expires_at: string
  filename?: string
  preflight?: ArchivePreflight
  job?: Job
}
export interface BridgeDeviceRequest {
  token: string
  name: string
  verification_code: string
  status: 'pending' | 'approved' | 'rejected'
  expires_at: string
  device_id?: string
  instance_id?: string
}
export interface BridgeDevice {
  id: string
  name: string
  created_at: string
  last_used_at?: string
}
export interface BridgeTransfer {
  id: string
  device_id: string
  device_name: string
  filename: string
  size: number
  sha256: string
  status: 'waiting_upload' | 'uploading' | 'awaiting_confirmation' | 'queued'
  created_at: string
  expires_at: string
  preflight?: ArchivePreflight
  job?: Job
}
export interface SearchHistory { id: number; query: string; filters?: string; created_at: string }

export interface AIProvider { id?: string; name: string; base_url: string; model: string; configured: boolean; active?: boolean }
export interface AIProviderProbe { provider_id: string; provider: string; model: string; reachable: boolean; latency_ms: number }
export interface AIScope { pids?: number[]; course?: string; teachers?: string[]; from?: number; to?: number; tag_ids?: number[]; origins?: string[]; has_media?: boolean }
export interface AISession { id: string; title: string; mode: 'selected' | 'local' | 'course'; provider_id?: string; provider: string; model: string; scope?: AIScope; created_at: string; updated_at: string }
export interface AISource { message_id?: string; ordinal?: number; origin?: 'local' | 'live'; pid: number; cid?: number; snippet?: string }
export type AIClaimStatus = 'supported' | 'partial' | 'unsupported' | 'unverified' | 'insufficient'
export interface AIEvidenceRef { origin: 'local' | 'live'; pid: number; cid?: number }
export interface AIClaim { ordinal: number; text: string; status: AIClaimStatus; reason?: string; sources?: AIEvidenceRef[] }
export interface AIEvidenceSummary { total: number; cited: number; supported: number; partial: number; unsupported: number; unverified: number; insufficient: number; citation_coverage: number }
export interface AIEvidenceReport { checked: boolean; summary: AIEvidenceSummary; claims: AIClaim[] }
export interface AIMessage { id: string; session_id: string; run_id?: string; role: 'user' | 'assistant'; content: string; model?: string; mode?: string; trace?: string; evidence?: AIEvidenceReport; created_at: string; sources: AISource[] }
export interface AIQuery { run_id: string; ordinal: number; round: number; tool: string; query?: string; reason?: string; matches: number; created_at: string }
export interface AIRun { id: string; session_id: string; status: 'running' | 'completed' | 'failed' | 'cancelled' | 'interrupted'; prompt: string; provider_id?: string; provider?: string; model?: string; prompt_version?: string; error?: string; started_at?: string; finished_at?: string; created_at: string; updated_at: string; queries?: AIQuery[] }
export interface AISessionDetail { session: AISession; messages: AIMessage[]; latest_run?: AIRun }
