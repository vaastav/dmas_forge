package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

const websiteAgentPrompt = `You are a senior web developer.

Generate a complete multi-page marketing website.
Required files:
- index.html
- about.html
- services.html
- contact.html
- style.css
- script.js

Output format:
Return valid JSON only:
{
  "files": {
    "index.html": "...",
    "about.html": "...",
    "services.html": "...",
    "contact.html": "...",
    "style.css": "...",
    "script.js": "..."
  }
}
`

type WebsiteAgentImpl struct {
	agent core.Agent
}

func NewWebsiteAgentImpl(ctx context.Context, agent core.Agent) (WebsiteAgent, error) {
	a := &WebsiteAgentImpl{agent: agent}
	if err := a.agent.AddSystemPrompt(ctx, websiteAgentPrompt); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *WebsiteAgentImpl) GenerateWebsite(ctx context.Context, domain string, brandInfo BrandInfo) (WebsiteContent, error) {
	query := fmt.Sprintf(
		"Domain: %s\\nBrand: %s\\nDescription: %s\\nKeywords: %s\\nAudience: %s",
		domain,
		brandInfo.Name,
		brandInfo.Description,
		strings.Join(brandInfo.Keywords, ", "),
		brandInfo.TargetAudience,
	)

	resp, err := a.agent.LLMCall(ctx, query)
	if err != nil {
		return WebsiteContent{}, err
	}

	var payload struct {
		Files map[string]string `json:"files"`
	}
	if unmarshalJSONFromLLMResponse(resp, &payload) && len(payload.Files) > 0 {
		return WebsiteContent{Files: payload.Files}, nil
	}

	// Fallback: wrap raw output as a single file.
	return WebsiteContent{Files: map[string]string{"raw_output.txt": resp}}, nil
}
