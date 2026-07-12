package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

const MaxItems = 20_000

var (
	pidPattern        = regexp.MustCompile(`^\d{5,7}$`)
	hashReference     = regexp.MustCompile(`#(\d{5,7})`)
	leadingReference  = regexp.MustCompile(`^\s*(\d{5,7})(?:\s+|$)`)
	contextReference  = regexp.MustCompile(`(?i)(?:见|参考|来自|查看|看到|提到|推荐)\s*#?(\d{5,7})|(?:#?)(\d{5,7})\s*(?:的洞|的\s*dz|这个洞|号洞)`)
	allowedZIPEntries = map[string]bool{"manifest.json": true, "data.json": true, "readable.txt": true, "media/index.json": true}
)

type rawItem struct {
	PID         string            `json:"pid"`
	Source      string            `json:"source"`
	Hole        json.RawMessage   `json:"hole"`
	Comments    []json.RawMessage `json:"comments"`
	FetchStatus string            `json:"fetchStatus"`
	shapeError  string
}

type v2Data struct {
	Items []rawItem `json:"items"`
}

func Parse(ctx context.Context, reader io.ReaderAt, size int64) (PreflightReport, error) {
	if reader == nil {
		return PreflightReport{}, errors.New("archive reader is required")
	}
	if size <= 0 {
		return PreflightReport{}, errors.New("archive is empty")
	}
	if size > MaxArchiveBytes {
		return PreflightReport{}, fmt.Errorf("archive exceeds %d bytes", MaxArchiveBytes)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return PreflightReport{}, err
	}
	hash, err := archiveHash(ctx, reader, size)
	if err != nil {
		return PreflightReport{}, err
	}
	header := make([]byte, 4)
	if _, err := reader.ReadAt(header, 0); err != nil && !errors.Is(err, io.EOF) {
		return PreflightReport{}, err
	}
	if bytes.HasPrefix(header, []byte{'P', 'K'}) {
		return parseV2(ctx, reader, size, hash)
	}
	data, err := readSection(ctx, reader, size, MaxArchiveBytes)
	if err != nil {
		return PreflightReport{}, err
	}
	return parseLegacy(ctx, data, hash)
}

