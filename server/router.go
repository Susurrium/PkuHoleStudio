package server

import (
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
	if len(conf.AllowOrigins) == 0 {
		conf.AllowOrigins = []string{"*"}
	}
	if len(conf.AllowHeaders) == 0 {
		conf.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "Last-Event-ID"}
	}
	if len(conf.AllowMethods) == 0 {
		conf.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	}
	e.Use(cors.New(conf))
}
