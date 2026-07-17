package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

const maxV3ResponseBytes = 2 << 20

// RemoteAPIError preserves the useful, non-secret part of an upstream API
// failure so callers can map authentication, validation and rate-limit errors
// without logging cookies, tokens, request bodies or response HTML.
type RemoteAPIError struct {
	Operation  string
	Endpoint   string
	HTTPStatus int
	Code       string
	Message    string
}

func (e *RemoteAPIError) Error() string {
	if e == nil {
		return "treehole request failed"
	}
	detail := e.Message
	if detail == "" {
		detail = http.StatusText(e.HTTPStatus)
	}
	if detail == "" {
		detail = "remote request failed"
	}
	meta := make([]string, 0, 2)
	if e.HTTPStatus > 0 {
		meta = append(meta, fmt.Sprintf("HTTP %d", e.HTTPStatus))
	}
	if e.Code != "" {
		meta = append(meta, "code "+e.Code)
	}
	if len(meta) == 0 {
		return detail
	}
	return fmt.Sprintf("%s (%s)", detail, strings.Join(meta, ", "))
}

const (
	chapiBaseURL        = "https://treehole.pku.edu.cn/chapi/api"
	v3BaseURL           = "https://treehole.pku.edu.cn/chapi/api/v3"
	chapiCourseTable    = chapiBaseURL + "/getCoursetable_v2"
	chapiCourseScore    = chapiBaseURL + "/course/score_v2"
	v3HoleListComments  = v3BaseURL + "/hole/list_comments"
	v3HoleOne           = v3BaseURL + "/hole/one"
	v3HoleGet           = v3BaseURL + "/hole/get"
	v3HoleAttention     = v3BaseURL + "/hole/attention"
	v3HolePraise        = v3BaseURL + "/hole/praise"
	v3HolePost          = v3BaseURL + "/hole/post"
	v3CommentList       = v3BaseURL + "/comment/list"
	v3CommentPost       = v3BaseURL + "/comment/post"
	v3TagsTree          = v3BaseURL + "/tags/tree"
	v3MediaThumbnail    = v3BaseURL + "/media/getThumbnail"
	v3MediaBinary       = v3BaseURL + "/media/getImageBinary"
	v3MediaUploadImage  = v3BaseURL + "/media/uploadImage"
	v3MessageIndex      = v3BaseURL + "/message/index"
	v3MessageSetRead    = v3BaseURL + "/message/setIntMsgReadByID"
	v3MessageSetAllRead = v3BaseURL + "/message/set_read"
)

type V3ListPostsParams struct {
	Page          int
	Limit         int
	CommentLimit  int
	CommentStream int
	Pid           int32
	Keyword       string
	Label         int
	Kind          int
	IsFollow      *bool
}

type CreatePostPayload struct {
	Type         string `json:"type"`
	Kind         int    `json:"kind"`
	RewardCost   int    `json:"reward_cost"`
	Text         string `json:"text"`
	IdentityShow int    `json:"identity_show"`
	IdentityType string `json:"identity_type"`
	ExclusiveID  string `json:"exclusive_id_id"`
	Fold         int    `json:"fold"`
	Mailbox      int    `json:"mailbox"`
	TagsIDs      string `json:"tags_ids"`
	MediaIDs     string `json:"media_ids"`
}

type CreateCommentPayload struct {
	PID          int32   `json:"pid"`
	CommentID    *string `json:"comment_id,omitempty"`
	Text         string  `json:"text"`
	MediaIDs     string  `json:"media_ids"`
	IdentityShow int     `json:"identity_show"`
	IdentityType string  `json:"identity_type"`
}

type notificationEnvelope struct {
	Data struct {
		List  []notificationDTO `json:"list"`
		Total int               `json:"total"`
	} `json:"data"`
}

