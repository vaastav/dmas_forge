package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/prompts"
)

type DataCollectorAgentImpl struct {
	agent  core.Agent
	bridge *MCPToolBridge
}

func NewDataCollectorAgentImpl(ctx context.Context, agent core.Agent, mcpServerURLs string) (DataCollectorAgent, error) {
	urls := parseMCPServerURLs(mcpServerURLs)
	bridge, err := NewMCPToolBridge(ctx, urls)
	if err != nil {
		return nil, fmt.Errorf("creating MCP tool bridge: %w", err)
	}

	if err := bridge.AddToolsToAgent(ctx, agent); err != nil {
		bridge.Close()
		return nil, fmt.Errorf("adding MCP tools to collector agent: %w", err)
	}

	a := &DataCollectorAgentImpl{
		agent:  agent,
		bridge: bridge,
	}

	if err := agent.RegisterToolCallHandler(ctx, bridge.HandleToolCall); err != nil {
		return nil, fmt.Errorf("registering MCP tool handler: %w", err)
	}

	sysPrompt := prompts.CollectorPrompt()
	if err := agent.AddSystemPrompt(ctx, sysPrompt); err != nil {
		return nil, fmt.Errorf("adding collector system prompt: %w", err)
	}

	return a, nil
}

func (a *DataCollectorAgentImpl) CollectData(ctx context.Context, req CollectionRequest) (CollectorResult, error) {
	company, mode, err := requireCompanyAndMode(req.Company, req.Mode)
	if err != nil {
		return CollectorResult{}, err
	}
	if strings.TrimSpace(req.Query) == "" {
		return CollectorResult{}, fmt.Errorf("collection query cannot be empty")
	}

	userMsg := fmt.Sprintf("Target company: %s\nRun mode: %s\n\n%s", company, mode, req.Query)
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
