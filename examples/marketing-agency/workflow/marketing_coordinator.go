package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

const coordinatorPrompt = `You are a marketing campaign coordinator.

You have access to four tools:
1) suggest_domains
2) create_website
3) create_marketing
4) generate_logo

You MUST create a complete campaign by using tools in this order:
1. suggest_domains
2. create_website
3. create_marketing
4. generate_logo

Rules:
- Call each required tool exactly once.
- Choose the best domain from the returned domain list.
- Pass the selected domain and relevant brand context to downstream tools.
- Finish by returning a JSON object with domains, selected_domain, website_files, marketing_strategy, logo_path, and summary. Put the generated logo file path in logo_path.
`

type MarketingCoordinatorImpl struct {
	agent        core.Agent
	domainSvc    DomainAgent
	websiteSvc   WebsiteAgent
	marketingSvc MarketingAgent
	logoSvc      LogoAgent
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

	if err := a.agent.AddSystemPrompt(ctx, coordinatorPrompt); err != nil {
		return nil, err
	}

	if err := a.agent.AddTools(ctx, map[string]openai.ChatCompletionToolParam{
		"suggest_domains":  domainToolSchema(),
		"create_website":   brandToolSchema("create_website", "Generate website files for the selected domain and brand."),
		"create_marketing": brandToolSchema("create_marketing", "Create campaign marketing strategy."),
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
	query := fmt.Sprintf(
		"Create a complete marketing campaign.\nBrand: %s\nKeywords: %s\nDescription: %s\nTarget audience: %s\n\nYou must call tools in order: suggest_domains, create_website, create_marketing, generate_logo. Return only final JSON with domains, selected_domain, website_files, marketing_strategy, logo_path, and summary. Put the generated logo file path in logo_path.",
		req.BrandName,
		strings.Join(req.Keywords, ", "),
		req.Description,
		req.TargetAudience,
	)

	summary, err := a.agent.LLMCallWithTools(ctx, query)
	if err != nil {
		return CampaignResult{}, err
	}

	var result CampaignResult
	if unmarshalJSONFromLLMResponse(summary, &result) {
		return result, nil
	}

	return CampaignResult{Summary: summary}, nil
}

// --- tool dispatch ---

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

// --- tool schemas ---

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

// brandToolSchema returns a shared schema used by create_website and create_marketing.
func brandToolSchema(name, description string) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(description),
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
				"required": []string{"domain", "brand_name", "description", "target_audience", "keywords"},
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

// --- tool handlers ---

// brandArgs is the shared argument structure for create_website and create_marketing.
type brandArgs struct {
	Domain         string   `json:"domain"`
	BrandName      string   `json:"brand_name"`
	Description    string   `json:"description"`
	TargetAudience string   `json:"target_audience"`
	Keywords       []string `json:"keywords"`
}

func (b *brandArgs) toBrandInfo() BrandInfo {
	return BrandInfo{
		Name:           b.BrandName,
		Description:    b.Description,
		TargetAudience: b.TargetAudience,
		Keywords:       b.Keywords,
	}
}

func (a *MarketingCoordinatorImpl) handleDomainTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	domains, err := a.domainSvc.SuggestDomains(ctx, args.Keywords)
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{"domains": domains, "count": len(domains)})
}

func (a *MarketingCoordinatorImpl) handleWebsiteTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args brandArgs
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	content, err := a.websiteSvc.GenerateWebsite(ctx, args.Domain, args.toBrandInfo())
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{"files": content.Files})
}

func (a *MarketingCoordinatorImpl) handleMarketingTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args brandArgs
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}

	strategy, err := a.marketingSvc.CreateStrategy(ctx, args.Domain, args.toBrandInfo())
	if err != nil {
		return "", err
	}

	return marshalJSON(map[string]interface{}{"marketing_strategy": strategy})
}

func (a *MarketingCoordinatorImpl) handleLogoTool(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		BrandName string `json:"brand_name"`
		Style     string `json:"style"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Style) == "" {
		args.Style = "modern minimal"
	}

	metadataJSON, err := a.logoSvc.GenerateLogo(ctx, args.BrandName, args.Style)
	if err != nil {
		return "", err
	}

	return metadataJSON, nil
}

// --- helpers ---

func marshalJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
