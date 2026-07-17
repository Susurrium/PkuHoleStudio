package ai

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
)

func TestBuildEvidenceReportBindsCitationsToTheirOwnClaim(t *testing.T) {
	cid := int32(11)
	answer := "## 结论\n作业很多。[#100/C11] 考试很难。\n资料不足，无法判断给分。\n| 教师 | 评价 |\n| --- | --- |\n| 张老师 | 讲课清晰 [#100] |"
	report := buildEvidenceReport(answer, []sourceRef{
		{PID: 100, Origin: service.SourceLocal, Snippet: "张老师讲课清晰"},
		{PID: 100, CID: &cid, Origin: service.SourceLocal, Snippet: "作业很多"},
	})
	if len(report.Claims) != 4 {
		t.Fatalf("claims = %+v", report.Claims)
	}
	if report.Claims[0].Status != claimStatusUnverified || len(report.Claims[0].Sources) != 1 || report.Claims[0].Sources[0].CID == nil || *report.Claims[0].Sources[0].CID != cid {
		t.Fatalf("cited claim = %+v", report.Claims[0])
	}
	if report.Claims[1].Status != claimStatusUnsupported {
		t.Fatalf("uncited claim = %+v", report.Claims[1])
	}
	if report.Claims[2].Status != claimStatusInsufficient {
		t.Fatalf("insufficient claim = %+v", report.Claims[2])
	}
	if report.Claims[3].Status != claimStatusUnverified || report.Claims[3].Text != "张老师：讲课清晰" {
		t.Fatalf("table claim = %+v", report.Claims[3])
	}
	if report.Summary.Total != 4 || report.Summary.Cited != 2 || report.Summary.Unsupported != 1 || report.Summary.Insufficient != 1 || math.Abs(report.Summary.CitationCoverage-2.0/3.0) > 0.001 {
		t.Fatalf("summary = %+v", report.Summary)
	}
}

func TestVerifyEvidenceReportPersistsSemanticSupportStatuses(t *testing.T) {
	cid := int32(21)
	sources := []sourceRef{
		{PID: 200, Origin: service.SourceLocal, Snippet: "考试为开卷"},
		{PID: 200, CID: &cid, Origin: service.SourceLocal, Snippet: "作业有三次，其中一次较难"},
	}
	report := buildEvidenceReport("考试为开卷。[#200]\n作业都很难。[#200/C21]", sources)
	provider := &fakeProvider{chat: ChatResponse{ToolCalls: []ToolCall{{
		ID: "check", Type: "function",
		Function: ToolCallFunction{Name: "record_evidence_assessment", Arguments: `{"assessments":[{"ordinal":1,"status":"supported","reason":"引文明确说明开卷"},{"ordinal":2,"status":"partial","reason":"只说明一次作业较难"}]}`},
	}}}}
	cfg := config.DefaultConfig().AI
	verified, err := verifyEvidenceReport(context.Background(), report, sources, providerRuntime{provider: provider, config: cfg, info: ProviderInfo{Name: "fake", Model: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Checked || verified.Summary.Supported != 1 || verified.Summary.Partial != 1 || verified.Summary.Unverified != 0 {
		t.Fatalf("verified report = %+v", verified)
	}
	if len(provider.chatRequests) != 1 || len(provider.chatRequests[0].Tools) != 1 || provider.chatRequests[0].Tools[0].Function.Name != "record_evidence_assessment" {
		t.Fatalf("verification request = %+v", provider.chatRequests)
	}
}

func TestBuildEvidenceReportSplitsEnglishSentencesWithoutSplittingDecimals(t *testing.T) {
	report := buildEvidenceReport("The average is 3.8. [#300] The exam is difficult.", []sourceRef{{PID: 300, Origin: service.SourceLocal, Snippet: "average 3.8"}})
	if len(report.Claims) != 2 || report.Claims[0].Text != "The average is 3.8" || report.Claims[0].Status != claimStatusUnverified || report.Claims[1].Status != claimStatusUnsupported {
		t.Fatalf("claims = %+v", report.Claims)
	}
}

func TestEvidenceReportJSONRoundTrip(t *testing.T) {
	report := models.AIEvidenceReport{Checked: true, Claims: []models.AIClaim{{Ordinal: 1, Text: "结论", Status: claimStatusSupported}}}
	refreshEvidenceSummary(&report)
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded models.AIEvidenceReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.Total != 1 || decoded.Summary.Supported != 1 || len(decoded.Claims) != 1 {
		t.Fatalf("report = %+v", decoded)
	}
}
