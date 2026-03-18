package core

import "context"

type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
}

type Chunk struct {
	Content     string
	Score       float64
	SourceDocID string
	Metadata    map[string]any
}

type KnowledgeBase interface {
	Index(ctx context.Context, doc Document) error
	Query(ctx context.Context, query string, topK int) ([]Chunk, error)
	Delete(ctx context.Context, docID string) error
}
