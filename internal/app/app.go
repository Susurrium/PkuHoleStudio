// Package app contains the application composition root shared by the TUI,
// crawler commands, HTTP server, and future background workers.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	aipkg "github.com/Susurrium/PkuHoleStudio/internal/ai"
	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

// Options supplies existing process-wide dependencies when an application is
// embedded in a command or test. Dependencies omitted here are created by
// Open and owned by the returned App.
type Options struct {
	Config     *config.Config
	Repository *db.Database
	Client     *client.Client
	DataDir    string
	Archive    service.ArchiveService
	AI         service.AIService
	Auth       service.AuthService
	Jobs       *jobs.Manager
}

// Ownership reports which resources were created by Open. Config and Client
// currently have no shutdown operation, but recording their ownership keeps
// lifecycle decisions explicit as those packages evolve.
type Ownership struct {
	Config     bool
	Repository bool
	Client     bool
	Jobs       bool
	AI         bool
}

// App is the common composition root for every user interface. Higher layers
// should depend on these services instead of assembling database, crawler, and
// online-client dependencies themselves.
type App struct {
	Config     *config.Config
	Repository *db.Database
	Client     *client.Client

	Posts         *service.PostService
	Search        *service.SearchService
	Sync          *service.SyncService
	Media         *service.MediaService
	Dashboard     *service.DashboardService
	Notifications *service.NotificationService
	Logs          *service.LogService
	Library       *service.LocalLibraryService
	Settings      *service.SettingsService
	Archive       service.ArchiveService
	AI            service.AIService
	Auth          service.AuthService
	Jobs          *jobs.Manager

	DataDir string

	ownership Ownership
	closeOnce sync.Once
	closeErr  error
}

