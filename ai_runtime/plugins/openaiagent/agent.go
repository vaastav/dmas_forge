package openaiagent

import (
	"context"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type OpenAILLMClient struct {
	hasSysMsg     bool
	tool_map      map[string]openai.ChatCompletionToolParam
	tools         []openai.ChatCompletionToolParam
	sysMsg        string
	client        *openai.Client
	model         string
	toolHandlerFn core.ToolHandlerFn
}

func NewOpenAILLMClient(ctx context.Context, url string, apikey string, model_name string) (*OpenAILLMClient, error) {
	client := openai.NewClient(option.WithBaseURL(url), option.WithAPIKey(apikey))
	return &OpenAILLMClient{client: &client, tool_map: make(map[string]openai.ChatCompletionToolParam), model: model_name}, nil
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
	c.tool_map = tooldefs
	c.tools = mapsToValues(tooldefs)
	return nil
}

func (c *OpenAILLMClient) LLMCall(ctx context.Context, query string) (string, error) {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(c.sysMsg),
			openai.UserMessage(query),
		},
		Model: c.model,
	}

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	return completion.Choices[0].Message.Content, nil
}

func (c *OpenAILLMClient) LLMCallWithTools(ctx context.Context, query string) (string, error) {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(c.sysMsg),
			openai.UserMessage(query),
		},
		Tools: c.tools,
		Model: c.model,
	}
	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}

	toolCalls := completion.Choices[0].Message.ToolCalls
	if len(toolCalls) == 0 {
		return completion.Choices[0].Message.Content, nil
	}

	// Handle tool calls by continuing the conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	for _, toolCall := range toolCalls {
		res, err := c.toolHandlerFn(ctx, toolCall)
		if err != nil {
			// Abort if the tool handler function was unable to handle the tool call
			return "", err
		}
		params.Messages = append(params.Messages, openai.ToolMessage(res, toolCall.ID))
	}

	// Continue conversation post tool-call
	completion, err = c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}

	return completion.Choices[0].Message.Content, nil
}

func mapsToValues(tooldefs map[string]openai.ChatCompletionToolParam) []openai.ChatCompletionToolParam {
	vals := []openai.ChatCompletionToolParam{}
	for _, v := range tooldefs {
		vals = append(vals, v)
	}
	return vals
}
