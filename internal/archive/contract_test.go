package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

type contractFixture struct {
	Manifest map[string]any `json:"manifest"`
	Data     map[string]any `json:"data"`
}

func TestVendoredArchiveContractFixtures(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate contract testdata")
	}
	fixtureRoot := filepath.Join(filepath.Dir(sourceFile), "testdata", "contract", "v2")
	for _, expectation := range []string{"valid", "invalid"} {
		entries, err := os.ReadDir(filepath.Join(fixtureRoot, expectation))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			t.Run(expectation+"/"+entry.Name(), func(t *testing.T) {
				encoded, err := os.ReadFile(filepath.Join(fixtureRoot, expectation, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				var fixture contractFixture
				if err := json.Unmarshal(encoded, &fixture); err != nil {
					t.Fatal(err)
				}
				content := makeV2ZIP(t, fixture.Manifest, fixture.Data)
				report, parseErr := Parse(context.Background(), bytes.NewReader(content), int64(len(content)))
				accepted := parseErr == nil && report.Status != StatusFailed
				if expectation == "valid" && !accepted {
					t.Fatalf("valid fixture rejected: report=%+v error=%v", report, parseErr)
				}
				if expectation == "invalid" && accepted {
					t.Fatalf("invalid fixture accepted: report=%+v", report)
				}
			})
		}
	}
}

func TestVendoredProducerArchiveGoldens(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate producer archive goldens")
	}
	archiveRoot := filepath.Join(filepath.Dir(sourceFile), "testdata", "contract", "v2", "archives", "valid")
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		t.Fatal(err)
	}
	accepted := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".zip" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(archiveRoot, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		report, err := Parse(context.Background(), bytes.NewReader(content), int64(len(content)))
		if err != nil || report.Format != FormatV2 || report.Counts.ValidItems != 1 {
			t.Fatalf("%s: report=%+v error=%v", entry.Name(), report, err)
		}
		accepted++
	}
	if accepted != 2 {
		t.Fatalf("accepted %d producer archives", accepted)
	}
}

func TestArchiveContractRejectsUnknownRequiredExtension(t *testing.T) {
	manifest := validManifest(1, 0)
	manifest["specVersion"] = ArchiveSpecVersion
	manifest["producer"] = map[string]any{"name": "future producer"}
	manifest["extensions"] = map[string]any{"example.future.feature": map[string]any{"version": 1}}
	manifest["requiredExtensions"] = []string{"example.future.feature"}
	data := map[string]any{"items": []any{map[string]any{
		"pid": "123456", "source": "followed", "fetchStatus": "ok",
		"hole": map[string]any{"pid": 123456}, "comments": []any{},
	}}}
	content := makeV2ZIP(t, manifest, data)
	if _, err := Parse(context.Background(), bytes.NewReader(content), int64(len(content))); err == nil {
		t.Fatal("archive requiring an unknown extension was accepted")
	}
}

func TestArchiveContractCapabilitiesAreNativeAndVersioned(t *testing.T) {
	capabilities := ContractCapabilities()
	if capabilities.WriteSpecVersion != ArchiveSpecVersion || capabilities.WriteZIPMethod != "store" || len(capabilities.ReadZIPMethods) != 2 || capabilities.Extensions[ArchiveExtensionMedia] != 1 || capabilities.MaxArchiveBytes != MaxArchiveBytes {
		t.Fatalf("capabilities = %+v", capabilities)
	}
}

func TestArchiveContractReadsLegacyStudioSourceKeys(t *testing.T) {
	manifest := validManifest(1, 0)
	manifest["specVersion"] = ArchiveSpecVersion
	manifest["producer"] = map[string]any{"name": "PkuHoleStudio", "version": "legacy-extension-writer"}
	manifest["extensions"] = map[string]any{ArchiveExtensionStudioSources: map[string]any{"version": 1}}
	data := map[string]any{"items": []any{map[string]any{
		"pid": "123456", "source": "followed", "fetchStatus": "ok",
		"hole": map[string]any{"pid": 123456}, "comments": []any{},
		"studioSources": []any{map[string]any{"source": "referenced", "source_ref": "legacy-run", "context_only": true}},
	}}}
	content := makeV2ZIP(t, manifest, data)
	report, err := Parse(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil || len(report.records) != 1 || len(report.records[0].StudioSources) != 1 {
		t.Fatalf("Parse() = %+v, %v", report, err)
	}
	source := report.records[0].StudioSources[0]
	if source.Source != "referenced" || source.RunID != "legacy-run" || !source.ContextOnly {
		t.Fatalf("legacy Studio source = %+v", source)
	}
}

func TestArchiveContractBoundsStudioSourceProvenance(t *testing.T) {
	manifest := validManifest(1, 0)
	manifest["specVersion"] = ArchiveSpecVersion
	manifest["extensions"] = map[string]any{ArchiveExtensionStudioSources: map[string]any{"version": 1}}
	sources := make([]any, 17)
	for index := range sources {
		sources[index] = map[string]any{"source": "explicit", "sourceRef": "source-" + strconv.Itoa(index)}
	}
	data := map[string]any{"items": []any{map[string]any{
		"pid": "123456", "source": "explicit", "fetchStatus": "ok",
		"hole": map[string]any{"pid": 123456}, "comments": []any{}, "studioSources": sources,
	}}}
	content := makeV2ZIP(t, manifest, data)
	report, err := Parse(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil || len(report.records) != 1 || len(report.records[0].StudioSources) != 16 {
		t.Fatalf("Parse() = %+v, %v", report, err)
	}
	foundWarning := false
	for _, issue := range report.Issues {
		foundWarning = foundWarning || issue.Code == "studio_sources_truncated"
	}
	if !foundWarning {
		t.Fatalf("issues = %+v", report.Issues)
	}
}
