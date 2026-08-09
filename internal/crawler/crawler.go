package crawler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/db"
	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"github.com/chai2010/webp"
)

var (
	rawResponses   []map[string]interface{}
	rawResponsesMu sync.Mutex
	rawSaveMu      sync.Mutex
)

var ErrNoRawResponses = errors.New("没有已缓存的原始响应；请先在顺序采集中启用原始 JSON 缓存")

type APIResponse struct {
	Code int                    `json:"code"`
	Data map[string]interface{} `json:"data"`
}

// FetchResult holds the result of fetching a single page
type FetchResult struct {
	PostCount    int
	CommentCount int
	Media        MediaDownloadResult
}

// MediaDownloadResult separates files that were already available from real
// download failures. The legacy crawler used to report both cases as skipped,
// which could make an incomplete run look successful.
type MediaDownloadResult struct {
	Downloaded int `json:"downloaded"`
	Existing   int `json:"existing"`
	Failed     int `json:"failed"`
}

func (r *MediaDownloadResult) add(other MediaDownloadResult) {
	r.Downloaded += other.Downloaded
	r.Existing += other.Existing
	r.Failed += other.Failed
}

// FetchAndSave fetches one page of posts and saves them to the database
func FetchAndSave(c *client.Client, database *db.Database, page int, saveJSON bool, postLimit int, commentLimit int, fetchImages bool, convertWebp bool, imageOutputDir string) (FetchResult, error) {
	var result FetchResult

	posts, comments, rawResponse, err := fetchPostsV3(c, page, postLimit, commentLimit)
	if err != nil {
		return result, err
	}

	result.PostCount = len(posts)
	result.CommentCount = len(comments)

	if err := database.SaveCrawlResultWithSource(posts, comments, "public", "treehole-live"); err != nil {
		return result, fmt.Errorf("写入抓取结果失败: %w", err)
	}

	// 如果启用JSON保存，将原始响应添加到全局列表
	if saveJSON && rawResponse != nil {
		rawResponsesMu.Lock()
		rawResponses = append(rawResponses, rawResponse)
		rawResponsesMu.Unlock()
		log.Printf("[Crawler] 原始响应已添加到内存缓存")
	}

	if fetchImages {
		var firstErr error
		for _, post := range posts {
			if post.Type != "image" && strings.TrimSpace(post.MediaIds) == "" {
				continue
			}
			mediaResult, mediaErr := downloadImages(c, post.MediaIds, post.Pid, imageOutputDir, convertWebp)
			result.Media.add(mediaResult)
			if firstErr == nil && mediaErr != nil {
				firstErr = mediaErr
			}
		}
		for _, comment := range comments {
			if strings.TrimSpace(comment.MediaIds) == "" {
				continue
			}
			mediaResult, mediaErr := downloadImages(c, comment.MediaIds, 0, imageOutputDir, convertWebp)
			result.Media.add(mediaResult)
			if firstErr == nil && mediaErr != nil {
				firstErr = mediaErr
			}
		}
		if result.Media.Failed > 0 {
			log.Printf("[Crawler] 页面正文已保存，但有图片未下载: downloaded=%d, existing=%d, failed=%d, first_error=%v",
				result.Media.Downloaded, result.Media.Existing, result.Media.Failed, firstErr)
		}
	}

	return result, nil
}

// RawResponses 返回当前存储的原始响应数量
func RawResponses() int {
	rawResponsesMu.Lock()
	defer rawResponsesMu.Unlock()
	return len(rawResponses)
}

// SaveRawResponsesToFile keeps the legacy TUI/CLI behavior of writing to the
// current directory. Web jobs use SaveRawResponsesTo with a managed path.
func SaveRawResponsesToFile() error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("raw_responses_%s.json", timestamp)
	_, _, err := SaveRawResponsesTo(filename)
	return err
}

