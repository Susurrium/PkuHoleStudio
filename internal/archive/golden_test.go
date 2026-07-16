package archive

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

// TestUpdateArchiveContractGolden uses Studio's production archive writer.
// It is skipped during normal tests and only writes to an explicitly supplied
// path, so updating shared fixtures remains deliberate.
func TestUpdateArchiveContractGolden(t *testing.T) {
	output := os.Getenv("PKUHOLE_ARCHIVE_FIXTURE_OUTPUT")
	if output == "" {
		t.Skip("set PKUHOLE_ARCHIVE_FIXTURE_OUTPUT to update the Studio golden archive")
	}
	dataDir := t.TempDir()
	mediaPath := filepath.Join(dataDir, "images", "123456.png")
	if err := os.MkdirAll(filepath.Dir(mediaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	media := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	if err := os.WriteFile(mediaPath, media, 0o644); err != nil {
		t.Fatal(err)
	}
	records := []ExportRecord{{
		Post:     models.Post{Pid: 123456, Text: "Studio contract fixture", Timestamp: 1784131200, Type: "image"},
		Comments: []models.Comment{{Cid: 1001, Pid: 123456, Text: "portable comment", Timestamp: 1784131260}},
		Sources:  []models.PostSource{{PID: 123456, Source: "followed", SourceRef: "studio-golden-source"}},
		Media:    []models.Media{{OwnerType: "post", OwnerID: 123456, Variant: "original", Path: "images/123456.png", Status: "available"}},
		Studio:   StudioMetadata{Tags: []PortableLocalTag{{Name: "contract", Color: "#2563eb"}}, Note: "portable note"},
	}}
	report := ExportReport{Format: ExportFormatTreeholeV2, Posts: 1, Comments: 1, Media: 1, RunID: "studio-golden-v2"}
	var archive bytes.Buffer
	if err := writeTreeholeV2(context.Background(), &archive, records, report, true, dataDir, time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote Studio Archive Contract fixture to %s", output)
}
