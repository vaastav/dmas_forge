package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type FinancialAnalystAgentImpl struct {
	agent          core.Agent
	defaultCompany string
	defaultMode    string
}

func NewFinancialAnalystAgentImpl(ctx context.Context, agent core.Agent, company string, mode string) (FinancialAnalystAgent, error) {
	return &FinancialAnalystAgentImpl{
		agent:          agent,
		defaultCompany: company,
		defaultMode:    NormalizeMode(mode),
	}, nil
}

func (a *FinancialAnalystAgentImpl) AnalyzeData(ctx context.Context, req AnalysisRequest) (string, error) {
	company := firstNonEmpty(req.Company, a.defaultCompany)
	mode := NormalizeMode(firstNonEmpty(req.Mode, a.defaultMode))

	if err := a.agent.AddSystemPrompt(ctx, prompts.AnalystPrompt(company, IsSanityMode(mode))); err != nil {
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
