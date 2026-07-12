package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	aipkg "github.com/Susurrium/PkuHoleStudio/internal/ai"
	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"

	"github.com/gin-gonic/gin"
)

const maxJSONBodyBytes = 1 << 20

type apiEnvelope struct {
	Data any `json:"data"`
}

type apiErrorEnvelope struct {
	Error apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type publicJob struct {
	ID             string          `json:"id"`
	Type           jobs.Type       `json:"type"`
	Status         jobs.Status     `json:"status"`
	Checkpoint     json.RawMessage `json:"checkpoint,omitempty"`
	CompletedItems int             `json:"completed_items"`
	FailedItems    int             `json:"failed_items"`
	TotalItems     int             `json:"total_items"`
	Attempts       int             `json:"attempts"`
	Error          string          `json:"error,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	FinishedAt     *time.Time      `json:"finished_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func registerAPIV1(group *gin.RouterGroup, dependencies Dependencies) {
	group.GET("/health", apiHealth(dependencies))
	group.GET("/capabilities", apiCapabilities(dependencies))
	group.GET("/posts", apiPosts(dependencies))
	group.GET("/posts/:pid", apiPost(dependencies))
	group.GET("/posts/:pid/comments", apiComments(dependencies))
	group.GET("/search", apiSearch(dependencies))
	group.GET("/search/history", apiSearchHistory(dependencies))
	group.GET("/media/:id", apiMedia(dependencies))
	group.GET("/session", apiSession(dependencies))
	group.POST("/session/probe", apiProbeSession(dependencies))
	group.POST("/session/login", apiLoginSession(dependencies))
	group.POST("/session/challenge", apiContinueSession(dependencies))

	group.GET("/jobs", apiJobs(dependencies))
	group.POST("/jobs", apiCreateJob(dependencies))
	group.GET("/jobs/:id", apiJob(dependencies))
	group.GET("/jobs/:id/events", apiJobEvents(dependencies))
	group.POST("/jobs/:id/pause", apiJobAction(dependencies, "pause"))
	group.POST("/jobs/:id/resume", apiJobAction(dependencies, "resume"))
	group.POST("/jobs/:id/cancel", apiJobAction(dependencies, "cancel"))
	group.POST("/jobs/:id/retry", apiJobAction(dependencies, "retry"))

	group.POST("/imports", apiCreateImport(dependencies))
	group.GET("/imports/:id", apiImport(dependencies))
	group.POST("/exports", apiCreateExport(dependencies))

	group.GET("/ai/providers", apiAIProviders(dependencies))
	group.GET("/ai/sessions", apiAISessions(dependencies))
	group.POST("/ai/sessions", apiCreateAISession(dependencies))
	group.GET("/ai/sessions/:id", apiAISession(dependencies))
	group.POST("/ai/sessions/:id/messages", apiCreateAIMessage(dependencies))
	group.GET("/ai/sessions/:id/events", apiAIEvents(dependencies))
	group.POST("/ai/sessions/:id/cancel", apiAICancel(dependencies))
}

func apiCreateExport(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Format          string  `json:"format"`
		PIDs            []int32 `json:"pids,omitempty"`
		IncludeComments *bool   `json:"include_comments,omitempty"`
	}
	return func(c *gin.Context) {
		if dependencies.Archive == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "archive export is unavailable", nil)
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		format := archive.ExportFormat(body.Format)
		if format == "" {
			format = archive.ExportFormatTreeholeV2
		}
		if format != archive.ExportFormatTreeholeV2 && format != archive.ExportFormatMarkdown {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "format must be treehole-v2 or markdown", gin.H{"field": "format"})
			return
		}
		if len(body.PIDs) > 2000 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "no more than 2000 PIDs may be selected", gin.H{"field": "pids"})
			return
		}
		seen := make(map[int32]bool, len(body.PIDs))
		selected := make([]int32, 0, len(body.PIDs))
		for _, pid := range body.PIDs {
			if pid <= 0 {
				apiFailure(c, http.StatusBadRequest, "invalid_input", "pids must contain positive integers", gin.H{"field": "pids"})
				return
			}
			if !seen[pid] {
				seen[pid] = true
				selected = append(selected, pid)
			}
		}
		includeComments := true
		if body.IncludeComments != nil {
			includeComments = *body.IncludeComments
		}
		exportDir := filepath.Join(dependencies.DataDir, "exports")
		if err := os.MkdirAll(exportDir, 0o700); err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		temporary, err := os.CreateTemp(exportDir, "export-*.zip")
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		path := temporary.Name()
		defer os.Remove(path)
		report, exportErr := dependencies.Archive.Export(c.Request.Context(), temporary, archive.ExportRequest{Format: format, PIDs: selected, IncludeComments: includeComments})
		closeErr := temporary.Close()
		if exportErr != nil || closeErr != nil {
			apiFailure(c, http.StatusBadRequest, "export_failed", errors.Join(exportErr, closeErr).Error(), nil)
			return
		}
		c.Header("X-Export-Posts", strconv.Itoa(report.Posts))
		c.Header("X-Export-Comments", strconv.Itoa(report.Comments))
		name := "pkuhole-studio-" + time.Now().Format("20060102-150405")
		if format == archive.ExportFormatMarkdown {
			name += "-markdown.zip"
		} else {
			name += ".treehole.zip"
		}
		c.FileAttachment(path, name)
	}
}

