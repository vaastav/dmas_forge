package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type DataCollectorAgentImpl struct {
	agent          core.Agent
	bridge         *MCPToolBridge
	defaultCompany string
	defaultMode    string
}

func NewDataCollectorAgentImpl(ctx context.Context, agent core.Agent, mcpServerURLs string, company string, mode string) (DataCollectorAgent, error) {
	urls := parseMCPServerURLs(mcpServerURLs)
	bridge, err := NewMCPToolBridge(ctx, urls)
	if err != nil {
		return nil, fmt.Errorf("creating MCP tool bridge: %w", err)
	}

	if err := bridge.AddToolsToAgent(ctx, agent); err != nil {
		bridge.Close()
		return nil, fmt.Errorf("adding MCP tools to collector agent: %w", err)
	}

	return &DataCollectorAgentImpl{
		agent:          agent,
		bridge:         bridge,
		defaultCompany: company,
		defaultMode:    NormalizeMode(mode),
	}, nil
}

func (a *DataCollectorAgentImpl) CollectData(ctx context.Context, req CollectionRequest) (CollectorResult, error) {
	company := firstNonEmpty(req.Company, a.defaultCompany)
	mode := NormalizeMode(firstNonEmpty(req.Mode, a.defaultMode))
	if strings.TrimSpace(req.Query) == "" {
		return CollectorResult{}, fmt.Errorf("collection query cannot be empty")
	}

	if err := a.agent.RegisterToolCallHandler(ctx, a.bridge.HandleToolCall); err != nil {
		return CollectorResult{}, fmt.Errorf("registering MCP tool handler: %w", err)
	}

	if err := a.agent.AddSystemPrompt(ctx, prompts.CollectorPrompt(company, IsSanityMode(mode))); err != nil {
		return CollectorResult{}, fmt.Errorf("adding collector system prompt: %w", err)
	}

	userMsg := req.Query
	if req.PriorResearch != "" {
		userMsg += fmt.Sprintf(
			"\n\n---\nPRIOR RESEARCH (improve on this, do not start from scratch):\n\n%s",
			req.PriorResearch,
		)
	}

	research, err := a.agent.LLMCallWithTools(ctx, userMsg)
	if err != nil {
		return CollectorResult{}, err
	}

	return CollectorResult{ResearchMarkdown: strings.TrimSpace(research)}, nil
}

func parseMCPServerURLs(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	var urls []string
	for _, item := range strings.Split(csv, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			urls = append(urls, item)
		}
	}
	return urls
}
