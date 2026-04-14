package workflow

import (
	"context"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type TravelPlannerAgentImpl struct {
	agent core.Agent
}

func NewTravelPlannerAgentImpl(ctx context.Context, agent core.Agent) (TravelPlannerAgent, error) {
	a := &TravelPlannerAgentImpl{agent: agent}
	err := a.agent.AddSystemPrompt(ctx, "You are a helpful assistant that can suggest a travel plan for a user based on their request.")
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *TravelPlannerAgentImpl) Plan(ctx context.Context, input string) (string, error) {
	return a.agent.LLMCall(ctx, input)
}

type LocalAgentImpl struct {
	agent core.Agent
}

func NewLocalAgentImpl(ctx context.Context, agent core.Agent) (LocalAgent, error) {
	a := &LocalAgentImpl{agent: agent}
	err := a.agent.AddSystemPrompt(ctx, "You are a helpful assistant that can suggest authentic and interesting local activities or places to visit for a user and can utilize any context information provided.")
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *LocalAgentImpl) Suggest(ctx context.Context, input string) (string, error) {
	return a.agent.LLMCall(ctx, input)
}

type LanguageAgentImpl struct {
	agent core.Agent
}

func NewLanguageAgentImpl(ctx context.Context, agent core.Agent) (LanguageAgent, error) {
	a := &LanguageAgentImpl{agent: agent}
	err := a.agent.AddSystemPrompt(ctx, "You are a helpful assistant that can review travel plans, providing feedback on important/critical tips about how best to address language or communication challenges for the given destination. If the plan already includes language tips, you can mention that the plan is satisfactory, with rationale.")
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *LanguageAgentImpl) Review(ctx context.Context, input string) (string, error) {
	return a.agent.LLMCall(ctx, input)
}

type TravelSummaryAgentImpl struct {
	agent core.Agent
}

func NewTravelSummaryAgentImpl(ctx context.Context, agent core.Agent) (TravelSummaryAgent, error) {
	a := &TravelSummaryAgentImpl{agent: agent}
	err := a.agent.AddSystemPrompt(ctx, "You are a helpful assistant that can take in all of the suggestions and advice from the other agents and provide a detailed final travel plan. You must ensure that the final plan is integrated and complete. YOUR FINAL RESPONSE MUST BE THE COMPLETE PLAN. Do NOT ask the user clarifying questions — make reasonable defaults for any unspecified preferences. When the plan is complete and all perspectives are integrated, you can respond with TERMINATE.")
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *TravelSummaryAgentImpl) Summarize(ctx context.Context, input string) (string, error) {
	return a.agent.LLMCall(ctx, input)
}
