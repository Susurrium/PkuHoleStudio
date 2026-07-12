package db

import (
	"bytes"
	"context"
	"testing"
	"time"

	archivepkg "github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestArchiveImportPersistsOnceAndPreservesLocalMetadata(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	legacy := []byte(`{"holes":[{"pid":123456,"text":"first"}],"comments":[[[{"cid":1001,"text":"comment"}]]]}`)
	importer := archivepkg.NewImporter(database)
	report, err := importer.Import(context.Background(), bytes.NewReader(legacy), int64(len(legacy)))
	if err != nil || report.Status != archivepkg.StatusCompleted {
		t.Fatalf("Import() = %+v, %v", report, err)
	}

	now := time.Now().UTC()
	tag := models.LocalTag{Name: "keep", CreatedAt: now, UpdatedAt: now}
	if err := database.db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.db.Create(&models.PostTag{PID: 123456, TagID: tag.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.db.Create(&models.Note{OwnerType: "post", OwnerID: 123456, Content: "local note", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	duplicate, err := importer.Import(context.Background(), bytes.NewReader(legacy), int64(len(legacy)))
	if err != nil || !duplicate.Duplicate {
		t.Fatalf("duplicate Import() = %+v, %v", duplicate, err)
	}
	for model, want := range map[any]int64{
		&models.Post{}: 1, &models.Comment{}: 1, &models.PostSource{}: 1,
		&models.ImportRun{}: 1, &models.PostTag{}: 1, &models.Note{}: 1,
	} {
		var count int64
		if err := database.db.Model(model).Count(&count).Error; err != nil || count != want {
			t.Fatalf("%T count = %d, %v; want %d", model, count, err, want)
		}
	}
	var note models.Note
	if err := database.db.First(&note, "owner_type = ? AND owner_id = ?", "post", 123456).Error; err != nil || note.Content != "local note" {
		t.Fatalf("note = %+v, %v", note, err)
	}
}

func TestArchiveTransactionRollsBackAllRows(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	err := database.Transaction(context.Background(), func(tx archivepkg.Transaction) error {
		if err := tx.UpsertPosts(context.Background(), []models.Post{{Pid: 123456, Text: "temporary"}}); err != nil {
			return err
		}
		return context.Canceled
	})
	if err == nil {
		t.Fatal("Transaction() error = nil")
	}
	var count int64
	if err := database.db.Model(&models.Post{}).Where("pid = ?", 123456).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("post count = %d, %v", count, err)
	}
}

func TestContextOnlyArchivePostsStayOutOfDefaultLibraryQueries(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	if err := database.Transaction(ctx, func(tx archivepkg.Transaction) error {
		if err := tx.UpsertPosts(ctx, []models.Post{{Pid: 123456, Text: "visible"}, {Pid: 234567, Text: "context keyword"}}); err != nil {
			return err
		}
		return tx.UpsertSources(ctx, []archivepkg.PostSource{
			{PID: 123456, Source: "followed", ArchiveHash: "one"},
			{PID: 234567, Source: "referenced", ArchiveHash: "one", ContextOnly: true},
		})
	}); err != nil {
		t.Fatal(err)
	}

	posts, err := database.GetPostsCursor(0, 10, false)
	if err != nil || len(posts) != 1 || posts[0].Pid != 123456 {
		t.Fatalf("default posts = %+v, %v", posts, err)
	}
	page, err := database.SearchFullText(models.FullTextQuery{Query: "context", Limit: 10})
	if err != nil || len(page.Hits) != 0 {
		t.Fatalf("default search = %+v, %v", page, err)
	}
	page, err = database.SearchFullText(models.FullTextQuery{Query: "context", Sources: []string{"referenced"}, Limit: 10})
	if err != nil || len(page.Hits) != 1 || page.Hits[0].Post.Pid != 234567 {
		t.Fatalf("referenced search = %+v, %v", page, err)
	}

	if err := database.Transaction(ctx, func(tx archivepkg.Transaction) error {
		return tx.UpsertSources(ctx, []archivepkg.PostSource{{PID: 234567, Source: "explicit", ArchiveHash: "two"}})
	}); err != nil {
		t.Fatal(err)
	}
	posts, err = database.GetPostsCursor(0, 10, false)
	if err != nil || len(posts) != 2 {
		t.Fatalf("promoted posts = %+v, %v", posts, err)
	}
}

func TestReferenceProjectionResolvesCommentOwners(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	sourceCID, targetCID := int32(1001), int32(2001)
	if err := database.Transaction(ctx, func(tx archivepkg.Transaction) error {
		if err := tx.UpsertPosts(ctx, []models.Post{{Pid: 123456}, {Pid: 234567}}); err != nil {
			return err
		}
		if err := tx.UpsertComments(ctx, []models.Comment{{Cid: sourceCID, Pid: 123456}, {Cid: targetCID, Pid: 234567}}); err != nil {
			return err
		}
		return tx.UpsertReferences(ctx, []archivepkg.Reference{{
			Kind: "quotes", SourcePID: 123456, SourceCID: &sourceCID,
			TargetPID: 234567, TargetCID: &targetCID,
		}})
	}); err != nil {
		t.Fatal(err)
	}
	edges, err := database.GetReferencesByPID(123456)
	if err != nil || len(edges) != 1 || edges[0].SourcePID != 123456 || edges[0].TargetPID != 234567 || edges[0].SourceCID == nil || edges[0].TargetCID == nil {
		t.Fatalf("edges = %+v, %v", edges, err)
	}
	reverse, err := database.GetReferencesByPID(234567)
	if err != nil || len(reverse) != 1 || reverse[0].SourcePID != 123456 || reverse[0].TargetPID != 234567 {
		t.Fatalf("reverse edges = %+v, %v", reverse, err)
	}
}

func TestRebuildReferencesDetectsContextualBarePID(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "看到7853541的dz推荐"}, {Pid: 7853541, Text: "context"}}); err != nil {
		t.Fatal(err)
	}
	count, err := database.RebuildReferences(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("RebuildReferences() = %d, %v", count, err)
	}
	edges, err := database.GetReferencesByPID(8133824)
	if err != nil || len(edges) != 1 || edges[0].Kind != "inferred" || edges[0].TargetPID != 7853541 {
		t.Fatalf("edges = %+v, %v", edges, err)
	}
}

