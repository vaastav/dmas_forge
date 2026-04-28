package workflow

import (
	"context"
	"fmt"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type DataEvaluatorAgentImpl struct {
	agent core.Agent
}

func NewDataEvaluatorAgentImpl(ctx context.Context, agent core.Agent) (DataEvaluatorAgent, error) {
	sysPrompt := prompts.EvaluatorPrompt()
	if err := agent.AddSystemPrompt(ctx, sysPrompt); err != nil {
		return nil, fmt.Errorf("adding evaluator system prompt: %w", err)
	}

	return &DataEvaluatorAgentImpl{
		agent: agent,
	}, nil
}

func (a *DataEvaluatorAgentImpl) EvaluateData(ctx context.Context, req EvaluationRequest) (EvaluationRecord, error) {
	company, mode, err := requireCompanyAndMode(req.Company, req.Mode)
	if err != nil {
		return EvaluationRecord{}, err
	}

	input := fmt.Sprintf(
		"Target company: %s\nRun mode: %s\n\nResearch to evaluate:\n\n%s",
		company,
		mode,
		req.ResearchMarkdown,
	)
	output, err := a.agent.LLMCall(ctx, input)
	if err != nil {
		return EvaluationRecord{}, err
	}

	parsed, err := parseEvaluationResponse(output)
	if err != nil {
		return EvaluationRecord{}, fmt.Errorf("parsing evaluator response: %w", err)
	}
	return parsed, nil
}
