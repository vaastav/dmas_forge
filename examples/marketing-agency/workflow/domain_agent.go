package workflow

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow/tools"
)

const domainAgentPrompt = `You are a domain naming specialist.

Task:
- Generate many domain ideas from brand keywords.
- Use check_domain on candidate domains.
- Return exactly 10 candidate domains with available=true.

Output format:
Return valid JSON only:
{"domains":["example.com","example.io",...]}
`

type DomainAgentImpl struct {
	agent core.Agent
}

func NewDomainAgentImpl(ctx context.Context, agent core.Agent) (DomainAgent, error) {
	a := &DomainAgentImpl{agent: agent}

	if err := a.agent.AddSystemPrompt(ctx, domainAgentPrompt); err != nil {
		return nil, err
	}

	if err := a.agent.AddTools(ctx, map[string]openai.ChatCompletionToolParam{
		"check_domain": tools.DomainCheckTool(),
	}); err != nil {
		return nil, err
	}

	if err := a.agent.RegisterToolCallHandler(ctx, tools.DomainCheckHandler()); err != nil {
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
		domains := filterDomains(parsed.Domains)
		if len(domains) > 0 {
			return domains, nil
		}
	}

	// Fallback: return raw response lines that look like domains.
	return filterDomains(strings.Split(resp, "\n")), nil
}

// filterDomains deduplicates, normalizes, and keeps only valid-looking domain names.
func filterDomains(candidates []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 10)
	for _, s := range candidates {
		d := strings.ToLower(strings.TrimSpace(s))
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		if !looksLikeDomain(d) {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
		if len(out) == 10 {
			break
		}
	}
	return out
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
