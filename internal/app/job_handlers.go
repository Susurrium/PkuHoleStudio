package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

type syncPagesPayload struct {
	StartPage int                  `json:"start_page"`
	Pages     int                  `json:"pages"`
	Options   service.CrawlOptions `json:"options"`
}

type pidJobPayload struct {
	PIDs []int32 `json:"pids"`
}

type mediaJobPayload struct {
	ConvertWebP bool `json:"convert_webp"`
}

type thumbnailJobPayload struct {
	StartID     int  `json:"start_id"`
	EndID       int  `json:"end_id"`
	ConvertWebP bool `json:"convert_webp"`
}

type monitorJobPayload struct {
	Pages           int                  `json:"pages"`
	IntervalSeconds int                  `json:"interval_seconds"`
	Options         service.CrawlOptions `json:"options"`
}

type cleanupJobPayload struct {
	RetentionDays int `json:"retention_days"`
}

type importArchivePayload struct {
	Path string `json:"path"`
	Size int64  `json:"size,omitempty"`
}

type exportArchivePayload struct {
	Format          archive.ExportFormat `json:"format"`
	PIDs            []int32              `json:"pids,omitempty"`
	IncludeComments bool                 `json:"include_comments"`
}

type exportArchiveCheckpoint struct {
	Filename  string               `json:"filename"`
	ExpiresAt time.Time            `json:"expires_at"`
	Report    archive.ExportReport `json:"report"`
}

type rawJSONCheckpoint struct {
	Filename  string    `json:"filename"`
	Responses int       `json:"responses"`
	Bytes     int64     `json:"bytes"`
	ExpiresAt time.Time `json:"expires_at"`
}

func registerJobHandlers(application *App) error {
	if application == nil || application.Jobs == nil {
		return nil
	}
	registrations := []struct {
		typeName jobs.Type
		handler  jobs.Handler
	}{
		{jobs.TypeSyncLatest, application.handleSyncLatest},
		{jobs.TypeSyncFollowed, application.handleSyncFollowed},
		{jobs.TypeSyncPIDs, application.handleSyncPIDs},
		{jobs.TypeRepairComments, application.handleRepairComments},
		{jobs.TypeRepairMedia, application.handleRepairMedia},
		{jobs.TypeImportArchive, application.handleImportArchive},
		{jobs.TypeRebuildSearchIndex, application.handleRebuildSearchIndex},
		{jobs.TypeRebuildReferences, application.handleRebuildReferences},
		{jobs.TypeSyncPages, application.handleSyncLatest},
		{jobs.TypeMonitorLatest, application.handleMonitorLatest},
		{jobs.TypeRepairThumbnails, application.handleRepairThumbnails},
		{jobs.TypeCleanupStaging, application.handleCleanupStaging},
		{jobs.TypeExportArchive, application.handleExportArchive},
		{jobs.TypeSaveRawJSON, application.handleSaveRawJSON},
		{jobs.TypeFetchImages, application.handleFetchImages},
	}
	for _, registration := range registrations {
		if err := application.Jobs.Register(registration.typeName, registration.handler); err != nil {
			return fmt.Errorf("register %s job: %w", registration.typeName, err)
		}
	}
	return nil
}

func (a *App) handleSaveRawJSON(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	if a.Sync == nil {
		return fmt.Errorf("sync service is not configured")
	}
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	directory := filepath.Join(a.DataDir, "raw")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	filename := job.ID + ".json"
	result, err := a.Sync.SaveRawResponsesTo(ctx, filepath.Join(directory, filename))
	if err != nil {
		_ = execution.ItemFailed(ctx, "raw_json", err)
		return err
	}
	checkpoint := rawJSONCheckpoint{Filename: filename, Responses: result.Responses, Bytes: result.Bytes, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)}
	if err := execution.Checkpoint(ctx, checkpoint); err != nil {
		_ = os.Remove(filepath.Join(directory, filename))
		return err
	}
	return execution.ItemSucceeded(ctx, "raw_json", checkpoint)
}

func (a *App) handleFetchImages(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	if a.Sync == nil {
		return fmt.Errorf("sync service is not configured")
	}
	var payload mediaJobPayload
	_ = json.Unmarshal(job.Payload, &payload)
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	if err := a.Sync.FetchImages(ctx, payload.ConvertWebP); err != nil {
		_ = execution.ItemFailed(ctx, "images", err)
		return err
	}
	return execution.ItemSucceeded(ctx, "images", map[string]any{"convert_webp": payload.ConvertWebP})
}