// SaveRawResponsesTo atomically drains the cached responses into path. The
// cache is only cleared after the file has been committed successfully.
func SaveRawResponsesTo(path string) (count int, size int64, err error) {
	rawSaveMu.Lock()
	defer rawSaveMu.Unlock()
	if strings.TrimSpace(path) == "" {
		return 0, 0, errors.New("原始响应保存路径不能为空")
	}
	rawResponsesMu.Lock()
	if len(rawResponses) == 0 {
		rawResponsesMu.Unlock()
		return 0, 0, ErrNoRawResponses
	}
	responsesCopy := make([]map[string]interface{}, len(rawResponses))
	copy(responsesCopy, rawResponses)
	rawResponsesMu.Unlock()

	data, err := json.MarshalIndent(responsesCopy, "", "  ")
	if err != nil {
		return 0, 0, fmt.Errorf("序列化原始响应失败: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return 0, 0, fmt.Errorf("创建原始响应目录失败: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".raw-*.tmp")
	if err != nil {
		return 0, 0, fmt.Errorf("创建原始响应临时文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return 0, 0, err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return 0, 0, fmt.Errorf("写入原始响应失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return 0, 0, fmt.Errorf("关闭原始响应文件失败: %w", err)
	}
	_ = os.Remove(path)
	if err := os.Rename(temporaryPath, path); err != nil {
		return 0, 0, fmt.Errorf("提交原始响应文件失败: %w", err)
	}

	rawResponsesMu.Lock()
	if len(rawResponses) >= len(responsesCopy) {
		rawResponses = append([]map[string]interface{}(nil), rawResponses[len(responsesCopy):]...)
	} else {
		rawResponses = nil
	}
	rawResponsesMu.Unlock()
	log.Printf("[Crawler] 原始响应已保存到文件: %s (%d 个响应)", filepath.Base(path), len(responsesCopy))
	return len(responsesCopy), int64(len(data)), nil
}

// TempPost 用于匹配API响应的临时帖子结构体
type TempPost struct {
	Pid             int32         `json:"pid"`
	Text            string        `json:"text"`
	Type            string        `json:"type"`
	Timestamp       int32         `json:"timestamp"`
	Hidden          int           `json:"hidden"`
	Reply           int16         `json:"reply"`
	Likenum         int16         `json:"likenum"`
	Extra           int32         `json:"extra"`
	Anonymous       int           `json:"anonymous"`
	Protected       int16         `json:"protected"`
	IsTop           int           `json:"is_top"`
	Label           int16         `json:"label"`
	Status          int16         `json:"status"`
	IsComment       int           `json:"is_comment"`
	TagsIds         string        `json:"tags_ids"`
	AutoTagsIds     string        `json:"auto_tags_ids"`
	MgrTagsIds      string        `json:"mgr_tags_ids"`
	MediaIds        string        `json:"media_ids"`
	Fold            int16         `json:"fold"`
	Kind            int16         `json:"kind"`
	RewardCost      int16         `json:"reward_cost"`
	RewardState     int16         `json:"reward_state"`
	IdentityShow    int           `json:"identity_show"`
	IdentityType    string        `json:"identity_type"`
	IdentityInfo    interface{}   `json:"identity_info"`
	ExclusiveIdId   int16         `json:"exclusive_id_id"`
	ExclusiveIdInfo interface{}   `json:"exclusive_id_info"`
	Mention         string        `json:"mention"`
	Mailbox         int16         `json:"mailbox"`
	ImageSize       string        `json:"image_size"`
	HasRewardGood   int           `json:"has_reward_good"`
	IsGodHole       int           `json:"is_god_hole"`
	IsProtect       int           `json:"is_protect"`
	TreadNum        int16         `json:"tread_num"`
	PraiseNum       int16         `json:"praise_num"`
	PraiseNumShow   int16         `json:"praise_num_show"`
	FoldNum         int16         `json:"fold_num"`
	IsFold          int           `json:"is_fold"`
	CannotReply     int           `json:"cannot_reply"`
	CommentList     []TempComment `json:"comment_list"`
}

// TempComment 用于匹配API响应的临时评论结构体
type TempComment struct {
	Anonymous       int         `json:"anonymous"`
	Cid             int32       `json:"cid"`
	Pid             int32       `json:"pid"`
	ExclusiveIdId   int16       `json:"exclusive_id_id"`
	Hidden          int         `json:"hidden"`
	IdentityShow    int         `json:"identity_show"`
	IdentityType    string      `json:"identity_type"`
	IdentityInfo    interface{} `json:"identity_info"`
	IsLz            int         `json:"is_lz"`
	Mention         string      `json:"mention"`
	NameTag         string      `json:"name_tag"`
	RewardGood      int16       `json:"reward_good"`
	Text            string      `json:"text"`
	Timestamp       int32       `json:"timestamp"`
	Quote           interface{} `json:"quote"`
	MediaIds        string      `json:"media_ids"`
	ExclusiveIdInfo interface{} `json:"exclusive_id_info"`
}

func fetchPostsV3(c *client.Client, page int, postLimit int, commentLimit int) ([]models.Post, []models.Comment, map[string]interface{}, error) {
	log.Printf("[Crawler] 正在请求 API: page=%d, postLimit=%d, commentLimit=%d", page, postLimit, commentLimit)
	resp, err := c.GetPostsList(page, postLimit, commentLimit, 1)
	if err != nil {
		log.Printf("[Crawler] API 请求失败: %v", err)
		return nil, nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[Crawler] API 返回非200状态码: %d", resp.StatusCode)
		return nil, nil, nil, fmt.Errorf("get posts failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Crawler] 读取响应体失败: %v", err)
		return nil, nil, nil, err
	}

	// 解析原始响应用于JSON保存
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		log.Printf("[Crawler] 解析原始响应失败: %v", err)
		return nil, nil, nil, err
	}

	// 检查API状态
	code, ok := rawResponse["code"].(float64)
	if !ok || code != 20000 {
		message, _ := rawResponse["message"].(string)
		log.Printf("[Crawler] API 业务错误: code=%v, message=%s", code, message)
		return nil, nil, nil, fmt.Errorf("API error: %s", message)
	}

	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		log.Printf("[Crawler] 无效的 data 格式")
		return nil, nil, nil, fmt.Errorf("invalid data format")
	}

	postsData, ok := data["list"].([]interface{})
	if !ok {
		log.Printf("[Crawler] 无效的 posts data 格式")
		return nil, nil, nil, fmt.Errorf("invalid posts data format")
	}

	// 将帖子数据反序列化到临时结构体
	var tempPosts []TempPost
	postsBytes, err := json.Marshal(postsData)
	if err != nil {
		log.Printf("[Crawler] 序列化帖子数据失败: %v", err)
		return nil, nil, nil, err
	}
	if err := json.Unmarshal(postsBytes, &tempPosts); err != nil {
		log.Printf("[Crawler] 反序列化临时帖子失败: %v", err)
		return nil, nil, nil, err
	}

	var posts []models.Post
	var comments []models.Comment

	// 转换临时结构体到最终模型，保持原始顺序
	for _, tempPost := range tempPosts {
		// 转换帖子
		post := models.Post{
			Pid:           tempPost.Pid,
			Text:          tempPost.Text,
			Type:          tempPost.Type,
			Timestamp:     tempPost.Timestamp,
			Hidden:        models.BoolFromInt(tempPost.Hidden),
			Reply:         tempPost.Reply,
			Likenum:       tempPost.Likenum,
			Extra:         tempPost.Extra,
			Anonymous:     models.BoolFromInt(tempPost.Anonymous),
			Protected:     tempPost.Protected,
			IsTop:         models.BoolFromInt(tempPost.IsTop),
			Label:         tempPost.Label,
			Status:        tempPost.Status,
			IsComment:     models.BoolFromInt(tempPost.IsComment),
			TagsIds:       tempPost.TagsIds,
			AutoTagsIds:   tempPost.AutoTagsIds,
			MgrTagsIds:    tempPost.MgrTagsIds,
			MediaIds:      tempPost.MediaIds,
			Fold:          tempPost.Fold,
			Kind:          tempPost.Kind,
			RewardCost:    tempPost.RewardCost,
			RewardState:   tempPost.RewardState,
			IdentityShow:  models.BoolFromInt(tempPost.IdentityShow),
			IdentityType:  tempPost.IdentityType,
			ExclusiveIdId: tempPost.ExclusiveIdId,
			Mention:       tempPost.Mention,
			Mailbox:       tempPost.Mailbox,
			ImageSize:     tempPost.ImageSize,
			HasRewardGood: models.BoolFromInt(tempPost.HasRewardGood),
			IsGodHole:     models.BoolFromInt(tempPost.IsGodHole),
			IsProtect:     models.BoolFromInt(tempPost.IsProtect),
			TreadNum:      tempPost.TreadNum,
			PraiseNum:     tempPost.PraiseNum,
			PraiseNumShow: tempPost.PraiseNumShow,
			FoldNum:       tempPost.FoldNum,
			IsFold:        models.BoolFromInt(tempPost.IsFold),
			CannotReply:   models.BoolFromInt(tempPost.CannotReply),
		}

		// 处理特殊字段
		post.IdentityInfo = getJSONStringFromInterface(tempPost.IdentityInfo)
		post.ExclusiveIdInfo = getJSONStringFromInterface(tempPost.ExclusiveIdInfo)

		// 处理评论
		var processedComments []models.Comment
		for _, tempComment := range tempPost.CommentList {
			// 提取引用评论ID
			var quoteID *int32
			if tempComment.Quote != nil {
				if quoteMap, ok := tempComment.Quote.(map[string]interface{}); ok {
					if cidVal, ok := quoteMap["cid"].(float64); ok {
						qid := int32(cidVal)
						quoteID = &qid
					}
				}
			}

			comment := models.Comment{
				Anonymous:     models.BoolFromInt(tempComment.Anonymous),
				Cid:           tempComment.Cid,
				Pid:           tempComment.Pid,
				ExclusiveIdId: tempComment.ExclusiveIdId,
				Hidden:        models.BoolFromInt(tempComment.Hidden),
				IdentityShow:  models.BoolFromInt(tempComment.IdentityShow),
				IdentityType:  tempComment.IdentityType,
				IsLz:          models.BoolFromInt(tempComment.IsLz),
				Mention:       tempComment.Mention,
				NameTag:       tempComment.NameTag,
				RewardGood:    tempComment.RewardGood,
				Text:          tempComment.Text,
				Timestamp:     tempComment.Timestamp,
				QuoteID:       quoteID,
				MediaIds:      tempComment.MediaIds,
			}

			// 处理评论的特殊字段
			comment.IdentityInfo = getJSONStringFromInterface(tempComment.IdentityInfo)
			comment.ExclusiveIdInfo = getJSONStringFromInterface(tempComment.ExclusiveIdInfo)

			processedComments = append(processedComments, comment)
			comments = append(comments, comment)
		}

		post.Comments = processedComments
		posts = append(posts, post)
	}

	log.Printf("[Crawler] 数据处理完成: %d 个帖子, %d 个评论", len(posts), len(comments))
	return posts, comments, rawResponse, nil
}

