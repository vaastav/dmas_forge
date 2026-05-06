package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/dmas_forge/ai_runtime/core"
	"github.com/vaastav/dmas_forge/examples/financial-analyzer/workflow/prompts"
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

	taskPrompt := prompts.CoordinatorTask(company, IsSanityMode(mode))

	output, err := a.agent.LLMCallWithTools(ctx, taskPrompt)
	if err != nil {
		return AnalysisResult{}, err
	}

	if parsed, err := parseAnalysisResult(output); err == nil {
		parsed.Company = firstNonEmpty(parsed.Company, company)
		parsed.Mode = firstNonEmpty(parsed.Mode, mode)
		parsed.RunID = firstNonEmpty(parsed.RunID, result.RunID)
		return parsed, nil
	}

	if strings.TrimSpace(output) != "" {
		result.ReportMarkdown = output
	} else {
		return AnalysisResult{}, fmt.Errorf("orchestrator finished without producing a report")
	}

	return *result, nil
}

func (a *FinancialAnalyzerCoordinatorImpl) coordinatorHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		switch tc.Function.Name {
		case "research_quality_controller":
			return a.handleResearchTool(ctx, tc)
		case "financial_analyst":
			return a.handleAnalystTool(ctx, tc)
		case "report_writer":
			return a.handleReportTool(ctx, tc)
		default:
			return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
		}
	}
}

func (a *FinancialAnalyzerCoordinatorImpl) handleResearchTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Company string `json:"company"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	res, err := a.researchSvc.RefineResearch(ctx, ResearchRequest{
		Company: args.Company,
		Mode:    args.Mode,
	})
	if err != nil {
		return toolErrorJSON(err)
	}

	return marshalJSON(map[string]interface{}{
		"research_markdown": res.ResearchMarkdown,
		"final_rating":      string(res.FinalRating),
		"refinement_count":  res.RefinementCount,
		"company":           args.Company,
		"mode":              args.Mode,
		"ok":                true,
	})
}

func (a *FinancialAnalyzerCoordinatorImpl) handleAnalystTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Company          string `json:"company"`
		Mode             string `json:"mode"`
		ResearchMarkdown string `json:"research_markdown"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	analysis, err := a.analystSvc.AnalyzeData(ctx, AnalysisRequest{
		Company:          args.Company,
		Mode:             args.Mode,
		ResearchMarkdown: args.ResearchMarkdown,
	})
	if err != nil {
		return toolErrorJSON(err)
	}

	return marshalJSON(map[string]string{"analysis_markdown": analysis})
}

func (a *FinancialAnalyzerCoordinatorImpl) handleReportTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Company          string `json:"company"`
		Mode             string `json:"mode"`
		ResearchMarkdown string `json:"research_markdown"`
		AnalysisMarkdown string `json:"analysis_markdown"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	report, err := a.writerSvc.WriteReport(ctx, ReportRequest{
		Company:          args.Company,
		Mode:             args.Mode,
		ResearchMarkdown: args.ResearchMarkdown,
		AnalysisMarkdown: args.AnalysisMarkdown,
	})
	if err != nil {
		return toolErrorJSON(err)
	}

	return marshalJSON(map[string]string{"report_markdown": report})
}

func coordinatorToolSchemas() map[string]openai.ChatCompletionToolParam {
	return map[string]openai.ChatCompletionToolParam{
		"research_quality_controller": researchToolSchema(),
		"financial_analyst":           analystToolSchema(),
		"report_writer":               reportToolSchema(),
	}
}

func researchToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "research_quality_controller",
			Description: openai.String("Gather financial research, evaluate quality, and refine until the research reaches the required threshold."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"company": map[string]interface{}{"type": "string", "description": "The company ticker or name to research."},
					"mode":    map[string]interface{}{"type": "string", "description": "Analysis mode: 'sanity' for a quick check or 'full' for comprehensive research."},
				},
				"required": []string{"company", "mode"},
			},
		},
	}
}

func analystToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "financial_analyst",
			Description: openai.String("Analyze verified research and produce investment analysis."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"company":           map[string]interface{}{"type": "string", "description": "The company ticker or name to analyze."},
					"mode":              map[string]interface{}{"type": "string", "description": "Analysis mode: 'sanity' or 'full'."},
					"research_markdown": map[string]interface{}{"type": "string", "description": "The verified research markdown to analyze."},
				},
				"required": []string{"company", "mode", "research_markdown"},
			},
		},
	}
}

func reportToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "report_writer",
			Description: openai.String("Generate the final markdown report from verified research and optional analyst notes."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"company":           map[string]interface{}{"type": "string", "description": "The company ticker or name for the report."},
					"mode":              map[string]interface{}{"type": "string", "description": "Report mode: 'sanity' or 'full'."},
					"research_markdown": map[string]interface{}{"type": "string", "description": "The verified research markdown to include."},
					"analysis_markdown": map[string]interface{}{"type": "string", "description": "Optional analyst notes to incorporate."},
				},
				"required": []string{"company", "mode", "research_markdown"},
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

func parseAnalysisResult(output string) (AnalysisResult, error) {
	var result AnalysisResult
	if !unmarshalJSONFromLLMResponse(output, &result) {
		return AnalysisResult{}, fmt.Errorf("invalid analysis result JSON")
	}
	if strings.TrimSpace(result.ReportMarkdown) == "" {
		return AnalysisResult{}, fmt.Errorf("missing report_markdown")
	}
	return result, nil
}
