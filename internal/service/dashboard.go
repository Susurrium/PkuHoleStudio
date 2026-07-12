package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultHotPostsEndpoint = "https://treeholestat.dfshfghj.workers.dev/posts"

type HotPost struct {
	ID        int32  `json:"id"`
	Text      string `json:"text"`
	FollowNum int    `json:"follownum"`
}

type DashboardService struct {
	endpoint string
	client   *http.Client
}

func NewDashboardService() *DashboardService {
	return &DashboardService{endpoint: defaultHotPostsEndpoint, client: &http.Client{Timeout: 5 * time.Second}}
}

func newDashboardService(endpoint string, client *http.Client) *DashboardService {
	return &DashboardService{endpoint: endpoint, client: client}
}

func (s *DashboardService) HotPosts(ctx context.Context, limit int, window time.Duration) ([]HotPost, error) {
	if s == nil || s.client == nil || s.endpoint == "" {
		return nil, errors.New("dashboard service is not configured")
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	if window <= 0 {
		window = 12 * time.Hour
	}
	now := time.Now().Unix()
	parsed, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("limit", strconv.Itoa(limit))
	query.Set("order_by", "likenum")
	query.Set("end_time", strconv.FormatInt(now, 10))
	query.Set("start_time", strconv.FormatInt(now-int64(window/time.Second), 10))
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("hot posts request failed: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Status int       `json:"status"`
		Data   []HotPost `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != 0 && payload.Status != http.StatusOK {
		return nil, fmt.Errorf("hot posts request failed: status %d", payload.Status)
	}
	if len(payload.Data) > limit {
		payload.Data = payload.Data[:limit]
	}
	return payload.Data, nil
}
