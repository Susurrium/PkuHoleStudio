package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	group.POST("/posts", apiCreatePost(dependencies))
	group.GET("/posts/hot", apiHotPosts(dependencies))
	group.GET("/posts/:pid", apiPost(dependencies))
	group.GET("/posts/:pid/comments", apiComments(dependencies))
	group.GET("/posts/:pid/references", apiPostReferenceGraph(dependencies))
	group.POST("/posts/:pid/comments", apiCreateComment(dependencies))
	group.POST("/posts/:pid/praise", apiPostToggle(dependencies, "praise"))
	group.POST("/posts/:pid/follow", apiPostToggle(dependencies, "follow"))
	group.GET("/tags", apiTags(dependencies))
	group.GET("/search", apiSearch(dependencies))
	group.GET("/search/history", apiSearchHistory(dependencies))
	group.GET("/media/:id", apiMedia(dependencies))
	group.POST("/media/uploads", apiUploadMedia(dependencies))
	group.GET("/remote-media/:id", apiRemoteMedia(dependencies))
	group.GET("/session", apiSession(dependencies))
	group.POST("/session/probe", apiProbeSession(dependencies))
	group.POST("/session/login", apiLoginSession(dependencies))
	group.POST("/session/sms", apiSendSessionSMS(dependencies))
	group.POST("/session/challenge", apiContinueSession(dependencies))
	group.POST("/session/logout", apiLogoutSession(dependencies))
	group.GET("/notifications", apiNotifications(dependencies))
	group.POST("/notifications/:id/read", apiNotificationRead(dependencies))
	group.POST("/notifications/read-all", apiNotificationsReadAll(dependencies))
	group.GET("/logs", apiLogs(dependencies))
	group.POST("/logs/clear", apiClearLogs(dependencies))
	group.GET("/campus/schedule", apiCampusSchedule(dependencies))
	group.GET("/campus/scores", apiCampusScores(dependencies))
	group.GET("/local-tags", apiLocalTags(dependencies))
	group.POST("/local-tags", apiCreateLocalTag(dependencies))
	group.PATCH("/local-tags/:id", apiUpdateLocalTag(dependencies))
	group.DELETE("/local-tags/:id", apiDeleteLocalTag(dependencies))
	group.GET("/posts/:pid/tags", apiPostTags(dependencies))
	group.PUT("/posts/:pid/tags", apiSetPostTags(dependencies))
	group.GET("/posts/:pid/note", apiPostNote(dependencies))
	group.PUT("/posts/:pid/note", apiSavePostNote(dependencies))
	group.GET("/comments/:cid/note", apiCommentNote(dependencies))
	group.PUT("/comments/:cid/note", apiSaveCommentNote(dependencies))
	group.GET("/settings", apiSettings(dependencies))
	group.PUT("/settings", apiUpdateSettings(dependencies))

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
	group.GET("/exports/jobs", apiExportJobs(dependencies))
	group.POST("/exports/jobs", apiCreateExportJob(dependencies))
	group.GET("/exports/:id/download", apiDownloadExport(dependencies))
	group.POST("/bridge/pairings", apiCreateBridgePairing(dependencies))
	group.GET("/bridge/pairings/:token", apiBridgePairing(dependencies))
	group.POST("/bridge/pairings/:token/archive", apiUploadBridgeArchive(dependencies))
	group.POST("/bridge/pairings/:token/confirm", apiConfirmBridgePairing(dependencies))
	group.POST("/bridge/pairings/:token/cancel", apiCancelBridgePairing(dependencies))

	group.GET("/ai/providers", apiAIProviders(dependencies))
	group.GET("/ai/sessions", apiAISessions(dependencies))
	group.POST("/ai/sessions", apiCreateAISession(dependencies))
	group.GET("/ai/sessions/:id", apiAISession(dependencies))
	group.POST("/ai/sessions/:id/messages", apiCreateAIMessage(dependencies))
	group.GET("/ai/sessions/:id/events", apiAIEvents(dependencies))
	group.POST("/ai/sessions/:id/cancel", apiAICancel(dependencies))
}

