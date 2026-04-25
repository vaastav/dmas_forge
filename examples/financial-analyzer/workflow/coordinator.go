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
	orchestrator   *LocalOrchestrator
	researchSvc    ResearchQualityController
	analystSvc     FinancialAnalystAgent
	writerSvc      ReportWriterAgent
	defaultCompany string
	defaultMode    string
}

func NewFinancialAnalyzerCoordinatorImpl(
	ctx context.Context,
	agent core.Agent,
	researchSvc ResearchQualityController,
	analystSvc FinancialAnalystAgent,
	writerSvc ReportWriterAgent,
	company string,
	mode string,
) (FinancialAnalyzerCoordinator, error) {
	orchestrator, err := NewLocalOrchestrator(ctx, agent, coordinatorToolSchemas())
	if err != nil {
		return nil, err
	}
	return &FinancialAnalyzerCoordinatorImpl{
		orchestrator:   orchestrator,
		researchSvc:    researchSvc,
		analystSvc:     analystSvc,
		writerSvc:      writerSvc,
		defaultCompany: company,
		defaultMode:    NormalizeMode(mode),
	}, nil
}

const (
	sanityModeTimeout = 3 * time.Minute
	fullModeTimeout   = 10 * time.Minute
)

func (a *FinancialAnalyzerCoordinatorImpl) Analyze(ctx context.Context, company string, mode string) (AnalysisResult, error) {
	resolvedMode := NormalizeMode(firstNonEmpty(mode, a.defaultMode))
	timeout := sanityModeTimeout
	if resolvedMode == ModeFull {
		timeout = fullModeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	company = firstNonEmpty(company, a.defaultCompany)
	mode = resolvedMode

	result := &AnalysisResult{
		Company: company,
		Mode:    mode,
		RunID:   time.Now().UTC().Format("20060102_150405"),
	}

	handler := a.buildHandler(ctx, result, company, mode)

	systemPrompt := prompts.CoordinatorPrompt(company, IsSanityMode(mode))
	taskPrompt := prompts.CoordinatorTask(company, IsSanityMode(mode))

	output, err := a.orchestrator.Run(ctx, handler, systemPrompt, taskPrompt)
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

func (a *FinancialAnalyzerCoordinatorImpl) buildHandler(ctx context.Context, result *AnalysisResult, company, mode string) core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		switch tc.Function.Name {
		case "run_research_quality_controller":
			return a.handleResearchTool(ctx, tc, result, company, mode)
		case "run_financial_analyst":
			return a.handleAnalystTool(ctx, tc, result, company, mode)
		case "run_report_writer":
			return a.handleReportTool(ctx, tc, result, company, mode)
		default:
			return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
		}
	}
}

func (a *FinancialAnalyzerCoordinatorImpl) handleResearchTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall, result *AnalysisResult, company, mode string) (string, error) {
	var args struct {
		Company string `json:"company"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	res, err := a.researchSvc.RefineResearch(ctx, ResearchRequest{
		Company: firstNonEmpty(args.Company, company),
		Mode:    firstNonEmpty(args.Mode, mode),
	})
	if err != nil {
		return marshalJSON(map[string]interface{}{
			"company": company,
			"mode":    mode,
			"ok":      false,
			"error":   err.Error(),
		})
	}

	result.ResearchMarkdown = res.ResearchMarkdown
	return marshalJSON(map[string]interface{}{
		"research_markdown": res.ResearchMarkdown,
		"final_rating":      string(res.FinalRating),
		"refinement_count":  res.RefinementCount,
		"company":           firstNonEmpty(args.Company, company),
		"ok":                true,
	})
}

func (a *FinancialAnalyzerCoordinatorImpl) handleAnalystTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall, result *AnalysisResult, company, mode string) (string, error) {
	if IsSanityMode(mode) {
		return "", fmt.Errorf("financial_analyst tool is not available in sanity mode")
	}

	var args struct {
		Company          string `json:"company"`
		Mode             string `json:"mode"`
		ResearchMarkdown string `json:"research_markdown"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	research := firstNonEmpty(args.ResearchMarkdown, result.ResearchMarkdown)
	if strings.TrimSpace(research) == "" {
		return marshalJSON(map[string]interface{}{
			"ok":    false,
			"error": "no research data available; run research_quality_controller first",
		})
	}

	analysis, err := a.analystSvc.AnalyzeData(ctx, AnalysisRequest{
		Company:          firstNonEmpty(args.Company, company),
		Mode:             firstNonEmpty(args.Mode, mode),
		ResearchMarkdown: research,
	})
	if err != nil {
		return "", err
	}

	result.AnalysisMarkdown = analysis
	return marshalJSON(map[string]string{"analysis_markdown": analysis})
}

func (a *FinancialAnalyzerCoordinatorImpl) handleReportTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall, result *AnalysisResult, company, mode string) (string, error) {
	var args struct {
		Company          string `json:"company"`
		Mode             string `json:"mode"`
		ResearchMarkdown string `json:"research_markdown"`
		AnalysisMarkdown string `json:"analysis_markdown"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	research := firstNonEmpty(args.ResearchMarkdown, result.ResearchMarkdown)
	if strings.TrimSpace(research) == "" {
		return marshalJSON(map[string]interface{}{
			"ok":    false,
			"error": "no research data available; run research_quality_controller first",
		})
	}

	report, err := a.writerSvc.WriteReport(ctx, ReportRequest{
		Company:          firstNonEmpty(args.Company, company),
		Mode:             firstNonEmpty(args.Mode, mode),
		ResearchMarkdown: research,
		AnalysisMarkdown: firstNonEmpty(args.AnalysisMarkdown, result.AnalysisMarkdown),
	})
	if err != nil {
		return "", err
	}

	result.ReportMarkdown = report
	return marshalJSON(map[string]string{"report_markdown": report})
}

func coordinatorToolSchemas() map[string]openai.ChatCompletionToolParam {
	return map[string]openai.ChatCompletionToolParam{
		"run_research_quality_controller": simpleTool(
			"run_research_quality_controller",
			"Gather financial research, evaluate quality, and refine until the research reaches the required threshold.",
			map[string]interface{}{"company": map[string]interface{}{"type": "string"}, "mode": map[string]interface{}{"type": "string"}},
		),
		"run_financial_analyst": simpleTool(
			"run_financial_analyst",
			"Analyze verified research and produce investment analysis.",
			map[string]interface{}{
				"company":           map[string]interface{}{"type": "string"},
				"mode":              map[string]interface{}{"type": "string"},
				"research_markdown": map[string]interface{}{"type": "string"},
			},
		),
		"run_report_writer": simpleTool(
			"run_report_writer",
			"Generate the final markdown report from verified research and optional analyst notes.",
			map[string]interface{}{
				"company":           map[string]interface{}{"type": "string"},
				"mode":              map[string]interface{}{"type": "string"},
				"research_markdown": map[string]interface{}{"type": "string"},
				"analysis_markdown": map[string]interface{}{"type": "string"},
			},
		),
	}
}

func simpleTool(name, description string, properties map[string]interface{}) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(description),
			Parameters: openai.FunctionParameters{
				"type":       "object",
				"properties": properties,
			},
		},
	}
}
