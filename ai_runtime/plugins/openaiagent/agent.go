package openaiagent

import (
	"context"
	"fmt"
	"strconv"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type OpenAILLMClient struct {
	hasSysMsg     bool
	tool_map      map[string]openai.ChatCompletionToolParam
	tools         []openai.ChatCompletionToolParam
	sysMsg        string
	client        *openai.Client
	model         string
	toolHandlerFn core.ToolHandlerFn
	maxToolRounds int
}

// NewOpenAILLMClient creates a new OpenAI LLM client.
// The maxToolRounds parameter specifies the maximum number of tool-call
// round-trips allowed in a single LLMCallWithTools invocation.
// If maxToolRounds is empty, unparseable, 0, or negative, the default value of 10 is used.
func NewOpenAILLMClient(ctx context.Context, url string, apikey string, model_name string, maxToolRounds string) (*OpenAILLMClient, error) {
	client := openai.NewClient(option.WithBaseURL(url), option.WithAPIKey(apikey))
	maxRounds, err := strconv.Atoi(maxToolRounds)
	if err != nil || maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}
	return &OpenAILLMClient{client: &client, tool_map: make(map[string]openai.ChatCompletionToolParam), model: model_name, maxToolRounds: maxRounds}, nil
}

func (c *OpenAILLMClient) AddSystemPrompt(ctx context.Context, prompt string) error {
	c.hasSysMsg = true
	// Create or update the system message
	c.sysMsg = prompt
	return nil
}

func (c *OpenAILLMClient) RegisterToolCallHandler(ctx context.Context, toolHandlerFn core.ToolHandlerFn) error {
	c.toolHandlerFn = toolHandlerFn
	return nil
}

func (c *OpenAILLMClient) AddTools(ctx context.Context, tooldefs map[string]openai.ChatCompletionToolParam) error {
	for k, v := range tooldefs {
		c.tool_map[k] = v
	}
	c.tools = mapsToValues(c.tool_map)
	return nil
}

func (c *OpenAILLMClient) LLMCall(ctx context.Context, query string) (string, error) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("github.com/vaastav/agentic_blueprint/ai_runtime/plugins/openaiagent")
	ctx, span := tracer.Start(ctx, "llm.call",
		trace.WithAttributes(
			attribute.String("llm.provider", "openai"),
			attribute.String("llm.model", c.model),
			attribute.String("llm.call_type", "LLMCall"),
			attribute.Bool("llm.tools_enabled", false),
		),
	)
	defer span.End()

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(c.sysMsg),
			openai.UserMessage(query),
		},
		Model: c.model,
	}

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		recordSpanError(span, err)
		return "", err
	}
	setTokenAttributes(span, completion.Usage)
	choice, err := firstChoice(completion)
	if err != nil {
		recordSpanError(span, err)
		return "", err
	}
	return choice.Message.Content, nil
}

// defaultMaxToolRounds is the default maximum number of tool-call round-trips
// allowed in a single LLMCallWithTools invocation. This prevents infinite loops
// if the model keeps requesting tool calls indefinitely.
// The value can be overridden via the maxToolRounds parameter in NewOpenAILLMClient.
const defaultMaxToolRounds = 10

