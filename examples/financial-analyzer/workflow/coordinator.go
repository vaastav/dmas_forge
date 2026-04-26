package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type FinancialAnalyzerCoordinatorImpl struct {
	agent       core.Agent
	researchSvc ResearchQualityController
	analystSvc  FinancialAnalystAgent
	writerSvc   ReportWriterAgent
}

func NewFinancialAnalyzerCoordinatorImpl(
	ctx context.Context,
	agent core.Agent,
	researchSvc ResearchQualityController,
	analystSvc FinancialAnalystAgent,
	writerSvc ReportWriterAgent,
) (FinancialAnalyzerCoordinator, error) {
	sysPrompt := prompts.CoordinatorPrompt()

	if err := agent.AddTools(ctx, coordinatorToolSchemas()); err != nil {
		return nil, fmt.Errorf("adding coordinator tools: %w", err)
	}

	a := &FinancialAnalyzerCoordinatorImpl{
		agent:       agent,
		researchSvc: researchSvc,
		analystSvc:  analystSvc,
		writerSvc:   writerSvc,
	}

	if err := agent.RegisterToolCallHandler(ctx, a.coordinatorHandler()); err != nil {
		return nil, fmt.Errorf("registering coordinator handler: %w", err)
	}

	if err := agent.AddSystemPrompt(ctx, sysPrompt); err != nil {
		return nil, fmt.Errorf("adding coordinator system prompt: %w", err)
	}

	return a, nil
}

const (
	sanityModeTimeout = 3 * time.Minute
	fullModeTimeout   = 10 * time.Minute
)

func (a *FinancialAnalyzerCoordinatorImpl) Analyze(ctx context.Context, requestedCompany string, requestedMode string) (AnalysisResult, error) {
	company, mode, err := requireCompanyAndMode(requestedCompany, requestedMode)
	if err != nil {
		return AnalysisResult{}, err
	}
	timeout := sanityModeTimeout
	if mode == ModeFull {
		timeout = fullModeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := &AnalysisResult{
		Company: company,
		Mode:    mode,
		RunID:   time.Now().UTC().Format("20060102_150405"),
	}

	ctx = withFinancialRunState(ctx, &financialRunState{
		Company: company,
		Mode:    mode,
		Result:  result,
	})

	taskPrompt := prompts.CoordinatorTask(company, IsSanityMode(mode))

	output, err := a.agent.LLMCallWithTools(ctx, taskPrompt)
	if err != nil {
		return AnalysisResult{}, err
	}

	if strings.TrimSpace(result.ReportMarkdown) == "" {
		if strings.TrimSpace(output) != "" {
			result.ReportMarkdown = output
		} else {
			return AnalysisResult{}, fmt.Errorf("orchestrator finished without producing a report")
		}
	}

	return *result, nil
}

func (a *FinancialAnalyzerCoordinatorImpl) coordinatorHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		switch tc.Function.Name {
		case "run_research_quality_controller":
			return a.handleResearchTool(ctx, tc)
		case "run_financial_analyst":
			return a.handleAnalystTool(ctx, tc)
		case "run_report_writer":
			return a.handleReportTool(ctx, tc)
		default:
			return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
		}
	}
}

func (a *FinancialAnalyzerCoordinatorImpl) handleResearchTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	state, err := financialRunStateFromContext(ctx)
	if err != nil {
		return "", err
	}

	var args struct {
		Company string `json:"company"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	company := firstNonEmpty(args.Company, state.Company)
	mode := firstNonEmpty(args.Mode, state.Mode)

	res, err := a.researchSvc.RefineResearch(ctx, ResearchRequest{
		Company: company,
		Mode:    mode,
	})
	if err != nil {
		return toolErrorJSON(err)
	}

	state.Result.ResearchMarkdown = res.ResearchMarkdown
	return marshalJSON(map[string]interface{}{
		"research_markdown": res.ResearchMarkdown,
		"final_rating":      string(res.FinalRating),
		"refinement_count":  res.RefinementCount,
		"company":           company,
		"ok":                true,
	})
}

func (a *FinancialAnalyzerCoordinatorImpl) handleAnalystTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	state, err := financialRunStateFromContext(ctx)
	if err != nil {
		return "", err
	}

	if IsSanityMode(state.Mode) {
		return toolErrorJSON(fmt.Errorf("financial_analyst tool is not available in sanity mode"))
	}

	var args struct {
		Company          string `json:"company"`
		Mode             string `json:"mode"`
		ResearchMarkdown string `json:"research_markdown"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	research := firstNonEmpty(args.ResearchMarkdown, state.Result.ResearchMarkdown)
	if strings.TrimSpace(research) == "" {
		return toolErrorJSON(fmt.Errorf("no research data available; run research_quality_controller first"))
	}

	analysis, err := a.analystSvc.AnalyzeData(ctx, AnalysisRequest{
		Company:          firstNonEmpty(args.Company, state.Company),
		Mode:             firstNonEmpty(args.Mode, state.Mode),
		ResearchMarkdown: research,
	})
	if err != nil {
		return toolErrorJSON(err)
	}

	state.Result.AnalysisMarkdown = analysis
	return marshalJSON(map[string]string{"analysis_markdown": analysis})
}

