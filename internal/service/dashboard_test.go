package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDashboardServiceLoadsBoundedRecentHotPosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("limit") != "1" || request.URL.Query().Get("order_by") != "likenum" || request.URL.Query().Get("start_time") == "" || request.URL.Query().Get("end_time") == "" {
			t.Fatalf("query = %s", request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{"status":0,"data":[{"id":123456,"text":"hot","follownum":9},{"id":234567,"text":"extra"}]}`))
	}))
	defer server.Close()
	service := newDashboardService(server.URL, server.Client())
	items, err := service.HotPosts(context.Background(), 1, 12*time.Hour)
	if err != nil || len(items) != 1 || items[0].ID != 123456 || items[0].FollowNum != 9 {
		t.Fatalf("HotPosts() = %+v, %v", items, err)
	}
}
