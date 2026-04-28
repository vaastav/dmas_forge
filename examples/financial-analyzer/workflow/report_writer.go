package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type ReportWriterAgentImpl struct {
	agent core.Agent
}

func NewReportWriterAgentImpl(ctx context.Context, agent core.Agent) (ReportWriterAgent, error) {
	sysPrompt := prompts.ReportPrompt()
	if err := agent.AddSystemPrompt(ctx, sysPrompt); err != nil {
		return nil, fmt.Errorf("adding report writer system prompt: %w", err)
	}

	return &ReportWriterAgentImpl{
		agent: agent,
	}, nil
}

func (a *ReportWriterAgentImpl) WriteReport(ctx context.Context, req ReportRequest) (string, error) {
	company, mode, err := requireCompanyAndMode(req.Company, req.Mode)
	if err != nil {
		return "", err
	}

	reportDate := time.Now().Format("January 02, 2006 at 3:04 PM MST")
	input := prompts.ReportTask(company, mode, reportDate, req.ResearchMarkdown, req.AnalysisMarkdown)

	report, err := a.agent.LLMCall(ctx, input)
	if err != nil {
		return "", err
	}
	report = strings.TrimSpace(report)
	if report == "" {
		return "", fmt.Errorf("report writer returned empty output")
	}
	return report, nil
}
