package main

import (
	"context"

	"github.com/Susurrium/PkuHoleStudio/internal/app"

	"github.com/spf13/cobra"
)

func newCrawlerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawler",
		Short: "Run the crawler",
		Long:  `启动 PKU Hole 爬虫，支持一次性抓取、无限抓取和监控模式。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
	}

	cmd.Flags().IntVar(&startPage, "start-page", 1, "starting page number")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "max pages to crawl per round (0 = infinite)")
	cmd.Flags().IntVar(&pageInterval, "page-interval", 1, "seconds between each page crawl")
	cmd.Flags().IntVar(&loopInterval, "loop-interval", 60, "seconds between rounds in loop mode")
	cmd.Flags().IntVarP(&loopPages, "loop-pages", "k", 0, "loop mode: repeatedly crawl first N pages")
	cmd.Flags().BoolVar(&saveJSON, "save-json", false, "save raw API responses to JSON files for analysis")
	cmd.Flags().IntVar(&postsPerReq, "posts-per-request", 200, "max posts per API request")
	cmd.Flags().IntVar(&commentsPerPost, "comments-per-post", 200, "max comments per post in API request")
	cmd.Flags().BoolVar(&fetchImages, "fetch-images", false, "download images into the selected --data-dir/images cache")
	cmd.Flags().BoolVar(&convertWebp, "convert-webp", true, "convert downloaded images to WebP format")

	cmd.AddCommand(newFetchImagesCmd())
	cmd.AddCommand(newFetchThumbnailsCmd())

	return cmd
}

func newFetchImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch-images",
		Short: "Download images for legacy local records",
		Long:  `兼容旧版资料：从数据库查找图片并保存到当前 --data-dir 下的 images 目录。日常使用请优先在 Web 中运行“补全缺失图片”。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			application, err := openCrawlerApplication(cmd.Context())
			if err != nil {
				return err
			}
			defer application.Close()

			result, err := application.Sync.FetchImages(cmd.Context(), convertWebp)
			cmd.Printf("images: downloaded=%d existing=%d failed=%d\n", result.Downloaded, result.Existing, result.Failed)
			return err
		},
	}

	cmd.Flags().BoolVar(&convertWebp, "convert-webp", true, "convert downloaded images to WebP format")

	return cmd
}

func newFetchThumbnailsCmd() *cobra.Command {
	var startID int
	var endID int
	var convertThumbWebp bool

	cmd := &cobra.Command{
		Use:   "fetch-thumbnails",
		Short: "Download legacy thumbnails by media ID range",
		Long:  `兼容旧版 TUI 缓存：按 media id 区间下载缩略图到当前 --data-dir 下的 thumbnails 目录。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			application, err := openCrawlerApplication(cmd.Context())
			if err != nil {
				return err
			}
			defer application.Close()

			result, err := application.Sync.FetchThumbnails(cmd.Context(), startID, endID, convertThumbWebp)
			cmd.Printf("thumbnails: downloaded=%d existing=%d failed=%d range=%d-%d\n", result.Downloaded, result.Existing, result.Failed, startID, endID)
			return err
		},
	}

	cmd.Flags().IntVar(&startID, "start-id", 0, "starting media id (inclusive)")
	cmd.Flags().IntVar(&endID, "end-id", 0, "ending media id (inclusive)")
	cmd.Flags().BoolVar(&convertThumbWebp, "convert-webp", false, "convert downloaded thumbnails to WebP format")
	_ = cmd.MarkFlagRequired("start-id")
	_ = cmd.MarkFlagRequired("end-id")

	return cmd
}

func openCrawlerApplication(ctx context.Context) (*app.App, error) {
	application, err := openApplication(ctx)
	if err != nil {
		return nil, err
	}
	if err := authenticateClientForCrawler(application.Client, application.Config); err != nil {
		_ = application.Close()
		return nil, err
	}
	return application, nil
}
