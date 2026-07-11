package service

import "context"

// SearchService keeps search policy behind its own boundary while sharing the
// same cursor and source semantics as PostService.
type SearchService struct {
	posts *PostService
}

func NewSearchService(posts *PostService) *SearchService {
	return &SearchService{posts: posts}
}

func (s *SearchService) Search(ctx context.Context, query PostQuery) (PostPage, error) {
	if s == nil || s.posts == nil {
		return PostPage{}, errRepositoryUnavailable
	}
	return s.posts.Search(ctx, query)
}