func (a *App) handleExportArchive(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	if a.Archive == nil {
		return fmt.Errorf("archive service is not configured")
	}
	var payload exportArchivePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode export payload: %w", err)
	}
	if payload.Format == "" {
		payload.Format = archive.ExportFormatTreeholeV2
	}
	if payload.Format != archive.ExportFormatTreeholeV2 && payload.Format != archive.ExportFormatMarkdown {
		return fmt.Errorf("unsupported export format %q", payload.Format)
	}
	if len(payload.PIDs) > 2000 {
		return fmt.Errorf("at most 2000 PIDs may be exported")
	}
	for _, pid := range payload.PIDs {
		if pid <= 0 {
			return fmt.Errorf("invalid PID %d", pid)
		}
	}
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	directory := filepath.Join(a.DataDir, "exports")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	suffix := ".treehole.zip"
	if payload.Format == archive.ExportFormatMarkdown {
		suffix = "-markdown.zip"
	}
	filename := job.ID + suffix
	finalPath := filepath.Join(directory, filename)
	temporary, err := os.CreateTemp(directory, ".export-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	report, exportErr := a.Archive.Export(ctx, temporary, archive.ExportRequest{Format: payload.Format, PIDs: payload.PIDs, IncludeComments: payload.IncludeComments})
	closeErr := temporary.Close()
	if exportErr != nil || closeErr != nil {
		failure := errors.Join(exportErr, closeErr)
		_ = execution.ItemFailed(ctx, "archive", failure)
		return failure
	}
	_ = os.Remove(finalPath)
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		_ = execution.ItemFailed(ctx, "archive", err)
		return err
	}
	checkpoint := exportArchiveCheckpoint{Filename: filename, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour), Report: report}
	if err := execution.Checkpoint(ctx, checkpoint); err != nil {
		_ = os.Remove(finalPath)
		return err
	}
	return execution.ItemSucceeded(ctx, "archive", checkpoint)
}

func (a *App) handleImportArchive(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	if a.Archive == nil {
		return fmt.Errorf("archive service is not configured")
	}
	var payload importArchivePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode archive payload: %w", err)
	}
	stagingDir, err := filepath.Abs(filepath.Join(a.DataDir, "imports", "staging"))
	if err != nil {
		return fmt.Errorf("resolve import staging directory: %w", err)
	}
	archivePath, err := filepath.Abs(payload.Path)
	if err != nil {
		return fmt.Errorf("resolve archive path: %w", err)
	}
	relative, err := filepath.Rel(stagingDir, archivePath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("archive path must be a file inside %s", stagingDir)
	}
	removeArchive := false
	defer func() {
		if removeArchive || ctx.Err() != nil {
			_ = os.Remove(archivePath)
		}
	}()
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > archive.MaxArchiveBytes {
		return fmt.Errorf("archive must be a non-empty regular file no larger than %d bytes", archive.MaxArchiveBytes)
	}
	if payload.Size > 0 && payload.Size != info.Size() {
		return fmt.Errorf("archive size changed after upload")
	}
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	report, err := a.Archive.Import(ctx, file, info.Size())
	if err != nil {
		_ = execution.ItemFailed(ctx, "archive", err)
		return err
	}
	if err := execution.ItemSucceeded(ctx, "archive", report); err != nil {
		return err
	}
	if err := execution.Checkpoint(ctx, report); err != nil {
		return err
	}
	if report.Status == archive.StatusCompleted || report.Status == archive.StatusDuplicate {
		removeArchive = true
	}
	if report.Status == archive.StatusPartial {
		return fmt.Errorf("archive imported partially; inspect the import report")
	}
	return nil
}

