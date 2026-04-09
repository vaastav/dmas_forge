package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type WebsiteAgentImpl struct {
	agent core.Agent
}

func NewWebsiteAgentImpl(ctx context.Context, agent core.Agent) (WebsiteAgent, error) {
	a := &WebsiteAgentImpl{agent: agent}
	if err := a.agent.AddSystemPrompt(ctx, WebsiteAgentPrompt); err != nil {
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
	if err := json.Unmarshal([]byte(extractJSONPayload(resp)), &payload); err == nil && len(payload.Files) > 0 {
		return WebsiteContent{Files: payload.Files}, nil
	}

	return WebsiteContent{Files: parseWebsiteFilesFallback(resp)}, nil
}

func parseWebsiteFilesFallback(response string) map[string]string {
	files := map[string]string{}
	lines := strings.Split(response, "\n")
	var current string
	var b strings.Builder
	inCode := false

	flush := func() {
		if current == "" {
			return
		}
		files[current] = strings.TrimSpace(b.String()) + "\n"
		b.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "FILENAME:") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "FILENAME:"))
			inCode = false
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			if !inCode {
				flush()
			}
			continue
		}
		if inCode && current != "" {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	flush()

	return files
}