type notificationDTO struct {
	ID        int    `json:"id"`
	PID       any    `json:"pid"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Contents  string `json:"contents"`
	IsRead    int    `json:"is_read"`
	HasRead   int    `json:"hasread"`
	CreatedAt string `json:"created_at"`
	Timestamp int64  `json:"timestamp"`
}

type courseTableEnvelope struct {
	Data struct {
		Course []courseScheduleDTO `json:"course"`
	} `json:"data"`
}

type courseScheduleDTO struct {
	TimeNum string       `json:"timeNum"`
	Mon     courseDayDTO `json:"mon"`
	Tue     courseDayDTO `json:"tue"`
	Wed     courseDayDTO `json:"wed"`
	Thu     courseDayDTO `json:"thu"`
	Fri     courseDayDTO `json:"fri"`
	Sat     courseDayDTO `json:"sat"`
	Sun     courseDayDTO `json:"sun"`
}

type courseDayDTO struct {
	CourseName string `json:"courseName"`
	Parity     string `json:"parity"`
	Style      string `json:"sty"`
}

type scoreEnvelope struct {
	Data struct {
		Score struct {
			Courses []courseScoreDTO `json:"cjxx"`
			GPAHM   struct {
				GPA         string `json:"gpa"`
				TotalCredit string `json:"zxf"`
				CourseCount string `json:"xkms"`
				CreditTaken string `json:"xxxf"`
			} `json:"gpaHM"`
			GPA struct {
				GPA    string `json:"gpa"`
				Credit string `json:"xxxf"`
			} `json:"gpa"`
		} `json:"score"`
		GPA struct {
			Data []gpaTermDTO `json:"data"`
		} `json:"gpa"`
	} `json:"data"`
}

type courseScoreDTO struct {
	YearTerm string `json:"xndxqpx"`
	Name     string `json:"kcmc"`
	Credit   string `json:"xf"`
	Score    string `json:"xqcj"`
	Category string `json:"kclbmc"`
}

type gpaTermDTO struct {
	YearTerm string `json:"xndxq"`
	GPA      string `json:"gpa"`
}

type SessionFailureKind string

const (
	SessionFailureNone    SessionFailureKind = "none"
	SessionFailureLogin   SessionFailureKind = "login"
	SessionFailureNetwork SessionFailureKind = "network"
)

type SessionStatus struct {
	HasSession     bool
	CanReadOnline  bool
	CanWriteOnline bool
	FailureKind    SessionFailureKind
	Message        string
}

func (c *Client) ProbeSession() SessionStatus {
	status := SessionStatus{HasSession: c.GetAuthorization() != ""}
	resp, err := c.UnRead()
	if err != nil {
		status.FailureKind = ClassifySessionError(err)
		status.Message = err.Error()
		return status
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		status.FailureKind = SessionFailureNetwork
		status.Message = err.Error()
		return status
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "<") {
			status.FailureKind = SessionFailureLogin
			status.Message = "登录态不可用"
			return status
		}
		status.FailureKind = SessionFailureNetwork
		status.Message = err.Error()
		return status
	}

	if success, ok := result["success"].(bool); ok && success {
		status.HasSession = c.GetAuthorization() != ""
		status.CanReadOnline = true
		status.CanWriteOnline = c.GetAuthorization() != "" && c.GetXSRFToken() != ""
		status.FailureKind = SessionFailureNone
		return status
	}

	if message, ok := result["message"].(string); ok && message != "" {
		status.Message = message
	}
	if status.Message == "" {
		status.Message = "登录态不可用"
	}
	status.FailureKind = SessionFailureLogin
	return status
}

// HasWriteCredentials is intentionally a local check. The actual mutation
// endpoint remains the source of truth; an unrelated read endpoint must not
// be able to block a write before it is attempted.
func (c *Client) HasWriteCredentials() bool {
	return c != nil && c.GetAuthorization() != "" && c.GetXSRFToken() != ""
}

func ClassifySessionError(err error) SessionFailureKind {
	var netErr net.Error
	lower := strings.ToLower(err.Error())
	for _, marker := range []string{
		"timeout", "connection", "no such host", "server misbehaving", "network",
		"tls", "eof", "refused", "reset", "dial", "lookup", "unreachable",
		"temporary", "i/o timeout", "context deadline exceeded", "no route to host",
		"broken pipe", "handshake",
	} {
		if strings.Contains(lower, marker) {
			return SessionFailureNetwork
		}
	}
	if strings.Contains(lower, "unexpected status") {
		return SessionFailureLogin
	}
	if strings.Contains(lower, "oauth") {
		return SessionFailureLogin
	}
	if err != nil && strings.Contains(fmt.Sprintf("%T", err), "*url.Error") {
		return SessionFailureNetwork
	}
	if errors.As(err, &netErr) && netErr != nil {
		return SessionFailureNetwork
	}
	return SessionFailureLogin
}

func (c *Client) ListPostsV3(params V3ListPostsParams) ([]models.Post, int, error) {
	return c.ListPostsV3Context(context.Background(), params)
}

func (c *Client) ListPostsV3Context(ctx context.Context, params V3ListPostsParams) ([]models.Post, int, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.CommentLimit < 0 {
		params.CommentLimit = 0
	}
	if params.CommentStream == 0 {
		params.CommentStream = 1
	}
	q := url.Values{}
	q.Set("page", strconv.Itoa(params.Page))
	q.Set("limit", strconv.Itoa(params.Limit))
	q.Set("comment_limit", strconv.Itoa(params.CommentLimit))
	q.Set("comment_stream", strconv.Itoa(params.CommentStream))
	if params.Pid > 0 {
		q.Set("pid", strconv.Itoa(int(params.Pid)))
	}
	if params.Keyword != "" {
		q.Set("keyword", params.Keyword)
	}
	if params.Label > 0 {
		q.Set("label", strconv.Itoa(params.Label))
	}
	if params.Kind > 0 {
		q.Set("kind", strconv.Itoa(params.Kind))
	}
	if params.IsFollow != nil {
		if *params.IsFollow {
			q.Set("is_follow", "1")
		} else {
			q.Set("is_follow", "0")
		}
	}

	var envelope struct {
		Data struct {
			List  []postDTO `json:"list"`
			Total int       `json:"total"`
		} `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodGet, v3HoleListComments, q, nil, false, &envelope); err != nil {
		return nil, 0, err
	}
	posts := make([]models.Post, 0, len(envelope.Data.List))
	for _, dto := range envelope.Data.List {
		posts = append(posts, dto.toModel())
	}
	return posts, envelope.Data.Total, nil
}

