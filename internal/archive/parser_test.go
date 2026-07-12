package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseV2ValidAndContextRecords(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{
			"pid": "123456", "source": "followed", "fetchStatus": "ok",
			"hole": map[string]any{"pid": 123456, "text": "see #234567"},
			"comments": []any{map[string]any{
				"cid": 1001, "pid": 123456, "text": "reply #345678", "name": "Alice",
				"quote": []any{map[string]any{"cid": 999}},
			}},
		},
		map[string]any{
			"pid": "234567", "source": "referenced", "fetchStatus": "ok",
			"hole":     map[string]any{"pid": 234567, "text": "context"},
			"comments": []any{map[string]any{"cid": 2001, "pid": 234567, "text": "context comment"}},
		},
	}}
	archiveBytes := makeV2ZIP(t, validManifest(2, 2), data)

	report, err := Parse(context.Background(), bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if report.Format != FormatV2 || report.Status != StatusCompleted {
		t.Fatalf("report = %+v", report)
	}
	if report.Counts.ValidItems != 2 || report.Counts.ContextOnly != 1 || report.Counts.Comments != 2 {
		t.Fatalf("counts = %+v", report.Counts)
	}
	if len(report.records) != 2 || !report.records[1].ContextOnly {
		t.Fatalf("records = %+v", report.records)
	}
	if got := report.records[0].Comments[0].NameTag; got != "Alice" {
		t.Fatalf("name tag = %q", got)
	}
	kinds := map[string]int{}
	for _, reference := range report.records[0].References {
		kinds[reference.Kind]++
	}
	if kinds["mentions"] != 2 || kinds["quotes"] != 1 {
		t.Fatalf("reference kinds = %#v", kinds)
	}
}

func TestParseV2AcceptsToolkitImageSizeArray(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{
			"pid": "123456", "source": "followed", "fetchStatus": "ok",
			"hole": map[string]any{
				"pid": 123456, "text": "toolkit export",
				"image_size": []any{[]any{1280, 720}, []any{640, 480}},
			},
			"comments": []any{map[string]any{"cid": 1001, "pid": 123456, "text": "comment"}},
		},
	}}
	archiveBytes := makeV2ZIP(t, validManifest(1, 1), data)

	report, err := Parse(context.Background(), bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if report.Status != StatusCompleted || report.Counts.ValidItems != 1 || report.Counts.Comments != 1 {
		t.Fatalf("report = %+v", report)
	}
	if got, want := report.records[0].Post.ImageSize, `[[1280,720],[640,480]]`; got != want {
		t.Fatalf("image_size = %q, want %q", got, want)
	}
}

func TestParseV2PartialRecordsAndManifestMismatch(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{
			"pid": "123456", "source": "explicit", "fetchStatus": "partial",
			"hole": map[string]any{"pid": 123456, "text": "valid"},
			"comments": []any{
				map[string]any{"cid": 1, "pid": 999999, "text": "wrong pid"},
				map[string]any{"pid": 123456, "text": "missing cid"},
			},
		},
		map[string]any{"pid": "bad", "source": "followed", "fetchStatus": "ok", "hole": map[string]any{}, "comments": []any{}},
	}}
	manifest := validManifest(9, 8)
	manifest["complete"] = false
	archiveBytes := makeV2ZIP(t, manifest, data)

	report, err := Parse(context.Background(), bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if report.Status != StatusPartial || report.Counts.ValidItems != 1 || report.Counts.SkippedItems != 1 || report.Counts.SkippedComments != 2 {
		t.Fatalf("report = %+v", report)
	}
	for _, code := range []string{"partial_item", "invalid_pid", "comment_pid_mismatch", "invalid_comment", "hole_count_mismatch", "comment_count_mismatch", "incomplete_export"} {
		if !hasIssue(report.Issues, code) {
			t.Errorf("missing issue %q in %+v", code, report.Issues)
		}
	}
}

