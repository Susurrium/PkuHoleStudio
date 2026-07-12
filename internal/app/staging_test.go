package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupExpiredImportStagingRemovesOnlyExpiredFiles(t *testing.T) {
	dataDir := t.TempDir()
	staging := filepath.Join(dataDir, "imports", "staging")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(staging, "old.treehole.zip")
	newPath := filepath.Join(staging, "new.treehole.zip")
	for _, path := range []string{oldPath, newPath} {
		if err := os.WriteFile(path, []byte("archive"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}
	if err := cleanupExpiredImportStaging(context.Background(), dataDir, nil, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired file still exists: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("fresh file was removed: %v", err)
	}
}
