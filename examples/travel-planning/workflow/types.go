package workflow

import "context"

type TravelPlannerAgent interface {
	Plan(ctx context.Context, input string) (string, error)
}

type LocalAgent interface {
	Suggest(ctx context.Context, input string) (string, error)
}

type LanguageAgent interface {
	Review(ctx context.Context, input string) (string, error)
}

type TravelSummaryAgent interface {
	Summarize(ctx context.Context, input string) (string, error)
}

type TravelCoordinator interface {
	Plan(ctx context.Context, task string) (TravelResult, error)
}

type TravelMessage struct {
	Source  string
	Content string
}

type TravelResult struct {
	Messages   []TravelMessage
	StopReason string
	Terminated bool
	FinalPlan  string
}
