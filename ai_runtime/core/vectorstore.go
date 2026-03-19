package core

import "context"

type VectorMatch struct {
	ID       string
	Score    float64
	Metadata map[string]any
}

// VectorStore defines the interface for vector storage and similarity search.
// Implementations store vectors with associated metadata and support
// nearest-neighbor queries based on cosine similarity.
//
// A VectorStore is a lower-level primitive used by KnowledgeBase implementations.
// It operates on pre-computed embeddings rather than raw text, making it suitable
// for any use case requiring vector similarity search, not just RAG.
type VectorStore interface {
	Store(ctx context.Context, id string, vector []float64, metadata map[string]any) error
	Query(ctx context.Context, vector []float64, topK int) ([]VectorMatch, error)
	Delete(ctx context.Context, id string) error
}