func apiCreateBridgePairing(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "Toolkit bridge is unavailable", nil)
			return
		}
		if !requireStudioBrowser(c) {
			return
		}
		host, port, err := net.SplitHostPort(c.Request.Host)
		_ = host
		if err != nil || port == "" {
			apiFailure(c, http.StatusBadRequest, "invalid_host", "the local Web port could not be determined", nil)
			return
		}
		pairing, err := dependencies.Bridge.Create(port + ":")
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "pairing_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, pairing)
	}
}

func notificationType(value string) (models.NotificationType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "interactive", "int_msg":
		return models.NotificationTypeInteractive, true
	case "system", "sys_msg":
		return models.NotificationTypeSystem, true
	default:
		return "", false
	}
}

func apiLogoutSession(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Auth == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "authentication is unavailable", nil)
			return
		}
		if err := dependencies.Auth.Logout(c.Request.Context()); err != nil {
			apiFailure(c, http.StatusInternalServerError, "logout_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, service.AuthStatus{Checked: true, Message: "已退出本机会话"})
	}
}

func apiNotifications(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Notifications == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "notification service is unavailable", nil)
			return
		}
		kind, ok := notificationType(c.Query("type"))
		if !ok {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "notification type must be interactive or system", gin.H{"field": "type"})
			return
		}
		page, ok := boundedIntQuery(c, "page", 1, 1, 10_000)
		if !ok {
			return
		}
		limit, ok := boundedIntQuery(c, "limit", 50, 1, 100)
		if !ok {
			return
		}
		items, total, err := dependencies.Notifications.List(c.Request.Context(), kind, page, limit)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "notifications_unavailable", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"items": items, "total": total, "page": page})
	}
}

func apiNotificationRead(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Notifications == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "notification service is unavailable", nil)
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "notification id must be positive", gin.H{"field": "id"})
			return
		}
		if err := dependencies.Notifications.MarkRead(c.Request.Context(), id); err != nil {
			apiFailure(c, http.StatusBadGateway, "notification_update_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"id": id, "read": true})
	}
}

func apiNotificationsReadAll(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Notifications == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "notification service is unavailable", nil)
			return
		}
		kind, ok := notificationType(c.Query("type"))
		if !ok {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "notification type must be interactive or system", gin.H{"field": "type"})
			return
		}
		if err := dependencies.Notifications.MarkAllRead(c.Request.Context(), kind); err != nil {
			apiFailure(c, http.StatusBadGateway, "notification_update_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"type": kind, "read": true})
	}
}

func apiLogs(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Logs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "log service is unavailable", nil)
			return
		}
		limit, ok := boundedIntQuery(c, "limit", 500, 1, 5_000)
		if !ok {
			return
		}
		items, err := dependencies.Logs.List(c.Request.Context(), c.Query("module"), c.Query("q"), limit)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, items)
	}
}

func apiClearLogs(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Logs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "log service is unavailable", nil)
			return
		}
		if err := dependencies.Logs.Clear(c.Request.Context(), c.Query("module")); err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"cleared": true})
	}
}

func apiCampusSchedule(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		rows, err := dependencies.Posts.GetCourseTable(c.Request.Context(), service.SourceLive)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "campus_unavailable", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, rows)
	}
}

func apiCampusScores(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		summary, err := dependencies.Posts.GetCourseScores(c.Request.Context(), service.SourceLive)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "campus_unavailable", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, summary)
	}
}

func requireLibrary(c *gin.Context, dependencies Dependencies) bool {
	if dependencies.Library != nil {
		return true
	}
	apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "local library service is unavailable", nil)
	return false
}

func apiLocalTags(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireLibrary(c, dependencies) {
			return
		}
		rows, err := dependencies.Library.Tags(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, rows)
	}
}

func apiCreateLocalTag(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireLibrary(c, dependencies) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		row, err := dependencies.Library.CreateTag(c.Request.Context(), body.Name, body.Color)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, row)
	}
}

func apiUpdateLocalTag(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireLibrary(c, dependencies) {
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "tag id must be positive", nil)
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		row, err := dependencies.Library.UpdateTag(c.Request.Context(), uint(id), body.Name, body.Color)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, row)
	}
}

