package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
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

type fakeExportStore struct {
	*fakeArchiveStore
	records []ExportRecord
}

func (s *fakeExportStore) ArchiveExportSnapshot(context.Context, []int32) ([]ExportRecord, error) {
	return s.records, nil
}
