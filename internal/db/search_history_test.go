package db

import (
	"fmt"
	"testing"
)

func TestSearchHistoryIsNewestFirstAndBounded(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	for i := 0; i < 105; i++ {
		if err := database.RecordSearch(fmt.Sprintf("query-%03d", i), `{}`); err != nil {
			t.Fatalf("RecordSearch(%d): %v", i, err)
		}
	}
	rows, err := database.ListSearchHistory(100)
	if err != nil || len(rows) != 100 {
		t.Fatalf("history length = %d, %v", len(rows), err)
	}
	if rows[0].Query != "query-104" || rows[len(rows)-1].Query != "query-005" {
		t.Fatalf("history bounds = %q ... %q", rows[0].Query, rows[len(rows)-1].Query)
	}
}
