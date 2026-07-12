package db

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	archivepkg "github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

// TestArchiveScaleFixture is opt-in because it deliberately writes 110,000
// records. CI and release smoke runs can enable it with PKUHOLE_SCALE_TEST=1.
func TestArchiveScaleFixture(t *testing.T) {
	if os.Getenv("PKUHOLE_SCALE_TEST") != "1" {
		t.Skip("set PKUHOLE_SCALE_TEST=1 to run the 10k/100k archive fixture")
	}
	const postCount, commentsPerPost = 10_000, 10
	type hole struct {
		PID  int32  `json:"pid"`
		Text string `json:"text"`
	}
	type comment struct {
		CID  int32  `json:"cid"`
		Text string `json:"text"`
	}
	holes := make([]hole, postCount)
	comments := make([][]comment, postCount)
	var cid int32
	for index := 0; index < postCount; index++ {
		holes[index] = hole{PID: int32(10_000 + index), Text: "synthetic searchable archive text"}
		comments[index] = make([]comment, commentsPerPost)
		for commentIndex := range comments[index] {
			cid++
			comments[index][commentIndex] = comment{CID: cid, Text: "synthetic comment evidence"}
		}
	}
	encoded, err := json.Marshal(map[string]any{"holes": holes, "comments": comments})
	if err != nil {
		t.Fatal(err)
	}
	database, cleanup := setupTestDB(t)
	defer cleanup()
	report, err := archivepkg.NewImporter(database).Import(context.Background(), bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil || report.Counts.ValidItems != postCount || report.Counts.Comments != postCount*commentsPerPost {
		t.Fatalf("large import = %+v, %v", report, err)
	}
	posts, _ := database.GetPostCount()
	commentsCount, _ := database.GetCommentCount()
	if posts != postCount || commentsCount != postCount*commentsPerPost {
		t.Fatalf("database counts = %d/%d", posts, commentsCount)
	}
	if available, _ := database.FTS5Available(); available {
		if err := database.RebuildSearchIndex(); err != nil {
			t.Fatal(err)
		}
		query := models.FullTextQuery{Query: "searchable archive", Limit: 10}
		started := time.Now()
		postMatches, commentMatches, err := database.searchFTS(query, parsePostSearchQuery(query.Query))
		searchElapsed := time.Since(started)
		started = time.Now()
		page, buildErr := database.buildSearchPage(query, postMatches, commentMatches)
		elapsed := searchElapsed + time.Since(started)
		if err == nil {
			err = buildErr
		}
		if err != nil || len(page.Hits) == 0 {
			t.Fatalf("large FTS query = %+v, %v", page, err)
		}
		t.Logf("large FTS first page returned in %s (SQL %s, posts=%d comments=%d)", elapsed, searchElapsed, len(postMatches), len(commentMatches))
		if elapsed > 500*time.Millisecond {
			t.Fatalf("large FTS query took %s, want <= 500ms", elapsed)
		}
	}
}
