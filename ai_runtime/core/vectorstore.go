package core

import "context"

type VectorMatch struct {
	ID       string
	Score    float64
	Metadata map[string]any
}

type VectorStore interface {
	Store(ctx context.Context, id string, vector []float64, metadata map[string]any) error
	Query(ctx context.Context, vector []float64, topK int) ([]VectorMatch, error)
	Delete(ctx context.Context, id string) error
}
