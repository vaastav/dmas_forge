package workflow

import (
	"context"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

// ChatAgent is a simple conversational agent interface.
type ChatAgent interface {
	Chat(ctx context.Context, message string) (string, error)
}

// ChatAgentImpl implements ChatAgent using a core.Agent.
// The workflow has no knowledge of memory -- if memory is enabled,
// it is added transparently at the wiring layer via MemoryAgent.
type ChatAgentImpl struct {
	agent core.Agent
}

// NewChatAgentImpl creates a new ChatAgentImpl with the given agent.
func NewChatAgentImpl(ctx context.Context, agent core.Agent) (ChatAgent, error) {
	a := &ChatAgentImpl{agent: agent}
	err := a.agent.AddSystemPrompt(ctx,
		"You are a friendly conversational assistant. Be helpful and remember "+
			"what the user tells you about themselves.")
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Chat sends a message to the agent and returns its response.
func (a *ChatAgentImpl) Chat(ctx context.Context, message string) (string, error) {
	return a.agent.LLMCallWithTools(ctx, message)
}
