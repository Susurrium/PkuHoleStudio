package archive

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestImporterIsTransactionalAndIdempotent(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{"pid": "123456", "source": "followed", "fetchStatus": "ok", "hole": map[string]any{"pid": 123456, "text": "main"}, "comments": []any{map[string]any{"cid": 1, "pid": 123456, "text": "comment"}}},
		map[string]any{"pid": "234567", "source": "referenced", "fetchStatus": "ok", "hole": map[string]any{"pid": 234567, "text": "context"}, "comments": []any{}},
	}}
	content := makeV2ZIP(t, validManifest(2, 1), data)
	store := &fakeArchiveStore{}
	importer := NewImporter(store)

	report, err := importer.Import(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if report.Status != StatusCompleted || len(store.posts) != 2 || len(store.comments) != 1 || len(store.sources) != 2 {
		t.Fatalf("report/store = %+v / %+v", report, store)
	}
	if !store.sources[1].ContextOnly {
		t.Fatalf("referenced source = %+v", store.sources[1])
	}

	duplicate, err := importer.Import(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err != nil || !duplicate.Duplicate || duplicate.Status != StatusDuplicate || store.transactions != 1 {
		t.Fatalf("duplicate = %+v, err = %v, transactions = %d", duplicate, err, store.transactions)
	}
}

func TestImporterRollsBackWhenTransactionStepFails(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{"pid": "123456", "source": "explicit", "fetchStatus": "ok", "hole": map[string]any{"pid": 123456, "text": "main"}, "comments": []any{map[string]any{"cid": 1, "pid": 123456, "text": "comment"}}},
	}}
	content := makeV2ZIP(t, validManifest(1, 1), data)
	store := &fakeArchiveStore{failAt: "comments"}
	_, err := NewImporter(store).Import(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err == nil || len(store.posts) != 0 || len(store.comments) != 0 || store.saved != nil {
		t.Fatalf("Import() error/store = %v / %+v", err, store)
	}
}

func TestImporterRejectsArchiveWithoutValidItemsBeforeTransaction(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{"pid": "123456", "source": "followed", "fetchStatus": "ok", "hole": map[string]any{"pid": 123456, "timestamp": map[string]any{"invalid": true}}, "comments": []any{}},
	}}
	content := makeV2ZIP(t, validManifest(1, 0), data)
	store := &fakeArchiveStore{}
	report, err := NewImporter(store).Import(context.Background(), bytes.NewReader(content), int64(len(content)))
	if err == nil || report.Status != StatusFailed || report.Counts.ValidItems != 0 || store.transactions != 0 || store.saved != nil {
		t.Fatalf("Import() report/error/store = %+v / %v / %+v", report, err, store)
	}
}

type fakeArchiveStore struct {
	posts        []models.Post
	comments     []models.Comment
	sources      []PostSource
	references   []Reference
	saved        *ImportRun
	failAt       string
	transactions int
}

func (s *fakeArchiveStore) FindImport(_ context.Context, hash, runID string) (ImportRun, bool, error) {
	if s.saved != nil && (s.saved.ArchiveHash == hash || (runID != "" && s.saved.RunID == runID)) {
		return *s.saved, true, nil
	}
	return ImportRun{}, false, nil
}

func (s *fakeArchiveStore) Transaction(ctx context.Context, fn func(Transaction) error) error {
	s.transactions++
	tx := &fakeArchiveTransaction{failAt: s.failAt}
	if err := fn(tx); err != nil {
		return err
	}
	s.posts = append(s.posts, tx.posts...)
	s.comments = append(s.comments, tx.comments...)
	s.sources = append(s.sources, tx.sources...)
	s.references = append(s.references, tx.references...)
	if tx.saved != nil {
		copy := *tx.saved
		s.saved = &copy
	}
	return ctx.Err()
}

type fakeArchiveTransaction struct {
	posts      []models.Post
	comments   []models.Comment
	sources    []PostSource
	references []Reference
	saved      *ImportRun
	failAt     string
}

func (t *fakeArchiveTransaction) UpsertPosts(_ context.Context, rows []models.Post) error {
	if t.failAt == "posts" {
		return errors.New("posts failed")
	}
	t.posts = append(t.posts, rows...)
	return nil
}

func (t *fakeArchiveTransaction) UpsertComments(_ context.Context, rows []models.Comment) error {
	if t.failAt == "comments" {
		return errors.New("comments failed")
	}
	t.comments = append(t.comments, rows...)
	return nil
}

func (t *fakeArchiveTransaction) UpsertSources(_ context.Context, rows []PostSource) error {
	if t.failAt == "sources" {
		return errors.New("sources failed")
	}
	t.sources = append(t.sources, rows...)
	return nil
}

func (t *fakeArchiveTransaction) UpsertReferences(_ context.Context, rows []Reference) error {
	if t.failAt == "references" {
		return errors.New("references failed")
	}
	t.references = append(t.references, rows...)
	return nil
}

func (t *fakeArchiveTransaction) SaveImportRun(_ context.Context, run ImportRun) error {
	if t.failAt == "run" {
		return errors.New("run failed")
	}
	t.saved = &run
	return nil
}
