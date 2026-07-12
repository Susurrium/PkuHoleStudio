package db

import (
	"path/filepath"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestLocalMetadataPersistsAndSurvivesPostRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.db")
	database := openDatabaseAt(t, path)

	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "original"}}); err != nil {
		t.Fatal(err)
	}
	tag, err := database.CreateLocalTag("课程", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetPostTags(8133824, []uint{tag.ID, tag.ID}); err != nil {
		t.Fatal(err)
	}
	note, err := database.UpsertNote("post", 8133824, "只保存在本机")
	if err != nil {
		t.Fatal(err)
	}

	// Remote refreshes and archive upserts only replace remote post fields.
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "refreshed"}}); err != nil {
		t.Fatal(err)
	}
	tags, err := database.GetPostTags(8133824)
	if err != nil || len(tags) != 1 || tags[0].ID != tag.ID {
		t.Fatalf("tags after refresh = %+v, error = %v", tags, err)
	}
	storedNote, err := database.GetNote("post", 8133824)
	if err != nil || storedNote == nil || storedNote.Content != note.Content {
		t.Fatalf("note after refresh = %+v, error = %v", storedNote, err)
	}

	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openDatabaseAt(t, path)
	defer reopened.Close()
	tags, err = reopened.GetPostTags(8133824)
	if err != nil || len(tags) != 1 || tags[0].Name != "课程" {
		t.Fatalf("tags after reopen = %+v, error = %v", tags, err)
	}
	storedNote, err = reopened.GetNote("post", 8133824)
	if err != nil || storedNote == nil || storedNote.Content != "只保存在本机" {
		t.Fatalf("note after reopen = %+v, error = %v", storedNote, err)
	}
}

func TestLocalTagNameMustBeUnique(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "tags.db"))
	defer database.Close()
	if _, err := database.CreateLocalTag("重点", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateLocalTag("重点", ""); err == nil {
		t.Fatal("duplicate tag name unexpectedly succeeded")
	}
}
