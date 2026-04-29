package rag

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OpenAIKnowledgeBase implements core.KnowledgeBase using OpenAI embeddings
// and a pluggable vector store. It handles document chunking, embedding
// generation via the OpenAI API, and delegates vector storage to the
// provided VectorStore implementation.
type OpenAIKnowledgeBase struct {
	client      *openai.Client
	model       string
	vectorStore core.VectorStore

	mu        sync.RWMutex
	docChunks map[string][]string
}

// baseURL accepts any OpenAI API-compatible endpoint (e.g., OpenAI, local
// models, etc.). apiKey and embeddingModel must be compatible with the API
// specs.
func NewOpenAIKnowledgeBase(ctx context.Context, baseURL string, apiKey string, embeddingModel string, vectorStore core.VectorStore) (*OpenAIKnowledgeBase, error) {
	client := openai.NewClient(option.WithBaseURL(baseURL), option.WithAPIKey(apiKey))
	return &OpenAIKnowledgeBase{
		client:      &client,
		model:       embeddingModel,
		vectorStore: vectorStore,
		docChunks:   make(map[string][]string),
	}, nil
}

func (kb *OpenAIKnowledgeBase) Index(ctx context.Context, doc core.Document) error {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("github.com/vaastav/agentic_blueprint/ai_runtime/plugins/rag")
	ctx, span := tracer.Start(ctx, "kb.index",
		trace.WithAttributes(
			attribute.String("kb.provider", "openai"),
			attribute.String("kb.embedding_model", kb.model),
			attribute.String("kb.document_id", doc.ID),
		),
	)
	defer span.End()

	chunks, err := chunkDocument(doc)
	if err != nil {
		recordKBError(span, err)
		return err
	}
	span.SetAttributes(attribute.Int("kb.chunk_count", len(chunks)))

	// Delete any existing chunks with this document ID
	if err := kb.Delete(ctx, doc.ID); err != nil {
		recordKBError(span, err)
		return err
	}

	inputs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		inputs = append(inputs, chunk.content)
	}

	embedCtx, embedSpan := tracer.Start(ctx, "embedding.create",
		trace.WithAttributes(
			attribute.String("embedding.provider", "openai"),
			attribute.String("embedding.model", kb.model),
			attribute.Int("embedding.input_count", len(inputs)),
		),
	)
	resp, err := kb.client.Embeddings.New(embedCtx, openai.EmbeddingNewParams{
		Input:          openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: inputs},
		Model:          kb.model,
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		recordKBError(embedSpan, err)
		embedSpan.End()
		recordKBError(span, err)
		return fmt.Errorf("openai knowledge base: create embeddings: %w", err)
	}
	embedSpan.SetStatus(codes.Ok, "")
	embedSpan.End()

	if len(resp.Data) != len(chunks) {
		err := fmt.Errorf("openai knowledge base: expected %d embeddings, got %d", len(chunks), len(resp.Data))
		recordKBError(span, err)
		return err
	}

	storedIDs := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		metadata := map[string]any{
			"content":             chunk.content,
			"source_doc_id":       chunk.docID,
			"source_doc_metadata": maps.Clone(chunk.metadata),
		}
		err := kb.vectorStore.Store(ctx, chunk.id, resp.Data[i].Embedding, metadata)

		// We delete any previously stored chunks if storing fails to avoid orphaned data.
		if err != nil {
			for _, storedID := range storedIDs {
				_ = kb.vectorStore.Delete(ctx, storedID)
			}
			recordKBError(span, err)
			return fmt.Errorf("openai knowledge base: store vector: %w", err)
		}

		storedIDs = append(storedIDs, chunk.id)
	}

	kb.mu.Lock()
	kb.docChunks[doc.ID] = storedIDs
	kb.mu.Unlock()
	span.SetStatus(codes.Ok, "")
	return nil
}

func (kb *OpenAIKnowledgeBase) Query(ctx context.Context, query string, topK int) ([]core.Chunk, error) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("github.com/vaastav/agentic_blueprint/ai_runtime/plugins/rag")
	ctx, span := tracer.Start(ctx, "kb.query",
		trace.WithAttributes(
			attribute.String("kb.provider", "openai"),
			attribute.String("kb.embedding_model", kb.model),
			attribute.Int("kb.top_k", topK),
		),
	)
	defer span.End()

	if topK <= 0 || strings.TrimSpace(query) == "" {
		span.SetStatus(codes.Ok, "")
		return []core.Chunk{}, nil
	}

	embedCtx, embedSpan := tracer.Start(ctx, "embedding.create",
		trace.WithAttributes(
			attribute.String("embedding.provider", "openai"),
			attribute.String("embedding.model", kb.model),
			attribute.Int("embedding.input_count", 1),
		),
	)
	resp, err := kb.client.Embeddings.New(embedCtx, openai.EmbeddingNewParams{
		Input:          openai.EmbeddingNewParamsInputUnion{OfString: openai.String(query)},
		Model:          kb.model,
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		recordKBError(embedSpan, err)
		embedSpan.End()
		recordKBError(span, err)
		return nil, fmt.Errorf("openai knowledge base: create query embedding: %w", err)
	}
	embedSpan.SetStatus(codes.Ok, "")
	embedSpan.End()
	if len(resp.Data) == 0 {
		span.SetStatus(codes.Ok, "")
		return []core.Chunk{}, nil
	}

	matches, err := kb.vectorStore.Query(ctx, resp.Data[0].Embedding, topK)
	if err != nil {
		recordKBError(span, err)
		return nil, fmt.Errorf("openai knowledge base: query vector store: %w", err)
	}

	chunks := make([]core.Chunk, 0, len(matches))
	for _, match := range matches {
		chunk := core.Chunk{Score: match.Score, Metadata: map[string]any{}}
		if content, ok := match.Metadata["content"].(string); ok {
			chunk.Content = content
		}
		if sourceDocID, ok := match.Metadata["source_doc_id"].(string); ok {
			chunk.SourceDocID = sourceDocID
		}
		if metadata, ok := match.Metadata["metadata"].(map[string]any); ok {
			chunk.Metadata = maps.Clone(metadata)
		} else {
			chunk.Metadata = map[string]any{}
		}
		chunks = append(chunks, chunk)
	}
	span.SetAttributes(attribute.Int("kb.result_count", len(chunks)))
	span.SetStatus(codes.Ok, "")
	return chunks, nil
}

func (kb *OpenAIKnowledgeBase) Delete(ctx context.Context, docID string) error {
	kb.mu.Lock()
	chunkIDs := append([]string(nil), kb.docChunks[docID]...)
	delete(kb.docChunks, docID)
	kb.mu.Unlock()

	for _, chunkID := range chunkIDs {
		if err := kb.vectorStore.Delete(ctx, chunkID); err != nil {
			return fmt.Errorf("openai knowledge base: delete chunk %s: %w", chunkID, err)
		}
	}
	return nil
}

func recordKBError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
