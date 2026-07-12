package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestPersistentExportJobProducesDownloadCheckpoint(t *testing.T) {
	cfg := sqliteConfig(filepath.Join(t.TempDir(), "export.db"))
	repository, err := db.NewDatabase(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	if err := repository.UpsertPosts([]models.Post{{Pid: 8133824, Text: "export me"}}); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "data")
	application, err := Open(context.Background(), Options{Config: cfg, Repository: repository, Client: &client.Client{}, DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	job, err := application.Jobs.Create(context.Background(), jobs.CreateRequest{Type: jobs.TypeExportArchive, Payload: exportArchivePayload{Format: "treehole-v2", PIDs: []int32{8133824}, IncludeComments: true}, TotalItems: 1})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !job.Status.Terminal() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		job, err = application.Jobs.Get(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if job.Status != jobs.StatusCompleted {
		t.Fatalf("export job = %+v", job)
	}
	var checkpoint exportArchiveCheckpoint
	if err := json.Unmarshal(job.Checkpoint, &checkpoint); err != nil {
		t.Fatal(err)
	}
	if checkpoint.Report.Posts != 1 || checkpoint.Filename == "" || checkpoint.ExpiresAt.Before(time.Now()) {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
	info, err := os.Stat(filepath.Join(dataDir, "exports", checkpoint.Filename))
	if err != nil || info.Size() == 0 {
		t.Fatalf("export file = %+v, %v", info, err)
	}
}
