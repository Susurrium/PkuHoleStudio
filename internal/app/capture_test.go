package app

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	studioarchive "github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

type captureRemote struct {
	post     models.Post
	comments []models.Comment
}

func (r *captureRemote) ListPosts(context.Context, service.RemoteListQuery) ([]models.Post, int, error) {
	return []models.Post{r.post}, 1, nil
}
func (r *captureRemote) GetPost(context.Context, int32) (*models.Post, error) {
	copy := r.post
	return &copy, nil
}
func (r *captureRemote) ListComments(_ context.Context, _ int32, query service.RemoteCommentQuery) ([]models.Comment, int, error) {
	start := (query.Page - 1) * query.Limit
	if start >= len(r.comments) {
		return nil, len(r.comments), nil
	}
	end := min(start+query.Limit, len(r.comments))
	return append([]models.Comment(nil), r.comments[start:end]...), len(r.comments), nil
}
func (r *captureRemote) ListTags(context.Context) ([]models.Tag, error) { return nil, nil }
func (r *captureRemote) GetCourseTable(context.Context) ([]models.CourseScheduleRow, error) {
	return nil, nil
}
func (r *captureRemote) GetCourseScores(context.Context) (*models.ScoreSummary, error) {
	return nil, nil
}
func (r *captureRemote) RefreshPost(ctx context.Context, pid int32) (*models.Post, error) {
	return r.GetPost(ctx, pid)
}
func (r *captureRemote) TogglePraise(context.Context, int32) error    { return nil }
func (r *captureRemote) ToggleAttention(context.Context, int32) error { return nil }
func (r *captureRemote) UploadImage(context.Context, string) (string, error) {
	return "", nil
}
func (r *captureRemote) CreatePost(context.Context, service.CreatePostRequest) (*models.Post, error) {
	return nil, nil
}
func (r *captureRemote) CreateComment(context.Context, service.CreateCommentRequest) (*models.Comment, error) {
	return nil, nil
}
func (r *captureRemote) CanWrite(context.Context) (bool, error) { return false, nil }

type captureMediaRemote struct{}

type failingCaptureMedia struct{}

type interruptedCaptureRemote struct {
	captureRemote
	status   int
	requests int
}

func (r *interruptedCaptureRemote) RefreshPost(_ context.Context, pid int32) (*models.Post, error) {
	r.requests++
	if pid == 12345 {
		return nil, &client.RemoteAPIError{HTTPStatus: r.status, Message: "upstream failure"}
	}
	return &models.Post{Pid: pid, Text: "available"}, nil
}

func TestBatchCaptureStopsOnAuthenticationButContinuesPastMissingPost(t *testing.T) {
	for _, status := range []int{401, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			repo, err := db.NewDatabase(sqliteConfig(filepath.Join(t.TempDir(), "batch.db")))
			if err != nil {
				t.Fatal(err)
			}
			defer repo.Close()
			manager, err := jobs.NewManager(context.Background(), repo)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			remote := &interruptedCaptureRemote{status: status}
			posts := service.NewPostService(repo, remote)
			updates := service.NewPostUpdateService(posts, repo, nil, func() (string, error) { return "test-account", nil })
			a := &App{Repository: repo, Posts: posts, PostUpdates: updates, Jobs: manager}
			if err := registerJobHandlers(a); err != nil {
				t.Fatal(err)
			}
			job, err := manager.Create(context.Background(), jobs.CreateRequest{Type: jobs.TypeSyncPIDs, Payload: pidJobPayload{PIDs: []int32{12345, 23456}}})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for !job.Status.Terminal() && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
				job, err = manager.Get(context.Background(), job.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			if status == 401 && (remote.requests != 1 || job.Status != jobs.StatusFailed) {
				t.Fatalf("auth failure continued: requests=%d job=%+v", remote.requests, job)
			}
			if status == 404 && (remote.requests != 2 || job.Status != jobs.StatusPartial || job.CompletedItems != 1) {
				t.Fatalf("missing post prevented other updates: requests=%d job=%+v", remote.requests, job)
			}
		})
	}
}

func (failingCaptureMedia) DownloadImage(context.Context, string, int32) ([]byte, error) {
	return nil, fmt.Errorf("image download failed")
}
func (failingCaptureMedia) DownloadThumbnail(context.Context, string, int32) ([]byte, error) {
	return nil, fmt.Errorf("thumbnail download failed")
}

