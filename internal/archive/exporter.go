package archive

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func (i *Importer) Export(ctx context.Context, writer io.Writer, request ExportRequest) (ExportReport, error) {
	if i == nil || i.exportStore == nil {
		return ExportReport{}, errors.New("archive export store is not configured")
	}
	if writer == nil {
		return ExportReport{}, errors.New("archive export writer is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if request.Format == "" {
		request.Format = ExportFormatTreeholeV2
	}
	if request.Format != ExportFormatTreeholeV2 && request.Format != ExportFormatMarkdown {
		return ExportReport{}, fmt.Errorf("unsupported export format %q", request.Format)
	}
	records, err := i.exportStore.ArchiveExportSnapshot(ctx, request.PIDs)
	if err != nil {
		return ExportReport{}, err
	}
	if len(records) == 0 {
		return ExportReport{}, errors.New("no posts matched the export request")
	}
	if !request.IncludeComments {
		for index := range records {
			records[index].Comments = nil
		}
	}
	runID := newExportRunID()
	report := ExportReport{Format: request.Format, Posts: len(records), RunID: runID}
	for _, record := range records {
		report.Comments += len(record.Comments)
		report.Media += len(record.Media)
		for _, item := range record.Media {
			if item.Status != "available" || strings.TrimSpace(item.Path) == "" {
				report.MissingMedia++
			}
		}
	}
	if request.Format == ExportFormatMarkdown {
		err = writeMarkdownBundle(ctx, writer, records, report, i.dataDir)
	} else {
		err = writeTreeholeV2(ctx, writer, records, report, len(request.PIDs) > 0, i.dataDir)
	}
	if err != nil {
		return ExportReport{}, err
	}
	return report, nil
}

func writeTreeholeV2(ctx context.Context, output io.Writer, records []ExportRecord, report ExportReport, selected bool, dataDir string) error {
	archiveWriter := zip.NewWriter(output)
	closeWithError := func(err error) error {
		return errors.Join(err, archiveWriter.Close())
	}
	manifestEntry, err := archiveWriter.Create("manifest.json")
	if err != nil {
		return closeWithError(err)
	}
	scopeType := "library"
	if selected {
		scopeType = "pids"
	}
	mediaIndex, mediaFiles := prepareExportMedia(ctx, records, dataDir)
	report.MissingMedia = 0
	for _, item := range mediaIndex {
		if item.Status != "available" {
			report.MissingMedia++
		}
	}
	manifest := map[string]any{
		"schemaVersion": 2, "toolVersion": "PkuHoleStudio", "runId": report.RunID,
		"exportedAt": time.Now().UTC().Format(time.RFC3339), "scope": map[string]any{"type": scopeType},
		"complete": true, "counts": map[string]any{
			"expectedHoles": report.Posts, "exportedHoles": report.Posts, "comments": report.Comments, "media": len(mediaIndex), "missingMedia": report.MissingMedia, "failed": 0,
		}, "errors": []any{},
	}
	if err := json.NewEncoder(manifestEntry).Encode(manifest); err != nil {
		return closeWithError(err)
	}
	dataEntry, err := archiveWriter.Create("data.json")
	if err != nil {
		return closeWithError(err)
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return closeWithError(err)
		}
		comments := make([]map[string]any, 0, len(record.Comments))
		for _, comment := range record.Comments {
			comments = append(comments, exportComment(comment))
		}
		items = append(items, map[string]any{
			"pid": strconv.FormatInt(int64(record.Post.Pid), 10), "source": preferredExportSource(record.Sources),
			"hole": record.Post, "comments": comments, "fetchStatus": "ok", "studioSources": record.Sources,
		})
	}
	if err := json.NewEncoder(dataEntry).Encode(map[string]any{"items": items}); err != nil {
		return closeWithError(err)
	}
	if len(mediaIndex) > 0 {
		indexEntry, err := archiveWriter.Create("media/index.json")
		if err != nil {
			return closeWithError(err)
		}
		if err := json.NewEncoder(indexEntry).Encode(mediaIndex); err != nil {
			return closeWithError(err)
		}
		paths := make([]string, 0, len(mediaFiles))
		for name := range mediaFiles {
			paths = append(paths, name)
		}
		sort.Strings(paths)
		for _, name := range paths {
			entry, err := archiveWriter.Create(name)
			if err != nil {
				return closeWithError(err)
			}
			if _, err := entry.Write(mediaFiles[name]); err != nil {
				return closeWithError(err)
			}
		}
	}
	readableEntry, err := archiveWriter.Create("readable.txt")
	if err != nil {
		return closeWithError(err)
	}
	if err := writeReadableText(readableEntry, records); err != nil {
		return closeWithError(err)
	}
	return archiveWriter.Close()
}

