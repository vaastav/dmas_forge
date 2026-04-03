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
	chunks, err := chunkDocument(doc)
	if err != nil {
		return err
	}

	// Delete any existing chunks with this document ID
	if err := kb.Delete(ctx, doc.ID); err != nil {
		return err
	}

	inputs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		inputs = append(inputs, chunk.content)
	}

	resp, err := kb.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input:          openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: inputs},
		Model:          kb.model,
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})

	if err != nil {
		return fmt.Errorf("openai knowledge base: create embeddings: %w", err)
	}
	if len(resp.Data) != len(chunks) {
		return fmt.Errorf("openai knowledge base: expected %d embeddings, got %d", len(chunks), len(resp.Data))
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
			return fmt.Errorf("openai knowledge base: store vector: %w", err)
		}

		storedIDs = append(storedIDs, chunk.id)
	}

	kb.mu.Lock()
	kb.docChunks[doc.ID] = storedIDs
	kb.mu.Unlock()
	return nil
}

func (kb *OpenAIKnowledgeBase) Query(ctx context.Context, query string, topK int) ([]core.Chunk, error) {
	if topK <= 0 || strings.TrimSpace(query) == "" {
		return []core.Chunk{}, nil
	}

	resp, err := kb.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input:          openai.EmbeddingNewParamsInputUnion{OfString: openai.String(query)},
		Model:          kb.model,
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, fmt.Errorf("openai knowledge base: create query embedding: %w", err)
	}
	if len(resp.Data) == 0 {
		return []core.Chunk{}, nil
	}

	matches, err := kb.vectorStore.Query(ctx, resp.Data[0].Embedding, topK)
	if err != nil {
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
