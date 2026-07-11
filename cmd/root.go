package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/app"
	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
	"github.com/Susurrium/PkuHoleStudio/internal/tui"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

func init() {
	if err := config.EnsureRuntimeFiles(); err != nil {
		log.Printf("[Init] 初始化 data 目录失败: %v", err)
		return
	}
	logPath, err := config.LogPath()
	if err != nil {
		log.Printf("[Init] 解析日志路径失败: %v", err)
		return
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	}
}

var (
	dbPath          string
	startPage       int
	maxPages        int
	pageInterval    int
	loopInterval    int
	resume          bool
	loopPages       int
	saveJSON        bool
	postsPerReq     int
	commentsPerPost int
	fetchImages     bool
	convertWebp     bool
	tuiCaptureDir   string
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "treehole",
		Short: "PkuHoleStudio local archive and Treehole client",
		Long:  `PkuHoleStudio：兼容 PKUHoleTUI 的本地资料库、TUI、采集与 API 工具。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db-path", "./treehole.db", "database file path")
	rootCmd.PersistentFlags().StringVar(&tuiCaptureDir, "tui-capture-dir", "", "write TUI raw ANSI output and latest frame snapshots to this directory")

	serverCmd := newServerCmd()
	if serverCmd != nil {
		rootCmd.AddCommand(serverCmd)
	}
	rootCmd.AddCommand(newCrawlerCmd())

	return rootCmd
}

func initDB() (*db.Database, func(), error) {
	// 加载配置文件以获取数据库配置
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("加载配置文件失败: %w", err)
	}

	database, err := db.NewDatabase(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	cleanup := func() {
		if err := database.Checkpoint(); err != nil {
			log.Printf("[Database] checkpoint failed: %v", err)
		}
		database.Close()
	}

	return database, cleanup, nil
}

func runTUI() error {
	application, err := app.Open(context.Background(), app.Options{})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := application.Close(); closeErr != nil {
			log.Printf("[App] close failed: %v", closeErr)
		}
	}()

	session := tui.InitSessionForTUI(application.Client, application.Config)

	if tuiLogPath, err := config.TUILogPath(); err == nil {
		if tuiLogFile, err := os.OpenFile(tuiLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			application.Client.SetLogger(tuiLogFile)
		}
	}

	model := tui.NewModel(application.Posts, application.Sync, application.Client, application.Config, session)
	model.Images = tui.NewKittyImageRenderer()
	opts := []tea.ProgramOption{}

	capture, err := tui.NewCaptureSink(tuiCaptureDir)
	if err != nil {
		return fmt.Errorf("初始化TUI捕获失败: %w", err)
	}
	if capture != nil {
		defer capture.Close()
		model.Capture = capture
		output := capture.OutputWriter(os.Stdout)
		if model.Images != nil {
			output = model.Images.OutputWriter(output)
		}
		opts = append(opts, tea.WithOutput(output))
	} else if model.Images != nil {
		opts = append(opts, tea.WithOutput(model.Images.OutputWriter(os.Stdout)))
	}

	p := tea.NewProgram(model, opts...)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI运行错误: %w", err)
	}

	return nil
}

func authenticateClientForCrawler(c *client.Client, cfg *config.Config) error {
	if c == nil || cfg == nil {
		return fmt.Errorf("crawler application is not initialized")
	}
	result := c.BootstrapSession(cfg)
	if result.Status.CanReadOnline {
		return nil
	}

	switch result.Challenge {
	case client.AuthChallengeSMS:
		return fmt.Errorf("登录需要短信验证，crawler 不支持交互式短信验证")
	case client.AuthChallengeOTP:
		if !cfg.HasTOTPSecret() {
			return fmt.Errorf("登录需要令牌验证，但未配置 secret_key")
		}
		return fmt.Errorf("令牌验证未完成: %s", result.ChallengeReason)
	default:
		if !result.LoginAttempted && !cfg.HasPasswordLogin() {
			return fmt.Errorf("没有可用登录态，且未配置 username/password")
		}
		if result.Status.Message != "" {
			return fmt.Errorf("登录失败: %s", result.Status.Message)
		}
		return fmt.Errorf("登录失败")
	}
}

func runDaemon() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	application, err := app.Open(ctx, app.Options{})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := application.Close(); closeErr != nil {
			log.Printf("[App] close failed: %v", closeErr)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if err := authenticateClientForCrawler(application.Client, application.Config); err != nil {
		return fmt.Errorf("初始化客户端失败: %w", err)
	}
	syncService := application.Sync

	if resume {
		log.Printf("[Daemon] 断点续爬模式已启用，但未保存断点页码；将从配置的第 %d 页开始", startPage)
	}

	monitorMode := loopPages > 0
	if monitorMode {
		log.Printf("[Daemon] 监控模式启动: 循环抓取前 %d 页, 每轮间隔 %ds", loopPages, loopInterval)
	} else if maxPages == 0 {
		log.Printf("[Daemon] 无限抓取模式启动: 从第 %d 页开始, 每页间隔 %ds", startPage, pageInterval)
	} else {
		log.Printf("[Daemon] 一次性抓取模式启动: 从第 %d 页开始, 抓取 %d 页", startPage, maxPages)
	}

	page := startPage
	round := 0
	totalPosts := 0
	totalComments := 0

	for {
		select {
		case <-sigCh:
			log.Printf("[Daemon] 收到退出信号，正在优雅停止...")
			if saveJSON && syncService.RawResponseCount() > 0 {
				// 保存所有收集的原始响应到JSON文件
				if err := syncService.SaveRawResponses(context.Background()); err != nil {
					log.Printf("[Daemon] 保存原始响应到JSON文件失败: %v", err)
				} else {
					log.Printf("[Daemon] 原始响应已保存到JSON文件")
				}
			}
			return nil
		default:
		}

		if monitorMode {
			page = 1
		}

		round++
		log.Printf("[Daemon] 开始第 %d 轮抓取", round)

		crawled := 0
		limit := maxPages
		if monitorMode {
			limit = loopPages
		}

		for {
			select {
			case <-sigCh:
				log.Printf("[Daemon] 收到退出信号，正在优雅停止...")
				return nil
			default:
			}

			if limit > 0 && crawled >= limit {
				break
			}

			result, err := syncService.FetchPage(ctx, page, service.CrawlOptions{
				SaveJSON:     saveJSON,
				PostLimit:    postsPerReq,
				CommentLimit: commentsPerPost,
				FetchImages:  fetchImages,
				ConvertWebP:  convertWebp,
			})
			if err != nil {
				log.Printf("[Daemon] 第 %d 页抓取失败: %v", page, err)
				time.Sleep(time.Duration(pageInterval) * time.Second)
				page++
				continue
			}

			totalPosts += result.PostCount
			totalComments += result.CommentCount
			crawled++

			log.Printf("[Daemon] 第 %d 页完成: +%d帖子 +%d评论",
				page, result.PostCount, result.CommentCount)

			page++

			if limit > 0 && crawled >= limit {
				break
			}

			time.Sleep(time.Duration(pageInterval) * time.Second)
		}

		if !monitorMode && maxPages > 0 {
			log.Printf("[Daemon] 抓取完成! 共处理 %d 页, +%d帖子 +%d评论", crawled, totalPosts, totalComments)
			if saveJSON {
				// 保存所有收集的原始响应到JSON文件
				if err := syncService.SaveRawResponses(ctx); err != nil {
					log.Printf("[Daemon] 保存原始响应到JSON文件失败: %v", err)
				} else {
					log.Printf("[Daemon] 原始响应已保存到JSON文件")
				}
			}
			return nil
		}

		if monitorMode {
			log.Printf("[Daemon] 第 %d 轮完成, 等待 %ds 后开始下一轮...", round, loopInterval)
		}

		select {
		case <-sigCh:
			log.Printf("[Daemon] 收到退出信号，正在优雅停止...")
			if saveJSON && syncService.RawResponseCount() > 0 {
				// 保存所有收集的原始响应到JSON文件
				if err := syncService.SaveRawResponses(context.Background()); err != nil {
					log.Printf("[Daemon] 保存原始响应到JSON文件失败: %v", err)
				} else {
					log.Printf("[Daemon] 原始响应已保存到JSON文件")
				}
			}
			return nil
		case <-time.After(time.Duration(loopInterval) * time.Second):
		}
	}
}
