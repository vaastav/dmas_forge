package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type FinancialAnalystAgentImpl struct {
	agent core.Agent
}

func NewFinancialAnalystAgentImpl(ctx context.Context, agent core.Agent) (FinancialAnalystAgent, error) {
	sysPrompt := prompts.AnalystPrompt()
	if err := agent.AddSystemPrompt(ctx, sysPrompt); err != nil {
		return nil, fmt.Errorf("adding analyst system prompt: %w", err)
	}

	return &FinancialAnalystAgentImpl{
		agent: agent,
	}, nil
}

func (a *FinancialAnalystAgentImpl) AnalyzeData(ctx context.Context, req AnalysisRequest) (string, error) {
	company, mode, err := requireCompanyAndMode(req.Company, req.Mode)
	if err != nil {
		return "", err
	}

	input := fmt.Sprintf(
		"Target company: %s\nRun mode: %s\n\nVerified research:\n\n%s",
		company,
		mode,
		req.ResearchMarkdown,
	)
	output, err := a.agent.LLMCall(ctx, input)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
