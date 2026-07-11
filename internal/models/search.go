package models

type FullTextQuery struct {
	Query    string
	Offset   int
	Limit    int
	From     int64
	To       int64
	Sources  []string
	HasMedia *bool
	TagIDs   []uint
}

type CommentSearchHit struct {
	CID     int32   `json:"cid"`
	PID     int32   `json:"pid"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

type FullTextHit struct {
	Post           Post               `json:"post"`
	Snippet        string             `json:"snippet,omitempty"`
	Score          float64            `json:"score"`
	CommentMatches []CommentSearchHit `json:"comment_matches,omitempty"`
}

type FullTextPage struct {
	Hits    []FullTextHit `json:"hits"`
	Total   int           `json:"total"`
	HasMore bool          `json:"has_more"`
}
