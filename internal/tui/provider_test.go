package tui

import (
	"testing"

	"treehole/internal/models"
)

func TestSplitPIDSearch(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPID     int32
		wantKeyword string
	}{
		{name: "plain keyword", input: "course review", wantKeyword: "course review"},
		{name: "pid only", input: "#8123", wantPID: 8123},
		{name: "pid with keyword", input: "#8123 course review", wantPID: 8123, wantKeyword: "course review"},
		{name: "invalid pid falls back", input: "#abc course", wantKeyword: "#abc course"},
		{name: "non prefix hash falls back", input: "course #8123", wantKeyword: "course #8123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPID, gotKeyword := splitPIDSearch(tc.input)
			if gotPID != tc.wantPID {
				t.Fatalf("pid = %d, want %d", gotPID, tc.wantPID)
			}
			if gotKeyword != tc.wantKeyword {
				t.Fatalf("keyword = %q, want %q", gotKeyword, tc.wantKeyword)
			}
		})
	}
}

func TestParsePostListSearch(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPID      int32
		wantKeyword  string
		wantIsFollow *bool
	}{
		{name: "empty", input: ""},
		{name: "follow only", input: ":follow", wantIsFollow: boolPtr(true)},
		{name: "follow with keyword", input: ":follow course review", wantKeyword: "course review", wantIsFollow: boolPtr(true)},
		{name: "follow with pid", input: "#8123 :follow", wantPID: 8123, wantIsFollow: boolPtr(true)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePostListSearch(tc.input)
			if got.pid != tc.wantPID {
				t.Fatalf("pid = %d, want %d", got.pid, tc.wantPID)
			}
			if got.keyword != tc.wantKeyword {
				t.Fatalf("keyword = %q, want %q", got.keyword, tc.wantKeyword)
			}
			switch {
			case got.isFollow == nil && tc.wantIsFollow == nil:
			case got.isFollow == nil || tc.wantIsFollow == nil:
				t.Fatalf("isFollow = %v, want %v", got.isFollow, tc.wantIsFollow)
			case *got.isFollow != *tc.wantIsFollow:
				t.Fatalf("isFollow = %v, want %v", *got.isFollow, *tc.wantIsFollow)
			}
		})
	}
}

func TestEnrichMentionedPostsFetchesMentionedPost(t *testing.T) {
	posts := []models.Post{
		{Pid: 1, Mention: "42"},
		{Pid: 2, Mention: "42"},
		{Pid: 3, Mention: ""},
	}
	fetchCount := 0

	enrichMentionedPosts(posts, func(pid int32) (*models.Post, error) {
		fetchCount++
		return &models.Post{Pid: pid, Text: "mentioned"}, nil
	})

	if fetchCount != 1 {
		t.Fatalf("fetch count = %d, want one fetch for shared mention", fetchCount)
	}
	if posts[0].MentionedPost == nil || posts[0].MentionedPost.Pid != 42 {
		t.Fatalf("first mentioned post = %+v, want #42", posts[0].MentionedPost)
	}
	if posts[1].MentionedPost == nil || posts[1].MentionedPost.Pid != 42 {
		t.Fatalf("second mentioned post = %+v, want #42", posts[1].MentionedPost)
	}
	if posts[2].MentionedPost != nil {
		t.Fatalf("empty mention should not be enriched: %+v", posts[2].MentionedPost)
	}
}

func TestEnrichMentionedPostsSkipsMissingMentionedPost(t *testing.T) {
	posts := []models.Post{
		{Pid: 1, Mention: "42"},
		{Pid: 2, Mention: "43"},
	}

	enrichMentionedPosts(posts, func(pid int32) (*models.Post, error) {
		if pid == 42 {
			return nil, nil
		}
		return &models.Post{Pid: 999, Text: "wrong post"}, nil
	})

	if posts[0].MentionedPost != nil {
		t.Fatalf("missing mentioned post should not be attached: %+v", posts[0].MentionedPost)
	}
	if posts[1].MentionedPost != nil {
		t.Fatalf("pid mismatch should not be attached: %+v", posts[1].MentionedPost)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
