package server

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
)

type bridgeArchiveStub struct{}

func (bridgeArchiveStub) Preflight(context.Context, io.ReaderAt, int64) (archive.PreflightReport, error) {
	return archive.PreflightReport{Counts: archive.Counts{ValidItems: 1}}, nil
}

func TestBridgeUploadUsesImportJobStagingDirectory(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewBridgeManager(dataDir, bridgeArchiveStub{}, nil)
	pairing, err := manager.Create("8080:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Upload(context.Background(), pairing.Token, "archive.treehole.zip", strings.NewReader("archive")); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	stagedPath := manager.pairings[pairing.Token].path
	manager.mu.Unlock()
	wantDir, _ := filepath.Abs(filepath.Join(dataDir, "imports", "staging"))
	gotDir, _ := filepath.Abs(filepath.Dir(stagedPath))
	if gotDir != wantDir {
		t.Fatalf("staged directory = %q, want %q", gotDir, wantDir)
	}
	manager.Cancel(pairing.Token)
}
