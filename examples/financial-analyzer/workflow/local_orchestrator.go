package workflow

import (
	"context"
	"fmt"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type LocalOrchestrator struct {
	agent core.Agent
}

func NewLocalOrchestrator(ctx context.Context, agent core.Agent, tools map[string]openai.ChatCompletionToolParam) (*LocalOrchestrator, error) {
	if err := agent.AddTools(ctx, tools); err != nil {
		return nil, fmt.Errorf("adding orchestrator tools: %w", err)
	}
	return &LocalOrchestrator{agent: agent}, nil
}

func (o *LocalOrchestrator) Run(ctx context.Context, handler core.ToolHandlerFn, systemPrompt, taskPrompt string) (string, error) {
	if err := o.agent.RegisterToolCallHandler(ctx, handler); err != nil {
		return "", fmt.Errorf("registering orchestrator handler: %w", err)
	}
	if err := o.agent.AddSystemPrompt(ctx, systemPrompt); err != nil {
		return "", fmt.Errorf("adding system prompt: %w", err)
	}
	return o.agent.LLMCallWithTools(ctx, taskPrompt)
}
