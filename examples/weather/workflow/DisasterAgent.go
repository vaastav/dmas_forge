package workflow

import (
	"context"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type DisasterAgent interface {
	Query(ctx context.Context, query string) (string, error)
}

type DisasterAgentImpl struct {
	agent core.Agent
}

func NewDisasterAgentImpl(ctx context.Context, agent core.Agent) (DisasterAgent, error) {

	a := &DisasterAgentImpl{agent: agent}
	system_prompt := "Based on the provided weather report, your job is to figure out if there is any chance of a natural disaster such as hurricanes, torandoes, tsunamis, etc. If there is not enough information then just say not enough information available"

	err := a.agent.AddSystemPrompt(ctx, system_prompt)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *DisasterAgentImpl) Query(ctx context.Context, query string) (string, error) {
	return a.agent.LLMCall(ctx, query)
}
