// Package rag provides retrieval-augmented generation capabilities for agents.
//
// # Wiring Spec Usage
//
//	rag_plugin.RAGAgent(spec, "my_agent", "existing_agent", "my_kb", rag_plugin.RAGAgentConfig{
//	    ToolExposure: rag_plugin.SearchOnly,
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
// The package implements a decorator pattern around [core.Agent]. The
// [RAGAgent] type adds retrieval capabilities in two different modes:
//
//  1. Tool exposure: When configured with SearchOnly or FullCRUD, the agent
//     exposes knowledge base tools (search_knowledge, index_document,
//     delete_document) that the LLM can call autonomously.
//
//  2. Auto-query: When enabled, all queries are automatically augmented with
//     relevant context from the knowledge base before being sent to the LLM.
//     This is invisible to the workflow and requires no tool calls.
//
// The two modes can be used independently or together. With NoTools, the
// agent functions purely as an auto-query wrapper. With SearchOnly and
// AutoQuery disabled, the LLM must explicitly call search_knowledge.
//
// # Tool Exposure Levels
//
//   - NoTools: No RAG tools exposed. Useful with AutoQuery enabled for
//     transparent context injection without LLM awareness.
//   - SearchOnly: Exposes search_knowledge tool. The LLM can query the KB
//     but cannot modify it. Suitable for read-only knowledge bases.
//   - FullCRUD: Exposes all RAG tools. The LLM can search, index, and
//     delete documents. Useful for dynamic knowledge bases.
//
// # Knowledge Base vs Vector Store
//
// A [core.KnowledgeBase] manages the full RAG pipeline: document chunking,
// embedding generation, and semantic search. It internally uses a
// [core.VectorStore] for raw vector operations. The [OpenAIKnowledgeBase]
// implementation uses OpenAI's embedding API and is pluggable with any
// VectorStore implementation.
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

// RAGAgent is a decorator that wraps any core.Agent with retrieval-augmented
// generation capabilities.
//
// Workflows interact with RAGAgent through the core.Agent interface and are
// unaware of RAG capabilities. Whether an agent has RAG is a wiring decision.
type RAGAgent struct {
	inner       core.Agent
	kb          core.KnowledgeBase
	config      RAGAgentConfig
	userHandler core.ToolHandlerFn
}

// NewRAGAgent wraps the given agent with RAG capabilities.
// Parameters are passed as strings for compatibility with the wiring system.
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

// AddSystemPrompt appends RAG-specific instructions to the user's prompt and
// forwards it to the inner agent. The instructions vary based on tool exposure
// and auto-query configuration.
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

// If AutoQuery is enabled, the query is augmented with relevant knowledge
// base context before being sent to the LLM. Otherwise, the query is sent as-is.
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

// If AutoQuery is enabled, the query is augmented with relevant knowledge
// base context before being sent to the LLM. Otherwise, the query is sent as-is.
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

// RegisterToolCallHandler handles RAG tool calls and delegates everything else
// to the user's handler.
func (r *RAGAgent) RegisterToolCallHandler(ctx context.Context, toolHandlerFn core.ToolHandlerFn) error {
	r.userHandler = toolHandlerFn
	if r.config.ToolExposure == NoTools {
		return r.inner.RegisterToolCallHandler(ctx, toolHandlerFn)
	}
	return r.inner.RegisterToolCallHandler(ctx, r.buildCompositeHandler())
}

// prepareQuery enriches the query with knowledge base context when AutoQuery
// is enabled.
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