func (c *OpenAILLMClient) LLMCallWithTools(ctx context.Context, query string) (string, error) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("github.com/vaastav/agentic_blueprint/ai_runtime/plugins/openaiagent")
	ctx, span := tracer.Start(ctx, "llm.call",
		trace.WithAttributes(
			attribute.String("llm.provider", "openai"),
			attribute.String("llm.model", c.model),
			attribute.String("llm.call_type", "LLMCallWithTools"),
			attribute.Bool("llm.tools_enabled", true),
			attribute.Int("llm.tool_count", len(c.tools)),
			attribute.Int("llm.max_tool_rounds", c.maxToolRounds),
		),
	)
	defer span.End()

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(c.sysMsg),
			openai.UserMessage(query),
		},
		Tools: c.tools,
		Model: c.model,
	}

	var inputTokens int64
	var outputTokens int64
	var totalTokens int64
	tokenUsageAvailable := false
	toolCallCount := 0

	for round := range c.maxToolRounds {
		completion, err := c.client.Chat.Completions.New(ctx, params)
		if err != nil {
			recordSpanError(span, err)
			return "", err
		}
		if hasTokenUsage(completion.Usage) {
			tokenUsageAvailable = true
			inputTokens += completion.Usage.PromptTokens
			outputTokens += completion.Usage.CompletionTokens
			totalTokens += completion.Usage.TotalTokens
		}

		choice, err := firstChoice(completion)
		if err != nil {
			recordSpanError(span, err)
			return "", err
		}

		toolCalls := choice.Message.ToolCalls
		if len(toolCalls) == 0 {
			setAggregatedTokenAttributes(span, tokenUsageAvailable, inputTokens, outputTokens, totalTokens)
			span.SetAttributes(attribute.Int("llm.tool_call_count", toolCallCount))
			return choice.Message.Content, nil
		}

		// Handle tool calls and continue the conversation
		params.Messages = append(params.Messages, choice.Message.ToParam())
		for _, toolCall := range toolCalls {
			toolCallCount++
			toolCtx, toolSpan := tracer.Start(ctx, "llm.tool_call",
				trace.WithAttributes(
					attribute.String("tool.name", toolCall.Function.Name),
					attribute.String("tool.id", toolCall.ID),
					attribute.Int("tool.round", round+1),
				),
			)
			res, err := c.toolHandlerFn(toolCtx, toolCall)
			if err != nil {
				// Abort if the tool handler function was unable to handle the tool call
				recordSpanError(toolSpan, err)
				toolSpan.End()
				recordSpanError(span, err)
				return "", err
			}
			toolSpan.SetStatus(codes.Ok, "")
			toolSpan.End()
			params.Messages = append(params.Messages, openai.ToolMessage(res, toolCall.ID))
		}
	}

	// Exhausted all rounds; make one final call without tools so the model
	// is forced to produce a text response.
	params.Tools = nil
	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		recordSpanError(span, err)
		return "", err
	}
	if hasTokenUsage(completion.Usage) {
		tokenUsageAvailable = true
		inputTokens += completion.Usage.PromptTokens
		outputTokens += completion.Usage.CompletionTokens
		totalTokens += completion.Usage.TotalTokens
	}
	choice, err := firstChoice(completion)
	if err != nil {
		recordSpanError(span, err)
		return "", err
	}
	setAggregatedTokenAttributes(span, tokenUsageAvailable, inputTokens, outputTokens, totalTokens)
	span.SetAttributes(
		attribute.Int("llm.tool_call_count", toolCallCount),
		attribute.Bool("llm.max_tool_rounds_exhausted", true),
	)
	return choice.Message.Content, nil
}

func firstChoice(completion *openai.ChatCompletion) (openai.ChatCompletionChoice, error) {
	if completion == nil || len(completion.Choices) == 0 {
		return openai.ChatCompletionChoice{}, fmt.Errorf("openai completion returned zero choices")
	}
	return completion.Choices[0], nil
}

func mapsToValues(tooldefs map[string]openai.ChatCompletionToolParam) []openai.ChatCompletionToolParam {
	vals := []openai.ChatCompletionToolParam{}
	for _, v := range tooldefs {
		vals = append(vals, v)
	}
	return vals
}

func hasTokenUsage(usage openai.CompletionUsage) bool {
	return usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.TotalTokens > 0
}

func setTokenAttributes(span interface{ SetAttributes(...attribute.KeyValue) }, usage openai.CompletionUsage) {
	setAggregatedTokenAttributes(span, hasTokenUsage(usage), usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}

func setAggregatedTokenAttributes(span interface{ SetAttributes(...attribute.KeyValue) }, available bool, inputTokens, outputTokens, totalTokens int64) {
	attrs := []attribute.KeyValue{attribute.Bool("llm.token_usage_available", available)}
	if available {
		attrs = append(attrs,
			attribute.Int64("llm.input_tokens", inputTokens),
			attribute.Int64("llm.output_tokens", outputTokens),
			attribute.Int64("llm.total_tokens", totalTokens),
		)
	}
	span.SetAttributes(attrs...)
}

func recordSpanError(span interface {
	RecordError(error, ...trace.EventOption)
	SetStatus(codes.Code, string)
}, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
