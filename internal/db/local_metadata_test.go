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

func TestResearchProjectsPersistVisiblePostMembership(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "projects.db"))
	defer database.Close()
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "first"}, {Pid: 8133825, Text: "second"}}); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateResearchProject("选课研究", "收集课程相关树洞", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetPostProjects(8133824, []uint{project.ID, project.ID}); err != nil {
		t.Fatal(err)
	}
	projects, err := database.ListResearchProjects()
	if err != nil || len(projects) != 1 || projects[0].PostCount != 1 {
		t.Fatalf("projects = %+v, %v", projects, err)
	}
	posts, err := database.GetResearchProjectPosts(project.ID)
	if err != nil || len(posts) != 1 || posts[0].Pid != 8133824 {
		t.Fatalf("project posts = %+v, %v", posts, err)
	}
	assigned, err := database.GetPostProjects(8133824)
	if err != nil || len(assigned) != 1 || assigned[0].ID != project.ID {
		t.Fatalf("post projects = %+v, %v", assigned, err)
	}
	if err := database.SetPostProjects(9999999, []uint{project.ID}); err == nil {
		t.Fatal("assigning a missing local post unexpectedly succeeded")
	}
	if err := database.DeleteResearchProject(project.ID); err != nil {
		t.Fatal(err)
	}
	assigned, err = database.GetPostProjects(8133824)
	if err != nil || len(assigned) != 0 {
		t.Fatalf("assignments after delete = %+v, %v", assigned, err)
	}
}

func TestBatchMetadataAssignmentsAppendWithoutReplacing(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "batch-metadata.db"))
	defer database.Close()
	if err := database.UpsertPosts([]models.Post{{Pid: 8133824, Text: "first"}, {Pid: 8133825, Text: "second"}}); err != nil {
		t.Fatal(err)
	}
	existingTag, err := database.CreateLocalTag("已有", "#64748b")
	if err != nil {
		t.Fatal(err)
	}
	addedTag, err := database.CreateLocalTag("批量添加", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetPostTags(8133824, []uint{existingTag.ID}); err != nil {
		t.Fatal(err)
	}
	if err := database.AddPostTags([]int32{8133824, 8133825}, []uint{addedTag.ID}); err != nil {
		t.Fatal(err)
	}
	firstTags, err := database.GetPostTags(8133824)
	if err != nil || len(firstTags) != 2 {
		t.Fatalf("first post tags = %+v, %v", firstTags, err)
	}
	secondTags, err := database.GetPostTags(8133825)
	if err != nil || len(secondTags) != 1 || secondTags[0].ID != addedTag.ID {
		t.Fatalf("second post tags = %+v, %v", secondTags, err)
	}

	existingProject, err := database.CreateResearchProject("已有项目", "", "#64748b")
	if err != nil {
		t.Fatal(err)
	}
	addedProject, err := database.CreateResearchProject("批量项目", "", "#0f766e")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetPostProjects(8133824, []uint{existingProject.ID}); err != nil {
		t.Fatal(err)
	}
	if err := database.AddPostsToProjects([]int32{8133824, 8133825}, []uint{addedProject.ID}); err != nil {
		t.Fatal(err)
	}
	firstProjects, err := database.GetPostProjects(8133824)
	if err != nil || len(firstProjects) != 2 {
		t.Fatalf("first post projects = %+v, %v", firstProjects, err)
	}
	secondProjects, err := database.GetPostProjects(8133825)
	if err != nil || len(secondProjects) != 1 || secondProjects[0].ID != addedProject.ID {
		t.Fatalf("second post projects = %+v, %v", secondProjects, err)
	}
	removed, err := database.RemovePostsFromProject([]int32{8133824, 8133825}, addedProject.ID)
	if err != nil || removed != 2 {
		t.Fatalf("removed project posts = %d, %v", removed, err)
	}
	firstProjects, err = database.GetPostProjects(8133824)
	if err != nil || len(firstProjects) != 1 || firstProjects[0].ID != existingProject.ID {
		t.Fatalf("existing project assignment after batch removal = %+v, %v", firstProjects, err)
	}
	if err := database.AddPostTags([]int32{8133824, 9999999}, []uint{addedTag.ID}); err == nil {
		t.Fatal("batch assignment with a missing local post unexpectedly succeeded")
	}
}