func apiDeleteLocalTag(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireLibrary(c, dependencies) {
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "tag id must be positive", nil)
			return
		}
		if err := dependencies.Library.DeleteTag(c.Request.Context(), uint(id)); err != nil {
			apiFailure(c, http.StatusInternalServerError, "delete_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"deleted": true})
	}
}

func apiPostTags(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireLibrary(c, dependencies) {
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		rows, err := dependencies.Library.PostTags(c.Request.Context(), pid)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, rows)
	}
}

func apiSetPostTags(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		TagIDs []uint `json:"tag_ids"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireLibrary(c, dependencies) {
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		if err := dependencies.Library.SetPostTags(c.Request.Context(), pid, body.TagIDs); err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		rows, err := dependencies.Library.PostTags(c.Request.Context(), pid)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, rows)
	}
}

func apiPostNote(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireLibrary(c, dependencies) {
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		row, err := dependencies.Library.Note(c.Request.Context(), "post", int64(pid))
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		if row == nil {
			apiRespond(c, http.StatusOK, gin.H{"owner_type": "post", "owner_id": pid, "content": ""})
			return
		}
		apiRespond(c, http.StatusOK, row)
	}
}

func apiPostReferenceGraph(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		depth := 1
		if raw := strings.TrimSpace(c.Query("depth")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 2 {
				apiFailure(c, http.StatusBadRequest, "invalid_input", "depth must be 1 or 2", nil)
				return
			}
			depth = parsed
		}
		graph, err := dependencies.Posts.ReferenceGraph(c.Request.Context(), pid, depth)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, graph)
	}
}

func apiSavePostNote(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Content string `json:"content"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireLibrary(c, dependencies) {
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		row, err := dependencies.Library.SaveNote(c.Request.Context(), "post", int64(pid), body.Content)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, row)
	}
}

func apiCommentNote(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireLibrary(c, dependencies) {
			return
		}
		cid, ok := positiveInt32Param(c, "cid")
		if !ok {
			return
		}
		row, err := dependencies.Library.Note(c.Request.Context(), "comment", int64(cid))
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		if row == nil {
			apiRespond(c, http.StatusOK, gin.H{"owner_type": "comment", "owner_id": cid, "content": ""})
			return
		}
		apiRespond(c, http.StatusOK, row)
	}
}

