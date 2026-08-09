package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/crawler"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
)

// CrawlOptions contains the behavior switches supported by the legacy crawler.
// Keeping the options here prevents UI and command packages from importing the
// crawler implementation directly.
type CrawlOptions struct {
	SaveJSON     bool `json:"save_json"`
	PostLimit    int  `json:"post_limit"`
	CommentLimit int  `json:"comment_limit"`
	FetchImages  bool `json:"fetch_images"`
	ConvertWebP  bool `json:"convert_webp"`
}

type SyncResult struct {
	PostCount    int
	CommentCount int
	Media        MediaResult
}

type MediaResult struct {
	Downloaded int `json:"downloaded"`
	Existing   int `json:"existing"`
	Failed     int `json:"failed"`
}

type ThumbnailResult = MediaResult

type RawJSONResult struct {
	Responses int   `json:"responses"`
	Bytes     int64 `json:"bytes"`
}

type syncRunner interface {
	FetchPage(page int, options CrawlOptions) (SyncResult, error)
	FetchImages(convertWebP bool) (MediaResult, error)
	FetchThumbnails(startID, endID int, convertWebP bool) (ThumbnailResult, error)
	SaveRawResponses() error
	SaveRawResponsesTo(path string) (RawJSONResult, error)
	RawResponseCount() int
}

// SyncService is the single entry point for crawler-backed writes. Persistent
// scheduling and checkpoints are layered on top of it by JobManager.
type SyncService struct {
	runner syncRunner
}

func NewSyncService(c *client.Client, database *db.Database, dataDir string) *SyncService {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = "data"
	}
	dataDir = filepath.Clean(dataDir)
	return &SyncService{runner: &legacySyncRunner{
		client:       c,
		database:     database,
		imageDir:     filepath.Join(dataDir, "images"),
		thumbnailDir: filepath.Join(dataDir, "thumbnails"),
	}}
}

func newSyncService(runner syncRunner) *SyncService {
	return &SyncService{runner: runner}
}

func (s *SyncService) FetchPage(ctx context.Context, page int, options CrawlOptions) (SyncResult, error) {
	if err := contextError(ctx); err != nil {
		return SyncResult{}, err
	}
	if s == nil || s.runner == nil {
		return SyncResult{}, errors.New("sync service is not configured")
	}
	result, err := s.runner.FetchPage(page, options)
	if err != nil {
		return result, err
	}
	if err := contextError(ctx); err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func (s *SyncService) FetchImages(ctx context.Context, convertWebP bool) (MediaResult, error) {
	if err := contextError(ctx); err != nil {
		return MediaResult{}, err
	}
	if s == nil || s.runner == nil {
		return MediaResult{}, errors.New("sync service is not configured")
	}
	result, err := s.runner.FetchImages(convertWebP)
	if err != nil {
		return result, err
	}
	if err := contextError(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (s *SyncService) FetchThumbnails(ctx context.Context, startID, endID int, convertWebP bool) (ThumbnailResult, error) {
	if err := contextError(ctx); err != nil {
		return ThumbnailResult{}, err
	}
	if s == nil || s.runner == nil {
		return ThumbnailResult{}, errors.New("sync service is not configured")
	}
	result, err := s.runner.FetchThumbnails(startID, endID, convertWebP)
	if err != nil {
		return result, err
	}
	if err := contextError(ctx); err != nil {
		return ThumbnailResult{}, err
	}
	return result, nil
}

func (s *SyncService) SaveRawResponses(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || s.runner == nil {
		return errors.New("sync service is not configured")
	}
	return s.runner.SaveRawResponses()
}

func (s *SyncService) SaveRawResponsesTo(ctx context.Context, path string) (RawJSONResult, error) {
	if err := contextError(ctx); err != nil {
		return RawJSONResult{}, err
	}
	if s == nil || s.runner == nil {
		return RawJSONResult{}, errors.New("sync service is not configured")
	}
	result, err := s.runner.SaveRawResponsesTo(path)
	if err != nil {
		return RawJSONResult{}, err
	}
	if err := contextError(ctx); err != nil {
		return RawJSONResult{}, err
	}
	return result, nil
}

func (s *SyncService) RawResponseCount() int {
	if s == nil || s.runner == nil {
		return 0
	}
	return s.runner.RawResponseCount()
}

type legacySyncRunner struct {
	client       *client.Client
	database     *db.Database
	imageDir     string
	thumbnailDir string
}

func (r *legacySyncRunner) FetchPage(page int, options CrawlOptions) (SyncResult, error) {
	result, err := crawler.FetchAndSave(
		r.client,
		r.database,
		page,
		options.SaveJSON,
		options.PostLimit,
		options.CommentLimit,
		options.FetchImages,
		options.ConvertWebP,
		r.imageDir,
	)
	return SyncResult{
		PostCount:    result.PostCount,
		CommentCount: result.CommentCount,
		Media:        mediaResultFromCrawler(result.Media),
	}, err
}

func (r *legacySyncRunner) FetchImages(convertWebP bool) (MediaResult, error) {
	result, err := crawler.FetchImagesFromDB(r.client, r.database, r.imageDir, convertWebP)
	return mediaResultFromCrawler(result), err
}

func (r *legacySyncRunner) FetchThumbnails(startID, endID int, convertWebP bool) (ThumbnailResult, error) {
	result, err := crawler.FetchThumbnailsByIDRange(r.client, startID, endID, r.thumbnailDir, convertWebP)
	return mediaResultFromCrawler(result), err
}

func (r *legacySyncRunner) SaveRawResponses() error { return crawler.SaveRawResponsesToFile() }
func (r *legacySyncRunner) SaveRawResponsesTo(path string) (RawJSONResult, error) {
	count, size, err := crawler.SaveRawResponsesTo(path)
	return RawJSONResult{Responses: count, Bytes: size}, err
}
func (r *legacySyncRunner) RawResponseCount() int { return crawler.RawResponses() }

func mediaResultFromCrawler(result crawler.MediaDownloadResult) MediaResult {
	return MediaResult{
		Downloaded: result.Downloaded,
		Existing:   result.Existing,
		Failed:     result.Failed,
	}
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
