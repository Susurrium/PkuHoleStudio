package crawler

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestSaveRawResponsesToManagedFileAndDrainAfterCommit(t *testing.T) {
	rawSaveMu.Lock()
	rawResponsesMu.Lock()
	rawResponses = []map[string]interface{}{{"page": float64(1)}, {"page": float64(2)}}
	rawResponsesMu.Unlock()
	rawSaveMu.Unlock()
	t.Cleanup(func() {
		rawResponsesMu.Lock()
		rawResponses = nil
		rawResponsesMu.Unlock()
	})
	path := filepath.Join(t.TempDir(), "raw", "responses.json")
	count, size, err := SaveRawResponsesTo(path)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || size <= 0 || RawResponses() != 0 {
		t.Fatalf("save result count=%d size=%d remaining=%d", count, size, RawResponses())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"page": 1`)) || !bytes.Contains(data, []byte(`"page": 2`)) {
		t.Fatalf("saved raw JSON = %s", data)
	}
}

func TestSaveRawResponsesToRejectsEmptyCache(t *testing.T) {
	rawResponsesMu.Lock()
	rawResponses = nil
	rawResponsesMu.Unlock()
	path := filepath.Join(t.TempDir(), "responses.json")
	_, _, err := SaveRawResponsesTo(path)
	if !errors.Is(err, ErrNoRawResponses) {
		t.Fatalf("SaveRawResponsesTo() error = %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("empty cache created a file: %v", statErr)
	}
}

func TestGetJSONStringFromInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, ""},
		{"empty array", []interface{}{}, ""},
		{"empty string", "", `""`},
		{"simple string", "hello", `"hello"`},
		{"number", 42, `42`},
		{"map", map[string]interface{}{"key": "val"}, `{"key":"val"}`},
		{"non-empty array", []interface{}{1, 2}, `[1,2]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getJSONStringFromInterface(tt.input)
			if result != tt.expected {
				t.Errorf("getJSONStringFromInterface(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetStringField(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"empty": "",
	}

	if getStringField(data, "name") != "test" {
		t.Errorf("getStringField(name) != test")
	}
	if getStringField(data, "empty") != "" {
		t.Errorf("getStringField(empty) != empty string")
	}
	if getStringField(data, "missing") != "" {
		t.Errorf("getStringField(missing) != empty string")
	}
}

func TestGetStringFieldNonString(t *testing.T) {
	data := map[string]interface{}{
		"num": 42,
	}
	if getStringField(data, "num") != "" {
		t.Errorf("getStringField(num) should return empty for non-string value")
	}
}

func TestGetIntField(t *testing.T) {
	data := map[string]interface{}{
		"float": float64(42.0),
		"int":   int(100),
	}

	if getIntField(data, "float") != 42 {
		t.Errorf("getIntField(float) = %d, want 42", getIntField(data, "float"))
	}
	if getIntField(data, "int") != 100 {
		t.Errorf("getIntField(int) = %d, want 100", getIntField(data, "int"))
	}
	if getIntField(data, "missing") != 0 {
		t.Errorf("getIntField(missing) = %d, want 0", getIntField(data, "missing"))
	}
}

func TestGetIntFieldNonNumeric(t *testing.T) {
	data := map[string]interface{}{
		"str": "hello",
	}
	if getIntField(data, "str") != 0 {
		t.Errorf("getIntField(str) should return 0 for non-numeric value")
	}
}

func TestSaveMediaStoresPNGWithOriginalExtension(t *testing.T) {
	pngData := mustEncodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData)
	}))
	defer srv.Close()

	c := mustNewTestClient(t)
	outputDir := t.TempDir()

	downloaded, err := saveMedia(c, srv.URL, 30001, outputDir, false)
	if err != nil || !downloaded {
		t.Fatalf("saveMedia() downloaded=%v error=%v, want downloaded", downloaded, err)
	}

	filename := filepath.Join(outputDir, "30001.png")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !bytes.Equal(data, pngData) {
		t.Fatal("saved file content mismatch")
	}
}

func TestDownloadMediaByIDRangeClassifiesDownloadedExistingAndFailed(t *testing.T) {
	pngData := mustEncodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("id") {
		case "30001":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngData)
		default:
			http.Error(w, "unavailable", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := mustNewTestClient(t)
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "30000.png"), pngData, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := downloadMediaByIDRange(c, 30000, 30002, srv.URL+"?id=%d", outputDir, false)
	if err == nil {
		t.Fatal("downloadMediaByIDRange() error=nil, want failure summary")
	}
	if result.Downloaded != 1 || result.Existing != 1 || result.Failed != 1 {
		t.Fatalf("result=%+v, want downloaded=1 existing=1 failed=1", result)
	}
	for _, id := range []string{"30000.png", "30001.png"} {
		if _, err := os.Stat(filepath.Join(outputDir, id)); err != nil {
			t.Fatalf("expected file %s to exist: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "30002.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed media should not be written: %v", err)
	}
}

func TestFetchImagesFromDBWritesOnlyToSelectedDataDirectory(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	profileDir := filepath.Join(t.TempDir(), "custom profile")
	database, err := db.NewDatabase(&config.Config{Database: config.DatabaseConfig{
		Type:   "sqlite3",
		DBFile: filepath.Join(t.TempDir(), "crawler.db"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.SaveCrawlResult([]models.Post{{Pid: 30001, Type: "image", MediaIds: "30001"}}, nil); err != nil {
		t.Fatal(err)
	}

	pngData := mustEncodePNG(t)
	c := mustNewTestClient(t)
	c.GetHttpClient().Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(pngData)),
			Request:    req,
		}, nil
	})

	result, err := FetchImagesFromDB(c, database, filepath.Join(profileDir, "images"), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Downloaded != 1 || result.Existing != 0 || result.Failed != 0 {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(filepath.Join(profileDir, "images", "30001.png")); err != nil {
		t.Fatalf("selected data directory does not contain image: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "data", "images", "30001.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy cwd data directory should remain untouched: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mustEncodePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 1, color.NRGBA{G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func mustNewTestClient(t *testing.T) *client.Client {
	t.Helper()
	c, err := client.NewClient("test-device")
	if err != nil {
		t.Fatalf("client.NewClient: %v", err)
	}
	return c
}
