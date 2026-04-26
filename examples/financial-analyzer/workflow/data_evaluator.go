package workflow

import (
	"context"
	"fmt"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type DataEvaluatorAgentImpl struct {
	agent          core.Agent
	defaultCompany string
	defaultMode    string
}

func NewDataEvaluatorAgentImpl(ctx context.Context, agent core.Agent, company string, mode string) (DataEvaluatorAgent, error) {
	return &DataEvaluatorAgentImpl{
		agent:          agent,
		defaultCompany: company,
		defaultMode:    NormalizeMode(mode),
	}, nil
}

func (a *DataEvaluatorAgentImpl) EvaluateData(ctx context.Context, req EvaluationRequest) (EvaluationRecord, error) {
	company := firstNonEmpty(req.Company, a.defaultCompany)
	mode := NormalizeMode(firstNonEmpty(req.Mode, a.defaultMode))

	if err := a.agent.AddSystemPrompt(ctx, prompts.EvaluatorPrompt(company, IsSanityMode(mode))); err != nil {
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
