package service

import (
	"context"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type fakeFullTextRepository struct {
	query models.FullTextQuery
	page  models.FullTextPage
}

func (f *fakeFullTextRepository) SearchFullText(query models.FullTextQuery) (models.FullTextPage, error) {
	f.query = query
	return f.page, nil
}

func TestSearchServiceMapsRankedLocalResults(t *testing.T) {
	repository := &fakeFullTextRepository{page: models.FullTextPage{
		Hits: []models.FullTextHit{{
			Post:    models.Post{Pid: 12345, Text: "result"},
			Snippet: "<mark>result</mark>",
			Score:   -2.5,
			CommentMatches: []models.CommentSearchHit{{
				CID: 9, PID: 12345, Snippet: "comment", Score: -1,
			}},
		}},
		HasMore: true,
	}}
	service := NewSearchService(NewPostService(&fakeRepository{}, nil), repository)
	hasMedia := true
	page, err := service.Search(context.Background(), PostQuery{
		Query: "result", Cursor: 10, Limit: 5, Source: SourceLocal,
		Origins: []string{"followed"}, TagIDs: []uint{3}, HasMedia: &hasMedia,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Pid != 12345 || page.Items[0].Snippet == "" || len(page.Items[0].CommentMatches) != 1 {
		t.Fatalf("Search() page = %+v", page)
	}
	if page.NextCursor != 11 || !page.HasMore {
		t.Fatalf("Search() cursor/more = %d/%v", page.NextCursor, page.HasMore)
	}
	if repository.query.Offset != 10 || repository.query.Limit != 5 || len(repository.query.Sources) != 1 || len(repository.query.TagIDs) != 1 {
		t.Fatalf("full-text query = %+v", repository.query)
	}
}
