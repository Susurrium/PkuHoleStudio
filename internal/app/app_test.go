package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

type archiveStub struct{}

func (*archiveStub) Preflight(context.Context, io.ReaderAt, int64) (service.ArchivePreflight, error) {
	return service.ArchivePreflight{}, nil
}

func (*archiveStub) Import(context.Context, io.ReaderAt, int64) (service.ArchiveImportReport, error) {
	return service.ArchiveImportReport{}, nil
}

type aiStub struct{}

func (*aiStub) Run(context.Context, service.AIRequest) (<-chan service.AIEvent, error) {
	events := make(chan service.AIEvent)
	close(events)
	return events, nil
}

func (*aiStub) Cancel(string) error { return nil }

func TestOpenUsesInjectedDependencies(t *testing.T) {
	cfg := sqliteConfig(filepath.Join(t.TempDir(), "injected.db"))
	repository, err := db.NewDatabase(cfg)
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}
	defer repository.Close()

	treeholeClient := &client.Client{}
	archive := &archiveStub{}
	ai := &aiStub{}
	jobs := &struct{ name string }{name: "jobs"}
	dataDir := filepath.Join(t.TempDir(), "library")

	application, err := Open(context.Background(), Options{
		Config:     cfg,
		Repository: repository,
		Client:     treeholeClient,
		DataDir:    dataDir,
		Archive:    archive,
		AI:         ai,
		Jobs:       jobs,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if application.Config != cfg {
		t.Error("Open() did not retain the injected config")
	}
	if application.Repository != repository {
		t.Error("Open() did not retain the injected repository")
	}
	if application.Client != treeholeClient {
		t.Error("Open() did not retain the injected client")
	}
	if application.Archive != archive || application.AI != ai || application.Jobs != jobs {
		t.Error("Open() did not retain an injected application boundary")
	}
	if application.DataDir != dataDir {
		t.Errorf("DataDir = %q, want %q", application.DataDir, dataDir)
	}
	if application.Posts == nil || application.Search == nil || application.Sync == nil || application.Media == nil {
		t.Fatal("Open() did not compose every concrete service")
	}
	if got := application.Ownership(); got != (Ownership{}) {
		t.Errorf("Ownership() = %+v, want no ownership for injected dependencies", got)
	}

	if err := application.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := repository.GetPostCount(); err != nil {
		t.Fatalf("injected repository was closed by App.Close(): %v", err)
	}
}

func TestOpenRejectsCancelledContextBeforeInitialization(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "must-not-be-created.db")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	application, err := Open(ctx, Options{
		Config: sqliteConfig(databasePath),
		Client: &client.Client{},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() error = %v, want context.Canceled", err)
	}
	if application != nil {
		t.Fatal("Open() returned an application for a cancelled context")
	}
	if matches, statErr := filepath.Glob(databasePath + "*"); statErr != nil {
		t.Fatalf("checking database artifacts: %v", statErr)
	} else if len(matches) != 0 {
		t.Fatalf("cancelled Open() initialized repository artifacts: %v", matches)
	}
}

func TestOpenCleansOwnedRepositoryWhenInitializationIsCancelled(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "cancel-during-open.db")
	ctx := newCancelOnCheckContext(3)

	application, err := Open(ctx, Options{
		Config: sqliteConfig(databasePath),
		Client: &client.Client{},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() error = %v, want context.Canceled", err)
	}
	if application != nil {
		t.Fatal("Open() returned a partially initialized application")
	}

	// On Windows an open SQLite handle prevents removing the database. A
	// successful removal therefore also guards the rollback path against a
	// leaked connection pool.
	if err := os.Remove(databasePath); err != nil {
		t.Fatalf("owned repository was not closed after initialization failed: %v", err)
	}
}

func TestCloseIsIdempotentAndClosesOwnedRepository(t *testing.T) {
	cfg := sqliteConfig(filepath.Join(t.TempDir(), "owned.db"))
	application, err := Open(context.Background(), Options{
		Config: cfg,
		Client: &client.Client{},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if got, want := application.Ownership(), (Ownership{Repository: true}); got != want {
		t.Errorf("Ownership() = %+v, want %+v", got, want)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if _, err := application.Repository.GetPostCount(); err == nil {
		t.Fatal("owned repository remains usable after App.Close()")
	}
}

func TestOpenLoadsConfigAndOwnsCreatedDependencies(t *testing.T) {
	t.Chdir(t.TempDir())

	application, err := Open(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer application.Close()

	if application.Config == nil || application.Repository == nil || application.Client == nil {
		t.Fatal("Open() did not create its default dependencies")
	}
	if got, want := application.Ownership(), (Ownership{Config: true, Repository: true, Client: true}); got != want {
		t.Errorf("Ownership() = %+v, want %+v", got, want)
	}
	if application.DataDir != "data" {
		t.Errorf("DataDir = %q, want data", application.DataDir)
	}
}

func sqliteConfig(path string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.DeviceUUID = "test-device"
	cfg.Database.Type = "sqlite3"
	cfg.Database.DBFile = path
	return &cfg
}

// cancelOnCheckContext deterministically models cancellation between two
// synchronous construction steps. Open checks Err after each owned resource,
// so cancelAt=3 cancels immediately after the repository has been opened.
type cancelOnCheckContext struct {
	calls    int
	cancelAt int
	done     chan struct{}
}

func newCancelOnCheckContext(cancelAt int) *cancelOnCheckContext {
	return &cancelOnCheckContext{cancelAt: cancelAt, done: make(chan struct{})}
}

func (*cancelOnCheckContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelOnCheckContext) Done() <-chan struct{}     { return c.done }
func (*cancelOnCheckContext) Value(any) any               { return nil }

func (c *cancelOnCheckContext) Err() error {
	c.calls++
	if c.calls < c.cancelAt {
		return nil
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return context.Canceled
}