func apiSaveCommentNote(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Content string `json:"content"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireLibrary(c, dependencies) {
			return
		}
		cid, ok := positiveInt32Param(c, "cid")
		if !ok {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		row, err := dependencies.Library.SaveNote(c.Request.Context(), "comment", int64(cid), body.Content)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, row)
	}
}

func apiSettings(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Settings == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "settings service is unavailable", nil)
			return
		}
		view, err := dependencies.Settings.Get(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "settings_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, view)
	}
}

func apiUpdateSettings(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Settings == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "settings service is unavailable", nil)
			return
		}
		var update service.SettingsUpdate
		if !decodeAPIJSON(c, &update) {
			return
		}
		view, err := dependencies.Settings.Update(c.Request.Context(), update)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_settings", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, view)
	}
}

func apiBridgePairing(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBrowser(c) {
			return
		}
		pairing, ok := dependencies.Bridge.Get(c.Param("token"))
		if !ok {
			apiFailure(c, http.StatusNotFound, "pairing_not_found", "pairing expired or was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, pairing)
	}
}

func apiUploadBridgeArchive(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireLoopback(c) {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, archive.MaxArchiveBytes+(1<<20))
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "multipart field file is required", nil)
			return
		}
		defer file.Close()
		pairing, err := dependencies.Bridge.Upload(c.Request.Context(), c.Param("token"), header.Filename, file)
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "pairing_not_found", "pairing expired or was not found", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_archive", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusAccepted, pairing)
	}
}

func apiConfirmBridgePairing(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBrowser(c) {
			return
		}
		pairing, err := dependencies.Bridge.Confirm(c.Request.Context(), c.Param("token"))
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "pairing_not_found", "pairing expired or was not found", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusConflict, "pairing_not_ready", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusAccepted, pairing)
	}
}

func apiCancelBridgePairing(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBrowser(c) {
			return
		}
		if !dependencies.Bridge.Cancel(c.Param("token")) {
			apiFailure(c, http.StatusNotFound, "pairing_not_found", "pairing expired or was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"status": "cancelled"})
	}
}

func apiSendSessionSMS(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Username string `json:"username"`
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
		if strings.TrimSpace(body.Username) == "" || len(body.Username) > 128 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "username is required", nil)
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Auth.SendSMS(c.Request.Context(), body.Username))
	}
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

type exportJobCheckpoint struct {
	Filename  string               `json:"filename"`
	ExpiresAt time.Time            `json:"expires_at"`
	Report    archive.ExportReport `json:"report"`
}

func apiCreateExportJob(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Format          archive.ExportFormat `json:"format"`
		PIDs            []int32              `json:"pids"`
		IncludeComments bool                 `json:"include_comments"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Jobs == nil || dependencies.Archive == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "persistent archive export is unavailable", nil)
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		if body.Format == "" {
			body.Format = archive.ExportFormatTreeholeV2
		}
		if body.Format != archive.ExportFormatTreeholeV2 && body.Format != archive.ExportFormatMarkdown {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "format must be treehole-v2 or markdown", nil)
			return
		}
		if len(body.PIDs) > 2000 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "at most 2000 PIDs may be exported", nil)
			return
		}
		for _, pid := range body.PIDs {
			if pid <= 0 {
				apiFailure(c, http.StatusBadRequest, "invalid_input", "PIDs must be positive", nil)
				return
			}
		}
		job, err := dependencies.Jobs.Create(c.Request.Context(), jobs.CreateRequest{Type: jobs.TypeExportArchive, Payload: body, TotalItems: 1})
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "job_create_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusAccepted, toPublicJob(job))
	}
}

func apiExportJobs(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
			return
		}
		rows, err := dependencies.Jobs.List(c.Request.Context(), 200)
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "query_failed", err.Error(), nil)
			return
		}
		result := make([]publicJob, 0)
		for _, row := range rows {
			if row.Type == jobs.TypeExportArchive {
				result = append(result, toPublicJob(row))
			}
		}
		apiRespond(c, http.StatusOK, result)
	}
}

func apiDownloadExport(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if dependencies.Jobs == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "job manager is unavailable", nil)
			return
		}
		job, err := dependencies.Jobs.Get(c.Request.Context(), c.Param("id"))
		if err != nil || job.Type != jobs.TypeExportArchive {
			apiFailure(c, http.StatusNotFound, "export_not_found", "export job was not found", nil)
			return
		}
		if job.Status != jobs.StatusCompleted {
			apiFailure(c, http.StatusConflict, "export_not_ready", "export job is not completed", gin.H{"status": job.Status})
			return
		}
		var checkpoint exportJobCheckpoint
		if json.Unmarshal(job.Checkpoint, &checkpoint) != nil || filepath.Base(checkpoint.Filename) != checkpoint.Filename || checkpoint.Filename == "" {
			apiFailure(c, http.StatusInternalServerError, "export_invalid", "export checkpoint is invalid", nil)
			return
		}
		if !checkpoint.ExpiresAt.IsZero() && time.Now().UTC().After(checkpoint.ExpiresAt) {
			_ = os.Remove(filepath.Join(dependencies.DataDir, "exports", checkpoint.Filename))
			apiFailure(c, http.StatusGone, "export_expired", "export file has expired; retry the job to regenerate it", nil)
			return
		}
		path := filepath.Join(dependencies.DataDir, "exports", checkpoint.Filename)
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			apiFailure(c, http.StatusGone, "export_missing", "export file is no longer available; retry the job", nil)
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, checkpoint.Filename))
		c.File(path)
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
		Stage     string `json:"stage,omitempty"`
		Challenge string `json:"challenge"`
		Username  string `json:"username,omitempty"`
		Password  string `json:"password,omitempty"`
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
		body.Stage = strings.TrimSpace(body.Stage)
		body.Code = strings.TrimSpace(body.Code)
		if (body.Challenge != "sms" && body.Challenge != "otp") || body.Code == "" || len(body.Code) > 32 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "a supported challenge and verification code are required", nil)
			return
		}
		if body.Stage == "iaaa" && (strings.TrimSpace(body.Username) == "" || body.Password == "") {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "username and password are required for IAAA verification", nil)
			return
		}
		if body.Stage != "" && body.Stage != "iaaa" && body.Stage != "treehole" {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "unsupported challenge stage", nil)
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Auth.Continue(c.Request.Context(), body.Stage, body.Challenge, body.Username, body.Password, body.Code))
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

