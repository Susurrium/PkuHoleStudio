package server

import (
	"net"
	"net/url"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
	"github.com/Susurrium/PkuHoleStudio/server/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Posts         *service.PostService
	Search        *service.SearchService
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
	Repository    *db.Database
	DataDir       string
	Bridge        *BridgeManager
}

func Init(e *gin.Engine, dependencies Dependencies) {
	if dependencies.Bridge == nil && dependencies.Archive != nil && dependencies.Jobs != nil {
		dependencies.Bridge = NewBridgeManager(dependencies.DataDir, dependencies.Archive, dependencies.Jobs)
	}
	Cors(e)
	legacy := handles.Dependencies{Posts: dependencies.Posts, Search: dependencies.Search, Media: dependencies.Media}
	e.GET("/health", handles.Health)
	e.GET("/help", handles.Help)
	e.GET("/posts", handles.GetPosts(legacy))
	e.GET("/post/:pid", handles.GetPost(dependencies.Posts))
	e.GET("/comment", handles.GetComment(dependencies.Posts))
	e.GET("/comments/:pid", handles.GetComments(dependencies.Posts))
	e.GET("/media/image", handles.GetImage(dependencies.Media))
	registerAPIV1(e.Group("/api/v1"), dependencies)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	if config.Conf != nil {
		conf.AllowOrigins = config.Conf.Cors.AllowOrigins
		conf.AllowHeaders = config.Conf.Cors.AllowHeaders
		conf.AllowMethods = config.Conf.Cors.AllowMethods
	}
	allowLoopbackOnly := len(conf.AllowOrigins) == 0
	for _, origin := range conf.AllowOrigins {
		allowLoopbackOnly = allowLoopbackOnly || strings.TrimSpace(origin) == "*"
	}
	if allowLoopbackOnly {
		// The production SPA is same-origin and needs no CORS grant. Keep local
		// Vite/localhost development working without sharing the local archive
		// API with arbitrary public websites.
		conf.AllowOrigins = nil
		conf.AllowOriginFunc = isLoopbackBrowserOrigin
	}
	if len(conf.AllowHeaders) == 0 {
		conf.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "Last-Event-ID"}
	}
	if len(conf.AllowMethods) == 0 {
		conf.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	}
	corsMiddleware := cors.New(conf)
	e.Use(func(c *gin.Context) {
		// A userscript-manager request carries the protocol header on the real
		// request and is validated again by bridge handlers. Browser JavaScript
		// cannot reach this branch because its CORS preflight does not carry the
		// requested custom header and is rejected by the loopback-only policy.
		if strings.TrimSpace(c.GetHeader(toolkitBridgeProtocolHeader)) == "2" {
			c.Next()
			return
		}
		corsMiddleware(c)
	})
}

func isLoopbackBrowserOrigin(origin string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}
