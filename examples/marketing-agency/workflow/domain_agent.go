package workflow

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow/tools"
)

type DomainAgentImpl struct {
	agent core.Agent
}

func NewDomainAgentImpl(ctx context.Context, agent core.Agent) (DomainAgent, error) {
	a := &DomainAgentImpl{agent: agent}

	if err := a.agent.AddSystemPrompt(ctx, DomainAgentPrompt); err != nil {
		return nil, err
	}

	if err := a.agent.AddTools(ctx, map[string]openai.ChatCompletionToolParam{
		"duckduckgo_search": tools.DuckDuckGoSearchTool(),
	}); err != nil {
		return nil, err
	}

	if err := a.agent.RegisterToolCallHandler(ctx, tools.DuckDuckGoSearchHandler()); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *DomainAgentImpl) SuggestDomains(ctx context.Context, keywords []string) ([]string, error) {
	query := fmt.Sprintf(
		"Brand keywords: %s\\nReturn exactly 10 candidate domains in JSON format.",
		strings.Join(keywords, ", "),
	)

	resp, err := a.agent.LLMCallWithTools(ctx, query)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Domains []string `json:"domains"`
	}
	if unmarshalJSONFromLLMResponse(resp, &parsed) {
		sanitized := sanitizeDomainList(parsed.Domains)
		if len(sanitized) > 0 {
			return sanitized, nil
		}
	}

	return sanitizeDomainList(parseDomainListFallback(resp)), nil
}

func sanitizeDomainList(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 10)
	for _, item := range in {
		normalized := strings.ToLower(strings.TrimSpace(item))
		normalized = strings.TrimPrefix(normalized, "- ")
		normalized = strings.TrimPrefix(normalized, "* ")
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		if !looksLikeDomain(normalized) {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
		if len(out) == 10 {
			break
		}
	}
	return out
}

func parseDomainListFallback(response string) []string {
	lines := strings.Split(response, "\n")
	items := make([]string, 0, 10)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for i := 1; i <= 20; i++ {
			line = strings.TrimPrefix(line, fmt.Sprintf("%d. ", i))
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if looksLikeDomain(line) {
			items = append(items, line)
		}
	}
	return items
}

func looksLikeDomain(s string) bool {
	if strings.Contains(s, " ") {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	return len(parts[len(parts)-1]) >= 2
}