func requireStudioBrowser(c *gin.Context) bool {
	if !requireLoopback(c) {
		return false
	}
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		apiFailure(c, http.StatusForbidden, "local_origin_required", "this action must be started from PkuHoleStudio", nil)
		return false
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		apiFailure(c, http.StatusForbidden, "local_origin_required", "this action must be started from PkuHoleStudio", nil)
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

func apiHotPosts(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Dashboard == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "dashboard service is unavailable", nil)
			return
		}
		items, err := dependencies.Dashboard.HotPosts(c.Request.Context(), 5, 12*time.Hour)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "hot_posts_unavailable", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, items)
	}
}

func apiTags(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Posts == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "post service is unavailable", nil)
			return
		}
		if normalizedAPISource(c.Query("source")) != service.SourceLive {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "tags are available for the live source", gin.H{"field": "source"})
			return
		}
		items, err := dependencies.Posts.ListTags(c.Request.Context(), service.SourceLive)
		if err != nil {
			apiFailure(c, http.StatusServiceUnavailable, "online_unavailable", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, items)
	}
}

func apiCreatePost(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Text     string   `json:"text"`
		MediaIDs []string `json:"media_ids,omitempty"`
	}
	return func(c *gin.Context) {
		if !requireOnlineWrite(c, dependencies) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		if len(body.Text) > 10_000 || !validRemoteMediaIDs(body.MediaIDs) {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "post text or media ids are invalid", nil)
			return
		}
		post, err := dependencies.Posts.PublishPost(c.Request.Context(), body.Text, body.MediaIDs, service.SourceLive)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "publish_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, post)
	}
}

func apiCreateComment(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Text     string   `json:"text"`
		QuoteCID *int32   `json:"quote_cid,omitempty"`
		MediaIDs []string `json:"media_ids,omitempty"`
	}
	return func(c *gin.Context) {
		if !requireOnlineWrite(c, dependencies) {
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		if len(body.Text) > 10_000 || (body.QuoteCID != nil && *body.QuoteCID <= 0) || !validRemoteMediaIDs(body.MediaIDs) {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "comment text, quote cid, or media ids are invalid", nil)
			return
		}
		comment, err := dependencies.Posts.Reply(c.Request.Context(), pid, body.Text, body.QuoteCID, body.MediaIDs, service.SourceLive)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "reply_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, comment)
	}
}

func apiPostToggle(dependencies Dependencies, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireOnlineWrite(c, dependencies) {
			return
		}
		pid, ok := positiveInt32Param(c, "pid")
		if !ok {
			return
		}
		var err error
		if action == "praise" {
			err = dependencies.Posts.TogglePraise(c.Request.Context(), pid, service.SourceLive)
		} else {
			err = dependencies.Posts.ToggleAttention(c.Request.Context(), pid, service.SourceLive)
		}
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "interaction_failed", err.Error(), nil)
			return
		}
		post, err := dependencies.Posts.RefreshPost(c.Request.Context(), pid, service.SourceLive)
		if err != nil {
			apiRespond(c, http.StatusOK, gin.H{"pid": pid, "updated": true})
			return
		}
		apiRespond(c, http.StatusOK, post)
	}
}

