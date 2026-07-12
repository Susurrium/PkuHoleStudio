package db

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type postMatchRow struct {
	PID     int32 `gorm:"column:pid"`
	Score   float64
	Snippet string
}

type commentMatchRow struct {
	CID     int32 `gorm:"column:cid"`
	PID     int32 `gorm:"column:pid"`
	Score   float64
	Snippet string
}

type accumulatedSearchHit struct {
	hit      models.FullTextHit
	hasScore bool
}

func (d *Database) SearchFullText(query models.FullTextQuery) (models.FullTextPage, error) {
	query.Query = strings.TrimSpace(query.Query)
	if query.Limit <= 0 {
		query.Limit = 25
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	parsed := parsePostSearchQuery(query.Query)
	if parsed.pid == nil && len(parsed.keywords) == 0 {
		return models.FullTextPage{Hits: []models.FullTextHit{}}, nil
	}

	available, err := d.FTS5Available()
	if err != nil {
		return models.FullTextPage{}, err
	}
	useFTS := available && len(parsed.keywords) > 0 && !hasShortSearchToken(parsed.keywords)
	var posts []postMatchRow
	var comments []commentMatchRow
	if useFTS {
		posts, comments, err = d.searchFTS(query, parsed)
	} else {
		posts, comments, err = d.searchLike(query, parsed)
	}
	if err != nil {
		return models.FullTextPage{}, err
	}
	return d.buildSearchPage(query, posts, comments)
}

func (d *Database) searchFTS(query models.FullTextQuery, parsed postSearchQuery) ([]postMatchRow, []commentMatchRow, error) {
	expression := ftsExpression(parsed.keywords)
	filter, args := searchFilterSQL(query, parsed.pid, "p")
	postArgs := append([]any{expression}, args...)
	postSQL := `SELECT CAST(posts_fts.pid AS INTEGER) AS pid,
		bm25(posts_fts) AS score,
		snippet(posts_fts, 1, '<mark>', '</mark>', '…', 24) AS snippet
		FROM posts_fts JOIN posts p ON p.pid = CAST(posts_fts.pid AS INTEGER)
		WHERE posts_fts MATCH ?` + filter
	var posts []postMatchRow
	if err := d.db.Raw(postSQL, postArgs...).Scan(&posts).Error; err != nil {
		return nil, nil, err
	}

	commentArgs := append([]any{expression}, args...)
	commentSQL := `SELECT CAST(comments_fts.cid AS INTEGER) AS cid,
		CAST(comments_fts.pid AS INTEGER) AS pid,
		bm25(comments_fts) AS score,
		snippet(comments_fts, 2, '<mark>', '</mark>', '…', 24) AS snippet
		FROM comments_fts
		JOIN posts p ON p.pid = CAST(comments_fts.pid AS INTEGER)
		WHERE comments_fts MATCH ?` + filter + ` ORDER BY score ASC, cid ASC`
	var comments []commentMatchRow
	if err := d.db.Raw(commentSQL, commentArgs...).Scan(&comments).Error; err != nil {
		return nil, nil, err
	}
	return posts, comments, nil
}

func (d *Database) searchLike(query models.FullTextQuery, parsed postSearchQuery) ([]postMatchRow, []commentMatchRow, error) {
	filter, filterArgs := searchFilterSQL(query, parsed.pid, "p")
	postWhere, postArgs := likeKeywordSQL(parsed.keywords, "p.text")
	postSQL := `SELECT p.pid, 0.0 AS score, p.text AS snippet FROM posts p WHERE 1=1` + filter + postWhere
	args := append(append([]any{}, filterArgs...), postArgs...)
	var posts []postMatchRow
	if err := d.db.Raw(postSQL, args...).Scan(&posts).Error; err != nil {
		return nil, nil, err
	}

	commentWhere, commentArgs := likeCommentKeywordSQL(parsed.keywords)
	commentSQL := `SELECT c.cid, c.pid, 0.0 AS score, c.text AS snippet
		FROM comments c JOIN posts p ON p.pid = c.pid WHERE 1=1` + filter + commentWhere + ` ORDER BY c.cid ASC`
	args = append(append([]any{}, filterArgs...), commentArgs...)
	var comments []commentMatchRow
	if len(parsed.keywords) > 0 {
		if err := d.db.Raw(commentSQL, args...).Scan(&comments).Error; err != nil {
			return nil, nil, err
		}
	}
	return posts, comments, nil
}

func (d *Database) buildSearchPage(query models.FullTextQuery, posts []postMatchRow, comments []commentMatchRow) (models.FullTextPage, error) {
	accumulated := make(map[int32]*accumulatedSearchHit, len(posts))
	for _, match := range posts {
		entry := accumulated[match.PID]
		if entry == nil {
			entry = &accumulatedSearchHit{}
			accumulated[match.PID] = entry
		}
		entry.hit.Snippet = match.Snippet
		entry.hit.Score = match.Score
		entry.hasScore = true
	}
	for _, match := range comments {
		entry := accumulated[match.PID]
		if entry == nil {
			entry = &accumulatedSearchHit{}
			accumulated[match.PID] = entry
		}
		if !entry.hasScore || match.Score < entry.hit.Score {
			entry.hit.Score = match.Score
			entry.hasScore = true
		}
		if len(entry.hit.CommentMatches) < 10 {
			entry.hit.CommentMatches = append(entry.hit.CommentMatches, models.CommentSearchHit{
				CID: match.CID, PID: match.PID, Snippet: match.Snippet, Score: match.Score,
			})
		}
	}
	if len(accumulated) == 0 {
		return models.FullTextPage{Hits: []models.FullTextHit{}}, nil
	}

	pids := make([]int32, 0, len(accumulated))
	for pid := range accumulated {
		pids = append(pids, pid)
	}
	var postModels []models.Post
	if err := d.db.Where("pid IN ?", pids).Find(&postModels).Error; err != nil {
		return models.FullTextPage{}, err
	}
	for _, post := range postModels {
		entry := accumulated[post.Pid]
		entry.hit.Post = post
	}
	hits := make([]models.FullTextHit, 0, len(postModels))
	for _, entry := range accumulated {
		if entry.hit.Post.Pid != 0 {
			hits = append(hits, entry.hit)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Post.Pid > hits[j].Post.Pid
		}
		return hits[i].Score < hits[j].Score
	})
	total := len(hits)
	if query.Offset >= total {
		return models.FullTextPage{Hits: []models.FullTextHit{}, Total: total}, nil
	}
	end := query.Offset + query.Limit
	if end > total {
		end = total
	}
	page := append([]models.FullTextHit(nil), hits[query.Offset:end]...)
	return models.FullTextPage{Hits: page, Total: total, HasMore: end < total}, nil
}