func (a *App) handleSyncLatest(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	var payload syncPagesPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode sync payload: %w", err)
		}
	}
	if payload.StartPage <= 0 {
		payload.StartPage = 1
	}
	if payload.Pages <= 0 {
		payload.Pages = 1
	}
	if payload.Options.PostLimit <= 0 {
		payload.Options.PostLimit = 200
	}
	if payload.Options.CommentLimit <= 0 {
		payload.Options.CommentLimit = 200
	}
	if err := execution.SetTotal(ctx, payload.Pages); err != nil {
		return err
	}
	completed := completedItemKeys(ctx, execution)
	for offset := 0; offset < payload.Pages; offset++ {
		page := payload.StartPage + offset
		key := "page:" + strconv.Itoa(page)
		if completed[key] {
			continue
		}
		result, err := a.Sync.FetchPage(ctx, page, payload.Options)
		if err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		checkpoint := map[string]any{"page": page, "posts": result.PostCount, "comments": result.CommentCount}
		if err := execution.ItemSucceeded(ctx, key, checkpoint); err != nil {
			return err
		}
		if err := execution.Checkpoint(ctx, checkpoint); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) handleSyncFollowed(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	var payload syncPagesPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode followed sync payload: %w", err)
		}
	}
	if payload.Pages <= 0 {
		payload.Pages = 1
	}
	if payload.Options.PostLimit <= 0 {
		payload.Options.PostLimit = 25
	}
	if err := execution.SetTotal(ctx, payload.Pages); err != nil {
		return err
	}
	completed := completedItemKeys(ctx, execution)
	cursor := 0
	for pageNumber := 1; pageNumber <= payload.Pages; pageNumber++ {
		key := "page:" + strconv.Itoa(pageNumber)
		if completed[key] {
			cursor = pageNumber
			continue
		}
		page, err := a.Posts.List(ctx, service.PostQuery{
			Cursor: cursor, Limit: payload.Options.PostLimit, Query: ":follow", Source: service.SourceLive,
		})
		if err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		posts, comments := flattenPostSummaries(page.Items)
		if err := a.Repository.SaveCrawlResultWithSource(posts, comments, "followed", "treehole-live"); err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		if err := execution.ItemSucceeded(ctx, key, map[string]any{"cursor": page.NextCursor, "posts": len(posts)}); err != nil {
			return err
		}
		cursor = page.NextCursor
		if !page.HasMore {
			break
		}
	}
	return nil
}