func apiUploadMedia(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireOnlineWrite(c, dependencies) {
			return
		}
		const maxUpload = int64(20 << 20)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUpload+(1<<20))
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "multipart field file is required", gin.H{"field": "file"})
			return
		}
		defer file.Close()
		uploadDir := filepath.Join(dependencies.DataDir, "uploads", "temporary")
		if err := os.MkdirAll(uploadDir, 0o700); err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		staged, err := os.CreateTemp(uploadDir, "upload-*")
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "storage_failed", err.Error(), nil)
			return
		}
		path := staged.Name()
		defer os.Remove(path)
		written, copyErr := io.Copy(staged, io.LimitReader(file, maxUpload+1))
		closeErr := staged.Close()
		if copyErr != nil || closeErr != nil || written <= 0 || written > maxUpload {
			apiFailure(c, http.StatusBadRequest, "invalid_media", "image is empty, unreadable, or too large", nil)
			return
		}
		content, err := os.ReadFile(path)
		if err != nil || !strings.HasPrefix(http.DetectContentType(content), "image/") {
			apiFailure(c, http.StatusBadRequest, "invalid_media", "uploaded file is not an image", gin.H{"filename": header.Filename})
			return
		}
		id, err := dependencies.Posts.UploadMedia(c.Request.Context(), path, service.SourceLive)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "upload_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, gin.H{"id": id, "filename": filepath.Base(header.Filename), "size": written})
	}
}

func requireOnlineWrite(c *gin.Context, dependencies Dependencies) bool {
	if !requireStudioBrowser(c) {
		return false
	}
	if dependencies.Posts == nil || !dependencies.Posts.CanWrite(c.Request.Context(), service.SourceLive) {
		apiFailure(c, http.StatusUnauthorized, "online_login_required", "a writable Treehole session is required", nil)
		return false
	}
	return true
}

func validRemoteMediaIDs(values []string) bool {
	if len(values) > 9 {
		return false
	}
	for _, value := range values {
		if _, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64); err != nil {
			return false
		}
	}
	return true
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
		recordID, parseErr := strconv.ParseUint(id, 10, 64)
		if parseErr != nil || recordID == 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "media id must be a positive integer", gin.H{"field": "id"})
			return
		}
		file, err := dependencies.Media.Locate(c.Request.Context(), service.MediaRequest{RecordID: uint(recordID), Thumbnail: c.Query("thumbnail") == "true"})
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

func apiRemoteMedia(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Media == nil {
			apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "media service is unavailable", nil)
			return
		}
		pidValue, err := strconv.ParseInt(c.Query("pid"), 10, 32)
		if err != nil || pidValue <= 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "pid must be a positive integer", gin.H{"field": "pid"})
			return
		}
		remoteID := strings.TrimSpace(c.Param("id"))
		if remoteID == "_" {
			remoteID = ""
		} else if _, err := strconv.ParseUint(remoteID, 10, 64); err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "remote media id must be numeric", gin.H{"field": "id"})
			return
		}
		request := service.MediaRequest{ID: remoteID, PID: int32(pidValue), Thumbnail: c.Query("thumbnail") == "true"}
		file, locateErr := dependencies.Media.Locate(c.Request.Context(), request)
		if locateErr != nil {
			file, locateErr = dependencies.Media.Download(c.Request.Context(), request)
		}
		if locateErr != nil {
			apiFailure(c, http.StatusBadGateway, "media_failed", locateErr.Error(), nil)
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
		if action == "cancel" && job.Type == jobs.TypeImportArchive {
			removeStagedImportFile(dependencies.DataDir, job.Payload)
		}
		apiRespond(c, http.StatusOK, toPublicJob(job))
	}
}

func removeStagedImportFile(dataDir string, raw json.RawMessage) {
	var payload struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(raw, &payload) != nil || strings.TrimSpace(payload.Path) == "" {
		return
	}
	root, err := filepath.Abs(filepath.Join(dataDir, "imports", "staging"))
	if err != nil {
		return
	}
	path, err := filepath.Abs(payload.Path)
	if err != nil {
		return
	}
	relative, err := filepath.Rel(root, path)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		_ = os.Remove(path)
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
	if raw := strings.TrimSpace(c.Query("label")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "label must be a positive integer", gin.H{"field": "label"})
			return service.PostQuery{}, false
		}
		query.Label = value
	}
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