func (c *Client) ListNotificationsV3(messageType models.NotificationType, page, limit int) ([]models.Notification, int, error) {
	return c.ListNotificationsV3Context(context.Background(), messageType, page, limit)
}

func (c *Client) ListNotificationsV3Context(ctx context.Context, messageType models.NotificationType, page, limit int) ([]models.Notification, int, error) {
	if err := validateNotificationType(messageType); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	query := url.Values{
		"page":         {strconv.Itoa(page)},
		"limit":        {strconv.Itoa(limit)},
		"message_type": {string(messageType)},
	}
	var envelope notificationEnvelope
	if err := c.doV3JSONContext(ctx, http.MethodGet, v3MessageIndex, query, nil, false, &envelope); err != nil {
		return nil, 0, err
	}
	items := make([]models.Notification, 0, len(envelope.Data.List))
	for _, item := range envelope.Data.List {
		items = append(items, item.toModel(messageType))
	}
	return items, envelope.Data.Total, nil
}

func (c *Client) SetNotificationReadV3(id int) error {
	return c.SetNotificationReadV3Context(context.Background(), id)
}

func (c *Client) SetNotificationReadV3Context(ctx context.Context, id int) error {
	if id <= 0 {
		return errors.New("notification id must be positive")
	}
	return c.doV3JSONContext(ctx, http.MethodPost, v3MessageSetRead, nil, map[string]int{"id": id}, true, nil)
}

func (c *Client) SetAllNotificationsReadV3(messageType models.NotificationType) error {
	return c.SetAllNotificationsReadV3Context(context.Background(), messageType)
}

func (c *Client) SetAllNotificationsReadV3Context(ctx context.Context, messageType models.NotificationType) error {
	if err := validateNotificationType(messageType); err != nil {
		return err
	}
	return c.doV3JSONContext(ctx, http.MethodPost, v3MessageSetAllRead, nil, map[string]string{
		"message_type": string(messageType),
	}, true, nil)
}

func validateNotificationType(messageType models.NotificationType) error {
	switch messageType {
	case models.NotificationTypeInteractive, models.NotificationTypeSystem:
		return nil
	default:
		return fmt.Errorf("unsupported notification type: %q", messageType)
	}
}

func (d notificationDTO) toModel(messageType models.NotificationType) models.Notification {
	content := d.Content
	if content == "" {
		content = d.Contents
	}
	read := d.HasRead != 0
	if messageType == models.NotificationTypeInteractive {
		read = d.IsRead != 0
	}
	return models.Notification{
		ID:        d.ID,
		PID:       parseNotificationPID(d.PID),
		Title:     d.Title,
		Content:   content,
		Read:      read,
		CreatedAt: d.CreatedAt,
		Timestamp: d.Timestamp,
		Type:      messageType,
	}
}

func parseNotificationPID(value any) int32 {
	switch v := value.(type) {
	case float64:
		return int32(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 32)
		return int32(n)
	}
	return 0
}

func (c *Client) GetPostOne(pid int32, commentStream int) (*models.Post, []models.Comment, int, error) {
	return c.GetPostOneContext(context.Background(), pid, commentStream)
}

func (c *Client) GetPostOneContext(ctx context.Context, pid int32, commentStream int) (*models.Post, []models.Comment, int, error) {
	if commentStream == 0 {
		commentStream = 1
	}
	q := url.Values{}
	q.Set("pid", strconv.Itoa(int(pid)))
	q.Set("comment_stream", strconv.Itoa(commentStream))
	var envelope struct {
		Data struct {
			Hole  postDTO      `json:"hole"`
			List  []commentDTO `json:"list"`
			Total int          `json:"total"`
		} `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodGet, v3HoleOne, q, nil, false, &envelope); err != nil {
		return nil, nil, 0, err
	}
	post := envelope.Data.Hole.toModel()
	comments := make([]models.Comment, 0, len(envelope.Data.List))
	for _, dto := range envelope.Data.List {
		comments = append(comments, dto.toModel())
	}
	return &post, comments, envelope.Data.Total, nil
}

func (c *Client) GetPostGet(pid int32) (*models.Post, error) {
	return c.GetPostGetContext(context.Background(), pid)
}

func (c *Client) GetPostGetContext(ctx context.Context, pid int32) (*models.Post, error) {
	q := url.Values{}
	q.Set("pid", strconv.Itoa(int(pid)))
	var envelope struct {
		Data postDTO `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodGet, v3HoleGet, q, nil, false, &envelope); err != nil {
		return nil, err
	}
	post := envelope.Data.toModel()
	return &post, nil
}

