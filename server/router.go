package server

import (
	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/server/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Init(e *gin.Engine, database *db.Database) {
	e.GET("/health", handles.Health)
	e.GET("/help", handles.Help)
	e.GET("/posts", handles.GetPosts(database))
	e.GET("/post/:pid", handles.GetPost(database))
	e.GET("/comment", handles.GetComment(database))
	e.GET("/comments/:pid", handles.GetComments(database))
	e.GET("/media/image", handles.GetImage)

	Cors(e)
}

func Cors(e *gin.Engine) {
	conf := cors.DefaultConfig()
	conf.AllowOrigins = config.Conf.Cors.AllowOrigins
	conf.AllowHeaders = config.Conf.Cors.AllowHeaders
	conf.AllowMethods = config.Conf.Cors.AllowMethods
	e.Use(cors.New(conf))
}