func TestStudioArchiveRoundTripsLocalMetadataWithoutOverwritingExistingNotes(t *testing.T) {
	source, cleanupSource := setupTestDB(t)
	defer cleanupSource()
	if err := source.SaveCrawlResult(
		[]models.Post{{Pid: 8133824, Text: "source"}},
		[]models.Comment{{Cid: 9001, Pid: 8133824, Text: "comment"}},
	); err != nil {
		t.Fatal(err)
	}
	tag, err := source.CreateLocalTag("课程", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.SetPostTags(8133824, []uint{tag.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := source.UpsertNote("post", 8133824, "source post note"); err != nil {
		t.Fatal(err)
	}
	if _, err := source.UpsertNote("comment", 9001, "source comment note"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.ArchiveExportSnapshot(context.Background(), nil)
	if err != nil || len(snapshot) != 1 || len(snapshot[0].Studio.Tags) != 1 || snapshot[0].Studio.Note == "" || len(snapshot[0].Studio.CommentNotes) != 1 {
		t.Fatalf("source metadata snapshot = %+v, error = %v", snapshot, err)
	}
	var output bytes.Buffer
	if _, err := archivepkg.NewImporterWithDataDir(source, t.TempDir()).Export(context.Background(), &output, archivepkg.ExportRequest{Format: archivepkg.ExportFormatTreeholeV2, IncludeComments: true}); err != nil {
		t.Fatal(err)
	}

	destination, cleanupDestination := setupTestDB(t)
	defer cleanupDestination()
	if _, err := destination.UpsertNote("post", 8133824, "destination wins"); err != nil {
		t.Fatal(err)
	}
	if _, err := archivepkg.NewImporterWithDataDir(destination, t.TempDir()).Import(context.Background(), bytes.NewReader(output.Bytes()), int64(output.Len())); err != nil {
		t.Fatal(err)
	}
	tags, err := destination.GetPostTags(8133824)
	if err != nil || len(tags) != 1 || tags[0].Name != "课程" || tags[0].Color != "#0f766e" {
		t.Fatalf("imported tags = %+v, error = %v", tags, err)
	}
	postNote, err := destination.GetNote("post", 8133824)
	if err != nil || postNote == nil || postNote.Content != "destination wins" {
		t.Fatalf("existing post note was overwritten: %+v, %v", postNote, err)
	}
	commentNote, err := destination.GetNote("comment", 9001)
	if err != nil || commentNote == nil || commentNote.Content != "source comment note" {
		t.Fatalf("comment note was not imported: %+v, %v", commentNote, err)
	}
}
