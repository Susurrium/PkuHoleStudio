package server

import (
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/server/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Dependencies = handles.Dependencies

func Init(e *gin.Engine, dependencies Dependencies) {
	e.GET("/health", handles.Health)
	e.GET("/help", handles.Help)
	e.GET("/posts", handles.GetPosts(dependencies))
	e.GET("/post/:pid", handles.GetPost(dependencies.Posts))
	e.GET("/comment", handles.GetComment(dependencies.Posts))
	e.GET("/comments/:pid", handles.GetComments(dependencies.Posts))
	e.GET("/media/image", handles.GetImage(dependencies.Media))

	Cors(e)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}
