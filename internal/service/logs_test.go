package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogServiceFiltersRedactsAndClears(t *testing.T) {
	root := t.TempDir()
	content := "ordinary\nAuthorization: Bearer secret\nmatch line\n"
	if err := os.WriteFile(filepath.Join(root, "crawler.log"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewLogService(root)
	items, err := service.List(t.Context(), "crawler", "", 10)
	if err != nil || len(items) != 3 || strings.Contains(items[1].Line, "secret") {
		t.Fatalf("List() = %+v, %v", items, err)
	}
	if err := service.Clear(t.Context(), "crawler"); err != nil {
		t.Fatal(err)
	}
	items, err = service.List(t.Context(), "crawler", "", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("after clear = %+v, %v", items, err)
	}
}

func TestLogServiceStreamsNewLinesAndSurvivesTruncation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "crawler.log")
	if err := os.WriteFile(path, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := NewLogService(root).Stream(ctx, "crawler", "")
	if err != nil {
		t.Fatal(err)
	}
	appendLine := func(value string) {
		file, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if openErr != nil {
			t.Fatal(openErr)
		}
		_, _ = file.WriteString(value + "\n")
		_ = file.Close()
	}
	appendLine("token=secret")
	select {
	case event := <-events:
		if strings.Contains(event.Line, "secret") {
			t.Fatalf("stream leaked secret: %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for streamed log")
	}
	if err := os.WriteFile(path, []byte("after truncate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.Line != "after truncate" {
			t.Fatalf("truncated stream = %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting after truncation")
	}
}
