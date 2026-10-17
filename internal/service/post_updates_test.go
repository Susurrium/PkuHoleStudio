package service_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

type updateRemote struct {
	service.Remote
	post     models.Post
	comments []models.Comment
	failPage int
	mu       sync.Mutex
	requests int
	block    chan struct{}
}

func (r *updateRemote) RefreshPost(ctx context.Context, pid int32) (*models.Post, error) {
	r.mu.Lock()
	r.requests++
	r.mu.Unlock()
	if r.block != nil {
		select {
		case <-r.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	post := r.post
	post.Pid = pid
	return &post, nil
}
func (r *updateRemote) ListComments(ctx context.Context, pid int32, q service.RemoteCommentQuery) ([]models.Comment, int, error) {
	r.mu.Lock()
	r.requests++
	r.mu.Unlock()
	if q.Page == r.failPage {
		return nil, 0, errors.New("comment connection interrupted")
	}
	items := slices.Clone(r.comments)
	if q.Sort == 1 {
		slices.Reverse(items)
	}
	start := (q.Page - 1) * q.Limit
	if start >= len(items) {
		return []models.Comment{}, len(items), nil
	}
	return items[start:min(start+q.Limit, len(items))], len(items), nil
}
func updateFixture(t *testing.T, count int) (*db.Database, *updateRemote, *service.PostUpdateService) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Database.Type = "sqlite3"
	cfg.Database.DBFile = filepath.Join(t.TempDir(), "updates.db")
	repo, err := db.NewDatabase(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	remote := &updateRemote{post: models.Post{Pid: 12345, Text: "new text", Likenum: 3, Reply: int16(count), IsFollow: true}}
	for i := 1; i <= count; i++ {
		remote.comments = append(remote.comments, models.Comment{Pid: 12345, Cid: int32(i), Text: fmt.Sprintf("comment %d", i)})
	}
	updates := service.NewPostUpdateService(service.NewPostService(repo, remote), repo, service.NewMediaServiceWithRepository(t.TempDir(), nil, repo), func() (string, error) { return "account-a", nil })
	return repo, remote, updates
}

func TestRefreshSavedCapturesAllNewPagesAndPreservesLocalMetadata(t *testing.T) {
	ctx := context.Background()
	repo, _, updates := updateFixture(t, 260)
	if err := repo.SaveCrawlResultWithSource([]models.Post{{Pid: 12345, Text: "old", Likenum: 20}}, []models.Comment{{Pid: 12345, Cid: 1, Text: "old comment"}}, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	tag, err := repo.CreateLocalTag("标签", "red")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetPostTags(12345, []uint{tag.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertNote("post", 12345, "个人笔记"); err != nil {
		t.Fatal(err)
	}
	result, err := updates.RefreshSaved(ctx, 12345, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommentPages != 3 {
		t.Fatalf("pages=%d", result.CommentPages)
	}
	items, err := repo.GetCommentsByPidCursor(12345, 0, 500, true)
	if err != nil || len(items) != 260 {
		t.Fatalf("comments=%d %v", len(items), err)
	}
	post, err := repo.GetPostByPid(12345)
	if err != nil || post.Text != "new text" || post.Likenum != 3 {
		t.Fatalf("post=%+v %v", post, err)
	}
	note, err := repo.GetNote("post", 12345)
	if err != nil || note.Content != "个人笔记" {
		t.Fatalf("note=%+v %v", note, err)
	}
	tags, err := repo.GetPostTags(12345)
	if err != nil || len(tags) != 1 {
		t.Fatalf("tags=%v %v", tags, err)
	}
	if _, err := updates.RefreshSaved(ctx, 12345, true); err != nil {
		t.Fatal(err)
	}
	items, _ = repo.GetCommentsByPidCursor(12345, 0, 500, true)
	if len(items) != 260 {
		t.Fatal("repeated refresh duplicated comments")
	}
}

func TestRefreshFailureDoesNotPublishPartialNewComments(t *testing.T) {
	ctx := context.Background()
	repo, remote, updates := updateFixture(t, 260)
	if err := repo.SaveCrawlResultWithSource([]models.Post{{Pid: 12345, Text: "old"}}, []models.Comment{{Pid: 12345, Cid: 1}}, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	remote.failPage = 2
	if _, err := updates.RefreshSaved(ctx, 12345, false); err == nil {
		t.Fatal("expected interrupted refresh")
	}
	items, _ := repo.GetCommentsByPidCursor(12345, 0, 500, true)
	post, _ := repo.GetPostByPid(12345)
	if len(items) != 1 || post.Text != "old" {
		t.Fatalf("published incomplete update: comments=%d text=%s", len(items), post.Text)
	}
	remote.failPage = 0
	if _, err := updates.RefreshSaved(ctx, 12345, false); err != nil {
		t.Fatal(err)
	}
	items, _ = repo.GetCommentsByPidCursor(12345, 0, 500, true)
	if len(items) != 260 {
		t.Fatalf("retry missed comments: %d", len(items))
	}
}

func TestWatchLimitsDurabilityAndUnfollow(t *testing.T) {
	ctx := context.Background()
	repo, remote, updates := updateFixture(t, 150)
	if err := repo.SaveCrawlResultWithSource([]models.Post{{Pid: 12345}}, nil, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := updates.Configure(ctx, service.PostUpdateSettings{Enabled: true, IntervalMinutes: 1, PIDs: []int32{12345}}); err == nil {
		t.Fatal("accepted too short interval")
	}
	if _, err := updates.Configure(ctx, service.PostUpdateSettings{IntervalMinutes: 60, PIDs: make([]int32, 21)}); err == nil {
		t.Fatal("accepted too many targets")
	}
	if _, err := updates.Configure(ctx, service.PostUpdateSettings{Enabled: true, IntervalMinutes: 30, PIDs: []int32{12345}}); err != nil {
		t.Fatal(err)
	}
	before, err := updates.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := updates.RunDue(ctx, before.NextRun.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	remote.mu.Lock()
	requests := remote.requests
	remote.mu.Unlock()
	if requests != 1 {
		t.Fatalf("ran early: %d", requests)
	}
	if err := updates.RunDue(ctx, before.NextRun); err != nil {
		t.Fatal(err)
	}
	items, _ := repo.GetCommentsByPidCursor(12345, 0, 200, true)
	if len(items) != 150 {
		t.Fatalf("background comments=%d", len(items))
	}
	restarted := service.NewPostUpdateService(service.NewPostService(repo, remote), repo, nil, func() (string, error) { return "account-a", nil })
	state, err := restarted.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !state.NextRun.After(before.NextRun) || !state.Enabled {
		t.Fatalf("schedule lost: %+v", state)
	}
	if _, err := restarted.Configure(ctx, service.PostUpdateSettings{Enabled: false, IntervalMinutes: 30, PIDs: []int32{12345}}); err != nil {
		t.Fatal(err)
	}
	after, err := restarted.Configure(ctx, service.PostUpdateSettings{Enabled: true, IntervalMinutes: 30, PIDs: []int32{12345}})
	if err != nil {
		t.Fatal(err)
	}
	if !after.NextRun.Equal(state.NextRun) {
		t.Fatal("toggle reset next run")
	}
	remote.post.IsFollow = false
	if err := restarted.RunDue(ctx, after.NextRun); err != nil {
		t.Fatal(err)
	}
	state, _ = restarted.Status(ctx)
	if state.Enabled || len(state.PIDs) != 0 {
		t.Fatalf("unfollowed target retained: %+v", state)
	}
	if post, _ := repo.GetPostByPid(12345); post == nil {
		t.Fatal("unfollow removed local content")
	}
}

func TestConcurrentRefreshCoalescesAndAccountSwitchPreventsCommit(t *testing.T) {
	ctx := context.Background()
	repo, remote, _ := updateFixture(t, 1)
	if err := repo.SaveCrawlResultWithSource([]models.Post{{Pid: 12345, Text: "old"}}, nil, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	var accountMu sync.Mutex
	account := "a"
	updates := service.NewPostUpdateService(service.NewPostService(repo, remote), repo, nil, func() (string, error) { accountMu.Lock(); defer accountMu.Unlock(); return account, nil })
	remote.block = make(chan struct{})
	results := make(chan error, 2)
	go func() { _, err := updates.RefreshSaved(ctx, 12345, false); results <- err }()
	for {
		remote.mu.Lock()
		n := remote.requests
		remote.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	go func() { _, err := updates.RefreshSaved(ctx, 12345, false); results <- err }()
	time.Sleep(20 * time.Millisecond)
	accountMu.Lock()
	account = "b"
	accountMu.Unlock()
	close(remote.block)
	if err := <-results; err == nil {
		t.Fatal("account change was accepted")
	}
	<-results
	post, _ := repo.GetPostByPid(12345)
	if post.Text != "old" {
		t.Fatal("account change published content")
	}
}

func TestBackgroundPaginationSurvivesBudgetAndRestart(t *testing.T) {
	ctx := context.Background()
	repo, remote, updates := updateFixture(t, 10450)
	if err := repo.SaveCrawlResultWithSource([]models.Post{{Pid: 12345}}, []models.Comment{{Pid: 12345, Cid: 1}}, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	state, err := updates.Configure(ctx, service.PostUpdateSettings{Enabled: true, IntervalMinutes: 30, PIDs: []int32{12345}})
	if err != nil {
		t.Fatal(err)
	}
	remote.mu.Lock()
	before := remote.requests
	remote.mu.Unlock()
	if err := updates.RunDue(ctx, state.NextRun); err != nil {
		t.Fatal(err)
	}
	remote.mu.Lock()
	used := remote.requests - before
	remote.mu.Unlock()
	if used > 100 {
		t.Fatalf("round used %d requests", used)
	}
	state, err = updates.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Targets) != 1 || state.Targets[0].Page < 2 {
		t.Fatalf("continuation lost: %+v", state)
	}
	restarted := service.NewPostUpdateService(service.NewPostService(repo, remote), repo, nil, func() (string, error) { return "account-a", nil })
	if err := restarted.RunDue(ctx, state.NextRun); err != nil {
		t.Fatal(err)
	}
	count, err := repo.GetCommentCount()
	if err != nil || count != 10450 {
		t.Fatalf("continued comments=%d %v", count, err)
	}
	state, _ = restarted.Status(ctx)
	if state.Targets[0].Page != 0 {
		t.Fatalf("continuation did not finish: %+v", state.Targets[0])
	}
}

func TestSnapshotAndQueuedAccountAreProtected(t *testing.T) {
	ctx := context.Background()
	repo, remote, updates := updateFixture(t, 1)
	if err := repo.SaveObserverBrowseRecord(ctx, models.Post{Pid: 12345, Text: "historical"}, nil, models.PostAvailability{PID: 12345, State: "confirmed_unavailable", ObservedAt: time.Now()}, "observer"); err != nil {
		t.Fatal(err)
	}
	if _, err := updates.RefreshSaved(ctx, 12345, false); err == nil {
		t.Fatal("snapshot automatically refreshed")
	}
	if _, err := updates.Capture(ctx, 12345, true, false); err == nil {
		t.Fatal("snapshot overwritten by full update")
	}
	remote.mu.Lock()
	calls := remote.requests
	remote.mu.Unlock()
	if calls != 0 {
		t.Fatal("snapshot reached upstream")
	}
}

func TestDisablingCancelsAnActiveRoundAndKeepsItsNextRun(t *testing.T) {
	ctx := context.Background()
	repo, remote, updates := updateFixture(t, 1)
	if err := repo.SaveCrawlResultWithSource([]models.Post{{Pid: 12345}}, nil, "explicit", "test"); err != nil {
		t.Fatal(err)
	}
	state, err := updates.Configure(ctx, service.PostUpdateSettings{Enabled: true, IntervalMinutes: 30, PIDs: []int32{12345}})
	if err != nil {
		t.Fatal(err)
	}
	remote.block = make(chan struct{})
	finished := make(chan error, 1)
	go func() { finished <- updates.RunDue(ctx, state.NextRun) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		remote.mu.Lock()
		count := remote.requests
		remote.mu.Unlock()
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("round did not start")
		}
		time.Sleep(time.Millisecond)
	}
	changed, err := updates.Configure(ctx, service.PostUpdateSettings{Enabled: false, IntervalMinutes: 30, PIDs: []int32{12345}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("disable did not cancel round")
	}
	if changed.Enabled || changed.Running || !changed.NextRun.After(state.NextRun) {
		t.Fatalf("disabled state=%+v", changed)
	}
}