func apiSession(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Auth == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "online authentication is unavailable", nil)
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Auth.CachedStatus(c.Request.Context()))
	}
}

func apiProbeSession(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Auth == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "online authentication is unavailable", nil)
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Auth.Probe(c.Request.Context()))
	}
}

func apiLoginSession(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	return func(c *gin.Context) {
		if dependencies.Auth == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "online authentication is unavailable", nil)
			return
		}
		if !requireLoopback(c) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if body.Username == "" || body.Password == "" || len(body.Username) > 128 || len(body.Password) > 512 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "username and password are required", nil)
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Auth.Login(c.Request.Context(), body.Username, body.Password))
	}
}

func apiContinueSession(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	return func(c *gin.Context) {
		if dependencies.Auth == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "online authentication is unavailable", nil)
			return
		}
		if !requireLoopback(c) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		body.Challenge = strings.TrimSpace(body.Challenge)
		body.Code = strings.TrimSpace(body.Code)
		if (body.Challenge != "sms" && body.Challenge != "otp") || body.Code == "" || len(body.Code) > 32 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "a supported challenge and verification code are required", nil)
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Auth.Continue(c.Request.Context(), body.Challenge, body.Code))
	}
}

func requireLoopback(c *gin.Context) bool {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || !ip.IsLoopback() {
		apiFailure(c, http.StatusForbidden, "local_access_required", "login endpoints are only available from this computer", nil)
		return false
	}
	return true
}

type aiAPIService interface {
	service.AIService
	Providers() []aipkg.ProviderInfo
	CreateSession(ctx context.Context, mode, title string) (models.AISession, error)
	ListSessions(ctx context.Context, limit int) ([]models.AISession, error)
	GetSession(ctx context.Context, id string) (aipkg.SessionDetail, error)
	Events(ctx context.Context, sessionID string) (<-chan service.AIEvent, error)
	LiveSearchEnabled() bool
}

func apiHealth(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := gin.H{"status": "ok"}
		if dependencies.Posts != nil {
			posts, comments, err := dependencies.Posts.Counts(c.Request.Context(), service.SourceLocal)
			if err == nil {
				result["posts"] = posts
				result["comments"] = comments
			}
		}
		apiRespond(c, http.StatusOK, result)
	}
}

