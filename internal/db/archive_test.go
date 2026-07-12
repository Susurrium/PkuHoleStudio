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
