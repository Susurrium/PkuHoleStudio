package models

import "time"

type SchemaMigration struct {
	Version   int       `gorm:"primaryKey;column:version" json:"version"`
	Name      string    `gorm:"column:name;not null" json:"name"`
	AppliedAt time.Time `gorm:"column:applied_at;not null" json:"applied_at"`
}

func (SchemaMigration) TableName() string { return "schema_migrations" }

type SyncRun struct {
	ID             string     `gorm:"primaryKey;column:id;size:64" json:"id"`
	JobID          string     `gorm:"column:job_id;size:64;index" json:"job_id,omitempty"`
	Type           string     `gorm:"column:type;size:32;not null;index" json:"type"`
	Status         string     `gorm:"column:status;size:16;not null;index" json:"status"`
	CheckpointJSON string     `gorm:"column:checkpoint_json;type:text" json:"checkpoint,omitempty"`
	PostCount      int        `gorm:"column:post_count;not null;default:0" json:"post_count"`
	CommentCount   int        `gorm:"column:comment_count;not null;default:0" json:"comment_count"`
	Error          string     `gorm:"column:error;type:text" json:"error,omitempty"`
	StartedAt      *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt     *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

type SyncRunItem struct {
	RunID      string    `gorm:"primaryKey;column:run_id;size:64" json:"run_id"`
	ItemKey    string    `gorm:"primaryKey;column:item_key;size:128" json:"item_key"`
	PID        int32     `gorm:"column:pid;index" json:"pid,omitempty"`
	Page       int       `gorm:"column:page" json:"page,omitempty"`
	Status     string    `gorm:"column:status;size:16;not null;index" json:"status"`
	Attempts   int       `gorm:"column:attempts;not null;default:0" json:"attempts"`
	Checkpoint string    `gorm:"column:checkpoint;type:text" json:"checkpoint,omitempty"`
	Error      string    `gorm:"column:error;type:text" json:"error,omitempty"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

type ImportRun struct {
	ID               string     `gorm:"primaryKey;column:id;size:64" json:"id"`
	JobID            string     `gorm:"column:job_id;size:64;index" json:"job_id,omitempty"`
	ArchiveRunID     string     `gorm:"column:archive_run_id;size:128;index" json:"archive_run_id,omitempty"`
	ArchiveHash      string     `gorm:"column:archive_hash;size:64;not null;uniqueIndex" json:"archive_hash"`
	Format           string     `gorm:"column:format;size:32;not null" json:"format"`
	Status           string     `gorm:"column:status;size:16;not null;index" json:"status"`
	ImportedPosts    int        `gorm:"column:imported_posts;not null;default:0" json:"imported_posts"`
	ImportedComments int        `gorm:"column:imported_comments;not null;default:0" json:"imported_comments"`
	SkippedRecords   int        `gorm:"column:skipped_records;not null;default:0" json:"skipped_records"`
	ReportJSON       string     `gorm:"column:report_json;type:text" json:"report,omitempty"`
	StartedAt        *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt       *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

type PostSource struct {
	PID         int32     `gorm:"primaryKey;column:pid" json:"pid"`
	Source      string    `gorm:"primaryKey;column:source;size:32" json:"source"`
	SourceRef   string    `gorm:"primaryKey;column:source_ref;size:128" json:"source_ref,omitempty"`
	ContextOnly bool      `gorm:"column:context_only;not null;default:false;index" json:"context_only"`
	FirstSeenAt time.Time `gorm:"column:first_seen_at;not null" json:"first_seen_at"`
	LastSeenAt  time.Time `gorm:"column:last_seen_at;not null;index" json:"last_seen_at"`
}

// ObserverSyncState is the durable cursor for one configured Observer. The
// remote instance UUID is stored separately so a reinstalled Observer can be
// detected and safely replayed from event zero.
type ObserverSyncState struct {
	ObserverID       string     `gorm:"primaryKey;column:observer_id;size:64" json:"observer_id"`
	RemoteInstanceID string     `gorm:"column:remote_instance_id;size:128;index" json:"remote_instance_id"`
	LastEventID      int64      `gorm:"column:last_event_id;not null;default:0" json:"last_event_id"`
	LastSuccessAt    *time.Time `gorm:"column:last_success_at" json:"last_success_at,omitempty"`
	LastError        string     `gorm:"column:last_error;type:text" json:"last_error,omitempty"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (ObserverSyncState) TableName() string { return "observer_sync_states" }

type ObserverEventReceipt struct {
	ObserverID       string    `gorm:"primaryKey;column:observer_id;size:64" json:"observer_id"`
	RemoteInstanceID string    `gorm:"primaryKey;column:remote_instance_id;size:128" json:"remote_instance_id"`
	EventID          int64     `gorm:"primaryKey;column:event_id" json:"event_id"`
	PID              int32     `gorm:"column:pid;index" json:"pid"`
	EventType        string    `gorm:"column:event_type;size:64;not null;index" json:"event_type"`
	PayloadHash      string    `gorm:"column:payload_hash;size:64;not null" json:"payload_hash"`
	AppliedAt        time.Time `gorm:"column:applied_at;not null;index" json:"applied_at"`
}

func (ObserverEventReceipt) TableName() string { return "observer_event_receipts" }

// PostAvailability is deliberately separate from posts. Archive v2's neutral
// post schema remains portable while Observer-specific state is newer-wins.
type PostAvailability struct {
	PID                int32      `gorm:"primaryKey;column:pid" json:"pid"`
	State              string     `gorm:"column:state;size:40;not null;index" json:"state"`
	ObservedAt         time.Time  `gorm:"column:observed_at;not null;index" json:"observed_at"`
	FirstUnavailableAt *time.Time `gorm:"column:first_unavailable_at;index" json:"first_unavailable_at,omitempty"`
	LastUnavailableAt  *time.Time `gorm:"column:last_unavailable_at;index" json:"last_unavailable_at,omitempty"`
	RestoredAt         *time.Time `gorm:"column:restored_at;index" json:"restored_at,omitempty"`
	ObserverID         string     `gorm:"column:observer_id;size:64;not null;index" json:"observer_id"`
	RemoteInstanceID   string     `gorm:"column:remote_instance_id;size:128;not null;index" json:"remote_instance_id"`
	Completeness       string     `gorm:"column:completeness;size:32;not null;default:unknown" json:"completeness"`
	SnapshotID         string     `gorm:"column:snapshot_id;size:128" json:"snapshot_id,omitempty"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (PostAvailability) TableName() string { return "post_availability" }

type LocalTag struct {
	ID        uint      `gorm:"primaryKey;column:id" json:"id"`
	Name      string    `gorm:"column:name;size:128;not null;uniqueIndex" json:"name"`
	Color     string    `gorm:"column:color;size:32" json:"color,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

type PostTag struct {
	PID       int32     `gorm:"primaryKey;column:pid" json:"pid"`
	TagID     uint      `gorm:"primaryKey;column:tag_id;index" json:"tag_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// ResearchProject is a durable user-curated collection of local posts. It is
// intentionally independent from tags: tags describe content, while projects
// capture an evolving research question or export set.
type ResearchProject struct {
	ID          uint      `gorm:"primaryKey;column:id" json:"id"`
	Name        string    `gorm:"column:name;size:160;not null" json:"name"`
	Description string    `gorm:"column:description;type:text" json:"description,omitempty"`
	Color       string    `gorm:"column:color;size:32" json:"color,omitempty"`
	PostCount   int       `gorm:"->;-:migration;column:post_count" json:"post_count"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;index" json:"updated_at"`
}

type ProjectPost struct {
	ProjectID uint      `gorm:"primaryKey;column:project_id;index" json:"project_id"`
	PID       int32     `gorm:"primaryKey;column:pid;index" json:"pid"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
}

type Note struct {
	ID        uint      `gorm:"primaryKey;column:id" json:"id"`
	OwnerType string    `gorm:"column:owner_type;size:16;not null;uniqueIndex:idx_notes_owner" json:"owner_type"`
	OwnerID   int64     `gorm:"column:owner_id;not null;uniqueIndex:idx_notes_owner" json:"owner_id"`
	Content   string    `gorm:"column:content;type:text;not null" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

type Reference struct {
	ID         uint      `gorm:"primaryKey;column:id" json:"id"`
	SourceType string    `gorm:"column:source_type;size:16;not null;uniqueIndex:idx_references_edge" json:"source_type"`
	SourceID   int64     `gorm:"column:source_id;not null;uniqueIndex:idx_references_edge" json:"source_id"`
	TargetType string    `gorm:"column:target_type;size:16;not null;uniqueIndex:idx_references_edge" json:"target_type"`
	TargetID   int64     `gorm:"column:target_id;not null;uniqueIndex:idx_references_edge" json:"target_id"`
	Kind       string    `gorm:"column:kind;size:32;not null;uniqueIndex:idx_references_edge" json:"kind"`
	CreatedAt  time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// ReferenceEdge is the PID/CID projection returned to application services.
// Reference stores polymorphic database IDs, while consumers need the owning
// post for comment endpoints as well.
type ReferenceEdge struct {
	Kind      string `json:"kind" gorm:"column:kind"`
	SourcePID int32  `json:"source_pid" gorm:"column:source_pid"`
	SourceCID *int32 `json:"source_cid,omitempty" gorm:"column:source_cid"`
	TargetPID int32  `json:"target_pid" gorm:"column:target_pid"`
	TargetCID *int32 `json:"target_cid,omitempty" gorm:"column:target_cid"`
}

func (Reference) TableName() string { return "references" }

type Media struct {
	ID          uint      `gorm:"primaryKey;column:id" json:"id"`
	RemoteID    string    `gorm:"column:remote_id;size:128;uniqueIndex:idx_media_owner_remote" json:"remote_id,omitempty"`
	RemoteURL   string    `gorm:"column:remote_url;type:text" json:"remote_url,omitempty"`
	ContentHash string    `gorm:"column:content_hash;size:64;index" json:"content_hash,omitempty"`
	OwnerType   string    `gorm:"column:owner_type;size:16;not null;uniqueIndex:idx_media_owner_remote" json:"owner_type"`
	OwnerID     int64     `gorm:"column:owner_id;not null;uniqueIndex:idx_media_owner_remote" json:"owner_id"`
	Variant     string    `gorm:"column:variant;size:16;not null;default:original;uniqueIndex:idx_media_owner_remote" json:"variant"`
	Path        string    `gorm:"column:path;type:text" json:"path,omitempty"`
	MIMEType    string    `gorm:"column:mime_type;size:128" json:"mime_type,omitempty"`
	Size        int64     `gorm:"column:size;not null;default:0" json:"size"`
	Width       int       `gorm:"column:width;not null;default:0" json:"width,omitempty"`
	Height      int       `gorm:"column:height;not null;default:0" json:"height,omitempty"`
	Status      string    `gorm:"column:status;size:16;not null;default:missing;index" json:"status"`
	LastError   string    `gorm:"column:last_error;type:text" json:"last_error,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Media) TableName() string { return "media" }

type MediaRepairCandidate struct {
	Media
	PID int32 `json:"pid" gorm:"column:pid"`
}

type SearchHistory struct {
	ID          uint      `gorm:"primaryKey;column:id" json:"id"`
	Query       string    `gorm:"column:query;type:text;not null" json:"query"`
	FiltersJSON string    `gorm:"column:filters_json;type:text" json:"filters,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
}

func (SearchHistory) TableName() string { return "search_history" }

type AISession struct {
	ID         string    `gorm:"primaryKey;column:id;size:64" json:"id"`
	Title      string    `gorm:"column:title;type:text" json:"title"`
	Mode       string    `gorm:"column:mode;size:32;not null" json:"mode"`
	ProviderID string    `gorm:"column:provider_id;size:64;index" json:"provider_id,omitempty"`
	Provider   string    `gorm:"column:provider;size:128" json:"provider,omitempty"`
	Model      string    `gorm:"column:model;size:128" json:"model,omitempty"`
	ScopeJSON  string    `gorm:"column:scope_json;type:text" json:"-"`
	Scope      AIScope   `gorm:"-" json:"scope"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// AIScope is the durable research boundary for an AI session. Keeping the
// boundary on the session makes follow-up questions reproducible after a page
// refresh or process restart instead of relying on transient form state.
type AIScope struct {
	PIDs     []int32  `json:"pids,omitempty"`
	Course   string   `json:"course,omitempty"`
	Teachers []string `json:"teachers,omitempty"`
	From     int64    `json:"from,omitempty"`
	To       int64    `json:"to,omitempty"`
	TagIDs   []uint   `json:"tag_ids,omitempty"`
	Origins  []string `json:"origins,omitempty"`
	HasMedia *bool    `json:"has_media,omitempty"`
}

type AIMessage struct {
	ID           string            `gorm:"primaryKey;column:id;size:64" json:"id"`
	SessionID    string            `gorm:"column:session_id;size:64;not null;index" json:"session_id"`
	RunID        string            `gorm:"column:run_id;size:64;index" json:"run_id,omitempty"`
	Ordinal      int64             `gorm:"column:ordinal;not null;default:0" json:"ordinal"`
	Role         string            `gorm:"column:role;size:16;not null" json:"role"`
	Content      string            `gorm:"column:content;type:text;not null" json:"content"`
	Provider     string            `gorm:"column:provider;size:128" json:"provider,omitempty"`
	Model        string            `gorm:"column:model;size:128" json:"model,omitempty"`
	Mode         string            `gorm:"column:mode;size:32" json:"mode,omitempty"`
	TraceJSON    string            `gorm:"column:trace_json;type:text" json:"trace,omitempty"`
	EvidenceJSON string            `gorm:"column:evidence_json;type:text" json:"-"`
	Evidence     *AIEvidenceReport `gorm:"-" json:"evidence,omitempty"`
	CreatedAt    time.Time         `gorm:"column:created_at;not null;index" json:"created_at"`
}

// AIEvidenceReport records which answer claim each citation is intended to
// support. It is stored as one JSON snapshot on the assistant message so old
// answers remain reproducible even if the archive or verifier later changes.
type AIEvidenceReport struct {
	Checked bool              `json:"checked"`
	Summary AIEvidenceSummary `json:"summary"`
	Claims  []AIClaim         `json:"claims"`
}

type AIEvidenceSummary struct {
	Total            int     `json:"total"`
	Cited            int     `json:"cited"`
	Supported        int     `json:"supported"`
	Partial          int     `json:"partial"`
	Unsupported      int     `json:"unsupported"`
	Unverified       int     `json:"unverified"`
	Insufficient     int     `json:"insufficient"`
	CitationCoverage float64 `json:"citation_coverage"`
}

type AIClaim struct {
	Ordinal int             `json:"ordinal"`
	Text    string          `json:"text"`
	Status  string          `json:"status"`
	Reason  string          `json:"reason,omitempty"`
	Sources []AIEvidenceRef `json:"sources,omitempty"`
}

type AIEvidenceRef struct {
	Origin string `json:"origin"`
	PID    int32  `json:"pid"`
	CID    *int32 `json:"cid,omitempty"`
}

type AISource struct {
	MessageID string `gorm:"primaryKey;column:message_id;size:64" json:"message_id"`
	Ordinal   int    `gorm:"primaryKey;column:ordinal" json:"ordinal"`
	Origin    string `gorm:"column:origin;size:16;not null;default:local;index" json:"origin"`
	PID       int32  `gorm:"column:pid;not null;index" json:"pid"`
	CID       *int32 `gorm:"column:cid;index" json:"cid,omitempty"`
	Snippet   string `gorm:"column:snippet;type:text" json:"snippet,omitempty"`
}

type AIRun struct {
	ID            string     `gorm:"primaryKey;column:id;size:64" json:"id"`
	SessionID     string     `gorm:"column:session_id;size:64;not null;index" json:"session_id"`
	Status        string     `gorm:"column:status;size:24;not null;index" json:"status"`
	Prompt        string     `gorm:"column:prompt;type:text;not null" json:"prompt"`
	ScopeJSON     string     `gorm:"column:scope_json;type:text" json:"scope,omitempty"`
	ProviderID    string     `gorm:"column:provider_id;size:64;index" json:"provider_id,omitempty"`
	Provider      string     `gorm:"column:provider;size:128" json:"provider,omitempty"`
	Model         string     `gorm:"column:model;size:128" json:"model,omitempty"`
	PromptVersion string     `gorm:"column:prompt_version;size:64" json:"prompt_version,omitempty"`
	ConfigJSON    string     `gorm:"column:config_json;type:text" json:"-"`
	Error         string     `gorm:"column:error;type:text" json:"error,omitempty"`
	StartedAt     *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt    *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	Queries       []AIQuery  `gorm:"-" json:"queries,omitempty"`
}

type AIQuery struct {
	RunID     string    `gorm:"primaryKey;column:run_id;size:64" json:"run_id"`
	Ordinal   int       `gorm:"primaryKey;column:ordinal" json:"ordinal"`
	Round     int       `gorm:"column:round;not null" json:"round"`
	Tool      string    `gorm:"column:tool;size:64;not null" json:"tool"`
	Query     string    `gorm:"column:query;type:text" json:"query,omitempty"`
	Reason    string    `gorm:"column:reason;type:text" json:"reason,omitempty"`
	Matches   int       `gorm:"column:matches;not null;default:0" json:"matches"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

type AIEventRecord struct {
	RunID     string    `gorm:"primaryKey;column:run_id;size:64" json:"run_id"`
	Sequence  int64     `gorm:"primaryKey;column:sequence" json:"sequence"`
	Type      string    `gorm:"column:type;size:32;not null" json:"type"`
	DataJSON  string    `gorm:"column:data_json;type:text" json:"data,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
}

type Job struct {
	ID             string     `gorm:"primaryKey;column:id;size:64" json:"id"`
	Type           string     `gorm:"column:type;size:32;not null;index" json:"type"`
	Status         string     `gorm:"column:status;size:16;not null;index" json:"status"`
	PayloadJSON    string     `gorm:"column:payload_json;type:text" json:"payload,omitempty"`
	CheckpointJSON string     `gorm:"column:checkpoint_json;type:text" json:"checkpoint,omitempty"`
	CompletedItems int        `gorm:"column:completed_items;not null;default:0" json:"completed_items"`
	FailedItems    int        `gorm:"column:failed_items;not null;default:0" json:"failed_items"`
	TotalItems     int        `gorm:"column:total_items;not null;default:0" json:"total_items"`
	Attempts       int        `gorm:"column:attempts;not null;default:0" json:"attempts"`
	Error          string     `gorm:"column:error;type:text" json:"error,omitempty"`
	StartedAt      *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt     *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

type JobItem struct {
	JobID      string    `gorm:"primaryKey;column:job_id;size:64" json:"job_id"`
	ItemKey    string    `gorm:"primaryKey;column:item_key;size:128" json:"item_key"`
	Status     string    `gorm:"column:status;size:16;not null;index" json:"status"`
	Attempts   int       `gorm:"column:attempts;not null;default:0" json:"attempts"`
	Checkpoint string    `gorm:"column:checkpoint;type:text" json:"checkpoint,omitempty"`
	Error      string    `gorm:"column:error;type:text" json:"error,omitempty"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

type JobEvent struct {
	JobID     string    `gorm:"primaryKey;column:job_id;size:64" json:"job_id"`
	Sequence  int64     `gorm:"primaryKey;column:sequence" json:"sequence"`
	Type      string    `gorm:"column:type;size:32;not null" json:"type"`
	DataJSON  string    `gorm:"column:data_json;type:text" json:"data,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
}
