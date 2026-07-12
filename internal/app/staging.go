package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
)

func cleanupExpiredImportStaging(ctx context.Context, dataDir string, manager *jobs.Manager, maxAge time.Duration) error {
	stagingDir, err := filepath.Abs(filepath.Join(dataDir, "imports", "staging"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return err
	}
	protected := make(map[string]bool)
	if manager != nil {
		rows, listErr := manager.List(ctx, 10_000)
		if listErr != nil {
			return listErr
		}
		for _, row := range rows {
			if row.Type != jobs.TypeImportArchive || row.Status.Terminal() {
				continue
			}
			var payload importArchivePayload
			if json.Unmarshal(row.Payload, &payload) == nil {
				if absolute, pathErr := filepath.Abs(payload.Path); pathErr == nil {
					protected[strings.ToLower(absolute)] = true
				}
			}
		}
	}
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(stagingDir, entry.Name())
		if protected[strings.ToLower(path)] {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			if errors.Is(infoErr, os.ErrNotExist) {
				continue
			}
			return infoErr
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func cleanupExpiredExports(ctx context.Context, dataDir string, maxAge time.Duration) error {
	directory := filepath.Join(dataDir, "exports")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".zip") && !strings.HasSuffix(entry.Name(), ".tmp")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}