func (c *Client) GetCourseTableV2() ([]models.CourseScheduleRow, error) {
	return c.GetCourseTableV2Context(context.Background())
}

func (c *Client) GetCourseTableV2Context(ctx context.Context) ([]models.CourseScheduleRow, error) {
	var envelope courseTableEnvelope
	if err := c.doV3JSONContext(ctx, http.MethodGet, chapiCourseTable, nil, nil, false, &envelope); err != nil {
		return nil, err
	}
	rows := make([]models.CourseScheduleRow, 0, len(envelope.Data.Course))
	for _, dto := range envelope.Data.Course {
		rows = append(rows, dto.toModel())
	}
	return rows, nil
}

func (c *Client) GetCourseScoresV2() (*models.ScoreSummary, error) {
	return c.GetCourseScoresV2Context(context.Background())
}

func (c *Client) GetCourseScoresV2Context(ctx context.Context) (*models.ScoreSummary, error) {
	var envelope scoreEnvelope
	if err := c.doV3JSONContext(ctx, http.MethodGet, chapiCourseScore, nil, nil, false, &envelope); err != nil {
		return nil, err
	}
	summary := &models.ScoreSummary{
		GPA:          envelope.Data.Score.GPAHM.GPA,
		TotalCredit:  envelope.Data.Score.GPAHM.TotalCredit,
		PassedCredit: envelope.Data.Score.GPAHM.CreditTaken,
		CourseCount:  envelope.Data.Score.GPAHM.CourseCount,
	}
	if summary.GPA == "" {
		summary.GPA = envelope.Data.Score.GPA.GPA
	}
	if summary.PassedCredit == "" {
		summary.PassedCredit = envelope.Data.Score.GPA.Credit
	}
	for _, dto := range envelope.Data.Score.Courses {
		summary.Scores = append(summary.Scores, models.CourseScore{
			YearTerm: dto.YearTerm,
			Name:     cleanCourseText(dto.Name),
			Credit:   dto.Credit,
			Score:    dto.Score,
			Category: dto.Category,
		})
	}
	for _, dto := range envelope.Data.GPA.Data {
		summary.GPATerms = append(summary.GPATerms, models.GPATerm{YearTerm: dto.YearTerm, GPA: dto.GPA})
	}
	return summary, nil
}

func (c *Client) ListCommentsV3(pid int32, page, limit int, sort, commentStream int) ([]models.Comment, int, error) {
	return c.ListCommentsV3Context(context.Background(), pid, page, limit, sort, commentStream)
}