// Open loads configuration, opens the repository, creates the Treehole client,
// and wires all application services. It only constructs local dependencies;
// creating the client does not perform a network login.
func Open(ctx context.Context, options Options) (_ *App, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	application := &App{
		Archive: options.Archive,
		AI:      options.AI,
		Auth:    options.Auth,
		Jobs:    options.Jobs,
		DataDir: normalizedDataDir(options.DataDir),
	}

	// Any future fallible constructor added below automatically participates in
	// rollback. Close is safe even when only part of App has been initialized.
	completed := false
	defer func() {
		if completed {
			return
		}
		if closeErr := application.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up application: %w", closeErr))
		}
	}()

	if options.Config != nil {
		application.Config = options.Config
	} else {
		application.Config, err = config.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		application.ownership.Config = true
	}
	if application.Config == nil {
		return nil, errors.New("load config: returned nil config")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if options.Repository != nil {
		application.Repository = options.Repository
	} else {
		application.Repository, err = db.NewDatabase(application.Config)
		if err != nil {
			return nil, fmt.Errorf("open repository: %w", err)
		}
		application.ownership.Repository = true
	}
	if application.Repository == nil {
		return nil, errors.New("open repository: returned nil repository")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if options.Client != nil {
		application.Client = options.Client
	} else {
		application.Client, err = client.NewClient(application.Config.DeviceUUID)
		if err != nil {
			return nil, fmt.Errorf("create treehole client: %w", err)
		}
		application.ownership.Client = true
	}
	if application.Client == nil {
		return nil, errors.New("create treehole client: returned nil client")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	remote := service.NewTreeholeRemote(application.Client)
	application.Posts = service.NewPostService(application.Repository, remote)
	application.Search = service.NewSearchService(application.Posts, application.Repository)
	application.Sync = service.NewSyncService(application.Client, application.Repository)
	application.Media = service.NewMediaServiceWithRepository(
		application.DataDir,
		service.NewTreeholeMediaRemote(application.Client),
		application.Repository,
	)
	application.Dashboard = service.NewDashboardService()
	application.Notifications = service.NewNotificationService(application.Client)
	application.Logs = service.NewLogService(application.DataDir)
	application.Library = service.NewLocalLibraryService(application.Repository)
	application.Settings = service.NewSettingsService(application.Config)
	if application.Auth == nil {
		application.Auth = service.NewAuthService(application.Client, application.Config)
	}
	if application.Archive == nil {
		application.Archive = archive.NewImporterWithDataDir(application.Repository, application.DataDir)
	}
	if application.AI == nil {
		aiConfig := application.Config.AI
		defaults := config.DefaultConfig().AI
		if aiConfig.MaxSearchRounds <= 0 {
			aiConfig.MaxSearchRounds = defaults.MaxSearchRounds
		}
		providerConfig := aiConfig.Provider
		if providerConfig.Name == "" {
			providerConfig.Name = defaults.Provider.Name
		}
		if providerConfig.BaseURL == "" {
			providerConfig.BaseURL = defaults.Provider.BaseURL
		}
		if providerConfig.Model == "" {
			providerConfig.Model = defaults.Provider.Model
		}
		if providerConfig.MaxOutputTokens <= 0 {
			providerConfig.MaxOutputTokens = defaults.Provider.MaxOutputTokens
		}
		if providerConfig.RequestTimeout <= 0 {
			providerConfig.RequestTimeout = defaults.Provider.RequestTimeout
		}
		aiConfig.Provider = providerConfig
		if apiKey := strings.TrimSpace(os.Getenv("PKUHOLE_AI_API_KEY")); apiKey != "" {
			providerConfig.APIKey = apiKey
			aiConfig.Provider.APIKey = apiKey
		}
		provider, providerErr := aipkg.NewOpenAIProvider(providerConfig)
		if providerErr != nil {
			return nil, fmt.Errorf("create AI provider: %w", providerErr)
		}
		info := provider.Info()
		info.Configured = aiConfig.Enabled && providerConfig.APIKey != ""
		application.AI = aipkg.NewService(ctx, application.Repository, application.Posts, application.Search, provider, aiConfig, info)
		application.ownership.AI = true
	}
	if options.Jobs != nil {
		application.Jobs = options.Jobs
	} else {
		application.Jobs, err = jobs.NewManager(ctx, application.Repository)
		if err != nil {
			return nil, fmt.Errorf("create job manager: %w", err)
		}
		application.ownership.Jobs = true
	}
	if err := registerJobHandlers(application); err != nil {
		return nil, err
	}
	if err := cleanupExpiredImportStaging(ctx, application.DataDir, application.Jobs, 7*24*time.Hour); err != nil {
		return nil, fmt.Errorf("clean import staging: %w", err)
	}
	if err := cleanupExpiredExports(ctx, application.DataDir, 30*24*time.Hour); err != nil {
		application.Close()
		return nil, fmt.Errorf("cleanup expired exports: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	completed = true
	return application, nil
}

// Ownership returns a snapshot of the application's dependency ownership.
func (a *App) Ownership() Ownership {
	if a == nil {
		return Ownership{}
	}
	return a.ownership
}

// Close releases resources created by Open. Injected repositories remain the
// caller's responsibility. Close is safe to call repeatedly and concurrently.
func (a *App) Close() error {
	if a == nil {
		return nil
	}

	a.closeOnce.Do(func() {
		var aiErr error
		if closer, ok := a.AI.(interface{ Close() error }); a.ownership.AI && ok {
			if err := closer.Close(); err != nil {
				aiErr = fmt.Errorf("close AI service: %w", err)
			}
		}
		var jobErr error
		if a.ownership.Jobs && a.Jobs != nil {
			if err := a.Jobs.Close(); err != nil {
				jobErr = fmt.Errorf("close job manager: %w", err)
			}
		}
		var checkpointErr, closeErr error
		if a.ownership.Repository && a.Repository != nil {
			// Always attempt Close, including when a checkpoint fails, so an
			// error cannot leave the owned connection pool running.
			checkpointErr = a.Repository.Checkpoint()
			closeErr = a.Repository.Close()
			if checkpointErr != nil {
				checkpointErr = fmt.Errorf("checkpoint repository: %w", checkpointErr)
			}
			if closeErr != nil {
				closeErr = fmt.Errorf("close repository: %w", closeErr)
			}
		}
		a.closeErr = errors.Join(aiErr, jobErr, checkpointErr, closeErr)
	})
	return a.closeErr
}

func normalizedDataDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return "data"
	}
	return dataDir
}