func searchFilterSQL(query models.FullTextQuery, pid *int32, postAlias string) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0)
	if len(query.Sources) == 0 {
		builder.WriteString(" AND (NOT EXISTS (SELECT 1 FROM post_sources visibility_source WHERE visibility_source.pid = " + postAlias + ".pid) OR EXISTS (SELECT 1 FROM post_sources visibility_source WHERE visibility_source.pid = " + postAlias + ".pid AND visibility_source.context_only = false))")
	}
	if pid != nil {
		builder.WriteString(" AND " + postAlias + ".pid = ?")
		args = append(args, *pid)
	}
	if query.From > 0 {
		builder.WriteString(" AND " + postAlias + ".timestamp >= ?")
		args = append(args, query.From)
	}
	if query.To > 0 {
		builder.WriteString(" AND " + postAlias + ".timestamp <= ?")
		args = append(args, query.To)
	}
	if query.HasMedia != nil {
		if *query.HasMedia {
			builder.WriteString(" AND (" + postAlias + ".type = 'image' OR COALESCE(" + postAlias + ".media_ids, '') <> '')")
		} else {
			builder.WriteString(" AND " + postAlias + ".type <> 'image' AND COALESCE(" + postAlias + ".media_ids, '') = ''")
		}
	}
	if len(query.Sources) > 0 {
		builder.WriteString(" AND EXISTS (SELECT 1 FROM post_sources ps WHERE ps.pid = " + postAlias + ".pid AND ps.source IN (")
		appendPlaceholders(&builder, len(query.Sources))
		builder.WriteString("))")
		for _, source := range query.Sources {
			args = append(args, source)
		}
	}
	if len(query.TagIDs) > 0 {
		builder.WriteString(" AND EXISTS (SELECT 1 FROM post_tags pt WHERE pt.pid = " + postAlias + ".pid AND pt.tag_id IN (")
		appendPlaceholders(&builder, len(query.TagIDs))
		builder.WriteString("))")
		for _, tagID := range query.TagIDs {
			args = append(args, tagID)
		}
	}
	return builder.String(), args
}

func appendPlaceholders(builder *strings.Builder, count int) {
	for i := 0; i < count; i++ {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteByte('?')
	}
}

func likeKeywordSQL(keywords []string, column string) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, len(keywords))
	for _, keyword := range keywords {
		builder.WriteString(" AND " + column + ` LIKE ? ESCAPE '\'`)
		args = append(args, searchLikePattern(keyword))
	}
	return builder.String(), args
}

func likeCommentKeywordSQL(keywords []string) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, len(keywords)*2)
	for _, keyword := range keywords {
		builder.WriteString(` AND (c.text LIKE ? ESCAPE '\' OR c.name_tag LIKE ? ESCAPE '\')`)
		pattern := searchLikePattern(keyword)
		args = append(args, pattern, pattern)
	}
	return builder.String(), args
}

func searchLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return "%" + value + "%"
}

func hasShortSearchToken(keywords []string) bool {
	for _, keyword := range keywords {
		if utf8.RuneCountInString(keyword) < 3 {
			return true
		}
	}
	return false
}

func ftsExpression(keywords []string) string {
	parts := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		parts = append(parts, `"`+strings.ReplaceAll(keyword, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " AND ")
}
