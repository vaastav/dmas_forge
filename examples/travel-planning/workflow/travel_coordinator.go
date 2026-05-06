package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/dmas_forge/ai_runtime/core"
)

type roundRobinStep struct {
	call   func(ctx context.Context, input string) (string, error)
	source string
}

type TravelCoordinatorImpl struct {
	steps []roundRobinStep
}

func NewTravelCoordinatorImpl(ctx context.Context, planner TravelPlannerAgent, local LocalAgent, lang LanguageAgent, summary TravelSummaryAgent, agent core.Agent) (TravelCoordinator, error) {
	// ctx and agent are unused but needed to compile and deploy agent in Docker
	_ = ctx
	_ = agent
	return &TravelCoordinatorImpl{
		steps: []roundRobinStep{
			{call: planner.Plan, source: "planner_agent"},
			{call: local.Suggest, source: "local_agent"},
			{call: lang.Review, source: "language_agent"},
			{call: summary.Summarize, source: "travel_summary_agent"},
		},
	}, nil
}

func (c *TravelCoordinatorImpl) Plan(ctx context.Context, task string) (TravelResult, error) {
	result := TravelResult{}
	if strings.TrimSpace(task) == "" {
		return result, fmt.Errorf("task cannot be empty")
	}

	result.Messages = append(result.Messages, TravelMessage{Source: "user", Content: task})

	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		for _, step := range c.steps {
			output, err := step.call(ctx, renderTranscript(result.Messages))
			if err != nil {
				return result, err
			}
			result.Messages = append(result.Messages, TravelMessage{Source: step.source, Content: output})
			if hasTerminate(output) {
				finalizeResult(&result, output)
				return result, nil
			}
		}
	}
}

func hasTerminate(text string) bool {
	return strings.Contains(text, "TERMINATE")
}

func finalizeResult(result *TravelResult, finalText string) {
	result.Terminated = true
	result.StopReason = "Text 'TERMINATE' mentioned"
	result.FinalPlan = strings.TrimSpace(strings.Replace(finalText, "TERMINATE", "", 1))
}

func renderTranscript(messages []TravelMessage) string {
	var b strings.Builder
	for i, msg := range messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("---------- ")
		b.WriteString(msg.Source)
		b.WriteString(" ----------\n")
		b.WriteString(msg.Content)
	}
	b.WriteString("\n\nContinue the travel-planning conversation from your role.")
	return b.String()
}