func parseV2(ctx context.Context, reader io.ReaderAt, size int64, hash string) (PreflightReport, error) {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return PreflightReport{}, fmt.Errorf("open archive ZIP: %w", err)
	}
	entries := make(map[string][]byte)
	seen := make(map[string]bool)
	var uncompressed uint64
	for _, file := range zipReader.File {
		if err := ctx.Err(); err != nil {
			return PreflightReport{}, err
		}
		name := strings.ReplaceAll(file.Name, `\`, `/`)
		if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
			return PreflightReport{}, fmt.Errorf("unsafe ZIP entry %q", file.Name)
		}
		if seen[name] {
			return PreflightReport{}, fmt.Errorf("duplicate ZIP entry %q", name)
		}
		seen[name] = true
		if !allowedZIPEntries[name] && !strings.HasPrefix(name, "media/") {
			return PreflightReport{}, fmt.Errorf("unexpected ZIP entry %q", name)
		}
		if strings.HasPrefix(name, "media/") && name != "media/index.json" && file.FileInfo().IsDir() {
			continue
		}
		uncompressed += file.UncompressedSize64
		if uncompressed > uint64(MaxUncompressedBytes) {
			return PreflightReport{}, fmt.Errorf("archive expands beyond %d bytes", MaxUncompressedBytes)
		}
		stream, err := file.Open()
		if err != nil {
			return PreflightReport{}, fmt.Errorf("open ZIP entry %s: %w", name, err)
		}
		remaining := MaxUncompressedBytes - int64(uncompressed-file.UncompressedSize64)
		content, readErr := io.ReadAll(io.LimitReader(stream, remaining+1))
		closeErr := stream.Close()
		if readErr != nil {
			return PreflightReport{}, fmt.Errorf("read ZIP entry %s: %w", name, readErr)
		}
		if closeErr != nil {
			return PreflightReport{}, fmt.Errorf("close ZIP entry %s: %w", name, closeErr)
		}
		if int64(len(content)) > remaining {
			return PreflightReport{}, fmt.Errorf("archive expands beyond %d bytes", MaxUncompressedBytes)
		}
		entries[name] = content
	}
	manifestBytes, hasManifest := entries["manifest.json"]
	dataBytes, hasData := entries["data.json"]
	if !hasManifest || !hasData {
		return PreflightReport{}, errors.New("archive ZIP must contain manifest.json and data.json")
	}

	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return PreflightReport{}, err
	}
	items, err := decodeV2Data(dataBytes)
	if err != nil {
		return PreflightReport{}, err
	}
	report := validateItems(ctx, FormatV2, hash, manifest.RunID, items)
	report.Manifest = &manifest
	if indexBytes, ok := entries["media/index.json"]; ok {
		media, mediaIssues := decodeArchiveMedia(indexBytes, entries, report.records)
		report.media = mergeMediaRecords(report.media, media)
		report.Issues = append(report.Issues, mediaIssues...)
		referencedPaths := make(map[string]bool, len(media))
		for _, item := range media {
			referencedPaths[item.Path] = item.Path != ""
		}
		for name := range entries {
			if strings.HasPrefix(name, "media/") && name != "media/index.json" && !referencedPaths[name] {
				report.Issues = append(report.Issues, issue(SeverityError, "unreferenced_media_file", "media file is not declared by media/index.json", name))
			}
		}
	} else {
		for name := range entries {
			if strings.HasPrefix(name, "media/") && name != "media/index.json" {
				report.Issues = append(report.Issues, issue(SeverityError, "missing_media_index", "archive contains media files but no media/index.json", name))
			}
		}
	}
	recountMedia(&report)
	if !manifest.Complete {
		report.Issues = append(report.Issues, issue(SeverityWarning, "incomplete_export", "manifest marks this export incomplete", "manifest.complete"))
	}
	if len(manifest.Errors) > 0 {
		report.Issues = append(report.Issues, issue(SeverityWarning, "manifest_errors", "manifest contains export errors", "manifest.errors"))
	}
	if manifest.Counts.ExportedHoles != nil && *manifest.Counts.ExportedHoles != len(items) {
		report.Issues = append(report.Issues, issue(SeverityWarning, "hole_count_mismatch", fmt.Sprintf("manifest exportedHoles=%d, data items=%d", *manifest.Counts.ExportedHoles, len(items)), "manifest.counts.exportedHoles"))
	}
	commentCount := 0
	for _, item := range items {
		commentCount += len(item.Comments)
	}
	report.Counts.ArchivedComments = commentCount
	if manifest.Counts.Comments != nil && *manifest.Counts.Comments != commentCount {
		report.Issues = append(report.Issues, issue(SeverityWarning, "comment_count_mismatch", fmt.Sprintf("manifest comments=%d, archived comments=%d", *manifest.Counts.Comments, commentCount), "manifest.counts.comments"))
	}
	finalizeReport(&report)
	return report, nil
}

func parseLegacy(ctx context.Context, data []byte, hash string) (PreflightReport, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return PreflightReport{}, fmt.Errorf("decode legacy JSON: %w", err)
	}
	holesRaw, ok := envelope["holes"]
	if !ok {
		return PreflightReport{}, errors.New("legacy JSON is missing holes")
	}
	var holes []json.RawMessage
	if err := json.Unmarshal(holesRaw, &holes); err != nil {
		return PreflightReport{}, errors.New("legacy holes must be an array")
	}
	if len(holes) > MaxItems {
		return PreflightReport{}, fmt.Errorf("legacy archive contains more than %d holes", MaxItems)
	}
	var commentGroups []json.RawMessage
	if raw := envelope["comments"]; len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &commentGroups); err != nil {
			return PreflightReport{}, errors.New("legacy comments must be an array")
		}
	}
	items := make([]rawItem, len(holes))
	for i, hole := range holes {
		pid, _ := rawIntegerField(hole, "pid")
		items[i] = rawItem{
			PID:         strconv.FormatInt(pid, 10),
			Source:      "legacy-v1",
			Hole:        hole,
			FetchStatus: "ok",
		}
		if i < len(commentGroups) {
			items[i].Comments = flattenCommentJSON(commentGroups[i])
		}
	}
	runID := "legacy-" + hash[:16]
	report := validateItems(ctx, FormatLegacyV1, hash, runID, items)
	for _, item := range items {
		report.Counts.ArchivedComments += len(item.Comments)
	}
	finalizeReport(&report)
	return report, nil
}

func validateItems(ctx context.Context, format Format, hash, runID string, items []rawItem) PreflightReport {
	report := PreflightReport{
		Format:      format,
		Status:      StatusCompleted,
		ArchiveHash: hash,
		RunID:       runID,
		Counts:      Counts{Items: len(items)},
		Issues:      []Issue{},
		records:     []Record{},
	}
	if len(items) > MaxItems {
		report.Issues = append(report.Issues, issue(SeverityError, "too_many_items", fmt.Sprintf("archive contains more than %d items", MaxItems), "data.items"))
		items = items[:MaxItems]
	}
	seenPIDs := make(map[int32]bool)
	seenCIDs := make(map[int32]bool)
	for index, item := range items {
		if err := ctx.Err(); err != nil {
			report.Issues = append(report.Issues, issue(SeverityError, "cancelled", err.Error(), ""))
			break
		}
		itemPath := fmt.Sprintf("data.items[%d]", index)
		if item.shapeError != "" {
			report.Issues = append(report.Issues, issue(SeverityError, "invalid_record", item.shapeError, itemPath))
			report.Counts.SkippedItems++
			continue
		}
		pid, err := parsePID(item.PID)
		if err != nil {
			report.Issues = append(report.Issues, issue(SeverityError, "invalid_pid", err.Error(), itemPath+".pid"))
			report.Counts.SkippedItems++
			continue
		}
		if seenPIDs[pid] {
			report.Issues = append(report.Issues, issueWithPID(SeverityError, "duplicate_pid", "duplicate PID in archive", itemPath+".pid", pid))
			report.Counts.SkippedItems++
			continue
		}
		seenPIDs[pid] = true
		if !validSource(item.Source) {
			report.Issues = append(report.Issues, issueWithPID(SeverityError, "invalid_source", fmt.Sprintf("unsupported source %q", item.Source), itemPath+".source", pid))
			report.Counts.SkippedItems++
			continue
		}
		if item.FetchStatus != "ok" && item.FetchStatus != "partial" {
			report.Issues = append(report.Issues, issueWithPID(SeverityError, "invalid_fetch_status", fmt.Sprintf("unsupported fetchStatus %q", item.FetchStatus), itemPath+".fetchStatus", pid))
			report.Counts.SkippedItems++
			continue
		}
		post, err := decodePost(item.Hole, pid)
		if err != nil {
			report.Issues = append(report.Issues, issueWithPID(SeverityError, "invalid_hole", err.Error(), itemPath+".hole", pid))
			report.Counts.SkippedItems++
			continue
		}
		if post.Pid != pid {
			report.Issues = append(report.Issues, issueWithPID(SeverityError, "pid_mismatch", fmt.Sprintf("item PID %d does not match hole PID %d", pid, post.Pid), itemPath+".hole.pid", pid))
			report.Counts.SkippedItems++
			continue
		}
		record := Record{PID: pid, Source: item.Source, FetchStatus: item.FetchStatus, Post: post, ContextOnly: item.Source == "referenced"}
		report.media = mergeMediaRecords(report.media, inferredMedia("post", int64(pid), item.Hole, post.Type == "image"))
		if record.ContextOnly {
			report.Counts.ContextOnly++
		}
		if item.FetchStatus == "partial" {
			report.Issues = append(report.Issues, issueWithPID(SeverityWarning, "partial_item", "archive item has partial fetch status", itemPath+".fetchStatus", pid))
		}
		for commentIndex, rawComment := range item.Comments {
			comment, err := decodeComment(rawComment, pid)
			commentPath := fmt.Sprintf("%s.comments[%d]", itemPath, commentIndex)
			if err != nil {
				report.Issues = append(report.Issues, issueWithPID(SeverityError, "invalid_comment", err.Error(), commentPath, pid))
				report.Counts.SkippedComments++
				continue
			}
			if comment.Pid != pid {
				report.Issues = append(report.Issues, issueWithComment(SeverityError, "comment_pid_mismatch", fmt.Sprintf("comment PID %d does not match item PID %d", comment.Pid, pid), commentPath+".pid", pid, comment.Cid))
				report.Counts.SkippedComments++
				continue
			}
			if seenCIDs[comment.Cid] {
				report.Issues = append(report.Issues, issueWithComment(SeverityError, "duplicate_cid", "duplicate CID in archive", commentPath+".cid", pid, comment.Cid))
				report.Counts.SkippedComments++
				continue
			}
			seenCIDs[comment.Cid] = true
			record.Comments = append(record.Comments, comment)
			report.media = mergeMediaRecords(report.media, inferredMedia("comment", int64(comment.Cid), rawComment, false))
			record.References = append(record.References, explicitTextReferences(pid, &comment.Cid, comment.Text)...)
			if comment.QuoteID != nil {
				targetCID := *comment.QuoteID
				record.References = append(record.References, Reference{
					Kind: "quoted_comment", SourcePID: pid, SourceCID: &comment.Cid,
					TargetPID: pid, TargetCID: &targetCID,
				})
			}
		}
		record.References = append(record.References, explicitTextReferences(pid, nil, post.Text)...)
		report.records = append(report.records, record)
		report.Counts.ValidItems++
		report.Counts.Comments += len(record.Comments)
		report.Counts.Sources++
		report.Counts.References += len(record.References)
	}
	knownTargets := make(map[int32]bool, len(report.records))
	for _, record := range report.records {
		knownTargets[record.PID] = true
	}
	for index := range report.records {
		record := &report.records[index]
		record.References = mergeReferences(record.References, inferredTextReferences(record.PID, nil, record.Post.Text, knownTargets))
		for _, comment := range record.Comments {
			record.References = mergeReferences(record.References, inferredTextReferences(record.PID, &comment.Cid, comment.Text, knownTargets))
		}
	}
	report.Counts.References = 0
	for _, record := range report.records {
		report.Counts.References += len(record.References)
	}
	recountMedia(&report)
	finalizeReport(&report)
	return report
}

func decodeArchiveMedia(data []byte, entries map[string][]byte, records []Record) ([]MediaRecord, []Issue) {
	var media []MediaRecord
	if err := json.Unmarshal(data, &media); err != nil {
		return nil, []Issue{issue(SeverityError, "invalid_media_index", "media/index.json must be an array: "+err.Error(), "media/index.json")}
	}
	validOwners := make(map[string]bool)
	for _, record := range records {
		validOwners[fmt.Sprintf("post:%d", record.PID)] = true
		for _, comment := range record.Comments {
			validOwners[fmt.Sprintf("comment:%d", comment.Cid)] = true
		}
	}
	result := make([]MediaRecord, 0, len(media))
	issues := make([]Issue, 0)
	for index, item := range media {
		itemPath := fmt.Sprintf("media/index.json[%d]", index)
		item.OwnerType = strings.ToLower(strings.TrimSpace(item.OwnerType))
		item.Variant = strings.ToLower(strings.TrimSpace(item.Variant))
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.Variant == "" {
			item.Variant = "original"
		}
		if item.Status == "" {
			item.Status = "missing"
		}
		if !validOwners[fmt.Sprintf("%s:%d", item.OwnerType, item.OwnerID)] {
			issues = append(issues, issue(SeverityError, "invalid_media_owner", "media owner is not present in valid archive records", itemPath))
			continue
		}
		if item.Variant != "original" && item.Variant != "thumbnail" {
			issues = append(issues, issue(SeverityError, "invalid_media_variant", "media variant must be original or thumbnail", itemPath+".variant"))
			continue
		}
		if item.Status != "available" && item.Status != "missing" && item.Status != "failed" {
			issues = append(issues, issue(SeverityError, "invalid_media_status", "media status must be available, missing, or failed", itemPath+".status"))
			continue
		}
		if item.Path == "" {
			item.Status = "missing"
			result = append(result, item)
			continue
		}
		clean := path.Clean(strings.ReplaceAll(item.Path, `\`, "/"))
		if clean != item.Path || !strings.HasPrefix(clean, "media/") || clean == "media/index.json" {
			issues = append(issues, issue(SeverityError, "unsafe_media_path", "media path must name a safe file below media/", itemPath+".path"))
			continue
		}
		content, ok := entries[clean]
		if !ok {
			issues = append(issues, issue(SeverityError, "missing_media_file", "media file is absent from the ZIP", itemPath+".path"))
			continue
		}
		if int64(len(content)) > MaxMediaBytes {
			issues = append(issues, issue(SeverityError, "media_too_large", fmt.Sprintf("individual media exceeds %d bytes", MaxMediaBytes), itemPath+".path"))
			continue
		}
		if item.Size != int64(len(content)) {
			issues = append(issues, issue(SeverityError, "media_size_mismatch", "media file size does not match index", itemPath+".size"))
			continue
		}
		digest := sha256.Sum256(content)
		actualHash := hex.EncodeToString(digest[:])
		if !strings.EqualFold(item.SHA256, actualHash) {
			issues = append(issues, issue(SeverityError, "media_hash_mismatch", "media SHA-256 does not match index", itemPath+".sha256"))
			continue
		}
		detected := http.DetectContentType(content)
		if !strings.HasPrefix(detected, "image/") {
			issues = append(issues, issue(SeverityError, "invalid_media_type", "archive media must be an image", itemPath+".mimeType"))
			continue
		}
		item.SHA256 = actualHash
		item.MIMEType = detected
		item.Status = "available"
		item.Content = content
		result = append(result, item)
	}
	return result, issues
}

func inferredMedia(ownerType string, ownerID int64, raw json.RawMessage, force bool) []MediaRecord {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil
	}
	mediaIDs := splitJSONText(fields["media_ids"])
	remoteURL := jsonText(fields["url"])
	if len(mediaIDs) == 0 && !force && remoteURL == "" {
		return nil
	}
	if len(mediaIDs) == 0 {
		mediaIDs = []string{""}
	}
	dimensions := imageDimensions(fields["image_size"])
	result := make([]MediaRecord, 0, len(mediaIDs))
	for index, remoteID := range mediaIDs {
		item := MediaRecord{OwnerType: ownerType, OwnerID: ownerID, RemoteID: remoteID, RemoteURL: remoteURL, Variant: "original", Status: "missing"}
		if index < len(dimensions) {
			item.Width, item.Height = dimensions[index][0], dimensions[index][1]
		} else if len(dimensions) == 1 {
			item.Width, item.Height = dimensions[0][0], dimensions[0][1]
		}
		result = append(result, item)
	}
	return result
}

func splitJSONText(raw json.RawMessage) []string {
	value := jsonText(raw)
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func jsonText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

func imageDimensions(raw json.RawMessage) [][2]int {
	var values any
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil {
		return nil
	}
	var result [][2]int
	var visit func(any)
	visit = func(value any) {
		items, ok := value.([]any)
		if !ok {
			return
		}
		if len(items) >= 2 {
			width, widthOK := items[0].(float64)
			height, heightOK := items[1].(float64)
			if widthOK && heightOK && width > 0 && height > 0 {
				result = append(result, [2]int{int(width), int(height)})
				return
			}
		}
		for _, item := range items {
			visit(item)
		}
	}
	visit(values)
	return result
}

func mergeMediaRecords(base, additions []MediaRecord) []MediaRecord {
	index := make(map[string]int, len(base)+len(additions))
	for current := range base {
		index[mediaRecordKey(base[current])] = current
	}
	for _, item := range additions {
		key := mediaRecordKey(item)
		if existing, ok := index[key]; ok {
			if item.Status == "available" || base[existing].Status != "available" {
				base[existing] = item
			}
			continue
		}
		index[key] = len(base)
		base = append(base, item)
	}
	return base
}

func mediaRecordKey(item MediaRecord) string {
	return fmt.Sprintf("%s:%d:%s:%s", item.OwnerType, item.OwnerID, item.Variant, item.RemoteID)
}

func recountMedia(report *PreflightReport) {
	report.Counts.Media = len(report.media)
	report.Counts.AvailableMedia = 0
	report.Counts.MissingMedia = 0
	for _, item := range report.media {
		if item.Status == "available" {
			report.Counts.AvailableMedia++
		} else {
			report.Counts.MissingMedia++
		}
	}
}

func decodeManifest(data []byte) (Manifest, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest.json: %w", err)
	}
	for _, required := range []string{"schemaVersion", "toolVersion", "runId", "exportedAt", "scope", "complete", "counts", "errors"} {
		if _, ok := fields[required]; !ok {
			return Manifest{}, fmt.Errorf("manifest is missing %s", required)
		}
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest.json: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return Manifest{}, fmt.Errorf("unsupported archive schemaVersion %d", manifest.SchemaVersion)
	}
	if manifest.ToolVersion == "" || manifest.RunID == "" || manifest.ExportedAt == "" {
		return Manifest{}, errors.New("manifest toolVersion, runId, and exportedAt must be non-empty")
	}
	if _, err := time.Parse(time.RFC3339, manifest.ExportedAt); err != nil {
		return Manifest{}, fmt.Errorf("manifest exportedAt is invalid: %w", err)
	}
	if !jsonObject(manifest.Scope) {
		return Manifest{}, errors.New("manifest scope must be an object")
	}
	if firstNonSpace(fields["errors"]) != '[' || firstNonSpace(fields["counts"]) != '{' {
		return Manifest{}, errors.New("manifest counts must be an object and errors must be an array")
	}
	var countFields map[string]json.RawMessage
	if err := json.Unmarshal(fields["counts"], &countFields); err != nil {
		return Manifest{}, errors.New("manifest counts must be an object")
	}
	for name, raw := range countFields {
		if name == "expectedHoles" && string(raw) == "null" {
			continue
		}
		if name != "expectedHoles" && name != "exportedHoles" && name != "comments" && name != "failed" {
			continue
		}
		var value int
		if err := json.Unmarshal(raw, &value); err != nil || value < 0 {
			return Manifest{}, fmt.Errorf("manifest counts.%s must be a non-negative integer or null", name)
		}
	}
	return manifest, nil
}

func decodeV2Data(data []byte) ([]rawItem, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("decode data.json: %w", err)
	}
	if len(fields) != 1 {
		return nil, errors.New("data.json may only contain items")
	}
	raw, ok := fields["items"]
	if !ok || firstNonSpace(raw) != '[' {
		return nil, errors.New("data.json is missing items array")
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil, errors.New("data.items must be an array")
	}
	if len(rawItems) > MaxItems {
		return nil, fmt.Errorf("archive contains more than %d items", MaxItems)
	}
	items := make([]rawItem, len(rawItems))
	for i, rawItemValue := range rawItems {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawItemValue, &fields); err != nil {
			items[i].shapeError = "item must be an object"
			continue
		}
		missing := make([]string, 0)
		for _, required := range []string{"pid", "source", "hole", "comments", "fetchStatus"} {
			if _, ok := fields[required]; !ok {
				missing = append(missing, required)
			}
		}
		if len(missing) > 0 {
			items[i].shapeError = "item is missing " + strings.Join(missing, ", ")
			continue
		}
		encoded, _ := json.Marshal(fields)
		if err := json.Unmarshal(encoded, &items[i]); err != nil {
			items[i].shapeError = err.Error()
			continue
		}
		if !jsonObject(items[i].Hole) || firstNonSpace(fields["comments"]) != '[' {
			items[i].shapeError = "hole must be an object and comments an array"
		}
	}
	return items, nil
}

func decodePost(raw json.RawMessage, expectedPID int32) (models.Post, error) {
	normalized, err := normalizeModelJSON(raw, postBoolFields, []string{"identity_info", "exclusive_id_info", "image_size"})
	if err != nil {
		return models.Post{}, err
	}
	delete(normalized, "comment_list")
	if pid, ok := rawIntegerField(raw, "pid"); ok {
		normalized["pid"] = json.RawMessage(strconv.FormatInt(pid, 10))
	}
	encoded, _ := json.Marshal(normalized)
	var post models.Post
	if err := json.Unmarshal(encoded, &post); err != nil {
		return models.Post{}, err
	}
	if _, ok := normalized["pid"]; !ok {
		return models.Post{}, errors.New("hole is missing pid")
	}
	if post.Pid == 0 {
		post.Pid = expectedPID
	}
	return post, nil
}

func decodeComment(raw json.RawMessage, inheritedPID int32) (models.Comment, error) {
	normalized, err := normalizeModelJSON(raw, commentBoolFields, []string{"identity_info", "exclusive_id_info"})
	if err != nil {
		return models.Comment{}, err
	}
	if _, ok := normalized["name_tag"]; !ok {
		if name, ok := normalized["name"]; ok {
			normalized["name_tag"] = name
		}
	}
	quoteID := extractQuoteID(normalized["quote"])
	delete(normalized, "quote")
	for _, name := range []string{"cid", "pid"} {
		if value, ok := rawIntegerField(raw, name); ok {
			normalized[name] = json.RawMessage(strconv.FormatInt(value, 10))
		}
	}
	encoded, _ := json.Marshal(normalized)
	var comment models.Comment
	if err := json.Unmarshal(encoded, &comment); err != nil {
		return models.Comment{}, err
	}
	if comment.Cid <= 0 {
		return models.Comment{}, errors.New("comment has invalid cid")
	}
	if _, ok := normalized["pid"]; !ok || comment.Pid == 0 {
		comment.Pid = inheritedPID
	}
	comment.QuoteID = quoteID
	return comment, nil
}

var postBoolFields = map[string]bool{
	"hidden": true, "anonymous": true, "is_top": true, "is_comment": true,
	"identity_show": true, "has_reward_good": true, "is_god_hole": true,
	"is_protect": true, "is_fold": true, "cannot_reply": true,
}

var commentBoolFields = map[string]bool{
	"anonymous": true, "hidden": true, "identity_show": true, "is_lz": true,
}

func normalizeModelJSON(raw json.RawMessage, boolFields map[string]bool, jsonStringFields []string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for name := range boolFields {
		value, ok := fields[name]
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(string(value))
		if trimmed == "true" {
			fields[name] = json.RawMessage("1")
		} else if trimmed == "false" || trimmed == "null" {
			fields[name] = json.RawMessage("0")
		}
	}
	for _, name := range jsonStringFields {
		value, ok := fields[name]
		if !ok || firstNonSpace(value) == '"' || string(value) == "null" {
			continue
		}
		encoded, _ := json.Marshal(string(value))
		fields[name] = encoded
	}
	return fields, nil
}

func extractQuoteID(raw json.RawMessage) *int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if firstNonSpace(raw) == '[' {
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) == nil {
			for _, value := range values {
				if id := extractQuoteID(value); id != nil {
					return id
				}
			}
		}
		return nil
	}
	id, ok := rawIntegerField(raw, "cid")
	if !ok || id <= 0 || id > int64(^uint32(0)>>1) {
		return nil
	}
	result := int32(id)
	return &result
}

func flattenCommentJSON(raw json.RawMessage) []json.RawMessage {
	if firstNonSpace(raw) == '{' {
		return []json.RawMessage{raw}
	}
	if firstNonSpace(raw) != '[' {
		return nil
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	result := make([]json.RawMessage, 0)
	for _, value := range values {
		result = append(result, flattenCommentJSON(value)...)
	}
	return result
}

func explicitTextReferences(sourcePID int32, sourceCID *int32, text string) []Reference {
	seen := make(map[int32]bool)
	result := make([]Reference, 0)
	appendMatch := func(value string) {
		pid, err := parsePID(value)
		if err != nil || pid == sourcePID || seen[pid] {
			return
		}
		seen[pid] = true
		result = append(result, Reference{Kind: "explicit", SourcePID: sourcePID, SourceCID: sourceCID, TargetPID: pid})
	}
	for _, match := range hashReference.FindAllStringSubmatch(text, -1) {
		appendMatch(match[1])
	}
	return result
}

func inferredTextReferences(sourcePID int32, sourceCID *int32, text string, knownTargets map[int32]bool) []Reference {
	result := make([]Reference, 0)
	seen := make(map[int32]bool)
	appendMatch := func(value string) {
		pid, err := parsePID(value)
		if err != nil || pid == sourcePID || seen[pid] || !knownTargets[pid] {
			return
		}
		seen[pid] = true
		result = append(result, Reference{Kind: "inferred", SourcePID: sourcePID, SourceCID: sourceCID, TargetPID: pid})
	}
	if match := leadingReference.FindStringSubmatch(text); len(match) > 1 {
		appendMatch(match[1])
	}
	for _, match := range contextReference.FindAllStringSubmatch(text, -1) {
		for _, value := range match[1:] {
			if value != "" {
				appendMatch(value)
			}
		}
	}
	return result
}

// ExtractTextReferences exposes the same conservative reference rules to the
// database repair job. Explicit #PID references do not require a local target;
// contextual bare PIDs do.
func ExtractTextReferences(sourcePID int32, sourceCID *int32, text string, knownTargets map[int32]bool) []Reference {
	return mergeReferences(explicitTextReferences(sourcePID, sourceCID, text), inferredTextReferences(sourcePID, sourceCID, text, knownTargets))
}

func mergeReferences(base, additions []Reference) []Reference {
	seen := make(map[string]bool, len(base)+len(additions))
	for _, item := range base {
		seen[referenceKey(item)] = true
	}
	for _, item := range additions {
		key := referenceKey(item)
		if !seen[key] {
			seen[key] = true
			base = append(base, item)
		}
	}
	return base
}

func referenceKey(item Reference) string {
	sourceCID, targetCID := int32(0), int32(0)
	if item.SourceCID != nil {
		sourceCID = *item.SourceCID
	}
	if item.TargetCID != nil {
		targetCID = *item.TargetCID
	}
	return fmt.Sprintf("%s:%d:%d:%d:%d", item.Kind, item.SourcePID, sourceCID, item.TargetPID, targetCID)
}

func rawIntegerField(raw json.RawMessage, name string) (int64, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return 0, false
	}
	value, ok := fields[name]
	if !ok {
		return 0, false
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		parsed, err := number.Int64()
		return parsed, err == nil
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		parsed, err := strconv.ParseInt(text, 10, 64)
		return parsed, err == nil
	}
	return 0, false
}

func parsePID(value string) (int32, error) {
	if !pidPattern.MatchString(value) {
		return 0, fmt.Errorf("PID %q must contain 5-7 digits", value)
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid PID %q", value)
	}
	return int32(parsed), nil
}

func validSource(source string) bool {
	return source == "followed" || source == "referenced" || source == "explicit" || source == "legacy-v1"
}

func finalizeReport(report *PreflightReport) {
	if report.Counts.ValidItems == 0 {
		report.Status = StatusFailed
		return
	}
	report.Status = StatusCompleted
	if report.Counts.SkippedItems > 0 || report.Counts.SkippedComments > 0 {
		report.Status = StatusPartial
		return
	}
	for _, current := range report.Issues {
		if current.Severity == SeverityError || current.Severity == SeverityWarning {
			report.Status = StatusPartial
			return
		}
	}
}

func archiveHash(ctx context.Context, reader io.ReaderAt, size int64) (string, error) {
	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	section := io.NewSectionReader(reader, 0, size)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		read, err := section.Read(buffer)
		if read > 0 {
			_, _ = hash.Write(buffer[:read])
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readSection(ctx context.Context, reader io.ReaderAt, size, limit int64) ([]byte, error) {
	if size > limit {
		return nil, fmt.Errorf("input exceeds %d bytes", limit)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return io.ReadAll(io.NewSectionReader(reader, 0, size))
}

func jsonObject(raw json.RawMessage) bool { return firstNonSpace(raw) == '{' }

func firstNonSpace(raw []byte) byte {
	for _, value := range raw {
		if value != ' ' && value != '\n' && value != '\r' && value != '\t' {
			return value
		}
	}
	return 0
}

func issue(severity Severity, code, message, path string) Issue {
	return Issue{Severity: severity, Code: code, Message: message, Path: path}
}

func issueWithPID(severity Severity, code, message, path string, pid int32) Issue {
	current := issue(severity, code, message, path)
	current.PID = pid
	return current
}

func issueWithComment(severity Severity, code, message, path string, pid, cid int32) Issue {
	current := issueWithPID(severity, code, message, path, pid)
	current.CID = cid
	return current
}
