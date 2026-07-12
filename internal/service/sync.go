package service

import (
	"context"
	"errors"

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
}

type ThumbnailResult struct {
	Downloaded int
	Skipped    int
}

type syncRunner interface {
	FetchPage(page int, options CrawlOptions) (SyncResult, error)
	FetchImages(convertWebP bool) error
	FetchThumbnails(startID, endID int, convertWebP bool) (ThumbnailResult, error)
	SaveRawResponses() error
	RawResponseCount() int
}

// SyncService is the single entry point for crawler-backed writes. Persistent
// scheduling and checkpoints are layered on top of it by JobManager.
type SyncService struct {
	runner syncRunner
}

func NewSyncService(c *client.Client, database *db.Database) *SyncService {
	return &SyncService{runner: &legacySyncRunner{client: c, database: database}}
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
		return SyncResult{}, err
	}
	if err := contextError(ctx); err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func (s *SyncService) FetchImages(ctx context.Context, convertWebP bool) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || s.runner == nil {
		return errors.New("sync service is not configured")
	}
	if err := s.runner.FetchImages(convertWebP); err != nil {
		return err
	}
	return contextError(ctx)
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
		return ThumbnailResult{}, err
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

func (s *SyncService) RawResponseCount() int {
	if s == nil || s.runner == nil {
		return 0
	}
	return s.runner.RawResponseCount()
}

type legacySyncRunner struct {
	client   *client.Client
	database *db.Database
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
	)
	if err != nil {
		return SyncResult{}, err
	}
	return SyncResult{PostCount: result.PostCount, CommentCount: result.CommentCount}, nil
}

func (r *legacySyncRunner) FetchImages(convertWebP bool) error {
	crawler.FetchImagesFromDB(r.client, r.database, convertWebP)
	return nil
}

func (r *legacySyncRunner) FetchThumbnails(startID, endID int, convertWebP bool) (ThumbnailResult, error) {
	downloaded, skipped, err := crawler.FetchThumbnailsByIDRange(r.client, startID, endID, convertWebP)
	if err != nil {
		return ThumbnailResult{}, err
	}
	return ThumbnailResult{Downloaded: downloaded, Skipped: skipped}, nil
}

func (r *legacySyncRunner) SaveRawResponses() error { return crawler.SaveRawResponsesToFile() }
func (r *legacySyncRunner) RawResponseCount() int   { return crawler.RawResponses() }

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