func apiCapabilities(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		fts, schemaVersion := false, 0
		aiConfigured, liveSearch := false, false
		if dependencies.Repository != nil {
			fts, _ = dependencies.Repository.FTS5Available()
			schemaVersion, _ = dependencies.Repository.SchemaVersion()
		}
		if aiService, ok := dependencies.AI.(aiAPIService); ok {
			for _, provider := range aiService.Providers() {
				aiConfigured = aiConfigured || provider.Configured
			}
			liveSearch = aiService.LiveSearchEnabled()
		}
		apiRespond(c, http.StatusOK, gin.H{
			"api_version": "v1", "schema_version": schemaVersion, "fts5": fts,
			"archive_import": dependencies.Archive != nil, "archive_export": dependencies.Archive != nil, "jobs": dependencies.Jobs != nil,
			"ai": aiConfigured, "live_search": liveSearch, "online_sync": dependencies.Auth != nil,
		})
	}
}

func apiAIProviders(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		apiRespond(c, http.StatusOK, aiService.Providers())
	}
}

func apiAISessions(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		limit, valid := boundedIntQuery(c, "limit", 50, 1, 100)
		if !valid {
			return
		}
		sessions, err := aiService.ListSessions(c.Request.Context(), limit)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, sessions)
	}
}

func apiCreateAISession(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Mode  string `json:"mode"`
		Title string `json:"title,omitempty"`
	}
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		session, err := aiService.CreateSession(c.Request.Context(), body.Mode, body.Title)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, session)
	}
}

func apiAISession(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		detail, err := aiService.GetSession(c.Request.Context(), c.Param("id"))
		if err != nil {
			apiFailure(c, http.StatusNotFound, "not_found", "AI session was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, detail)
	}
}

func apiCreateAIMessage(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Prompt   string   `json:"prompt"`
		PIDs     []int32  `json:"pids,omitempty"`
		Course   string   `json:"course,omitempty"`
		Teachers []string `json:"teachers,omitempty"`
	}
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		for _, pid := range body.PIDs {
			if pid <= 0 {
				apiFailure(c, http.StatusBadRequest, "invalid_input", "pids must contain positive integers", gin.H{"field": "pids"})
				return
			}
		}
		detail, err := aiService.GetSession(c.Request.Context(), c.Param("id"))
		if err != nil {
			apiFailure(c, http.StatusNotFound, "not_found", "AI session was not found", nil)
			return
		}
		events, err := aiService.Run(context.Background(), service.AIRequest{SessionID: detail.Session.ID, Mode: detail.Session.Mode, Prompt: body.Prompt, PIDs: body.PIDs, Course: body.Course, Teachers: body.Teachers})
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "ai_rejected", err.Error(), nil)
			return
		}
		go func() {
			for range events {
			}
		}()
		apiRespond(c, http.StatusAccepted, gin.H{"session_id": detail.Session.ID, "status": "started"})
	}
}

func apiAIEvents(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		events, err := aiService.Events(c.Request.Context(), c.Param("id"))
		if err != nil {
			apiFailure(c, http.StatusNotFound, "not_found", err.Error(), nil)
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		c.Writer.Flush()
		for {
			select {
			case event, open := <-events:
				if !open {
					return
				}
				encoded, _ := json.Marshal(event.Data)
				_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, encoded)
				c.Writer.Flush()
			case <-c.Request.Context().Done():
				return
			}
		}
	}
}

func apiAICancel(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		aiService, ok := dependencies.AI.(aiAPIService)
		if !ok {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "AI service is unavailable", nil)
			return
		}
		if err := aiService.Cancel(c.Param("id")); err != nil {
			apiFailure(c, http.StatusConflict, "invalid_state", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"status": "cancelling"})
	}
}

func apiPosts(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, ok := parsePostQuery(c)
		if !ok {
			return
		}
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		var page service.PostPage
		var err error
		if strings.TrimSpace(query.Query) != "" || len(query.Origins) > 0 || len(query.TagIDs) > 0 || query.From > 0 || query.To > 0 || query.HasMedia != nil {
			if dependencies.Search == nil {
				apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "search service is unavailable", nil)
				return
			}
			page, err = dependencies.Search.Search(c.Request.Context(), query)
		} else {
			page, err = dependencies.Posts.List(c.Request.Context(), query)
		}
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, page)
	}
}

