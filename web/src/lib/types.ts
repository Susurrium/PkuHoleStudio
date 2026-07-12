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
export interface PostDetail { post: Post; comments: Comment[]; references: Reference[]; next_comment_cursor?: number; has_more_comments: boolean }

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
export interface Capabilities {
  api_version: string
  schema_version: number
  fts5: boolean
  archive_import: boolean
  jobs: boolean
  ai: boolean
  live_search: boolean
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

export interface ImportCreated { job: Job; preflight: ArchivePreflight }
export interface SearchHistory { id: number; query: string; filters?: string; created_at: string }
