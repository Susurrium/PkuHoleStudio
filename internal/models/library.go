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
	RemoteID    string    `gorm:"column:remote_id;size:128;index;uniqueIndex:idx_media_owner_remote" json:"remote_id,omitempty"`
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
	ID        string    `gorm:"primaryKey;column:id;size:64" json:"id"`
	Title     string    `gorm:"column:title;type:text" json:"title"`
	Mode      string    `gorm:"column:mode;size:32;not null" json:"mode"`
	Provider  string    `gorm:"column:provider;size:128" json:"provider,omitempty"`
	Model     string    `gorm:"column:model;size:128" json:"model,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

type AIMessage struct {
	ID        string    `gorm:"primaryKey;column:id;size:64" json:"id"`
	SessionID string    `gorm:"column:session_id;size:64;not null;index" json:"session_id"`
	Role      string    `gorm:"column:role;size:16;not null" json:"role"`
	Content   string    `gorm:"column:content;type:text;not null" json:"content"`
	Provider  string    `gorm:"column:provider;size:128" json:"provider,omitempty"`
	Model     string    `gorm:"column:model;size:128" json:"model,omitempty"`
	Mode      string    `gorm:"column:mode;size:32" json:"mode,omitempty"`
	TraceJSON string    `gorm:"column:trace_json;type:text" json:"trace,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
}

type AISource struct {
	MessageID string `gorm:"primaryKey;column:message_id;size:64" json:"message_id"`
	Ordinal   int    `gorm:"primaryKey;column:ordinal" json:"ordinal"`
	PID       int32  `gorm:"column:pid;not null;index" json:"pid"`
	CID       *int32 `gorm:"column:cid;index" json:"cid,omitempty"`
	Snippet   string `gorm:"column:snippet;type:text" json:"snippet,omitempty"`
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
