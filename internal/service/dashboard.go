package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type HotPost struct {
	ID                int32   `json:"id"`
	Text              string  `json:"text"`
	FollowNum         int     `json:"follownum"`
	ReplyNum          int     `json:"reply,omitempty"`
	Timestamp         int64   `json:"timestamp,omitempty"`
	Score             float64 `json:"score,omitempty"`
	AvailabilityState string  `json:"availability_state,omitempty"`
}

type HotPostsResult struct {
	Items                []HotPost `json:"items"`
	Source               string    `json:"source"`
	WindowHours          int       `json:"window_hours"`
	UpdatedAt            int64     `json:"updated_at"`
	LatestTimestamp      int64     `json:"latest_timestamp,omitempty"`
	Stale                bool      `json:"stale"`
	Approximate          bool      `json:"approximate"`
	Message              string    `json:"message,omitempty"`
	GeneratedAt          string    `json:"generated_at,omitempty"`
	LastSuccessfulScanAt string    `json:"last_successful_scan_at,omitempty"`
	LatestPostAt         string    `json:"latest_post_at,omitempty"`
	ScoreVersion         string    `json:"score_version,omitempty"`
}

type ObserverHotSource interface {
	HotPosts(context.Context, int, time.Duration) (HotPostsResult, error)
}

type RecentLivePostSource interface {
	RecentLivePosts(context.Context, int) ([]models.Post, error)
}

type DashboardService struct {
	observer ObserverHotSource
	cacheMu  sync.RWMutex
	cache    HotPostsResult
	recent   RecentLivePostSource
}

func NewDashboardService(observer ...ObserverHotSource) *DashboardService {
	service := &DashboardService{}
	if len(observer) > 0 {
		service.observer = observer[0]
	}
	return service
}

func (s *DashboardService) SetRecentLiveSource(source RecentLivePostSource) {
	if s != nil {
		s.recent = source
	}
}

func (s *DashboardService) HotPosts(ctx context.Context, limit int, window time.Duration) (HotPostsResult, error) {
	if s != nil && s.observer != nil {
		observerResult, observerErr := s.observer.HotPosts(ctx, limit, window)
		if observerErr == nil {
			s.cacheMu.Lock()
			s.cache = observerResult
			s.cacheMu.Unlock()
			if !observerResult.Stale && len(observerResult.Items) > 0 {
				return observerResult, nil
			}
		}
		if s.recent != nil {
			recentContext, cancel := context.WithTimeout(ctx, 8*time.Second)
			posts, liveErr := s.recent.RecentLivePosts(recentContext, 100)
			cancel()
			if liveErr == nil {
				live := s.ApproximateFromRecent(posts, limit, window)
				if len(live.Items) > 0 {
					return live, nil
				}
			}
		}
		if observerErr == nil {
			return observerResult, nil
		}
		s.cacheMu.RLock()
		cached := s.cache
		s.cacheMu.RUnlock()
		if len(cached.Items) > 0 {
			cached.Stale = true
			cached.Source = "observer_cache"
			cached.Message = "Observer is temporarily unavailable; showing its last cached result."
			return cached, nil
		}
		return HotPostsResult{}, observerErr
	}
	return HotPostsResult{}, errors.New("dashboard service is not configured")
}

func (s *DashboardService) ApproximateFromRecent(posts []models.Post, limit int, window time.Duration) HotPostsResult {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	if window <= 0 {
		window = 12 * time.Hour
	}
	now := time.Now()
	cutoff := now.Add(-window).Unix()
	candidates := make([]models.Post, 0, len(posts))
	for _, post := range posts {
		if post.Pid > 0 && int64(post.Timestamp) >= cutoff {
			candidates = append(candidates, post)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		leftPraise := max(int(candidates[i].PraiseNum), int(candidates[i].Likenum))
		rightPraise := max(int(candidates[j].PraiseNum), int(candidates[j].Likenum))
		if leftPraise != rightPraise {
			return leftPraise > rightPraise
		}
		if candidates[i].Reply != candidates[j].Reply {
			return candidates[i].Reply > candidates[j].Reply
		}
		return candidates[i].Timestamp > candidates[j].Timestamp
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	items := make([]HotPost, 0, len(candidates))
	for _, post := range candidates {
		items = append(items, HotPost{
			ID:        post.Pid,
			Text:      post.Text,
			FollowNum: max(int(post.PraiseNum), int(post.Likenum)),
			ReplyNum:  int(post.Reply),
			Timestamp: int64(post.Timestamp),
		})
	}
	result := hotPostsResult(items, "live_recent", window, now)
	result.Approximate = true
	result.Message = "Observer 热榜不可用，已根据在线时间线的近期内容按点赞和评论近似排序。"
	return result
}

func hotPostsResult(items []HotPost, source string, window time.Duration, now time.Time) HotPostsResult {
	if items == nil {
		items = []HotPost{}
	}
	latest := int64(0)
	for _, item := range items {
		if item.Timestamp > latest {
			latest = item.Timestamp
		}
	}
	result := HotPostsResult{
		Items:           items,
		Source:          source,
		WindowHours:     max(1, int(window.Round(time.Hour)/time.Hour)),
		UpdatedAt:       now.Unix(),
		LatestTimestamp: latest,
	}
	if latest > 0 && latest < now.Add(-window).Unix() {
		result.Stale = true
	}
	return result
}
