package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type MarketingCoordinatorImpl struct {
	agent        core.Agent
	domainSvc    DomainAgent
	websiteSvc   WebsiteAgent
	marketingSvc MarketingAgent
	logoSvc      LogoAgent

	mu      sync.Mutex
	current CampaignRequest
	result  CampaignResult
}

func NewMarketingCoordinatorImpl(
	ctx context.Context,
	agent core.Agent,
	domainSvc DomainAgent,
	websiteSvc WebsiteAgent,
	marketingSvc MarketingAgent,
	logoSvc LogoAgent,
) (MarketingCoordinator, error) {
	a := &MarketingCoordinatorImpl{
		agent:        agent,
		domainSvc:    domainSvc,
		websiteSvc:   websiteSvc,
		marketingSvc: marketingSvc,
		logoSvc:      logoSvc,
	}

	if err := a.agent.AddSystemPrompt(ctx, CoordinatorPrompt); err != nil {
		return nil, err
	}

	if err := a.agent.AddTools(ctx, map[string]openai.ChatCompletionToolParam{
		"suggest_domains":  domainToolSchema(),
		"create_website":   websiteToolSchema(),
		"create_marketing": marketingToolSchema(),
		"generate_logo":    logoToolSchema(),
	}); err != nil {
		return nil, err
	}

	if err := a.agent.RegisterToolCallHandler(ctx, a.compositeHandler()); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *MarketingCoordinatorImpl) CreateCampaign(ctx context.Context, req CampaignRequest) (CampaignResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.current = req
	a.result = CampaignResult{}

	query := fmt.Sprintf(
		"Create a complete marketing campaign.\\nBrand: %s\\nKeywords: %s\\nDescription: %s\\nTarget audience: %s\\n\\nYou must call tools in order: suggest_domains, create_website, create_marketing, generate_logo.",
		req.BrandName,
		strings.Join(req.Keywords, ", "),
		req.Description,
		req.TargetAudience,
	)

	summary, err := a.agent.LLMCallWithTools(ctx, query)
	if err != nil {
		return CampaignResult{}, err
	}

	a.result.Summary = summary
	return a.result, nil
}

func (a *MarketingCoordinatorImpl) compositeHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		switch tc.Function.Name {
		case "suggest_domains":
			return a.handleDomainTool(ctx, tc)
		case "create_website":
			return a.handleWebsiteTool(ctx, tc)
		case "create_marketing":
			return a.handleMarketingTool(ctx, tc)
		case "generate_logo":
			return a.handleLogoTool(ctx, tc)
		default:
			return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
		}
	}
}

func domainToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "suggest_domains",
			Description: openai.String("Suggest candidate domains from keywords."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"keywords": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"keywords"},
			},
		},
	}
}

func websiteToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "create_website",
			Description: openai.String("Generate website files for the selected domain and brand."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"domain":          map[string]interface{}{"type": "string"},
					"brand_name":      map[string]interface{}{"type": "string"},
					"description":     map[string]interface{}{"type": "string"},
					"target_audience": map[string]interface{}{"type": "string"},
					"keywords": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"domain", "brand_name"},
			},
		},
	}
}

func marketingToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "create_marketing",
			Description: openai.String("Create campaign marketing strategy."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"domain":          map[string]interface{}{"type": "string"},
					"brand_name":      map[string]interface{}{"type": "string"},
					"description":     map[string]interface{}{"type": "string"},
					"target_audience": map[string]interface{}{"type": "string"},
					"keywords": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"domain", "brand_name"},
			},
		},
	}
}

func logoToolSchema() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "generate_logo",
			Description: openai.String("Generate and save a brand logo image."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"brand_name": map[string]interface{}{"type": "string"},
					"style":      map[string]interface{}{"type": "string"},
				},
				"required": []string{"brand_name"},
			},
		},
	}
}

func (a *MarketingCoordinatorImpl) handleDomainTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	result, err := a.domainSvc.SuggestDomains(ctx, args.Keywords)
	if err != nil {
		return "", err
	}

	a.result.Domains = result
	if a.result.SelectedDomain == "" && len(result) > 0 {
		a.result.SelectedDomain = result[0]
	}

	b, err := json.Marshal(map[string]interface{}{
		"domains": result,
		"count":   len(result),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *MarketingCoordinatorImpl) handleWebsiteTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Domain         string   `json:"domain"`
		BrandName      string   `json:"brand_name"`
		Description    string   `json:"description"`
		TargetAudience string   `json:"target_audience"`
		Keywords       []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Domain) == "" {
		args.Domain = a.result.SelectedDomain
	}
	if strings.TrimSpace(args.BrandName) == "" {
		args.BrandName = a.current.BrandName
	}
	if strings.TrimSpace(args.Description) == "" {
		args.Description = a.current.Description
	}
	if strings.TrimSpace(args.TargetAudience) == "" {
		args.TargetAudience = a.current.TargetAudience
	}
	if len(args.Keywords) == 0 {
		args.Keywords = a.current.Keywords
	}

	result, err := a.websiteSvc.GenerateWebsite(ctx, args.Domain, BrandInfo{
		Name:           args.BrandName,
		Description:    args.Description,
		TargetAudience: args.TargetAudience,
		Keywords:       args.Keywords,
	})
	if err != nil {
		return "", err
	}

	a.result.SelectedDomain = firstNonEmpty(args.Domain, a.result.SelectedDomain)
	a.result.WebsiteFiles = result.Files

	b, err := json.Marshal(map[string]interface{}{
		"files": result.Files,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *MarketingCoordinatorImpl) handleMarketingTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Domain         string   `json:"domain"`
		BrandName      string   `json:"brand_name"`
		Description    string   `json:"description"`
		TargetAudience string   `json:"target_audience"`
		Keywords       []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Domain) == "" {
		args.Domain = a.result.SelectedDomain
	}
	if strings.TrimSpace(args.BrandName) == "" {
		args.BrandName = a.current.BrandName
	}
	if strings.TrimSpace(args.Description) == "" {
		args.Description = a.current.Description
	}
	if strings.TrimSpace(args.TargetAudience) == "" {
		args.TargetAudience = a.current.TargetAudience
	}
	if len(args.Keywords) == 0 {
		args.Keywords = a.current.Keywords
	}

	result, err := a.marketingSvc.CreateStrategy(ctx, args.Domain, BrandInfo{
		Name:           args.BrandName,
		Description:    args.Description,
		TargetAudience: args.TargetAudience,
		Keywords:       args.Keywords,
	})
	if err != nil {
		return "", err
	}

	a.result.SelectedDomain = firstNonEmpty(args.Domain, a.result.SelectedDomain)
	a.result.MarketingStrategy = result

	b, err := json.Marshal(map[string]interface{}{
		"strategy_markdown": result,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *MarketingCoordinatorImpl) handleLogoTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		BrandName string `json:"brand_name"`
		Style     string `json:"style"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.BrandName) == "" {
		args.BrandName = a.current.BrandName
	}

	style := args.Style
	if strings.TrimSpace(style) == "" {
		style = "modern minimal"
	}

	result, err := a.logoSvc.GenerateLogo(ctx, args.BrandName, style)
	if err != nil {
		return "", err
	}

	a.result.LogoFilepath = result

	b, err := json.Marshal(map[string]interface{}{
		"filepath": result,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