func (a *FinancialAnalyzerCoordinatorImpl) handleReportTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	state, err := financialRunStateFromContext(ctx)
	if err != nil {
		return "", err
	}

	var args struct {
		Company          string `json:"company"`
		Mode             string `json:"mode"`
		ResearchMarkdown string `json:"research_markdown"`
		AnalysisMarkdown string `json:"analysis_markdown"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	research := firstNonEmpty(args.ResearchMarkdown, state.Result.ResearchMarkdown)
	if strings.TrimSpace(research) == "" {
		return toolErrorJSON(fmt.Errorf("no research data available; run research_quality_controller first"))
	}

	report, err := a.writerSvc.WriteReport(ctx, ReportRequest{
		Company:          firstNonEmpty(args.Company, state.Company),
		Mode:             firstNonEmpty(args.Mode, state.Mode),
		ResearchMarkdown: research,
		AnalysisMarkdown: firstNonEmpty(args.AnalysisMarkdown, state.Result.AnalysisMarkdown),
	})
	if err != nil {
		return toolErrorJSON(err)
	}

	state.Result.ReportMarkdown = report
	return marshalJSON(map[string]string{"report_markdown": report})
}

func coordinatorToolSchemas() map[string]openai.ChatCompletionToolParam {
	return map[string]openai.ChatCompletionToolParam{
		"run_research_quality_controller": simpleTool(
			"run_research_quality_controller",
			"Gather financial research, evaluate quality, and refine until the research reaches the required threshold.",
			map[string]toolProperty{
				"company": {Type: "string", Description: "The company ticker or name to research."},
				"mode":    {Type: "string", Description: "Analysis mode: 'sanity' for a quick check or 'full' for comprehensive research."},
			},
			[]string{"company", "mode"},
		),
		"run_financial_analyst": simpleTool(
			"run_financial_analyst",
			"Analyze verified research and produce investment analysis.",
			map[string]toolProperty{
				"company":           {Type: "string", Description: "The company ticker or name to analyze."},
				"mode":              {Type: "string", Description: "Analysis mode: 'sanity' or 'full'."},
				"research_markdown": {Type: "string", Description: "The verified research markdown to analyze."},
			},
			[]string{"company", "mode", "research_markdown"},
		),
		"run_report_writer": simpleTool(
			"run_report_writer",
			"Generate the final markdown report from verified research and optional analyst notes.",
			map[string]toolProperty{
				"company":           {Type: "string", Description: "The company ticker or name for the report."},
				"mode":              {Type: "string", Description: "Report mode: 'sanity' or 'full'."},
				"research_markdown": {Type: "string", Description: "The verified research markdown to include."},
				"analysis_markdown": {Type: "string", Description: "Optional analyst notes to incorporate."},
			},
			[]string{"company", "mode", "research_markdown", "analysis_markdown"},
		),
	}
}

type toolProperty struct {
	Type        string
	Description string
}

func simpleTool(name, description string, properties map[string]toolProperty, required []string) openai.ChatCompletionToolParam {
	props := make(map[string]interface{}, len(properties))
	for k, v := range properties {
		props[k] = map[string]interface{}{
			"type":        v.Type,
			"description": v.Description,
		}
	}
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(description),
			Parameters: openai.FunctionParameters{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		},
	}
}

func toolErrorJSON(err error) (string, error) {
	return marshalJSON(map[string]interface{}{
		"ok":    false,
		"error": err.Error(),
	})
}