func apiPost(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		limit, ok := boundedIntQuery(c, "comment_limit", 50, 1, 100)
		if !ok {
			return
		}
		detail, err := dependencies.Posts.Get(c.Request.Context(), pid, service.CommentQuery{Limit: limit, Sort: c.DefaultQuery("comment_sort", "asc"), Source: normalizedAPISource(c.Query("source"))})
		if err != nil {
			apiFailure(c, http.StatusNotFound, "not_found", "post was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, detail)
	}
}

func apiComments(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		cursor, ok := int64Query(c, "cursor", 0, 0)
		if !ok || cursor > int64(^uint32(0)>>1) {
			if ok {
				apiFailure(c, http.StatusBadRequest, "invalid_input", "cursor is out of range", gin.H{"field": "cursor"})
			}
			return
		}
		limit, ok := boundedIntQuery(c, "limit", 50, 1, 100)
		if !ok {
			return
		}
		page, err := dependencies.Posts.Comments(c.Request.Context(), pid, service.CommentQuery{Cursor: int32(cursor), Limit: limit, Sort: c.DefaultQuery("sort", "asc"), Source: normalizedAPISource(c.Query("source"))})
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, page)
	}
}

func apiSearch(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.Query("q")) == "" {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "q is required", gin.H{"field": "q"})
			return
		}
		query, ok := parsePostQuery(c)
		if !ok {
			return
		}
		if dependencies.Search == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "search service is unavailable", nil)
			return
		}
		page, err := dependencies.Search.Search(c.Request.Context(), query)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "search_failed", err.Error(), nil)
			return
		}
		if dependencies.Repository != nil && query.Source == service.SourceLocal {
			filters, _ := json.Marshal(query)
			_ = dependencies.Repository.RecordSearch(query.Query, string(filters))
		}
		apiRespond(c, http.StatusOK, page)
	}
}

func apiSearchHistory(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Repository == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "search history is unavailable", nil)
			return
		}
		limit, ok := boundedIntQuery(c, "limit", 12, 1, 100)
		if !ok {
			return
		}
		rows, err := dependencies.Repository.ListSearchHistory(limit)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, rows)
	}
}

func apiMedia(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" || dependencies.Media == nil {
			apiFailure(c, http.StatusNotFound, "not_found", "media was not found", nil)
			return
		}
		file, err := dependencies.Media.Locate(c.Request.Context(), service.MediaRequest{ID: id, Thumbnail: c.Query("thumbnail") == "true"})
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "not_found", "media was not found", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "media_failed", err.Error(), nil)
			return
		}
		if file.ContentType != "" {
			c.Header("Content-Type", file.ContentType)
		}
		c.Header("Cache-Control", "private, max-age=3600")
		c.File(file.Path)
	}
}

func apiJobs(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
			return
		}
		limit, ok := boundedIntQuery(c, "limit", 50, 1, 200)
		if !ok {
			return
		}
		items, err := dependencies.Jobs.List(c.Request.Context(), limit)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		result := make([]publicJob, len(items))
		for i, item := range items {
			result[i] = toPublicJob(item)
		}
		apiRespond(c, http.StatusOK, result)
	}
}

func apiCreateJob(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Type       jobs.Type       `json:"type"`
		Payload    json.RawMessage `json:"payload,omitempty"`
		TotalItems int             `json:"total_items,omitempty"`
	}
	return func(c *gin.Context) {
		if dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		if !body.Type.Valid() || body.Type == jobs.TypeImportArchive {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "unsupported job type", gin.H{"field": "type"})
			return
		}
		var payload any
		if len(body.Payload) > 0 {
			payload = body.Payload
		}
		job, err := dependencies.Jobs.Create(c.Request.Context(), jobs.CreateRequest{Type: body.Type, Payload: payload, TotalItems: body.TotalItems})
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "job_rejected", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusAccepted, toPublicJob(job))
	}
}

func apiJob(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		job, ok := getAPIJob(c, dependencies)
		if ok {
			apiRespond(c, http.StatusOK, toPublicJob(job))
		}
	}
}