func TestParseV2AcceptsProtocolMinimalCounts(t *testing.T) {
	manifest := validManifest(0, 0)
	manifest["counts"] = map[string]any{}
	data := map[string]any{"items": []any{}}
	archiveBytes := makeV2ZIP(t, manifest, data)
	report, err := Parse(context.Background(), bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil || report.Status != StatusFailed || report.Counts.ValidItems != 0 {
		t.Fatalf("Parse() = %+v, %v", report, err)
	}
}

func TestParseLegacyFlattensCommentsByHoleIndex(t *testing.T) {
	legacy := []byte(`{"holes":[{"pid":123456,"text":"one"},{"pid":234567,"text":"two"}],"comments":[[[{"cid":1,"text":"a"}],[{"cid":2,"text":"b"}]],[]]}`)
	report, err := Parse(context.Background(), bytes.NewReader(legacy), int64(len(legacy)))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if report.Format != FormatLegacyV1 || report.Counts.ValidItems != 2 || report.Counts.Comments != 2 {
		t.Fatalf("report = %+v", report)
	}
	if got := report.records[0].Comments; len(got) != 2 || got[0].Pid != 123456 || got[1].Pid != 123456 {
		t.Fatalf("comments = %+v", got)
	}
}

func TestParseRejectsUnsafeDuplicateMissingAndCorruptZIP(t *testing.T) {
	manifestJSON, _ := json.Marshal(validManifest(0, 0))
	dataJSON := []byte(`{"items":[]}`)
	tests := []struct {
		name    string
		entries []zipEntry
		want    string
	}{
		{"unsafe", []zipEntry{{"../manifest.json", manifestJSON}, {"data.json", dataJSON}}, "unsafe ZIP entry"},
		{"duplicate", []zipEntry{{"manifest.json", manifestJSON}, {"manifest.json", manifestJSON}, {"data.json", dataJSON}}, "duplicate ZIP entry"},
		{"missing", []zipEntry{{"manifest.json", manifestJSON}}, "must contain"},
		{"unexpected", []zipEntry{{"manifest.json", manifestJSON}, {"data.json", dataJSON}, {"secret.txt", nil}}, "unexpected ZIP entry"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := makeZIP(t, test.entries)
			_, err := Parse(context.Background(), bytes.NewReader(content), int64(len(content)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}

	corrupt := makeZIPStored(t, []zipEntry{{"manifest.json", manifestJSON}, {"data.json", dataJSON}})
	index := bytes.Index(corrupt, dataJSON)
	if index < 0 {
		t.Fatal("stored data entry not found")
	}
	corrupt[index+2] ^= 0xff
	if _, err := Parse(context.Background(), bytes.NewReader(corrupt), int64(len(corrupt))); err == nil {
		t.Fatal("Parse() accepted a CRC-corrupt ZIP")
	}
}

type zipEntry struct {
	name string
	data []byte
}

func validManifest(holes, comments int) map[string]any {
	return map[string]any{
		"schemaVersion": 2,
		"toolVersion":   "test",
		"runId":         "run-test",
		"exportedAt":    "2026-01-02T03:04:05Z",
		"scope":         map[string]any{"type": "test"},
		"complete":      true,
		"counts": map[string]any{
			"expectedHoles": holes, "exportedHoles": holes, "comments": comments, "failed": 0,
		},
		"errors": []any{},
	}
}

func makeV2ZIP(t *testing.T, manifest, data any) []byte {
	t.Helper()
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return makeZIP(t, []zipEntry{{"manifest.json", manifestJSON}, {"data.json", dataJSON}, {"readable.txt", []byte("readable")}})
}

func makeZIP(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	return makeZIPWithMethod(t, entries, zip.Deflate)
}

func makeZIPStored(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	return makeZIPWithMethod(t, entries, zip.Store)
}

func makeZIPWithMethod(t *testing.T, entries []zipEntry, method uint16) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: method}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func hasIssue(issues []Issue, code string) bool {
	for _, current := range issues {
		if current.Code == code {
			return true
		}
	}
	return false
}
