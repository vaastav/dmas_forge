package rag

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

var ragToolDefs = map[string]openai.ChatCompletionToolParam{
	"search_knowledge": {
		Function: openai.FunctionDefinitionParam{
			Name:        "search_knowledge",
			Description: openai.String("Search the knowledge base for information relevant to the user's request."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]string{
						"type":        "string",
						"description": "The search query to run against the knowledge base",
					},
					"top_k": map[string]any{
						"type":        "integer",
						"description": "Optional maximum number of results to return",
					},
				},
				"required": []string{"query"},
			},
		},
	},
	"index_document": {
		Function: openai.FunctionDefinitionParam{
			Name:        "index_document",
			Description: openai.String("Index a document into the knowledge base for future retrieval."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]string{
						"type":        "string",
						"description": "A unique ID for the document",
					},
					"content": map[string]string{
						"type":        "string",
						"description": "The document content to index",
					},
					"metadata": map[string]any{
						"type":                 "object",
						"description":          "Optional metadata to store with the document",
						"additionalProperties": true,
					},
				},
				"required": []string{"id", "content"},
			},
		},
	},
	"delete_document": {
		Function: openai.FunctionDefinitionParam{
			Name:        "delete_document",
			Description: openai.String("Delete a document from the knowledge base by its ID."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"doc_id": map[string]string{
						"type":        "string",
						"description": "The ID of the document to delete",
					},
				},
				"required": []string{"doc_id"},
			},
		},
	},
}

var ragToolNames = map[string]bool{
	"search_knowledge": true,
	"index_document":   true,
	"delete_document":  true,
}

func (r *RAGAgent) registerTools(ctx context.Context) error {
	if r.config.ToolExposure == NoTools {
		return nil
	}

	toolDefs := map[string]openai.ChatCompletionToolParam{
		"search_knowledge": ragToolDefs["search_knowledge"],
	}
	if r.config.ToolExposure == FullCRUD {
		toolDefs["index_document"] = ragToolDefs["index_document"]
		toolDefs["delete_document"] = ragToolDefs["delete_document"]
	}

	if err := r.inner.AddTools(ctx, toolDefs); err != nil {
		return fmt.Errorf("rag agent: failed to add tools: %w", err)
	}
	if err := r.inner.RegisterToolCallHandler(ctx, r.buildCompositeHandler()); err != nil {
		return fmt.Errorf("rag agent: failed to register tool handler: %w", err)
	}
	return nil
}

func (r *RAGAgent) buildCompositeHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		if ragToolNames[tc.Function.Name] {
			if r.config.ToolExposure == SearchOnly && tc.Function.Name != "search_knowledge" {
				return "", fmt.Errorf("unsupported tool call: %s", tc.Function.Name)
			}
			return r.handleRAGToolCall(ctx, tc)
		}
		if r.userHandler != nil {
			return r.userHandler(ctx, tc)
		}
		return "", fmt.Errorf("unsupported tool call: %s", tc.Function.Name)
	}
}

func (r *RAGAgent) handleRAGToolCall(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	switch tc.Function.Name {
	case "search_knowledge":
		return r.handleSearchKnowledge(ctx, tc)
	case "index_document":
		return r.handleIndexDocument(ctx, tc)
	case "delete_document":
		return r.handleDeleteDocument(ctx, tc)
	default:
		return "", fmt.Errorf("unknown rag tool: %s", tc.Function.Name)
	}
}

func (r *RAGAgent) handleSearchKnowledge(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("search_knowledge: invalid arguments: %w", err)
	}
	if args.TopK <= 0 {
		args.TopK = defaultSearchTopK
	}
	chunks, err := r.kb.Query(ctx, args.Query, args.TopK)
	if err != nil {
		return "", fmt.Errorf("search_knowledge: %w", err)
	}
	return formatChunks(chunks), nil
}

func (r *RAGAgent) handleIndexDocument(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		ID       string         `json:"id"`
		Content  string         `json:"content"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("index_document: invalid arguments: %w", err)
	}
	if err := r.kb.Index(ctx, core.Document{ID: args.ID, Content: args.Content, Metadata: args.Metadata}); err != nil {
		return "", fmt.Errorf("index_document: %w", err)
	}
	return fmt.Sprintf("Indexed document '%s'.", args.ID), nil
}

func (r *RAGAgent) handleDeleteDocument(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args struct {
		DocID string `json:"doc_id"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("delete_document: invalid arguments: %w", err)
	}
	if err := r.kb.Delete(ctx, args.DocID); err != nil {
		return "", fmt.Errorf("delete_document: %w", err)
	}
	return fmt.Sprintf("Deleted document '%s'.", args.DocID), nil
}
