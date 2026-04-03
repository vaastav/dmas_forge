package core

import "context"

// VectorMatch represents a single match returned by a VectorStore query.
type VectorMatch struct {
	ID       string
	Score    float64
	Metadata map[string]any
}

// VectorStore defines the interface for vector storage and similarity search.
type VectorStore interface {
	// Store the given vector with its associated ID and metadata.
	Store(ctx context.Context, id string, vector []float64, metadata map[string]any) error

	// Retrieve the topK most similar vectors to the given query vector.
	Query(ctx context.Context, vector []float64, topK int) ([]VectorMatch, error)

	// Remove the vector associated with the given ID.
	Delete(ctx context.Context, id string) error
}
