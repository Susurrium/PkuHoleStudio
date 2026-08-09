package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

func TestRepairCommentsJobCapturesEveryPageAndReportsTotals(t *testing.T) {
	repository, err := db.NewDatabase(sqliteConfig(filepath.Join(t.TempDir(), "repair-comments.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	manager, err := jobs.NewManager(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	const pid int32 = 8328353
	comments := make([]models.Comment, 150)
	for index := range comments {
		comments[index] = models.Comment{Cid: int32(index + 1), Pid: pid, Text: fmt.Sprintf("comment %d", index+1)}
	}
	application := &App{
		Repository: repository,
		Posts: service.NewPostService(repository, &captureRemote{
			post:     models.Post{Pid: pid, Text: "post", Reply: int16(len(comments))},
			comments: comments,
		}),
	}
	if err := manager.Register(jobs.TypeRepairComments, application.handleRepairComments); err != nil {
		t.Fatal(err)
	}

	job, err := manager.Create(context.Background(), jobs.CreateRequest{
		Type:    jobs.TypeRepairComments,
		Payload: pidJobPayload{PIDs: []int32{pid}},
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !job.Status.Terminal() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		job, err = manager.Get(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if job.Status != jobs.StatusCompleted || job.CompletedItems != 1 || job.FailedItems != 0 {
		t.Fatalf("repair comments job = %+v", job)
	}

	items, err := repository.ListItems(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != jobs.ItemCompleted {
		t.Fatalf("repair comments items = %+v", items)
	}
	var checkpoint struct {
		PID          int32 `json:"pid"`
		Comments     int   `json:"comments"`
		CommentPages int   `json:"comment_pages"`
	}
	if err := json.Unmarshal(items[0].Checkpoint, &checkpoint); err != nil {
		t.Fatal(err)
	}
	if checkpoint.PID != pid || checkpoint.Comments != 150 || checkpoint.CommentPages != 2 {
		t.Fatalf("repair comments checkpoint = %+v", checkpoint)
	}

	stored, err := repository.GetCommentsByPidCursor(pid, 0, 200, true)
	if err != nil || len(stored) != 150 {
		t.Fatalf("stored comments = %d, %v", len(stored), err)
	}
}
