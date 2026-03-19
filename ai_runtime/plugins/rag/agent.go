package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type ToolExposure int

const (
	NoTools ToolExposure = iota
	SearchOnly
	FullCRUD
)

type RAGAgentConfig struct {
	ToolExposure ToolExposure `json:"tool_exposure"`
	AutoQuery    bool         `json:"auto_query"`
	TopK         int          `json:"top_k"`
}

// RAGAgent is a decorator that adds retrieval-augmented generation capabilities to any core.Agent.
type RAGAgent struct {
	inner       core.Agent
	kb          core.KnowledgeBase
	config      RAGAgentConfig
	userHandler core.ToolHandlerFn
}

func NewRAGAgent(ctx context.Context, agent core.Agent, kb core.KnowledgeBase, toolExposure string, autoQuery string, topK string) (*RAGAgent, error) {
	toolExposureValue, err := strconv.Atoi(toolExposure)
	if err != nil {
		return nil, fmt.Errorf("rag agent: invalid tool exposure: %w", err)
	}
	autoQueryValue, err := strconv.ParseBool(autoQuery)
	if err != nil {
		return nil, fmt.Errorf("rag agent: invalid auto_query: %w", err)
	}
	topKValue, err := strconv.Atoi(topK)
	if err != nil {
		return nil, fmt.Errorf("rag agent: invalid top_k: %w", err)
	}

	config := RAGAgentConfig{
		ToolExposure: ToolExposure(toolExposureValue),
		AutoQuery:    autoQueryValue,
		TopK:         topKValue,
	}
	switch config.ToolExposure {
	case NoTools, SearchOnly, FullCRUD:
	default:
		return nil, fmt.Errorf("rag agent: invalid tool exposure value: %d", toolExposureValue)
	}
	if config.TopK <= 0 {
		config.TopK = defaultAutoQueryTopK
	}

	r := &RAGAgent{inner: agent, kb: kb, config: config}

	if err := r.registerTools(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RAGAgent) AddSystemPrompt(ctx context.Context, prompt string) error {
	parts := []string{
		prompt,
		"\n\nYou are an assistant with access to a knowledge base.",
	}

	searchToolInfo := "You can search a knowledge base with `search_knowledge` whenever additional context would improve your answer."
	crudToolInfo := "You can persist new information with `index_document` and remove outdated information with `delete_document`."
	autoQueryInfo := "Relevant knowledge base context may be injected automatically before each request."

	if r.config.ToolExposure == SearchOnly {
		parts = append(parts, searchToolInfo)
	} else if r.config.ToolExposure == FullCRUD {
		parts = append(parts, searchToolInfo, crudToolInfo)
	}

	if r.config.AutoQuery {
		parts = append(parts, autoQueryInfo)
	}

	return r.inner.AddSystemPrompt(ctx, strings.Join(parts, " "))
}

func (r *RAGAgent) AddTools(ctx context.Context, tooldefs map[string]openai.ChatCompletionToolParam) error {
	return r.inner.AddTools(ctx, tooldefs)
}

func (r *RAGAgent) LLMCall(ctx context.Context, query string) (string, error) {
	preparedQuery, err := r.prepareQuery(ctx, query)
	if err != nil {
		return "", err
	}

	response, err := r.inner.LLMCall(ctx, preparedQuery)
	if err != nil {
		return "", err
	}
	return response, nil
}

func (r *RAGAgent) LLMCallWithTools(ctx context.Context, query string) (string, error) {
	preparedQuery, err := r.prepareQuery(ctx, query)
	if err != nil {
		return "", err
	}

	response, err := r.inner.LLMCallWithTools(ctx, preparedQuery)
	if err != nil {
		return "", err
	}
	return response, nil
}

func (r *RAGAgent) RegisterToolCallHandler(ctx context.Context, toolHandlerFn core.ToolHandlerFn) error {
	r.userHandler = toolHandlerFn
	if r.config.ToolExposure == NoTools {
		return r.inner.RegisterToolCallHandler(ctx, toolHandlerFn)
	}
	return r.inner.RegisterToolCallHandler(ctx, r.buildCompositeHandler())
}

func (r *RAGAgent) prepareQuery(ctx context.Context, query string) (string, error) {
	if !r.config.AutoQuery {
		return query, nil
	}
	chunks, err := r.kb.Query(ctx, query, r.config.TopK)
	if err != nil {
		return "", fmt.Errorf("rag agent: auto-query failed: %w", err)
	}
	if len(chunks) == 0 {
		return query, nil
	}

	var builder strings.Builder
	builder.WriteString("Use the following knowledge base context if it is relevant to the user's request. ")
	builder.WriteString("Prefer the retrieved facts when they help answer accurately.\n\n")
	builder.WriteString("Knowledge base context:\n")
	builder.WriteString(formatChunks(chunks))
	builder.WriteString("\n\nUser request:\n")
	builder.WriteString(query)
	return builder.String(), nil
}