// FetchImagesFromDB 从数据库中查找有图片的帖子和评论，下载缺失的图片。
// imageOutputDir 必须来自当前 profile 的 data-dir，不能依赖进程工作目录。
func FetchImagesFromDB(c *client.Client, database *db.Database, imageOutputDir string, convertWebp bool) (MediaDownloadResult, error) {
	var result MediaDownloadResult
	log.Println("[Crawler] 开始从数据库下载缺失的图片...")

	// 下载帖子图片
	postCount, err := database.GetPostsWithImagesCount()
	if err != nil {
		return result, fmt.Errorf("query posts with images: %w", err)
	}
	log.Printf("[Crawler] 数据库中有 %d 个帖子包含图片", postCount)

	batchSize := 100
	var firstErr error
	for offset := 0; offset < postCount; offset += batchSize {
		posts, err := database.GetPostsWithImages(offset, batchSize)
		if err != nil {
			return result, fmt.Errorf("query posts with images at offset %d: %w", offset, err)
		}

		for _, post := range posts {
			if post.Type == "image" || post.MediaIds != "" {
				mediaResult, mediaErr := downloadImages(c, post.MediaIds, post.Pid, imageOutputDir, convertWebp)
				result.add(mediaResult)
				if firstErr == nil && mediaErr != nil {
					firstErr = mediaErr
				}
			}
		}
		log.Printf("[Crawler] 帖子图片进度: %d/%d", offset+len(posts), postCount)
	}

	// 下载评论图片
	commentCount, err := database.GetCommentsWithImagesCount()
	if err != nil {
		return result, fmt.Errorf("query comments with images: %w", err)
	}
	log.Printf("[Crawler] 数据库中有 %d 个评论包含图片", commentCount)

	for offset := 0; offset < commentCount; offset += batchSize {
		comments, err := database.GetCommentsWithImages(offset, batchSize)
		if err != nil {
			return result, fmt.Errorf("query comments with images at offset %d: %w", offset, err)
		}

		for _, comment := range comments {
			if comment.MediaIds != "" {
				mediaResult, mediaErr := downloadImages(c, comment.MediaIds, 0, imageOutputDir, convertWebp)
				result.add(mediaResult)
				if firstErr == nil && mediaErr != nil {
					firstErr = mediaErr
				}
			}
		}
		log.Printf("[Crawler] 评论图片进度: %d/%d", offset+len(comments), commentCount)
	}

	log.Printf("[Crawler] 图片下载完成: downloaded=%d, existing=%d, failed=%d, dir=%s",
		result.Downloaded, result.Existing, result.Failed, imageOutputDir)
	if result.Failed > 0 {
		return result, mediaDownloadError(result.Failed, firstErr)
	}
	return result, nil
}
func getJSONStringFromInterface(val interface{}) string {
	if val == nil {
		return ""
	}

	// 检查是否为空数组
	if arr, ok := val.([]interface{}); ok && len(arr) == 0 {
		return ""
	}

	if bytes, err := json.Marshal(val); err == nil {
		jsonStr := string(bytes)
		if jsonStr == "[]" {
			return ""
		}
		return jsonStr
	}
	return ""
}

