package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

const memorySystemPromptSuffix = "\n\nYou have access to a persistent long-term memory. " +
	"Use it to remember important information across conversations, such as user preferences, " +
	"previously discussed topics, or any facts you want to retain. " +
	"Use `list_memories` to check what you already know, `recall_memory` to retrieve specific facts, " +
	"`store_memory` to save new information, and `delete_memory` to remove outdated information. " +
	"Proactively use your memory when it would improve your responses."

// memoryToolDefs defines the 4 memory tools exposed to the LLM.
var memoryToolDefs = map[string]openai.ChatCompletionToolParam{
	"store_memory": {
		Function: openai.FunctionDefinitionParam{
			Name: "store_memory",
			Description: openai.String(
				"Save a piece of information to your persistent long-term memory. " +
					"Use clear, descriptive keys (e.g., 'user_name', 'user_preferred_units'). " +
					"Values should be concise but complete."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]string{
						"type":        "string",
						"description": "A descriptive key for the memory entry",
					},
					"value": map[string]string{
						"type":        "string",
						"description": "The information to store",
					},
				},
				"required": []string{"key", "value"},
			},
		},
	},
	"recall_memory": {
		Function: openai.FunctionDefinitionParam{
			Name: "recall_memory",
			Description: openai.String(
				"Retrieve a specific piece of information from your long-term memory by its exact key."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]string{
						"type":        "string",
						"description": "The exact key of the memory entry to retrieve",
					},
				},
				"required": []string{"key"},
			},
		},
	},
	"delete_memory": {
		Function: openai.FunctionDefinitionParam{
			Name: "delete_memory",
			Description: openai.String(
				"Remove a piece of information from your long-term memory when it is no longer relevant or accurate."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]string{
						"type":        "string",
						"description": "The exact key of the memory entry to remove",
					},
				},
				"required": []string{"key"},
			},
		},
	},
	"list_memories": {
		Function: openai.FunctionDefinitionParam{
			Name: "list_memories",
			Description: openai.String(
				"List all keys currently stored in your long-term memory. " +
					"Use this to check what information you have available before trying to recall specific items."),
			Parameters: openai.FunctionParameters{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
}

// memoryToolNames is the set of tool names handled by the memory agent.
var memoryToolNames = map[string]bool{
	"store_memory":  true,
	"recall_memory": true,
	"delete_memory": true,
	"list_memories": true,
}

// MemoryAgent is a decorator that wraps any core.Agent with LLM-driven memory
// capabilities. It registers memory tools on the inner agent and composes tool
// handlers so that memory tool calls are handled internally while all other tool
// calls are delegated to the user's handler.
//
// Workflows interact with MemoryAgent through the core.Agent interface and are
// completely unaware of memory. Whether an agent has memory is a wiring decision.
type MemoryAgent struct {
	inner       core.Agent
	memory      core.Memory
	userHandler core.ToolHandlerFn
}

// NewMemoryAgent wraps the given agent with memory capabilities.
// It registers memory tool definitions and a default handler on the inner agent.
func NewMemoryAgent(ctx context.Context, agent core.Agent, memStore core.Memory) (*MemoryAgent, error) {
	m := &MemoryAgent{
		inner:  agent,
		memory: memStore,
	}

	// Register memory tools on the inner agent
	err := agent.AddTools(ctx, memoryToolDefs)
	if err != nil {
		return nil, fmt.Errorf("memory agent: failed to add memory tools: %w", err)
	}

	// Register the default handler (memory-only, no user handler yet)
	err = agent.RegisterToolCallHandler(ctx, m.buildCompositeHandler())
	if err != nil {
		return nil, fmt.Errorf("memory agent: failed to register tool handler: %w", err)
	}

	return m, nil
}

// AddSystemPrompt appends a memory instruction to the user's prompt and forwards
// it to the inner agent.
func (m *MemoryAgent) AddSystemPrompt(ctx context.Context, prompt string) error {
	return m.inner.AddSystemPrompt(ctx, prompt+memorySystemPromptSuffix)
}

// AddTools forwards tool registration to the inner agent. Memory tools were
// already registered in the constructor.
func (m *MemoryAgent) AddTools(ctx context.Context, tooldefs map[string]openai.ChatCompletionToolParam) error {
	return m.inner.AddTools(ctx, tooldefs)
}

// LLMCall forwards to the inner agent.
func (m *MemoryAgent) LLMCall(ctx context.Context, query string) (string, error) {
	return m.inner.LLMCall(ctx, query)
}

// LLMCallWithTools forwards to the inner agent.
func (m *MemoryAgent) LLMCallWithTools(ctx context.Context, query string) (string, error) {
	return m.inner.LLMCallWithTools(ctx, query)
}

// RegisterToolCallHandler stores the user's handler and re-registers a composite
// handler on the inner agent that dispatches memory tool calls internally and
// delegates everything else to the user's handler.
func (m *MemoryAgent) RegisterToolCallHandler(ctx context.Context, toolHandlerFn core.ToolHandlerFn) error {
	m.userHandler = toolHandlerFn
	return m.inner.RegisterToolCallHandler(ctx, m.buildCompositeHandler())
}

// buildCompositeHandler returns a ToolHandlerFn that handles memory tools internally
// and falls through to the user's handler for everything else.
func (m *MemoryAgent) buildCompositeHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		if memoryToolNames[tc.Function.Name] {
			return m.handleMemoryToolCall(ctx, tc)
		}
		if m.userHandler != nil {
			return m.userHandler(ctx, tc)
		}
		return "", fmt.Errorf("unsupported tool call: %s", tc.Function.Name)
	}
}

// handleMemoryToolCall dispatches a memory tool call to the appropriate handler.
func (m *MemoryAgent) handleMemoryToolCall(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	switch tc.Function.Name {
	case "store_memory":
		return m.handleStoreMemory(ctx, tc)
	case "recall_memory":
		return m.handleRecallMemory(ctx, tc)
	case "delete_memory":
		return m.handleDeleteMemory(ctx, tc)
	case "list_memories":
		return m.handleListMemories(ctx, tc)
	default:
		return "", fmt.Errorf("unknown memory tool: %s", tc.Function.Name)
	}
}

func (m *MemoryAgent) handleStoreMemory(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("store_memory: invalid arguments: %w", err)
	}
	if err := m.memory.Store(ctx, args.Key, args.Value); err != nil {
		return "", fmt.Errorf("store_memory: %w", err)
	}
	return fmt.Sprintf("Stored memory for key '%s'.", args.Key), nil
}

func (m *MemoryAgent) handleRecallMemory(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("recall_memory: invalid arguments: %w", err)
	}
	value, err := m.memory.Recall(ctx, args.Key)
	if err != nil {
		return "", fmt.Errorf("recall_memory: %w", err)
	}
	return value, nil
}

func (m *MemoryAgent) handleDeleteMemory(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("delete_memory: invalid arguments: %w", err)
	}
	if err := m.memory.Delete(ctx, args.Key); err != nil {
		return "", fmt.Errorf("delete_memory: %w", err)
	}
	return fmt.Sprintf("Deleted memory for key '%s'.", args.Key), nil
}

func (m *MemoryAgent) handleListMemories(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	keys, err := m.memory.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list_memories: %w", err)
	}
	if len(keys) == 0 {
		return "No memories stored yet.", nil
	}
	return "Stored memory keys: " + strings.Join(keys, ", "), nil
}
