package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
