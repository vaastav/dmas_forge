// Package rag provides retrieval-augmented generation capabilities for agents.
//
// # Wiring Spec Usage
//
//	my_vector_store := rag_plugin.VectorStore[*vectorstore.InMemoryVectorStore](spec, "my_vector_store")
//	my_kb := rag_plugin.OpenAIKnowledgeBase(spec, "my_kb", "https://api.openai.com", "api-key", "text-embedding-3-small", "my_vector_store")
//	existing_agent := openai_plugin.OpenAILLMAgent(spec, "existing_agent", "https://api.openai.com", "api-key", "gpt-5.4-nano", openai_plugin.AgentConfig{})
//	my_agent := rag_plugin.RAGAgent(spec, "my_agent", "existing_agent", "my_kb", ragruntime.RAGAgentConfig{
//	    ToolExposure: ragruntime.SearchOnly,
//	    AutoQuery:    true,
//	    TopK:         5,
//	})
//
// This creates a new RAGAgent named "my_agent" that wraps "existing_agent"
// and uses "my_kb" as its knowledge base. The agent gains RAG capabilities
// while remaining transparent to workflows that interact with it through
// the core.Agent interface.
//
// # Architecture
//
// The package provides a [RAGAgent] that can wrap any [core.Agent]
// implementation, adding retrieval capabilities in two different modes:
//
//  1. Tool exposure: When configured with SearchOnly or FullCRUD, the agent
//     exposes knowledge base tools (search_knowledge, index_document,
//     delete_document) that the agent can call autonomously.
//
//  2. Auto-query: When enabled, all queries are automatically augmented with
//     relevant context from the knowledge base before being sent to the agent.
//     This requires no tool calls.
//
// The two modes can be used independently or together. With NoTools, the
// agent functions purely as an auto-query wrapper. With SearchOnly and
// AutoQuery disabled, the agent must explicitly call search_knowledge.
//
// # Tool Exposure Levels
//
//   - NoTools: No RAG tools exposed.
//   - SearchOnly: Exposes search_knowledge tool. The agent can query the KB
//     but cannot modify it.
//   - FullCRUD: Exposes all RAG tools. The agent can search, index, and
//     delete documents.
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
	// ToolExposure determines which RAG tools (if any) are exposed to the underlying agent.
	ToolExposure ToolExposure `json:"tool_exposure"`

	// AutoQuery controls whether context is automatically injected into queries.
	AutoQuery bool `json:"auto_query"`

	// TopK specifies how many relevant chunks to retrieve for auto-query.
	TopK int `json:"top_k"`
}

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

	switch r.config.ToolExposure {
	case SearchOnly:
		parts = append(parts, searchToolInfo)
	case FullCRUD:
		parts = append(parts, searchToolInfo, crudToolInfo)
	case NoTools:
		// No tools exposed.
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
