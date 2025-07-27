package core

import (
	"context"

	"github.com/openai/openai-go"
)

type ToolHandlerFn func(context.Context, openai.ChatCompletionMessageToolCall) (string, error)

type Agent interface {
	AddSystemPrompt(ctx context.Context, prompt string) error
	AddTools(ctx context.Context, tooldefs map[string]openai.ChatCompletionToolParam) error
	LLMCall(ctx context.Context, query string) (string, error)
	LLMCallWithTools(ctx context.Context, query string) (string, error)
	RegisterToolCallHandler(ctx context.Context, toolHandlerFn ToolHandlerFn) error
}
