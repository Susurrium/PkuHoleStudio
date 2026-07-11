package handles

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
	"github.com/Susurrium/PkuHoleStudio/server/utils"

	"github.com/gin-gonic/gin"
)

// Dependencies contains the application services used by the legacy HTTP
// handlers. Keeping the old routes behind this boundary preserves compatibility
// while preventing handlers from reaching into GORM directly.
type Dependencies struct {
	Posts  *service.PostService
	Search *service.SearchService
	Media  *service.MediaService
}

func Health(c *gin.Context) {
	utils.RespondSuccess(c, gin.H{"status": "ok", "message": "PKU Hole API is running"})
}

func Help(c *gin.Context) {
	utils.RespondSuccess(c, []gin.H{
		{"method": "GET", "route": "/health", "description": "Check whether the API server is running."},
		{"method": "GET", "route": "/help", "description": "List available routes with brief English descriptions."},
		{"method": "GET", "route": "/posts?begin=0&limit=25&keyword=&order_by=", "description": "List posts with optional cursor pagination, keyword search, and ordering."},
		{"method": "GET", "route": "/post/:pid", "description": "Fetch a single post by post ID."},
		{"method": "GET", "route": "/comment?cid=123", "description": "Fetch a single comment by comment ID."},
		{"method": "GET", "route": "/comments/:pid?begin=0&limit=25&sort=0", "description": "List comments for a post with optional cursor pagination and sort order."},
		{"method": "GET", "route": "/media/image?id=123 or /media/image?pid=456", "description": "Serve a local image by media ID or post ID."},
	})
}

func GetPost(posts *service.PostService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
		if err != nil || pid <= 0 {
			utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("invalid pid"))
			return
		}
		if posts == nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", errors.New("post service is not configured"))
			return
		}

		post, err := posts.RefreshPost(c.Request.Context(), int32(pid), service.SourceLocal)
		if err != nil {
			utils.RespondError(c, http.StatusNotFound, "NotFound", err)
			return
		}
		utils.RespondSuccess(c, serializePost(*post))
	}
}

func GetComment(posts *service.PostService) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid, err := strconv.ParseInt(c.Query("cid"), 10, 32)
		if err != nil || cid <= 0 {
			utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("invalid cid"))
			return
		}
		if posts == nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", errors.New("post service is not configured"))
			return
		}

		comment, err := posts.GetComment(c.Request.Context(), int32(cid), service.SourceLocal)
		if err != nil {
			utils.RespondError(c, http.StatusNotFound, "NotFound", err)
			return
		}
		utils.RespondSuccess(c, serializeComment(comment))
	}
}

// GetPosts preserves the legacy begin/order_by response contract while all
// query policy is delegated to application services.
func GetPosts(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Posts == nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", errors.New("post service is not configured"))
			return
		}

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
		if err != nil || limit < 1 {
			limit = 25
		}
		if limit > 100 {
			limit = 100
		}
		cursor, _ := strconv.Atoi(c.DefaultQuery("begin", "0"))
		keyword := c.Query("keyword")
		orderBy := c.Query("order_by")

		if id := c.Query("id"); id != "" {
			pid, parseErr := strconv.ParseInt(id, 10, 32)
			if parseErr != nil || pid <= 0 {
				utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("invalid id"))
				return
			}
			post, getErr := dependencies.Posts.RefreshPost(c.Request.Context(), int32(pid), service.SourceLocal)
			if getErr != nil {
				utils.RespondError(c, http.StatusNotFound, "NotFound", getErr)
				return
			}
			utils.RespondSuccess(c, []map[string]any{serializePost(*post)})
			return
		}

		query := service.PostQuery{
			Cursor: cursor,
			Limit:  limit,
			Query:  keyword,
			Source: service.SourceLocal,
			Sort:   orderBy,
		}
		var page service.PostPage
		if keyword != "" || orderBy != "" {
			if dependencies.Search == nil {
				utils.RespondError(c, http.StatusInternalServerError, "ServerError", errors.New("search service is not configured"))
				return
			}
			page, err = dependencies.Search.Search(c.Request.Context(), query)
		} else {
			page, err = dependencies.Posts.List(c.Request.Context(), query)
		}
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", err)
			return
		}

		postData := make([]map[string]any, len(page.Items))
		for i := range page.Items {
			postData[i] = serializePost(page.Items[i].Post)
		}
		utils.RespondSuccess(c, postData)
	}
}

func GetComments(posts *service.PostService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
		if err != nil || pid <= 0 {
			utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("invalid pid"))
			return
		}
		if posts == nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", errors.New("post service is not configured"))
			return
		}

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
		if err != nil || limit < 1 {
			limit = 25
		}
		if limit > 100 {
			limit = 100
		}
		cursor, _ := strconv.ParseInt(c.DefaultQuery("begin", "0"), 10, 32)
		sort := "asc"
		if c.DefaultQuery("sort", "0") == "1" {
			sort = "desc"
		}

		page, err := posts.Comments(c.Request.Context(), int32(pid), service.CommentQuery{
			Cursor: int32(cursor),
			Limit:  limit,
			Sort:   sort,
			Source: service.SourceLocal,
		})
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", err)
			return
		}

		commentData := make([]map[string]any, len(page.Items))
		for i := range page.Items {
			commentData[i] = serializeComment(&page.Items[i])
		}
		utils.RespondSuccess(c, commentData)
	}
}

func serializePost(post models.Post) map[string]any {
	username := "anonymous"
	if !post.Anonymous {
		username = ""
	}
	return map[string]any{
		"id":        post.Pid,
		"text":      post.Text,
		"userid":    65535,
		"username":  username,
		"timestamp": post.Timestamp,
		"reply":     post.Reply,
		"follownum": post.Likenum,
		"is_follow": 0,
		"likenum":   post.PraiseNum,
		"is_like":   0,
		"type":      post.Type,
		"tags":      []string{},
		"media_ids": post.MediaIds,
	}
}

func serializeComment(comment *models.Comment) map[string]any {
	return map[string]any{
		"cid":       comment.Cid,
		"pid":       comment.Pid,
		"userid":    65535,
		"username":  comment.NameTag,
		"text":      comment.Text,
		"timestamp": comment.Timestamp,
		"quote":     comment.Quote,
		"media_ids": comment.MediaIds,
	}
}

func GetImage(media *service.MediaService) gin.HandlerFunc {
	return func(c *gin.Context) {
		idValue := c.Query("id")
		pidValue := c.Query("pid")
		if idValue == "" && pidValue == "" {
			utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("missing id or pid parameter"))
			return
		}
		if media == nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", errors.New("media service is not configured"))
			return
		}

		request := service.MediaRequest{}
		if idValue != "" {
			if _, err := strconv.ParseInt(idValue, 10, 64); err != nil {
				utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("invalid id"))
				return
			}
			request.ID = idValue
		} else {
			pid, err := strconv.ParseInt(pidValue, 10, 32)
			if err != nil || pid <= 0 {
				utils.RespondError(c, http.StatusBadRequest, "InvalidParam", errors.New("invalid pid"))
				return
			}
			request.PID = int32(pid)
		}

		file, err := media.Locate(c.Request.Context(), request)
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "ServerError", err)
			return
		}
		c.File(file.Path)
	}
}
