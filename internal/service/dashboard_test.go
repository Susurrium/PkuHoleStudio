package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type dashboardObserverStub struct {
	result HotPostsResult
	err    error
	calls  int
}

func (s *dashboardObserverStub) HotPosts(context.Context, int, time.Duration) (HotPostsResult, error) {
	s.calls++
	return s.result, s.err
}

type dashboardRecentStub struct{ posts []models.Post }

func (s dashboardRecentStub) RecentLivePosts(context.Context, int) ([]models.Post, error) {
	return s.posts, nil
}

func TestDashboardServiceUsesIndependentObserverHotPosts(t *testing.T) {
	observer := &dashboardObserverStub{result: HotPostsResult{
		Items: []HotPost{{ID: 123456, Text: "hot", FollowNum: 9}}, Source: "observer", WindowHours: 12,
	}}
	service := NewDashboardService(observer)
	result, err := service.HotPosts(context.Background(), 1, 12*time.Hour)
	if err != nil || observer.calls != 1 || len(result.Items) != 1 || result.Items[0].ID != 123456 || result.Source != "observer" {
		t.Fatalf("HotPosts() = %+v, calls=%d, err=%v", result, observer.calls, err)
	}
}

func TestDashboardServiceFallsBackToRecentOnlinePosts(t *testing.T) {
	observer := &dashboardObserverStub{err: errors.New("Observer unavailable")}
	service := NewDashboardService(observer)
	service.SetRecentLiveSource(dashboardRecentStub{posts: []models.Post{
		{Pid: 2, Text: "comments", Timestamp: int32(time.Now().Add(-time.Minute).Unix()), PraiseNum: 3, Reply: 8},
		{Pid: 3, Text: "likes", Timestamp: int32(time.Now().Add(-30 * time.Second).Unix()), PraiseNum: 8, Reply: 1},
	}})
	result, err := service.HotPosts(context.Background(), 5, 12*time.Hour)
	if err != nil || !result.Approximate || result.Source != "live_recent" || len(result.Items) != 2 || result.Items[0].ID != 3 {
		t.Fatalf("HotPosts() = %+v, err=%v", result, err)
	}
}

func TestDashboardServiceUsesLastObserverCacheWhenRemoteFails(t *testing.T) {
	observer := &dashboardObserverStub{result: HotPostsResult{Items: []HotPost{{ID: 123456}}, Source: "observer"}}
	service := NewDashboardService(observer)
	if _, err := service.HotPosts(context.Background(), 5, 12*time.Hour); err != nil {
		t.Fatal(err)
	}
	observer.err = errors.New("temporary failure")
	observer.result = HotPostsResult{}
	result, err := service.HotPosts(context.Background(), 5, 12*time.Hour)
	if err != nil || result.Source != "observer_cache" || !result.Stale || len(result.Items) != 1 {
		t.Fatalf("HotPosts() = %+v, err=%v", result, err)
	}
}

func TestDashboardServiceApproximatesRecentLiveRanking(t *testing.T) {
	now := int32(time.Now().Unix())
	service := NewDashboardService()
	result := service.ApproximateFromRecent([]models.Post{
		{Pid: 1, Text: "older", Timestamp: now - int32((13 * time.Hour).Seconds()), PraiseNum: 99},
		{Pid: 2, Text: "comments", Timestamp: now - 60, PraiseNum: 3, Reply: 8},
		{Pid: 3, Text: "likes", Timestamp: now - 30, PraiseNum: 8, Reply: 1},
	}, 5, 12*time.Hour)
	if !result.Approximate || result.Source != "live_recent" || len(result.Items) != 2 || result.Items[0].ID != 3 || result.Items[1].ID != 2 {
		t.Fatalf("ApproximateFromRecent() = %+v", result)
	}
}
