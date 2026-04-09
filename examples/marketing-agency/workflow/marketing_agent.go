package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type MarketingAgentImpl struct {
	agent core.Agent
}

func NewMarketingAgentImpl(ctx context.Context, agent core.Agent) (MarketingAgent, error) {
	a := &MarketingAgentImpl{agent: agent}
	if err := a.agent.AddSystemPrompt(ctx, MarketingAgentPrompt); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *MarketingAgentImpl) CreateStrategy(ctx context.Context, domain string, brandInfo BrandInfo) (string, error) {
	query := fmt.Sprintf(
		"Domain: %s\\nBrand: %s\\nDescription: %s\\nKeywords: %s\\nTarget Audience: %s",
		domain,
		brandInfo.Name,
		brandInfo.Description,
		strings.Join(brandInfo.Keywords, ", "),
		brandInfo.TargetAudience,
	)

	resp, err := a.agent.LLMCall(ctx, query)
	if err != nil {
		return "", err
	}

	var payload struct {
		StrategyMarkdown string `json:"strategy_markdown"`
	}
	if err := json.Unmarshal([]byte(extractJSONPayload(resp)), &payload); err == nil && strings.TrimSpace(payload.StrategyMarkdown) != "" {
		return payload.StrategyMarkdown, nil
	}

	return resp, nil
}
