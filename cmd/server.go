package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Susurrium/PkuHoleStudio/internal/app"
	"github.com/Susurrium/PkuHoleStudio/server"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var (
	serverPort string
	serverHost string
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the API server",
		Long:  `启动 PKU Hole API 服务器，提供 RESTful 接口。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	}

	cmd.Flags().StringVarP(&serverPort, "port", "p", "8081", "server port")
	cmd.Flags().StringVar(&serverHost, "host", "0.0.0.0", "server host")

	return cmd
}

func runServer() error {
	application, err := app.Open(context.Background(), app.Options{})
	if err != nil {
		return err
	}
	defer application.Close()

	r := newServerEngine(application)

	addr := fmt.Sprintf("%s:%s", serverHost, serverPort)
	log.Printf("Starting PKU Hole API server on %s...", addr)
	log.Printf("API endpoints:")
	log.Printf("  GET http://%s:%s/help", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/posts?begin=0&limit=25", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/post/:pid", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/comment?cid=123", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/comments/:pid?begin=0&limit=25&sort=0", serverHost, serverPort)
	log.Printf("  GET http://%s:%s/health", serverHost, serverPort)

	if err := r.Run(addr); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func newServerEngine(application *app.App) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	server.Init(r, server.Dependencies{
		Posts:         application.Posts,
		Search:        application.Search,
		Media:         application.Media,
		Dashboard:     application.Dashboard,
		Notifications: application.Notifications,
		Archive:       application.Archive,
		AI:            application.AI,
		Auth:          application.Auth,
		Jobs:          application.Jobs,
		Repository:    application.Repository,
		DataDir:       application.DataDir,
	})
	return r
}
