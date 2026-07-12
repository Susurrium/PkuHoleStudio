package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestExporterTreeholeV2RoundTripsThroughParser(t *testing.T) {
	quoteID := int32(1000)
	store := &fakeExportStore{fakeArchiveStore: &fakeArchiveStore{}, records: []ExportRecord{{
		Post:     models.Post{Pid: 123456, Text: "main #234567", ImageSize: `[[1280,720]]`},
		Comments: []models.Comment{{Cid: 1001, Pid: 123456, NameTag: "Alice", Text: "reply", QuoteID: &quoteID}},
		Sources:  []models.PostSource{{PID: 123456, Source: "followed"}},
	}}}
	var output bytes.Buffer
	report, err := NewImporter(store).Export(context.Background(), &output, ExportRequest{Format: ExportFormatTreeholeV2, IncludeComments: true})
	if err != nil || report.Posts != 1 || report.Comments != 1 {
		t.Fatalf("Export() = %+v, %v", report, err)
	}
	parsed, err := Parse(context.Background(), bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil || parsed.Status != StatusCompleted || parsed.Counts.ValidItems != 1 || parsed.Counts.Comments != 1 {
		t.Fatalf("Parse(export) = %+v, %v", parsed, err)
	}
	if got := parsed.records[0].Source; got != "followed" {
		t.Fatalf("source = %q", got)
	}
	if got := parsed.records[0].Comments[0].QuoteID; got == nil || *got != quoteID {
		t.Fatalf("quote id = %v", got)
	}
}

func TestExporterTreeholeV2CarriesMediaAndImporterPersistsIt(t *testing.T) {
	dataDir := t.TempDir()
	content := []byte("\xff\xd8\xff\xe0test image payload")
	mediaPath := filepath.Join(dataDir, "images", "123456.jpg")
	if err := os.MkdirAll(filepath.Dir(mediaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mediaPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	store := &fakeExportStore{fakeArchiveStore: &fakeArchiveStore{}, records: []ExportRecord{{
		Post:  models.Post{Pid: 123456, Text: "image post", Type: "image"},
		Media: []models.Media{{OwnerType: "post", OwnerID: 123456, Variant: "original", Path: "images/123456.jpg", Status: "available"}},
	}}}
	var output bytes.Buffer
	report, err := NewImporterWithDataDir(store, dataDir).Export(context.Background(), &output, ExportRequest{Format: ExportFormatTreeholeV2})
	if err != nil || report.Media != 1 || report.MissingMedia != 0 {
		t.Fatalf("Export() = %+v, %v", report, err)
	}
	parsed, err := Parse(context.Background(), bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil || parsed.Counts.Media != 1 || parsed.Counts.AvailableMedia != 1 || len(parsed.media) != 1 {
		t.Fatalf("Parse() = %+v, %v", parsed, err)
	}
	destination := t.TempDir()
	importStore := &fakeArchiveStore{}
	importReport, err := NewImporterWithDataDir(importStore, destination).Import(context.Background(), bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil || importReport.Counts.AvailableMedia != 1 || len(importStore.media) != 1 {
		t.Fatalf("Import() = %+v, %v, media=%+v", importReport, err, importStore.media)
	}
	stored := filepath.Join(destination, filepath.FromSlash(importStore.media[0].Path))
	if got, err := os.ReadFile(stored); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("stored media = %x, %v", got, err)
	}
}

func TestExporterWritesPerPostMarkdownBundle(t *testing.T) {
	store := &fakeExportStore{fakeArchiveStore: &fakeArchiveStore{}, records: []ExportRecord{{
		Post:     models.Post{Pid: 123456, Text: "main text"},
		Comments: []models.Comment{{Cid: 1001, Pid: 123456, NameTag: "Alice", Text: "reply text"}},
	}, {
		Post:     models.Post{Pid: 234567, Text: "second post"},
		Comments: []models.Comment{{Cid: 2001, Pid: 234567, NameTag: "Bob", Text: "second reply"}},
	}}}
	var output bytes.Buffer
	_, err := NewImporter(store).Export(context.Background(), &output, ExportRequest{Format: ExportFormatMarkdown, IncludeComments: true})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{}
	for _, file := range reader.File {
		stream, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		entries[file.Name] = string(content)
	}
	if !strings.Contains(entries["index.md"], "[#123456](posts/123456.md)") || !strings.Contains(entries["index.md"], "[#234567](posts/234567.md)") || !strings.Contains(entries["posts/123456.md"], "reply text") || !strings.Contains(entries["posts/234567.md"], "second reply") {
		t.Fatalf("markdown entries = %#v", entries)
	}
}

func TestExporterMarkdownBundleIncludesAvailableMedia(t *testing.T) {
	dataDir := t.TempDir()
	content := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	path := filepath.Join(dataDir, "images", "123456.png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	store := &fakeExportStore{fakeArchiveStore: &fakeArchiveStore{}, records: []ExportRecord{{
		Post:  models.Post{Pid: 123456, Text: "image post"},
		Media: []models.Media{{OwnerType: "post", OwnerID: 123456, Variant: "original", Path: "images/123456.png", Status: "available"}},
	}}}
	var output bytes.Buffer
	if _, err := NewImporterWithDataDir(store, dataDir).Export(context.Background(), &output, ExportRequest{Format: ExportFormatMarkdown}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string][]byte{}
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name], err = io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	markdown := string(entries["posts/123456.md"])
	if !strings.Contains(markdown, "![图片](../media/") {
		t.Fatalf("markdown = %q", markdown)
	}
	found := false
	for name, data := range entries {
		if strings.HasPrefix(name, "media/") && bytes.Equal(data, content) {
			found = true
		}
	}
	if !found {
		t.Fatalf("media entries = %#v", entries)
	}
}

type fakeExportStore struct {
	*fakeArchiveStore
	records []ExportRecord
}

func (s *fakeExportStore) ArchiveExportSnapshot(context.Context, []int32) ([]ExportRecord, error) {
	return s.records, nil
}
