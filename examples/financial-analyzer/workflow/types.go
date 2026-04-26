package workflow

import (
	"context"
	"strings"
)

const (
	ModeSanity = "sanity"
	ModeFull   = "full"
)

func NormalizeMode(mode string) string {
	if mode == ModeFull {
		return ModeFull
	}
	return ModeSanity
}

func IsSanityMode(mode string) bool {
	return NormalizeMode(mode) != ModeFull
}

type QualityRating string

const (
	RatingExcellent QualityRating = "EXCELLENT"
	RatingGood      QualityRating = "GOOD"
	RatingFair      QualityRating = "FAIR"
	RatingPoor      QualityRating = "POOR"
)

func NormalizeQualityRating(raw string) QualityRating {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(RatingExcellent):
		return RatingExcellent
	case string(RatingGood):
		return RatingGood
	case string(RatingFair):
		return RatingFair
	default:
		return RatingPoor
	}
}

func MeetsMinRating(rating, minRating QualityRating) bool {
	order := map[QualityRating]int{
		RatingExcellent: 4,
		RatingGood:      3,
		RatingFair:      2,
		RatingPoor:      1,
	}
	return order[rating] >= order[minRating]
}

type EvaluationRecord struct {
	Completeness  QualityRating `json:"completeness"`
	Accuracy      QualityRating `json:"accuracy"`
	Currency      QualityRating `json:"currency"`
	OverallRating QualityRating `json:"overall_rating"`
	Feedback      string        `json:"feedback"`
	Raw           string        `json:"raw,omitempty"`
}

type CollectionRequest struct {
	Company       string `json:"company"`
	Mode          string `json:"mode"`
	Query         string `json:"query"`
	PriorResearch string `json:"prior_research,omitempty"`
}

type CollectorResult struct {
	ResearchMarkdown string `json:"research_markdown"`
}

type EvaluationRequest struct {
	Company          string `json:"company"`
	Mode             string `json:"mode"`
	ResearchMarkdown string `json:"research_markdown"`
}

type ResearchRequest struct {
	Company string `json:"company"`
	Mode    string `json:"mode"`
}

type ResearchQualityResult struct {
	ResearchMarkdown string        `json:"research_markdown"`
	FinalRating      QualityRating `json:"final_rating"`
	RefinementCount  int           `json:"refinement_count"`
}

type AnalysisRequest struct {
	Company          string `json:"company"`
	Mode             string `json:"mode"`
	ResearchMarkdown string `json:"research_markdown"`
}

type ReportRequest struct {
	Company          string `json:"company"`
	Mode             string `json:"mode"`
	ResearchMarkdown string `json:"research_markdown"`
	AnalysisMarkdown string `json:"analysis_markdown,omitempty"`
}

type AnalysisResult struct {
	ReportMarkdown   string `json:"report_markdown"`
	ResearchMarkdown string `json:"research_markdown"`
	AnalysisMarkdown string `json:"analysis_markdown,omitempty"`
	Mode             string `json:"mode"`
	Company          string `json:"company"`
	RunID            string `json:"run_id"`
}

type DataCollectorAgent interface {
	CollectData(ctx context.Context, req CollectionRequest) (CollectorResult, error)
}

type DataEvaluatorAgent interface {
	EvaluateData(ctx context.Context, req EvaluationRequest) (EvaluationRecord, error)
}

type ResearchQualityController interface {
	RefineResearch(ctx context.Context, req ResearchRequest) (ResearchQualityResult, error)
}

type FinancialAnalystAgent interface {
	AnalyzeData(ctx context.Context, req AnalysisRequest) (string, error)
}

type ReportWriterAgent interface {
	WriteReport(ctx context.Context, req ReportRequest) (string, error)
}

type FinancialAnalyzerCoordinator interface {
	Analyze(ctx context.Context, company string, mode string) (AnalysisResult, error)
}