func getStringField(data map[string]interface{}, key string) string {
	if val, exists := data[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntField 安全获取整数字段（处理JSON数字默认为float64的问题）
func getIntField(data map[string]interface{}, key string) int {
	if val, exists := data[key]; exists {
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return 0
}

func downloadImages(c *client.Client, mediaIDs string, fallbackPID int32, outputDir string, convertWebp bool) (MediaDownloadResult, error) {
	var result MediaDownloadResult
	var firstErr error

	ids := strings.Split(mediaIDs, ",")
	if strings.TrimSpace(mediaIDs) == "" {
		if fallbackPID <= 0 {
			result.Failed++
			return result, errors.New("missing media id and fallback PID")
		}
		ids = []string{strconv.FormatInt(int64(fallbackPID), 10)}
	}

	for _, idText := range ids {
		idText = strings.TrimSpace(idText)
		if idText == "" {
			continue
		}
		id, err := strconv.ParseInt(idText, 10, 32)
		if err != nil || id <= 0 {
			result.Failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid media id %q", idText)
			}
			continue
		}

		queryName := "id"
		if strings.TrimSpace(mediaIDs) == "" {
			queryName = "pid"
		}
		url := fmt.Sprintf("https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?%s=%d", queryName, id)
		downloaded, saveErr := saveMedia(c, url, int32(id), outputDir, convertWebp)
		switch {
		case saveErr != nil:
			result.Failed++
			if firstErr == nil {
				firstErr = saveErr
			}
		case downloaded:
			result.Downloaded++
		default:
			result.Existing++
		}
	}

	if result.Failed > 0 {
		return result, firstErr
	}
	return result, nil
}

// FetchThumbnailsByIDRange downloads a legacy thumbnail range into the
// selected profile and reports existing files separately from failures.
func FetchThumbnailsByIDRange(c *client.Client, startID int, endID int, outputDir string, convertWebp bool) (MediaDownloadResult, error) {
	if startID <= 0 || endID <= 0 {
		return MediaDownloadResult{}, fmt.Errorf("media id 必须大于 0")
	}
	if startID > endID {
		return MediaDownloadResult{}, fmt.Errorf("startID(%d) 不能大于 endID(%d)", startID, endID)
	}

	urlTemplate := "https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?id=%d"
	result, err := downloadMediaByIDRange(c, startID, endID, urlTemplate, outputDir, convertWebp)
	log.Printf("[Crawler] 缩略图批量下载完成: range=%d-%d, downloaded=%d, existing=%d, failed=%d, dir=%s",
		startID, endID, result.Downloaded, result.Existing, result.Failed, outputDir)
	return result, err
}

func downloadMediaByIDRange(c *client.Client, startID int, endID int, urlTemplate string, outputDir string, convertWebp bool) (MediaDownloadResult, error) {
	var result MediaDownloadResult
	var firstErr error
	for id := startID; ; id++ {
		url := fmt.Sprintf(urlTemplate, id)
		downloaded, err := saveMedia(c, url, int32(id), outputDir, convertWebp)
		switch {
		case err != nil:
			result.Failed++
			if firstErr == nil {
				firstErr = err
			}
		case downloaded:
			result.Downloaded++
		default:
			result.Existing++
		}
		if id == endID {
			break
		}
	}
	if result.Failed > 0 {
		return result, mediaDownloadError(result.Failed, firstErr)
	}
	return result, nil
}

// saveMedia returns downloaded=false with no error only when the file already
// exists. Every actual download or filesystem failure is returned to callers.
func saveMedia(c *client.Client, url string, mediaID int32, outputDir string, convertWebp bool) (bool, error) {
	if strings.TrimSpace(outputDir) == "" {
		return false, errors.New("media output directory is empty")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return false, fmt.Errorf("create media directory %q: %w", outputDir, err)
	}

	basePath := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID))
	if fileExists(basePath) {
		return false, nil
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("create media request for %d: %w", mediaID, err)
	}
	if auth := c.GetAuthorization(); auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	resp, err := c.GetHttpClient().Do(req)
	if err != nil {
		return false, fmt.Errorf("download media %d: %w", mediaID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("download media %d: HTTP %d", mediaID, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read media %d: %w", mediaID, err)
	}
	if len(body) == 0 {
		return false, fmt.Errorf("download media %d: empty response", mediaID)
	}

	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = strings.ToLower(http.DetectContentType(body))
		if !strings.HasPrefix(contentType, "image/") {
			return false, fmt.Errorf("download media %d: response is not an image", mediaID)
		}
	}
	isGIF := contentType == "image/gif"
	if isGIF || !convertWebp {
		ext := ".jpg"
		switch contentType {
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		}
		filename := basePath + ext
		if err := os.WriteFile(filename, body, 0o644); err != nil {
			return false, fmt.Errorf("save media %d: %w", mediaID, err)
		}
		log.Printf("[Crawler] 图片已保存: %s (%d bytes)", filename, len(body))
		return true, nil
	}

	img, format, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		filename := basePath + ".jpg"
		if writeErr := os.WriteFile(filename, body, 0o644); writeErr != nil {
			return false, fmt.Errorf("save undecoded media %d: %w", mediaID, writeErr)
		}
		log.Printf("[Crawler] 图片无法转换，已按原响应保存: %s", filename)
		return true, nil
	}

	filename := basePath + ".webp"
	buf := new(bytes.Buffer)
	if err := webp.Encode(buf, img, &webp.Options{Lossless: false, Quality: 80}); err != nil {
		log.Printf("[Crawler] 编码 WebP 失败: mediaID=%d, err=%v，回退到原格式", mediaID, err)
		buf.Reset()
		switch format {
		case "png":
			filename = basePath + ".png"
			err = png.Encode(buf, img)
		case "jpeg":
			filename = basePath + ".jpg"
			err = jpeg.Encode(buf, img, nil)
		default:
			filename = basePath + ".jpg"
			_, err = buf.Write(body)
		}
		if err != nil {
			return false, fmt.Errorf("encode fallback media %d: %w", mediaID, err)
		}
	}

	if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
		return false, fmt.Errorf("save media %d: %w", mediaID, err)
	}
	origSize := len(body)
	newSize := buf.Len()
	saved := 0
	if origSize > 0 {
		saved = 1 - newSize*100/origSize
	}
	log.Printf("[Crawler] 图片已保存: %s (%d → %d bytes, 节省 %d%%)", filename, origSize, newSize, saved)
	return true, nil
}

func mediaDownloadError(failed int, firstErr error) error {
	if firstErr == nil {
		return fmt.Errorf("%d media downloads failed", failed)
	}
	return fmt.Errorf("%d media downloads failed; first error: %w", failed, firstErr)
}

// fileExists 检查文件是否存在（尝试所有支持的扩展名）
func fileExists(basePath string) bool {
	exts := []string{".jpg", ".png", ".gif", ".webp"}
	for _, ext := range exts {
		if _, err := os.Stat(basePath + ext); err == nil {
			return true
		}
	}
	return false
}