func (c *Client) ListCommentsV3Context(ctx context.Context, pid int32, page, limit int, sort, commentStream int) ([]models.Comment, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{}
	q.Set("pid", strconv.Itoa(int(pid)))
	q.Set("page", strconv.Itoa(page))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("sort", strconv.Itoa(sort))
	q.Set("comment_stream", strconv.Itoa(commentStream))
	var envelope struct {
		Data struct {
			List  []commentDTO `json:"list"`
			Total int          `json:"total"`
		} `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodGet, v3CommentList, q, nil, false, &envelope); err != nil {
		return nil, 0, err
	}
	comments := make([]models.Comment, 0, len(envelope.Data.List))
	for _, dto := range envelope.Data.List {
		comments = append(comments, dto.toModel())
	}
	return comments, envelope.Data.Total, nil
}

func (c *Client) ToggleAttentionV3(pid int32) error {
	return c.ToggleAttentionV3Context(context.Background(), pid)
}

func (c *Client) ToggleAttentionV3Context(ctx context.Context, pid int32) error {
	body := map[string]int32{"pid": pid}
	return c.doV3JSONContext(ctx, http.MethodPost, v3HoleAttention, nil, body, true, nil)
}

func (c *Client) TogglePraiseV3(pid int32) error {
	return c.TogglePraiseV3Context(context.Background(), pid)
}

func (c *Client) TogglePraiseV3Context(ctx context.Context, pid int32) error {
	body := map[string]int32{"pid": pid}
	return c.doV3JSONContext(ctx, http.MethodPost, v3HolePraise, nil, body, true, nil)
}

func (c *Client) CreatePostV3(payload CreatePostPayload) (*models.Post, error) {
	return c.CreatePostV3Context(context.Background(), payload)
}

func (c *Client) CreatePostV3Context(ctx context.Context, payload CreatePostPayload) (*models.Post, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodPost, v3HolePost, nil, payload, true, &envelope); err != nil {
		return nil, err
	}
	if firstJSONToken(envelope.Data) == '{' {
		var dto postDTO
		if err := json.Unmarshal(envelope.Data, &dto); err != nil {
			return nil, fmt.Errorf("decode created post: %w", err)
		}
		post := dto.toModel()
		return &post, nil
	}
	pid, err := positiveInt32ResponseID(envelope.Data, "post id")
	if err != nil {
		return nil, err
	}
	return &models.Post{Pid: pid, Text: payload.Text, Type: payload.Type, MediaIds: payload.MediaIDs}, nil
}

func (c *Client) CreateCommentV3(payload CreateCommentPayload) (*models.Comment, error) {
	return c.CreateCommentV3Context(context.Background(), payload)
}

func (c *Client) CreateCommentV3Context(ctx context.Context, payload CreateCommentPayload) (*models.Comment, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodPost, v3CommentPost, nil, payload, true, &envelope); err != nil {
		return nil, err
	}
	if firstJSONToken(envelope.Data) == '{' {
		var dto commentDTO
		if err := json.Unmarshal(envelope.Data, &dto); err != nil {
			return nil, fmt.Errorf("decode created comment: %w", err)
		}
		comment := dto.toModel()
		return &comment, nil
	}
	cid, err := positiveInt32ResponseID(envelope.Data, "comment id")
	if err != nil {
		return nil, err
	}
	return &models.Comment{Cid: cid, Pid: payload.PID, Text: payload.Text, MediaIds: payload.MediaIDs}, nil
}

func firstJSONToken(raw json.RawMessage) byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return 0
	}
	return trimmed[0]
}

// The current Treehole write endpoints return the newly-created PID/CID as a
// bare JSON number. Older deployments returned the whole object, and a few
// mirrors have returned the ID as a JSON string, so accept all three shapes.
func positiveInt32ResponseID(raw json.RawMessage, name string) (int32, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return 0, fmt.Errorf("create response missing %s", name)
	}
	var number json.Number
	if firstJSONToken(trimmed) == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return 0, fmt.Errorf("decode create response %s: %w", name, err)
		}
		number = json.Number(strings.TrimSpace(value))
	} else {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		if err := decoder.Decode(&number); err != nil {
			return 0, fmt.Errorf("decode create response %s: %w", name, err)
		}
	}
	value, err := strconv.ParseInt(number.String(), 10, 32)
	if err != nil || value <= 0 {
		if err == nil {
			err = errors.New("id must be positive")
		}
		return 0, fmt.Errorf("invalid create response %s %q: %w", name, number.String(), err)
	}
	return int32(value), nil
}

func (c *Client) CreateCommentV3WithQuote(pid int32, text string, quoteID *int32) (*models.Comment, error) {
	return c.CreateCommentV3(CreateCommentPayload{
		PID:       pid,
		CommentID: commentIDStringPtr(quoteID),
		Text:      text,
	})
}

func (c *Client) UploadImageV3(path string) (string, error) {
	return c.UploadImageV3Context(context.Background(), path)
}

func (c *Client) UploadImageV3Context(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	// The official web client posts the image in the multipart field named
	// "data". The legacy client used "file", which the current API rejects
	// with business code 40001 even though the image bytes are otherwise valid.
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", multipart.FileContentDisposition("data", filepath.Base(path)))
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "application/octet-stream"
	}
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	var envelope struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	if err := c.doV3MultipartContext(ctx, http.MethodPost, v3MediaUploadImage, writer.FormDataContentType(), &body, &envelope); err != nil {
		return "", err
	}
	if envelope.Data.ID <= 0 {
		return "", fmt.Errorf("upload image response missing media id")
	}
	return strconv.Itoa(envelope.Data.ID), nil
}

func (c *Client) GetTagsTreeV3() ([]models.Tag, error) {
	return c.GetTagsTreeV3Context(context.Background())
}

func (c *Client) GetTagsTreeV3Context(ctx context.Context) ([]models.Tag, error) {
	var envelope struct {
		Data struct {
			List interface{} `json:"list"`
		} `json:"data"`
	}
	if err := c.doV3JSONContext(ctx, http.MethodGet, v3TagsTree, nil, nil, false, &envelope); err != nil {
		return nil, err
	}
	return flattenTags(envelope.Data.List, 0), nil
}

func (c *Client) DownloadImageBinary(id string, pid int32) ([]byte, error) {
	return c.DownloadImageBinaryContext(context.Background(), id, pid)
}

func (c *Client) DownloadImageBinaryContext(ctx context.Context, id string, pid int32) ([]byte, error) {
	q := url.Values{}
	if id != "" {
		q.Set("id", id)
	} else {
		q.Set("pid", strconv.Itoa(int(pid)))
	}
	return c.doV3BinaryContext(ctx, http.MethodGet, v3MediaBinary, q)
}

func (c *Client) DownloadThumbnail(id string, pid int32) ([]byte, error) {
	return c.DownloadThumbnailContext(context.Background(), id, pid)
}

func (c *Client) DownloadThumbnailContext(ctx context.Context, id string, pid int32) ([]byte, error) {
	q := url.Values{}
	if id != "" {
		q.Set("id", id)
	} else {
		q.Set("pid", strconv.Itoa(int(pid)))
	}
	return c.doV3BinaryContext(ctx, http.MethodGet, v3MediaThumbnail, q)
}

func (c *Client) doV3Binary(method, endpoint string, query url.Values) ([]byte, error) {
	return c.doV3BinaryContext(context.Background(), method, endpoint, query)
}

func (c *Client) doV3BinaryContext(ctx context.Context, method, endpoint string, query url.Values) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
	}
	c.applyV3Headers(req, false)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := readBoundedV3Response(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newRemoteAPIError(method, endpoint, resp.StatusCode, payload)
	}
	return payload, nil
}

func (c *Client) doV3JSON(method, endpoint string, query url.Values, body interface{}, write bool, out interface{}) error {
	return c.doV3JSONContext(context.Background(), method, endpoint, query, body, write, out)
}

func (c *Client) doV3JSONContext(ctx context.Context, method, endpoint string, query url.Values, body interface{}, write bool, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
	}
	c.applyV3Headers(req, write)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bodyBytes, err := readBoundedV3Response(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return newRemoteAPIError(method, endpoint, resp.StatusCode, bodyBytes)
	}
	if err := verifyV3SuccessForRequest(method, endpoint, resp.StatusCode, bodyBytes); err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) doV3Multipart(method, endpoint, contentType string, body io.Reader, out interface{}) error {
	return c.doV3MultipartContext(context.Background(), method, endpoint, contentType, body, out)
}

func (c *Client) doV3MultipartContext(ctx context.Context, method, endpoint, contentType string, body io.Reader, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	c.applyV3Headers(req, true)
	req.Header.Set("Content-Type", contentType)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bodyBytes, err := readBoundedV3Response(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return newRemoteAPIError(method, endpoint, resp.StatusCode, bodyBytes)
	}
	if err := verifyV3SuccessForRequest(method, endpoint, resp.StatusCode, bodyBytes); err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return err
		}
	}
	return nil
}

func verifyV3Success(payload []byte) error {
	return verifyV3SuccessForRequest("", "", http.StatusOK, payload)
}

func verifyV3SuccessForRequest(method, endpoint string, status int, payload []byte) error {
	code, message, failed, err := parseV3Envelope(payload)
	if err != nil {
		return err
	}
	if !failed {
		return nil
	}
	return &RemoteAPIError{Operation: method, Endpoint: remoteEndpointPath(endpoint), HTTPStatus: status, Code: code, Message: message}
}

func newRemoteAPIError(method, endpoint string, status int, payload []byte) error {
	code, message, _, _ := parseV3Envelope(payload)
	if message == "" {
		message = http.StatusText(status)
	}
	return &RemoteAPIError{Operation: method, Endpoint: remoteEndpointPath(endpoint), HTTPStatus: status, Code: code, Message: message}
}

func parseV3Envelope(payload []byte) (code, message string, failed bool, err error) {
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(payload, &envelope); err != nil {
		return "", "", false, err
	}
	if raw, ok := envelope["message"]; ok {
		_ = json.Unmarshal(raw, &message)
		message = sanitizeRemoteMessage(message)
	}
	rawCode, ok := envelope["code"]
	if !ok {
		return "", message, false, nil
	}
	var number json.Number
	if err := json.Unmarshal(rawCode, &number); err == nil {
		code = number.String()
	} else {
		_ = json.Unmarshal(rawCode, &code)
	}
	failed = code != "" && code != "20000"
	if failed && message == "" {
		message = "Treehole rejected the request"
	}
	return code, message, failed, nil
}

func sanitizeRemoteMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	const maxLength = 500
	if len(message) > maxLength {
		message = message[:maxLength] + "…"
	}
	return message
}

func remoteEndpointPath(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err == nil && parsed.Path != "" {
		return parsed.Path
	}
	return endpoint
}

func readBoundedV3Response(reader io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxV3ResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxV3ResponseBytes {
		return nil, errors.New("treehole response exceeded the safety limit")
	}
	return payload, nil
}

func (c *Client) applyV3Headers(req *http.Request, write bool) {
	_ = write
	c.applyTreeholeHeaders(req)
}

func (d courseScheduleDTO) toModel() models.CourseScheduleRow {
	return models.CourseScheduleRow{
		TimeNum: parseTimeNum(d.TimeNum),
		Mon:     d.Mon.toModel(),
		Tue:     d.Tue.toModel(),
		Wed:     d.Wed.toModel(),
		Thu:     d.Thu.toModel(),
		Fri:     d.Fri.toModel(),
		Sat:     d.Sat.toModel(),
		Sun:     d.Sun.toModel(),
	}
}

func (d courseDayDTO) toModel() models.CourseDay {
	return models.CourseDay{
		CourseName: firstCourseTitle(d.CourseName),
		Parity:     d.Parity,
		Style:      d.Style,
	}
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

var timeNumPattern = regexp.MustCompile(`第(.+?)节`)

var chineseDigit = map[rune]int{
	'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
	'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
}

func parseTimeNum(raw string) string {
	m := timeNumPattern.FindStringSubmatch(raw)
	if len(m) < 2 {
		return raw
	}
	inner := m[1]
	if n, err := strconv.Atoi(inner); err == nil {
		return strconv.Itoa(n)
	}
	runes := []rune(inner)
	if len(runes) == 1 {
		if v, ok := chineseDigit[runes[0]]; ok {
			return strconv.Itoa(v)
		}
		return raw
	}
	if len(runes) == 2 && runes[0] == '十' {
		if v, ok := chineseDigit[runes[1]]; ok {
			return strconv.Itoa(10 + v)
		}
	}
	if len(runes) == 2 && runes[1] == '十' {
		if v, ok := chineseDigit[runes[0]]; ok {
			return strconv.Itoa(v * 10)
		}
	}
	if len(runes) == 3 && runes[1] == '十' {
		a, ok1 := chineseDigit[runes[0]]
		b, ok2 := chineseDigit[runes[2]]
		if ok1 && ok2 {
			return strconv.Itoa(a*10 + b)
		}
	}
	return raw
}

func firstCourseTitle(raw string) string {
	cleaned := cleanCourseText(raw)
	if cleaned == "" {
		return ""
	}
	for _, line := range strings.Split(cleaned, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "上课信息") || strings.HasPrefix(line, "考试信息") {
			continue
		}
		return line
	}
	return strings.TrimSpace(strings.ReplaceAll(cleaned, "\n", " "))
}

func cleanCourseText(raw string) string {
	text := strings.ReplaceAll(raw, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = htmlTagPattern.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type postDTO struct {
	Pid             int32        `json:"pid"`
	Text            string       `json:"text"`
	Type            string       `json:"type"`
	Timestamp       int32        `json:"timestamp"`
	Hidden          int          `json:"hidden"`
	Reply           int16        `json:"reply"`
	Likenum         int16        `json:"likenum"`
	Extra           int32        `json:"extra"`
	Anonymous       int          `json:"anonymous"`
	Protected       int16        `json:"protected"`
	IsTop           int          `json:"is_top"`
	Label           int16        `json:"label"`
	Status          int16        `json:"status"`
	IsComment       int          `json:"is_comment"`
	TagsIds         string       `json:"tags_ids"`
	AutoTagsIds     string       `json:"auto_tags_ids"`
	MgrTagsIds      string       `json:"mgr_tags_ids"`
	MediaIds        string       `json:"media_ids"`
	Fold            int16        `json:"fold"`
	Kind            int16        `json:"kind"`
	RewardCost      int16        `json:"reward_cost"`
	RewardState     int16        `json:"reward_state"`
	IdentityShow    int          `json:"identity_show"`
	IdentityType    string       `json:"identity_type"`
	IdentityInfo    interface{}  `json:"identity_info"`
	ExclusiveIdId   int16        `json:"exclusive_id_id"`
	ExclusiveIdInfo interface{}  `json:"exclusive_id_info"`
	Mention         string       `json:"mention"`
	Mailbox         int16        `json:"mailbox"`
	ImageSize       string       `json:"image_size"`
	HasRewardGood   int          `json:"has_reward_good"`
	IsGodHole       int          `json:"is_god_hole"`
	IsProtect       int          `json:"is_protect"`
	TreadNum        int16        `json:"tread_num"`
	PraiseNum       int16        `json:"praise_num"`
	PraiseNumShow   int16        `json:"praise_num_show"`
	FoldNum         int16        `json:"fold_num"`
	IsFold          int          `json:"is_fold"`
	CannotReply     int          `json:"cannot_reply"`
	IsFollow        int          `json:"is_follow"`
	IsPraise        int          `json:"is_praise"`
	CommentList     []commentDTO `json:"comment_list"`
}

func (dto postDTO) toModel() models.Post {
	post := models.Post{
		Pid:             dto.Pid,
		Text:            dto.Text,
		Type:            dto.Type,
		Timestamp:       dto.Timestamp,
		Hidden:          models.BoolFromInt(dto.Hidden),
		Reply:           dto.Reply,
		Likenum:         dto.Likenum,
		Extra:           dto.Extra,
		Anonymous:       models.BoolFromInt(dto.Anonymous),
		Protected:       dto.Protected,
		IsTop:           models.BoolFromInt(dto.IsTop),
		Label:           dto.Label,
		Status:          dto.Status,
		IsComment:       models.BoolFromInt(dto.IsComment),
		TagsIds:         dto.TagsIds,
		AutoTagsIds:     dto.AutoTagsIds,
		MgrTagsIds:      dto.MgrTagsIds,
		MediaIds:        dto.MediaIds,
		Fold:            dto.Fold,
		Kind:            dto.Kind,
		RewardCost:      dto.RewardCost,
		RewardState:     dto.RewardState,
		IdentityShow:    models.BoolFromInt(dto.IdentityShow),
		IdentityType:    dto.IdentityType,
		ExclusiveIdId:   dto.ExclusiveIdId,
		Mention:         dto.Mention,
		Mailbox:         dto.Mailbox,
		ImageSize:       dto.ImageSize,
		HasRewardGood:   models.BoolFromInt(dto.HasRewardGood),
		IsGodHole:       models.BoolFromInt(dto.IsGodHole),
		IsProtect:       models.BoolFromInt(dto.IsProtect),
		TreadNum:        dto.TreadNum,
		PraiseNum:       dto.PraiseNum,
		PraiseNumShow:   dto.PraiseNumShow,
		FoldNum:         dto.FoldNum,
		IsFold:          models.BoolFromInt(dto.IsFold),
		CannotReply:     models.BoolFromInt(dto.CannotReply),
		IsFollow:        models.BoolFromInt(dto.IsFollow),
		IsPraise:        models.BoolFromInt(dto.IsPraise),
		IdentityInfo:    jsonString(dto.IdentityInfo),
		ExclusiveIdInfo: jsonString(dto.ExclusiveIdInfo),
	}
	if len(dto.CommentList) > 0 {
		post.Comments = make([]models.Comment, 0, len(dto.CommentList))
		for _, comment := range dto.CommentList {
			post.Comments = append(post.Comments, comment.toModel())
		}
	}
	return post
}

type commentDTO struct {
	Anonymous       int         `json:"anonymous"`
	Cid             int32       `json:"cid"`
	Pid             int32       `json:"pid"`
	ExclusiveIdId   int16       `json:"exclusive_id_id"`
	Hidden          int         `json:"hidden"`
	IdentityShow    int         `json:"identity_show"`
	IdentityType    string      `json:"identity_type"`
	IdentityInfo    interface{} `json:"identity_info"`
	IsAuthor        int         `json:"is_author"`
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

func (dto commentDTO) toModel() models.Comment {
	comment := models.Comment{
		Anonymous:       models.BoolFromInt(dto.Anonymous),
		Cid:             dto.Cid,
		Pid:             dto.Pid,
		ExclusiveIdId:   dto.ExclusiveIdId,
		Hidden:          models.BoolFromInt(dto.Hidden),
		IdentityShow:    models.BoolFromInt(dto.IdentityShow),
		IdentityType:    dto.IdentityType,
		IdentityInfo:    jsonString(dto.IdentityInfo),
		IsAuthor:        models.BoolFromInt(dto.IsAuthor),
		IsLz:            models.BoolFromInt(dto.IsLz),
		Mention:         dto.Mention,
		NameTag:         dto.NameTag,
		RewardGood:      dto.RewardGood,
		Text:            dto.Text,
		Timestamp:       dto.Timestamp,
		MediaIds:        dto.MediaIds,
		ExclusiveIdInfo: jsonString(dto.ExclusiveIdInfo),
	}
	if quote := quoteToModel(dto.Quote); quote != nil {
		comment.Quote = quote
		quoteID := quote.Cid
		comment.QuoteID = &quoteID
	}
	return comment
}

func quoteToModel(v interface{}) *models.Comment {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]interface{}); ok && len(arr) == 0 {
		return nil
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var dto commentDTO
	if err := json.Unmarshal(payload, &dto); err != nil {
		return nil
	}
	quote := dto.toModel()
	return &quote
}

func jsonString(v interface{}) string {
	if v == nil {
		return ""
	}
	if arr, ok := v.([]interface{}); ok && len(arr) == 0 {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	if string(data) == "[]" {
		return ""
	}
	return string(data)
}

func commentIDStringPtr(id *int32) *string {
	if id == nil {
		return nil
	}
	value := strconv.Itoa(int(*id))
	return &value
}

func flattenTags(data interface{}, parentID int) []models.Tag {
	items, ok := data.([]interface{})
	if !ok {
		return nil
	}
	var tags []models.Tag
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		tag := models.Tag{
			ID:       intFromMap(m, "id"),
			Name:     stringFromMap(m, "name"),
			Label:    stringFromMap(m, "label"),
			ParentID: parentID,
		}
		if tag.Name == "" {
			tag.Name = stringFromMap(m, "tag_name")
		}
		if tag.Name == "" {
			tag.Name = tag.Label
		}
		if tag.Label == "" {
			tag.Label = tag.Name
		}
		if tag.ParentID == 0 {
			tag.ParentID = intFromMap(m, "type_id")
		}
		if tag.ID != 0 || tag.Name != "" || tag.Label != "" {
			tags = append(tags, tag)
		}
		for _, key := range []string{"children", "list", "child"} {
			if children, exists := m[key]; exists {
				tags = append(tags, flattenTags(children, tag.ID)...)
			}
		}
	}
	return tags
}

func intFromMap(m map[string]interface{}, key string) int {
	if value, ok := m[key]; ok {
		switch v := value.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			i, _ := strconv.Atoi(v)
			return i
		}
	}
	return 0
}

func stringFromMap(m map[string]interface{}, key string) string {
	if value, ok := m[key]; ok {
		if s, ok := value.(string); ok {
			return s
		}
	}
	return ""
}