func TestCaptureReportsPartialWhenOnlyMediaFails(t *testing.T) {
	repo, err := db.NewDatabase(sqliteConfig(filepath.Join(t.TempDir(), "partial.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	manager, err := jobs.NewManager(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	remote := &captureRemote{post: models.Post{Pid: 12345, Text: "saved content", MediaIds: "image-1", Type: "image"}, comments: []models.Comment{{Pid: 12345, Cid: 1, Text: "saved comment"}}}
	posts := service.NewPostService(repo, remote)
	media := service.NewMediaServiceWithRepository(t.TempDir(), failingCaptureMedia{}, repo)
	updates := service.NewPostUpdateService(posts, repo, media, func() (string, error) { return "test-account", nil })
	a := &App{Repository: repo, Posts: posts, Media: media, PostUpdates: updates, Jobs: manager}
	if err := registerJobHandlers(a); err != nil {
		t.Fatal(err)
	}
	job, err := manager.Create(context.Background(), jobs.CreateRequest{Type: jobs.TypeSyncPIDs, Payload: pidJobPayload{PIDs: []int32{12345}, IncludeComments: true, IncludeMedia: true}})
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
	if job.Status != jobs.StatusPartial || job.Error == "" {
		t.Fatalf("media failure job=%+v", job)
	}
	post, err := repo.GetPostByPid(12345)
	if err != nil || post.Text != "saved content" {
		t.Fatalf("post=%+v %v", post, err)
	}
	comments, err := repo.GetCommentsByPidCursor(12345, 0, 20, true)
	if err != nil || len(comments) != 1 {
		t.Fatalf("comments=%v %v", comments, err)
	}
}

func (captureMediaRemote) DownloadImage(context.Context, string, int32) ([]byte, error) {
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}, nil
}
func (captureMediaRemote) DownloadThumbnail(context.Context, string, int32) ([]byte, error) {
	return nil, fmt.Errorf("thumbnail not expected")
}

func TestCapturePIDStoresEveryCommentPageAndMedia(t *testing.T) {
	repository, err := db.NewDatabase(sqliteConfig(filepath.Join(t.TempDir(), "capture.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	const pid int32 = 8328353
	comments := make([]models.Comment, 150)
	for index := range comments {
		comments[index] = models.Comment{Cid: int32(index + 1), Pid: pid, Text: fmt.Sprintf("comment %d", index+1)}
	}
	remote := &captureRemote{post: models.Post{Pid: pid, Text: "post with image", Type: "image", MediaIds: "image-1", Reply: int16(len(comments))}, comments: comments}
	dataDir := t.TempDir()
	application := &App{
		Repository: repository,
		Posts:      service.NewPostService(repository, remote),
		Media:      service.NewMediaServiceWithRepository(dataDir, captureMediaRemote{}, repository),
	}

	result, err := application.capturePID(context.Background(), pid, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Comments != 150 || result.CommentPages != 2 || result.Media != 1 || result.MissingMedia != 0 {
		t.Fatalf("capture result = %+v", result)
	}
	storedComments, err := repository.GetCommentsByPidCursor(pid, 0, 200, true)
	if err != nil || len(storedComments) != 150 {
		t.Fatalf("stored comments = %d, %v", len(storedComments), err)
	}
	media, err := repository.GetMediaByPID(pid)
	if err != nil || len(media) != 1 || media[0].Status != "available" || media[0].Path == "" {
		t.Fatalf("stored media = %+v, %v", media, err)
	}
	var output bytes.Buffer
	exporter := studioarchive.NewImporterWithDataDir(repository, dataDir)
	report, err := exporter.Export(context.Background(), &output, studioarchive.ExportRequest{Format: studioarchive.ExportFormatTreeholeV2, PIDs: []int32{pid}, IncludeComments: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Posts != 1 || report.Comments != 150 || report.Media != 1 || report.MissingMedia != 0 {
		t.Fatalf("export report = %+v", report)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	foundImage := false
	for _, file := range reader.File {
		if len(file.Name) > len("media/") && file.Name[:len("media/")] == "media/" && file.Name != "media/index.json" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatal("export archive did not contain the captured image")
	}
}
