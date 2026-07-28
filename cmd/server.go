package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/app"
	"github.com/Susurrium/PkuHoleStudio/server"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var (
	serverPort        string
	serverHost        string
	serverAllowRemote bool
	serverAPIToken    string
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
	cmd.Flags().StringVar(&serverHost, "host", "127.0.0.1", "server host")
	cmd.Flags().BoolVar(&serverAllowRemote, "allow-remote", false, "allow listening on a non-loopback host (requires an API token)")
	cmd.Flags().StringVar(&serverAPIToken, "api-token", "", "Bearer token for remote API access (prefer PKUHOLE_SERVER_API_TOKEN)")

	return cmd
}

func runServer() error {
	listenHost := strings.Trim(strings.TrimSpace(serverHost), "[]")
	remote := !isLoopbackServerHost(listenHost)
	token := strings.TrimSpace(serverAPIToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("PKUHOLE_SERVER_API_TOKEN"))
	}
	if remote && !serverAllowRemote {
		return fmt.Errorf("refusing to listen on non-loopback host %q without --allow-remote", serverHost)
	}
	if remote && len(token) < 24 {
		return fmt.Errorf("remote API access requires PKUHOLE_SERVER_API_TOKEN or --api-token with at least 24 characters")
	}

	application, err := openApplication(context.Background())
	if err != nil {
		return err
	}
	defer application.Close()

	r := newServerEngine(application, token)

	addr := net.JoinHostPort(listenHost, serverPort)
	log.Printf("Starting PKU Hole API server on %s...", addr)
	if remote {
		log.Printf("Remote API access is enabled and requires Authorization: Bearer <token>")
	}
	log.Printf("API endpoints:")
	log.Printf("  GET http://%s/help", addr)
	log.Printf("  GET http://%s/posts?begin=0&limit=25", addr)
	log.Printf("  GET http://%s/post/:pid", addr)
	log.Printf("  GET http://%s/comment?cid=123", addr)
	log.Printf("  GET http://%s/comments/:pid?begin=0&limit=25&sort=0", addr)
	log.Printf("  GET http://%s/health", addr)

	if err := r.Run(addr); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func newServerEngine(application *app.App, token string) *gin.Engine {
	r := newBaseServerEngine()
	if token != "" {
		r.Use(serverBearerAuth(token))
	}
	server.Init(r, serverDependencies(application))
	return r
}

func newWebServerEngine(application *app.App) (*gin.Engine, error) {
	r := newBaseServerEngine()
	if err := server.AttachSPA(r); err != nil {
		return nil, err
	}
	server.Init(r, serverDependencies(application))
	return r, nil
}

func newBaseServerEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

func isLoopbackServerHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serverBearerAuth(token string) gin.HandlerFunc {
	expected := []byte(token)
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		provided, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(provided)), expected) != 1 {
			c.Header("WWW-Authenticate", `Bearer realm="PkuHoleStudio API"`)
			c.AbortWithStatusJSON(401, gin.H{"error": gin.H{"code": "unauthorized", "message": "a valid API bearer token is required"}})
			return
		}
		c.Next()
	}
}

func serverDependencies(application *app.App) server.Dependencies {
	return server.Dependencies{
		Posts:         application.Posts,
		Search:        application.Search,
		Media:         application.Media,
		Dashboard:     application.Dashboard,
		Observer:      application.Observer,
		Notifications: application.Notifications,
		Logs:          application.Logs,
		Library:       application.Library,
		Settings:      application.Settings,
		Archive:       application.Archive,
		AI:            application.AI,
		Auth:          application.Auth,
		Jobs:          application.Jobs,
		Repository:    application.Repository,
		DataDir:       application.DataDir,
	}
}
