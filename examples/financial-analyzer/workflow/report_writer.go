package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type ReportWriterAgentImpl struct {
	agent          core.Agent
	defaultCompany string
	defaultMode    string
}

func NewReportWriterAgentImpl(ctx context.Context, agent core.Agent, company string, mode string) (ReportWriterAgent, error) {
	return &ReportWriterAgentImpl{
		agent:          agent,
		defaultCompany: company,
		defaultMode:    NormalizeMode(mode),
	}, nil
}

func (a *ReportWriterAgentImpl) WriteReport(ctx context.Context, req ReportRequest) (string, error) {
	company := firstNonEmpty(req.Company, a.defaultCompany)
	mode := NormalizeMode(firstNonEmpty(req.Mode, a.defaultMode))

	if err := a.agent.AddSystemPrompt(ctx, prompts.ReportPrompt(company, IsSanityMode(mode))); err != nil {
		return "", err
	}

	var input strings.Builder
	input.WriteString(fmt.Sprintf("Target company: %s\nRun mode: %s\n\n", company, mode))
	input.WriteString("Verified research:\n\n")
	input.WriteString(req.ResearchMarkdown)
	if strings.TrimSpace(req.AnalysisMarkdown) != "" {
		input.WriteString("\n\nFinancial analysis:\n\n")
		input.WriteString(req.AnalysisMarkdown)
	}

	report, err := a.agent.LLMCall(ctx, input.String())
	if err != nil {
		return "", err
	}
	report = strings.TrimSpace(report)
	if report == "" {
		return "", fmt.Errorf("report writer returned empty output")
	}
	return report, nil
}
