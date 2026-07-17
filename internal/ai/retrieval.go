package ai

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

const (
	maxResearchQueryVariants = 8
	reciprocalRankConstant   = 60.0
	maxCommentsPerSearchHit  = 3
)

type researchQueryVariant struct {
	Query  string
	Weight float64
}

type researchRanking struct {
	Items  []service.PostSummary
	Weight float64
}

type fusedComment struct {
	hit   models.CommentSearchHit
	score float64
	rank  int
}

type fusedPost struct {
	item     service.PostSummary
	score    float64
	bestRank int
	comments map[int32]*fusedComment
}

var researchTermGroups = [][]string{
	{"给分", "成绩", "绩点", "分数", "背刺", "捞"},
	{"作业量", "大作业", "小作业", "作业", "ddl"},
	{"考试", "考核", "期中", "期末", "开卷", "闭卷"},
	{"难度", "水课", "硬核", "轻松", "好过", "任务重"},
	{"教学", "讲课", "上课", "老师"},
	{"选课", "抽签", "补退选", "退课"},
}

// expandResearchQueries broadens colloquial Treehole vocabulary without
// changing the public full-text query syntax. The original query always has
// the highest weight; deterministic variants are only additional candidates.
func expandResearchQueries(raw string) []researchQueryVariant {
	original := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if original == "" {
		return nil
	}
	result := []researchQueryVariant{{Query: original, Weight: 1}}
	seen := map[string]bool{strings.ToLower(original): true}
	appendVariant := func(value string, weight float64) {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		key := strings.ToLower(value)
		if value == "" || seen[key] || len(result) >= maxResearchQueryVariants {
			return
		}
		seen[key] = true
		result = append(result, researchQueryVariant{Query: value, Weight: weight})
	}

	for _, group := range researchTermGroups {
		matched := ""
		for _, term := range group {
			if strings.Contains(strings.ToLower(original), strings.ToLower(term)) && utf8.RuneCountInString(term) > utf8.RuneCountInString(matched) {
				matched = term
			}
		}
		if matched == "" {
			continue
		}
		for _, alternate := range group {
			if !strings.EqualFold(matched, alternate) {
				appendVariant(replaceAllFold(original, matched, alternate), 0.82)
			}
		}
	}

	// Removing conversational filler supplies a broader fallback for prompts
	// such as “高数给分怎么样”, while keeping it below exact and synonym hits.
	broader := original
	for _, filler := range []string{"怎么样", "如何", "请问", "帮我", "总结一下", "总结", "评价一下"} {
		broader = strings.ReplaceAll(broader, filler, "")
	}
	if utf8.RuneCountInString(strings.TrimSpace(broader)) >= 2 {
		appendVariant(broader, 0.55)
	}
	return result
}

func replaceAllFold(value, old, replacement string) string {
	if old == "" {
		return value
	}
	var result strings.Builder
	remaining := value
	oldLower := strings.ToLower(old)
	for {
		index := strings.Index(strings.ToLower(remaining), oldLower)
		if index < 0 {
			result.WriteString(remaining)
			return result.String()
		}
		result.WriteString(remaining[:index])
		result.WriteString(replacement)
		remaining = remaining[index+len(old):]
	}
}

func (s *Service) searchResearchArchive(ctx context.Context, request service.AIRequest, rawQuery string, limit int) ([]service.PostSummary, []researchQueryVariant, error) {
	variants := expandResearchQueries(rawQuery)
	if len(variants) == 0 {
		return []service.PostSummary{}, nil, nil
	}
	candidateLimit := limit * 4
	if candidateLimit < 20 {
		candidateLimit = 20
	}
	if candidateLimit > 60 {
		candidateLimit = 60
	}
	rankings := make([]researchRanking, 0, len(variants))
	for _, variant := range variants {
		page, err := s.search.Search(ctx, scopedPostQuery(request, variant.Query, candidateLimit))
		if err != nil {
			return nil, variants, err
		}
		rankings = append(rankings, researchRanking{Items: page.Items, Weight: variant.Weight})
	}
	return fuseResearchRankings(rankings, limit), variants, nil
}

func fuseResearchRankings(rankings []researchRanking, limit int) []service.PostSummary {
	if limit <= 0 {
		limit = 8
	}
	posts := make(map[int32]*fusedPost)
	for _, ranking := range rankings {
		weight := ranking.Weight
		if weight <= 0 {
			weight = 1
		}
		for rank, item := range ranking.Items {
			entry := posts[item.Pid]
			if entry == nil {
				entry = &fusedPost{item: item, bestRank: rank, comments: make(map[int32]*fusedComment)}
				entry.item.CommentMatches = nil
				posts[item.Pid] = entry
			} else {
				if rank < entry.bestRank {
					entry.bestRank = rank
				}
				if strings.TrimSpace(entry.item.Snippet) == "" && strings.TrimSpace(item.Snippet) != "" {
					entry.item.Snippet = item.Snippet
				}
			}
			entry.score += weight / (reciprocalRankConstant + float64(rank+1))
			for commentRank, match := range item.CommentMatches {
				comment := entry.comments[match.CID]
				combinedRank := rank + commentRank
				if comment == nil {
					comment = &fusedComment{hit: match, rank: combinedRank}
					entry.comments[match.CID] = comment
				} else if combinedRank < comment.rank {
					comment.rank = combinedRank
					comment.hit = match
				}
				comment.score += weight / (reciprocalRankConstant + float64(combinedRank+1))
			}
		}
	}

	ordered := make([]*fusedPost, 0, len(posts))
	for _, entry := range posts {
		comments := make([]*fusedComment, 0, len(entry.comments))
		for _, comment := range entry.comments {
			comments = append(comments, comment)
		}
		sort.SliceStable(comments, func(i, j int) bool {
			if comments[i].score == comments[j].score {
				if comments[i].rank == comments[j].rank {
					return comments[i].hit.CID < comments[j].hit.CID
				}
				return comments[i].rank < comments[j].rank
			}
			return comments[i].score > comments[j].score
		})
		for index := 0; index < len(comments) && index < maxCommentsPerSearchHit; index++ {
			entry.item.CommentMatches = append(entry.item.CommentMatches, comments[index].hit)
		}
		entry.item.Score = -entry.score
		ordered = append(ordered, entry)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].score == ordered[j].score {
			if ordered[i].bestRank == ordered[j].bestRank {
				return ordered[i].item.Pid > ordered[j].item.Pid
			}
			return ordered[i].bestRank < ordered[j].bestRank
		}
		return ordered[i].score > ordered[j].score
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	result := make([]service.PostSummary, len(ordered))
	for index, entry := range ordered {
		result[index] = entry.item
	}
	return result
}
