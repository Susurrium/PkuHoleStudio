package models

type CourseDay struct {
	CourseName string `json:"courseName"`
	Parity     string `json:"parity"`
	Style      string `json:"sty"`
}

type CourseScheduleRow struct {
	TimeNum string    `json:"time_num"`
	Mon     CourseDay `json:"mon"`
	Tue     CourseDay `json:"tue"`
	Wed     CourseDay `json:"wed"`
	Thu     CourseDay `json:"thu"`
	Fri     CourseDay `json:"fri"`
	Sat     CourseDay `json:"sat"`
	Sun     CourseDay `json:"sun"`
}

type CourseScore struct {
	YearTerm string `json:"year_term"`
	Name     string `json:"name"`
	Credit   string `json:"credit"`
	Score    string `json:"score"`
	Category string `json:"category"`
}

type GPATerm struct {
	YearTerm string `json:"year_term"`
	GPA      string `json:"gpa"`
}

type ScoreSummary struct {
	GPA          string        `json:"gpa"`
	TotalCredit  string        `json:"total_credit"`
	PassedCredit string        `json:"passed_credit"`
	CourseCount  string        `json:"course_count"`
	Scores       []CourseScore `json:"scores"`
	GPATerms     []GPATerm     `json:"gpa_terms"`
}