func apiJobAction(dependencies Dependencies, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
			return
		}
		var job jobs.Job
		var err error
		switch action {
		case "pause":
			job, err = dependencies.Jobs.Pause(c.Request.Context(), c.Param("id"))
		case "resume":
			job, err = dependencies.Jobs.Resume(c.Request.Context(), c.Param("id"))
		case "cancel":
			job, err = dependencies.Jobs.Cancel(c.Request.Context(), c.Param("id"))
		case "retry":
			job, err = dependencies.Jobs.Retry(c.Request.Context(), c.Param("id"))
		}
		if err != nil {
			status, code := http.StatusConflict, "invalid_state"
			if errors.Is(err, jobs.ErrNotFound) {
				status, code = http.StatusNotFound, "not_found"
			}
			apiFailure(c, status, code, err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, toPublicJob(job))
	}
}

func apiJobEvents(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
			return
		}
		after := int64(0)
		value := c.GetHeader("Last-Event-ID")
		if value == "" {
			value = c.Query("after")
		}
		if value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed < 0 {
				apiFailure(c, http.StatusBadRequest, "invalid_input", "Last-Event-ID must be a non-negative integer", nil)
				return
			}
			after = parsed
		}
		events, err := dependencies.Jobs.Events(c.Request.Context(), c.Param("id"), after)
		if err != nil {
			apiFailure(c, http.StatusNotFound, "not_found", "job was not found", nil)
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		c.Writer.Flush()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case event, open := <-events:
				if !open {
					return
				}
				encoded, _ := json.Marshal(event)
				_, _ = fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, encoded)
				c.Writer.Flush()
				if terminalEvent(event.Type) {
					return
				}
			case <-heartbeat.C:
				_, _ = io.WriteString(c.Writer, ": heartbeat\n\n")
				c.Writer.Flush()
			case <-c.Request.Context().Done():
				return
			}
		}
	}
}

func apiCreateImport(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Archive == nil || dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "archive import is unavailable", nil)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, archive.MaxArchiveBytes+(1<<20))
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "multipart field file is required", gin.H{"field": "file"})
			return
		}
		defer file.Close()
		stagingDir := filepath.Join(dependencies.DataDir, "imports", "staging")
		if err := os.MkdirAll(stagingDir, 0o700); err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		staged, err := os.CreateTemp(stagingDir, "upload-*.treehole.zip")
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		stagedPath := staged.Name()
		keep := false
		defer func() {
			_ = staged.Close()
			if !keep {
				_ = os.Remove(stagedPath)
			}
		}()
		written, copyErr := io.Copy(staged, io.LimitReader(file, archive.MaxArchiveBytes+1))
		if copyErr != nil || written <= 0 || written > archive.MaxArchiveBytes {
			apiFailure(c, http.StatusBadRequest, "invalid_archive", "archive is empty, unreadable, or too large", nil)
			return
		}
		if err := staged.Sync(); err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		if _, err := staged.Seek(0, io.SeekStart); err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		preflight, err := dependencies.Archive.Preflight(c.Request.Context(), staged, written)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_archive", err.Error(), gin.H{"filename": header.Filename})
			return
		}
		if preflight.Counts.ValidItems == 0 {
			apiFailure(c, http.StatusUnprocessableEntity, "archive_no_valid_items", "archive contains no valid items", gin.H{
				"filename":  header.Filename,
				"preflight": preflight,
			})
			return
		}
		absolutePath, _ := filepath.Abs(stagedPath)
		job, err := dependencies.Jobs.Create(c.Request.Context(), jobs.CreateRequest{Type: jobs.TypeImportArchive, Payload: gin.H{"path": absolutePath, "size": written}, TotalItems: 1})
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "job_rejected", err.Error(), nil)
			return
		}
		keep = true
		apiRespond(c, http.StatusAccepted, gin.H{"job": toPublicJob(job), "preflight": preflight})
	}
}