func prepareExportMedia(ctx context.Context, records []ExportRecord, dataDir string) ([]MediaRecord, map[string][]byte) {
	index := make([]MediaRecord, 0)
	files := make(map[string][]byte)
	for _, record := range records {
		for _, row := range record.Media {
			if ctx.Err() != nil {
				return index, files
			}
			item := MediaRecord{
				OwnerType: row.OwnerType, OwnerID: row.OwnerID, RemoteID: row.RemoteID,
				RemoteURL: row.RemoteURL, Variant: row.Variant, MIMEType: row.MIMEType,
				Size: row.Size, SHA256: row.ContentHash, Width: row.Width, Height: row.Height,
				Status: "missing",
			}
			if item.Variant == "" {
				item.Variant = "original"
			}
			localPath := strings.TrimSpace(row.Path)
			if localPath != "" && !filepath.IsAbs(localPath) {
				localPath = filepath.Join(dataDir, filepath.FromSlash(localPath))
			}
			content, err := os.ReadFile(localPath)
			if err == nil && len(content) > 0 {
				digest := sha256.Sum256(content)
				item.SHA256 = hex.EncodeToString(digest[:])
				item.Size = int64(len(content))
				item.MIMEType = http.DetectContentType(content)
				if strings.HasPrefix(item.MIMEType, "image/") {
					item.Status = "available"
					item.Path = "media/" + item.SHA256 + mediaExtension(item.MIMEType)
					files[item.Path] = content
				}
			}
			index = append(index, item)
		}
	}
	return index, files
}

func writeMarkdownBundle(ctx context.Context, output io.Writer, records []ExportRecord, report ExportReport, dataDir string) error {
	archiveWriter := zip.NewWriter(output)
	mediaIndex, mediaFiles := prepareExportMedia(ctx, records, dataDir)
	index, err := archiveWriter.Create("index.md")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(index, "# PkuHoleStudio 资料导出\n\n- 帖子：%d\n- 评论：%d\n- 导出时间：%s\n\n## 帖子索引\n\n", report.Posts, report.Comments, time.Now().Format("2006-01-02 15:04:05")); err != nil {
		return err
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(index, "- [#%d](posts/%d.md)\n", record.Post.Pid, record.Post.Pid); err != nil {
			return err
		}
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := archiveWriter.Create(fmt.Sprintf("posts/%d.md", record.Post.Pid))
		if err != nil {
			return err
		}
		if err := writePostMarkdown(entry, record, mediaIndex); err != nil {
			return err
		}
	}
	paths := make([]string, 0, len(mediaFiles))
	for name := range mediaFiles {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		entry, err := archiveWriter.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(mediaFiles[name]); err != nil {
			return err
		}
	}
	return archiveWriter.Close()
}

func writePostMarkdown(writer io.Writer, record ExportRecord, media []MediaRecord) error {
	if _, err := fmt.Fprintf(writer, "# #%d\n\n- 时间戳：%d\n- 来源：%s\n\n%s\n", record.Post.Pid, record.Post.Timestamp, preferredExportSource(record.Sources), record.Post.Text); err != nil {
		return err
	}
	if err := writeMarkdownMedia(writer, media, "post", int64(record.Post.Pid)); err != nil {
		return err
	}
	if len(record.Comments) == 0 {
		return nil
	}
	if _, err := io.WriteString(writer, "\n## 评论\n"); err != nil {
		return err
	}
	for _, comment := range record.Comments {
		if _, err := fmt.Fprintf(writer, "\n### C%d · %s\n\n%s\n", comment.Cid, comment.NameTag, comment.Text); err != nil {
			return err
		}
		if err := writeMarkdownMedia(writer, media, "comment", int64(comment.Cid)); err != nil {
			return err
		}
	}
	return nil
}

func writeMarkdownMedia(writer io.Writer, media []MediaRecord, ownerType string, ownerID int64) error {
	for _, item := range media {
		if item.OwnerType != ownerType || item.OwnerID != ownerID {
			continue
		}
		if item.Status == "available" && item.Path != "" {
			if _, err := fmt.Fprintf(writer, "\n![图片](../%s)\n", item.Path); err != nil {
				return err
			}
		} else if _, err := io.WriteString(writer, "\n> 图片尚未下载，归档中仅保留了远程元数据。\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeReadableText(writer io.Writer, records []ExportRecord) error {
	for _, record := range records {
		if _, err := fmt.Fprintf(writer, "#%d\n%s\n", record.Post.Pid, record.Post.Text); err != nil {
			return err
		}
		for _, comment := range record.Comments {
			if _, err := fmt.Fprintf(writer, "  C%d %s: %s\n", comment.Cid, comment.NameTag, comment.Text); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(writer, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func exportComment(comment models.Comment) map[string]any {
	encoded, _ := json.Marshal(comment)
	result := map[string]any{}
	_ = json.Unmarshal(encoded, &result)
	if comment.QuoteID != nil {
		result["quote"] = []any{map[string]any{"cid": *comment.QuoteID}}
	}
	return result
}

func preferredExportSource(sources []models.PostSource) string {
	priority := map[string]int{"followed": 0, "explicit": 1, "legacy-v1": 2, "referenced": 3}
	candidates := make([]string, 0, len(sources))
	for _, source := range sources {
		if _, ok := priority[source.Source]; ok {
			candidates = append(candidates, source.Source)
		}
	}
	if len(candidates) == 0 {
		return "explicit"
	}
	sort.SliceStable(candidates, func(left, right int) bool { return priority[candidates[left]] < priority[candidates[right]] })
	return candidates[0]
}

func newExportRunID() string {
	random := make([]byte, 6)
	_, _ = rand.Read(random)
	return "studio-" + strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "") + "-" + hex.EncodeToString(random)
}
