package service

import (
	"context"
	"errors"
	"math"
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
		{Pid: 2, Text: "praise is excluded", Timestamp: int32(time.Now().Add(-time.Minute).Unix()), Likenum: 5, PraiseNum: 999},
		{Pid: 3, Text: "more follows", Timestamp: int32(time.Now().Add(-30 * time.Second).Unix()), Likenum: 8, PraiseNum: 1},
	}})
	result, err := service.HotPosts(context.Background(), 5, 12*time.Hour)
	if err != nil || !result.Approximate || result.Source != "live_recent" || result.ScoreVersion != engagementV2ScoreVersion || len(result.Items) != 2 || result.Items[0].ID != 3 {
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
		{Pid: 1, Text: "outside window", Timestamp: now - int32((13 * time.Hour).Seconds()), Likenum: 500, PraiseNum: 500, Reply: 500},
		{Pid: 2, Text: "discussion", Timestamp: now - 60, Likenum: 7, PraiseNum: 999, Reply: 100},
		{Pid: 3, Text: "follows", Timestamp: now - 30, Likenum: 40, PraiseNum: 1},
		{Pid: 4, Text: "praise only", Timestamp: now - 10, PraiseNum: 5_000},
	}, 5, 12*time.Hour)
	if !result.Approximate || result.Source != "live_recent" || result.WindowHours != 12 || result.ScoreVersion != engagementV2ScoreVersion || len(result.Items) != 3 {
		t.Fatalf("ApproximateFromRecent() = %+v", result)
	}
	if result.Items[0].ID != 3 || result.Items[0].FollowNum != 40 || result.Items[0].PraiseNum != 1 || result.Items[0].Score != 40 {
		t.Fatalf("first item = %+v", result.Items[0])
	}
	if result.Items[1].ID != 2 || result.Items[1].FollowNum != 7 || result.Items[1].PraiseNum != 999 || result.Items[1].ReplyNum != 100 || math.Abs(result.Items[1].Score-37) > 0.001 {
		t.Fatalf("second item = %+v", result.Items[1])
	}
	if result.Items[2].ID != 4 || result.Items[2].FollowNum != 0 || result.Items[2].PraiseNum != 5_000 || result.Items[2].Score != 0 {
		t.Fatalf("third item = %+v", result.Items[2])
	}
}

func TestEngagementV2ScoreMatchesObserverAnchorsAndExcludesPraise(t *testing.T) {
	tests := []struct {
		name string
		post models.Post
		want float64
	}{
		{name: "forty replies", post: models.Post{Reply: 40, PraiseNum: 999}, want: 15.1789},
		{name: "hundred replies", post: models.Post{Reply: 100, PraiseNum: 999}, want: 30},
		{name: "follows and replies", post: models.Post{Likenum: 7, Reply: 100, PraiseNum: 999}, want: 37},
		{name: "follows only", post: models.Post{Likenum: 40, PraiseNum: 999}, want: 40},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := engagementV2Score(test.post); math.Abs(got-test.want) > 0.001 {
				t.Fatalf("engagementV2Score() = %f, want %f", got, test.want)
			}
		})
	}
}