func apiImport(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		job, ok := getAPIJob(c, dependencies)
		if !ok {
			return
		}
		if job.Type != jobs.TypeImportArchive {
			apiFailure(c, http.StatusNotFound, "not_found", "import was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, toPublicJob(job))
	}
}

func parsePostQuery(c *gin.Context) (service.PostQuery, bool) {
	cursor, ok := boundedIntQuery(c, "cursor", 0, 0, int(^uint(0)>>1))
	if !ok {
		return service.PostQuery{}, false
	}
	limit, ok := boundedIntQuery(c, "limit", 25, 1, 100)
	if !ok {
		return service.PostQuery{}, false
	}
	from, ok := int64Query(c, "from", 0, 0)
	if !ok {
		return service.PostQuery{}, false
	}
	to, ok := int64Query(c, "to", 0, 0)
	if !ok {
		return service.PostQuery{}, false
	}
	query := service.PostQuery{Cursor: cursor, Limit: limit, Query: c.Query("q"), Source: normalizedAPISource(c.Query("source")), Sort: c.Query("sort"), From: from, To: to}
	if raw, exists := c.GetQuery("has_media"); exists {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "has_media must be true or false", gin.H{"field": "has_media"})
			return service.PostQuery{}, false
		}
		query.HasMedia = &value
	}
	query.Origins = splitQueryValues(c.QueryArray("origin"))
	for _, raw := range splitQueryValues(c.QueryArray("tag")) {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "tag values must be positive integers", gin.H{"field": "tag"})
			return service.PostQuery{}, false
		}
		query.TagIDs = append(query.TagIDs, uint(value))
	}
	return query, true
}

func normalizedAPISource(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), service.SourceLive) {
		return service.SourceLive
	}
	return service.SourceLocal
}

func splitQueryValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func positiveInt32Param(c *gin.Context, name string) (int32, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 32)
	if err != nil || value <= 0 {
		apiFailure(c, http.StatusBadRequest, "invalid_input", name+" must be a positive integer", gin.H{"field": name})
		return 0, false
	}
	return int32(value), true
}

func boundedIntQuery(c *gin.Context, name string, fallback, minimum, maximum int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		apiFailure(c, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%s must be between %d and %d", name, minimum, maximum), gin.H{"field": name})
		return 0, false
	}
	return value, true
}

func int64Query(c *gin.Context, name string, fallback, minimum int64) (int64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minimum {
		apiFailure(c, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%s must be at least %d", name, minimum), gin.H{"field": name})
		return 0, false
	}
	return value, true
}

func decodeAPIJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		apiFailure(c, http.StatusBadRequest, "invalid_input", "request body is not valid JSON", gin.H{"reason": err.Error()})
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		apiFailure(c, http.StatusBadRequest, "invalid_input", "request body must contain one JSON value", nil)
		return false
	}
	return true
}

func getAPIJob(c *gin.Context, dependencies Dependencies) (jobs.Job, bool) {
	if dependencies.Jobs == nil {
		apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
		return jobs.Job{}, false
	}
	job, err := dependencies.Jobs.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		apiFailure(c, http.StatusNotFound, "not_found", "job was not found", nil)
		return jobs.Job{}, false
	}
	return job, true
}

func toPublicJob(job jobs.Job) publicJob {
	return publicJob{
		ID: job.ID, Type: job.Type, Status: job.Status, Checkpoint: job.Checkpoint,
		CompletedItems: job.CompletedItems, FailedItems: job.FailedItems, TotalItems: job.TotalItems,
		Attempts: job.Attempts, Error: job.Error, StartedAt: job.StartedAt, FinishedAt: job.FinishedAt,
		CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}
}

func terminalEvent(eventType string) bool {
	switch eventType {
	case string(jobs.StatusCompleted), string(jobs.StatusPartial), string(jobs.StatusFailed), string(jobs.StatusCancelled):
		return true
	default:
		return false
	}
}

func apiRespond(c *gin.Context, status int, data any) {
	c.JSON(status, apiEnvelope{Data: data})
}

func apiFailure(c *gin.Context, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	c.AbortWithStatusJSON(status, apiErrorEnvelope{Error: apiErrorBody{Code: code, Message: message, Details: details}})
}
