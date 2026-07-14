package server

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func apiDiagnosticBundle(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		schema := 0
		if dependencies.Repository != nil {
			schema, _ = dependencies.Repository.SchemaVersion()
		}
		settings := map[string]any{"schema_version": schema}
		if dependencies.Settings != nil {
			if view, err := dependencies.Settings.Get(c.Request.Context()); err == nil {
				settings["database_type"] = view.DatabaseType
				settings["ai_enabled"] = view.AIEnabled
				settings["ai_live_search"] = view.AILiveSearch
				settings["ai_active_provider"] = view.AIActiveProvider
				settings["ai_provider_count"] = len(view.AIProviders)
			}
		}
		manifest := map[string]any{"generated_at": time.Now().UTC(), "go_version": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "settings": settings}
		jobsView := make([]map[string]any, 0)
		if dependencies.Jobs != nil {
			if values, err := dependencies.Jobs.List(c.Request.Context(), 100); err == nil {
				for _, job := range values {
					hash := sha256.Sum256([]byte(job.ID))
					jobsView = append(jobsView, map[string]any{"id": hex.EncodeToString(hash[:6]), "type": job.Type, "status": job.Status, "completed_items": job.CompletedItems, "failed_items": job.FailedItems, "total_items": job.TotalItems, "attempts": job.Attempts, "error": redactDiagnosticText(job.Error, dependencies.DataDir), "created_at": job.CreatedAt, "updated_at": job.UpdatedAt})
				}
			}
		}
		logs := map[string]any{}
		if dependencies.Logs != nil {
			if values, err := dependencies.Logs.List(c.Request.Context(), "all", "", 1000); err == nil {
				logs["lines"] = values
			}
		}
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", `attachment; filename="pkuholestudio-diagnostics.zip"`)
		writer := zip.NewWriter(c.Writer)
		for name, value := range map[string]any{"manifest.json": manifest, "jobs.json": jobsView, "logs.json": logs} {
			entry, err := writer.Create(name)
			if err != nil {
				_ = writer.Close()
				return
			}
			encoded, _ := json.MarshalIndent(value, "", "  ")
			_, _ = entry.Write(encoded)
		}
		_ = writer.Close()
	}
}

func redactDiagnosticText(value, dataDir string) string {
	if dataDir != "" {
		value = strings.ReplaceAll(value, dataDir, "<data-dir>")
	}
	if len(value) > 1000 {
		value = value[:1000] + "…"
	}
	for _, marker := range []string{"authorization", "api_key", "token", "password", "cookie"} {
		if strings.Contains(strings.ToLower(value), marker) {
			return fmt.Sprintf("<redacted: contains %s>", marker)
		}
	}
	return value
}
