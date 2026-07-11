package service

import (
	"context"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type FullTextRepository interface {
	SearchFullText(query models.FullTextQuery) (models.FullTextPage, error)
}

type SearchIndexRepository interface {
	FullTextRepository
	RebuildSearchIndex() error
}

// SearchService keeps search policy behind its own boundary while sharing the
// same cursor and source semantics as PostService.
type SearchService struct {
	posts      *PostService
	repository FullTextRepository
}

func (s *SearchService) RebuildIndex(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	repository, ok := s.repository.(SearchIndexRepository)
	if !ok || repository == nil {
		return errRepositoryUnavailable
	}
	if err := repository.RebuildSearchIndex(); err != nil {
		return err
	}
	return contextError(ctx)
}

func NewSearchService(posts *PostService, repositories ...FullTextRepository) *SearchService {
	service := &SearchService{posts: posts}
	if len(repositories) > 0 {
		service.repository = repositories[0]
	}
	return service
}

func (s *SearchService) Search(ctx context.Context, query PostQuery) (PostPage, error) {
	if s == nil || s.posts == nil {
		return PostPage{}, errRepositoryUnavailable
	}
	if err := contextError(ctx); err != nil {
		return PostPage{}, err
	}
	if normalizeSource(query.Source) == SourceLocal && s.repository != nil && strings.TrimSpace(query.Query) != "" {
		page, err := s.repository.SearchFullText(models.FullTextQuery{
			Query:    query.Query,
			Offset:   query.Cursor,
			Limit:    query.Limit,
			From:     query.From,
			To:       query.To,
			Sources:  query.Origins,
			HasMedia: query.HasMedia,
			TagIDs:   query.TagIDs,
		})
		if err != nil {
			return PostPage{}, err
		}
		items := make([]PostSummary, len(page.Hits))
		for i, hit := range page.Hits {
			items[i] = PostSummary{
				Post:           hit.Post,
				Snippet:        hit.Snippet,
				Score:          hit.Score,
				CommentMatches: hit.CommentMatches,
			}
		}
		return PostPage{
			Items:      items,
			NextCursor: query.Cursor + len(items),
			HasMore:    page.HasMore,
		}, contextError(ctx)
	}
	return s.posts.Search(ctx, query)
}
