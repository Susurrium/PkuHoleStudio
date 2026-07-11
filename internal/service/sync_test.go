package service

import (
	"context"
	"errors"
	"testing"
)

type fakeSyncRunner struct {
	fetchedPage int
	options     CrawlOptions
	images      bool
	thumbs      bool
	rawCount    int
	err         error
}

func (f *fakeSyncRunner) FetchPage(page int, options CrawlOptions) (SyncResult, error) {
	f.fetchedPage = page
	f.options = options
	return SyncResult{PostCount: 2, CommentCount: 3}, f.err
}

func (f *fakeSyncRunner) FetchImages(bool) error {
	f.images = true
	return f.err
}

func (f *fakeSyncRunner) FetchThumbnails(int, int, bool) (ThumbnailResult, error) {
	f.thumbs = true
	return ThumbnailResult{Downloaded: 4, Skipped: 1}, f.err
}

func (f *fakeSyncRunner) SaveRawResponses() error { return f.err }
func (f *fakeSyncRunner) RawResponseCount() int   { return f.rawCount }

func TestSyncServiceDelegatesCrawlerOperations(t *testing.T) {
	runner := &fakeSyncRunner{rawCount: 7}
	svc := newSyncService(runner)
	options := CrawlOptions{SaveJSON: true, PostLimit: 10, CommentLimit: 20, FetchImages: true, ConvertWebP: true}

	result, err := svc.FetchPage(context.Background(), 3, options)
	if err != nil {
		t.Fatalf("FetchPage() error = %v", err)
	}
	if runner.fetchedPage != 3 || runner.options != options {
		t.Fatalf("runner received page=%d options=%+v", runner.fetchedPage, runner.options)
	}
	if result.PostCount != 2 || result.CommentCount != 3 {
		t.Fatalf("FetchPage() result = %+v", result)
	}
	if err := svc.FetchImages(context.Background(), true); err != nil || !runner.images {
		t.Fatalf("FetchImages() err=%v called=%v", err, runner.images)
	}
	thumbs, err := svc.FetchThumbnails(context.Background(), 1, 5, false)
	if err != nil || !runner.thumbs || thumbs.Downloaded != 4 || thumbs.Skipped != 1 {
		t.Fatalf("FetchThumbnails() result=%+v err=%v called=%v", thumbs, err, runner.thumbs)
	}
	if svc.RawResponseCount() != 7 {
		t.Fatalf("RawResponseCount() = %d", svc.RawResponseCount())
	}
}

func TestSyncServiceHonorsCancelledContext(t *testing.T) {
	runner := &fakeSyncRunner{}
	svc := newSyncService(runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.FetchPage(ctx, 1, CrawlOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchPage() error = %v, want context.Canceled", err)
	}
	if runner.fetchedPage != 0 {
		t.Fatal("runner was called after cancellation")
	}
}