func (a *App) handleSyncPIDs(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	payload, err := decodePIDPayload(job.Payload)
	if err != nil {
		return err
	}
	if err := execution.SetTotal(ctx, len(payload.PIDs)); err != nil {
		return err
	}
	completed := completedItemKeys(ctx, execution)
	for _, pid := range payload.PIDs {
		key := "pid:" + strconv.FormatInt(int64(pid), 10)
		if completed[key] {
			continue
		}
		post, err := a.Posts.RefreshPost(ctx, pid, service.SourceLive)
		if err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		if err := a.Repository.SaveCrawlResultWithSource([]models.Post{*post}, nil, "explicit", "treehole-live"); err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		if err := execution.ItemSucceeded(ctx, key, map[string]any{"pid": pid}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) handleRepairComments(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	payload, err := decodePIDPayload(job.Payload)
	if err != nil {
		return err
	}
	if err := execution.SetTotal(ctx, len(payload.PIDs)); err != nil {
		return err
	}
	completed := completedItemKeys(ctx, execution)
	for _, pid := range payload.PIDs {
		key := "pid:" + strconv.FormatInt(int64(pid), 10)
		if completed[key] {
			continue
		}
		detail, err := a.Posts.Get(ctx, pid, service.CommentQuery{Limit: 100, Source: service.SourceLive})
		if err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		if err := a.Repository.SaveCrawlResult([]models.Post{detail.Post}, detail.Comments); err != nil {
			_ = execution.ItemFailed(ctx, key, err)
			return err
		}
		if err := execution.ItemSucceeded(ctx, key, map[string]any{"pid": pid, "comments": len(detail.Comments)}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) handleRepairMedia(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	candidates, err := a.Media.PendingRepairs(ctx, 10_000)
	if err != nil {
		return err
	}
	if err := execution.SetTotal(ctx, len(candidates)); err != nil {
		return err
	}
	completed := completedItemKeys(ctx, execution)
	for _, candidate := range candidates {
		key := "media:" + strconv.FormatUint(uint64(candidate.ID), 10)
		if completed[key] {
			continue
		}
		file, repairErr := a.Media.Repair(ctx, candidate)
		if repairErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_ = execution.ItemFailed(ctx, key, repairErr)
			continue
		}
		if err := execution.ItemSucceeded(ctx, key, map[string]any{"id": candidate.ID, "size": file.Size}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) handleRebuildSearchIndex(ctx context.Context, execution *jobs.Execution, _ jobs.Job) error {
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	if err := a.Search.RebuildIndex(ctx); err != nil {
		_ = execution.ItemFailed(ctx, "index", err)
		return err
	}
	return execution.ItemSucceeded(ctx, "index", nil)
}

func (a *App) handleRebuildReferences(ctx context.Context, execution *jobs.Execution, _ jobs.Job) error {
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	count, err := a.Repository.RebuildReferences(ctx)
	if err != nil {
		_ = execution.ItemFailed(ctx, "references", err)
		return err
	}
	return execution.ItemSucceeded(ctx, "references", map[string]any{"references": count})
}

func (a *App) handleMonitorLatest(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	var payload monitorJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return err
		}
	}
	if payload.Pages <= 0 || payload.Pages > 50 {
		payload.Pages = 3
	}
	if payload.IntervalSeconds < 15 {
		payload.IntervalSeconds = 60
	}
	if payload.Options.PostLimit <= 0 {
		payload.Options.PostLimit = 200
	}
	if payload.Options.CommentLimit <= 0 {
		payload.Options.CommentLimit = 200
	}
	if err := execution.SetTotal(ctx, payload.Pages); err != nil {
		return err
	}
	cycle := 0
	for {
		cycle++
		for page := 1; page <= payload.Pages; page++ {
			result, err := a.Sync.FetchPage(ctx, page, payload.Options)
			key := "page:" + strconv.Itoa(page)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				_ = execution.ItemFailed(ctx, key, err)
				continue
			}
			checkpoint := map[string]any{"cycle": cycle, "page": page, "posts": result.PostCount, "comments": result.CommentCount}
			if err := execution.ItemSucceeded(ctx, key, checkpoint); err != nil {
				return err
			}
			if err := execution.Checkpoint(ctx, checkpoint); err != nil {
				return err
			}
		}
		timer := time.NewTimer(time.Duration(payload.IntervalSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (a *App) handleRepairThumbnails(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	var payload thumbnailJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}
	if payload.StartID <= 0 || payload.EndID < payload.StartID || payload.EndID-payload.StartID > 100_000 {
		return fmt.Errorf("thumbnail id range is invalid or too large")
	}
	if err := execution.SetTotal(ctx, 1); err != nil {
		return err
	}
	result, err := a.Sync.FetchThumbnails(ctx, payload.StartID, payload.EndID, payload.ConvertWebP)
	if err != nil {
		_ = execution.ItemFailed(ctx, "thumbnails", err)
		return err
	}
	return execution.ItemSucceeded(ctx, "thumbnails", result)
}

func (a *App) handleCleanupStaging(ctx context.Context, execution *jobs.Execution, job jobs.Job) error {
	var payload cleanupJobPayload
	_ = json.Unmarshal(job.Payload, &payload)
	if payload.RetentionDays <= 0 || payload.RetentionDays > 365 {
		payload.RetentionDays = 7
	}
	if err := execution.SetTotal(ctx, 3); err != nil {
		return err
	}
	if err := cleanupExpiredImportStaging(ctx, a.DataDir, a.Jobs, time.Duration(payload.RetentionDays)*24*time.Hour); err != nil {
		_ = execution.ItemFailed(ctx, "staging", err)
		return err
	}
	if err := execution.ItemSucceeded(ctx, "staging", map[string]any{"retention_days": payload.RetentionDays}); err != nil {
		return err
	}
	if err := cleanupExpiredExports(ctx, a.DataDir, 30*24*time.Hour); err != nil {
		_ = execution.ItemFailed(ctx, "exports", err)
		return err
	}
	if err := execution.ItemSucceeded(ctx, "exports", map[string]any{"retention_days": 30}); err != nil {
		return err
	}
	if err := cleanupExpiredRawJSON(ctx, a.DataDir, 30*24*time.Hour); err != nil {
		_ = execution.ItemFailed(ctx, "raw_json", err)
		return err
	}
	return execution.ItemSucceeded(ctx, "raw_json", map[string]any{"retention_days": 30})
}

func decodePIDPayload(raw json.RawMessage) (pidJobPayload, error) {
	var payload pidJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, fmt.Errorf("decode PID payload: %w", err)
	}
	if len(payload.PIDs) == 0 {
		return payload, fmt.Errorf("at least one PID is required")
	}
	for _, pid := range payload.PIDs {
		if pid <= 0 {
			return payload, fmt.Errorf("invalid PID %d", pid)
		}
	}
	return payload, nil
}

func completedItemKeys(ctx context.Context, execution *jobs.Execution) map[string]bool {
	completed := make(map[string]bool)
	items, err := execution.Items(ctx)
	if err != nil {
		return completed
	}
	for _, item := range items {
		if item.Status == jobs.ItemCompleted {
			completed[item.Key] = true
		}
	}
	return completed
}

func flattenPostSummaries(summaries []service.PostSummary) ([]models.Post, []models.Comment) {
	posts := make([]models.Post, len(summaries))
	comments := make([]models.Comment, 0)
	for i, summary := range summaries {
		posts[i] = summary.Post
		comments = append(comments, summary.Comments...)
	}
	return posts, comments
}
