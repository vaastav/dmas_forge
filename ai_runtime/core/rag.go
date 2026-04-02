package core

import "context"

// Document represents a single document to be indexed in the knowledge base.
type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// Chunk represents a single chunk of a document to be retrieved during a query.
type Chunk struct {
	// the text content of the chunk
	Content string

	// the similarity score between the chunk and the query
	Score float64

	// the ID of the source document this chunk belongs to
	SourceDocID string

	// Optional metadata associated with this chunk (e.g. timestamp)
	Metadata map[string]any
}

// KnowledgeBase defines the interface for RAG knowledge storage.
// Implementations should handle document chunking, embedding generation, and
// similarity search.
type KnowledgeBase interface {
	// Split the given document into chunks, generate their embeddings, and store them
	Index(ctx context.Context, doc Document) error

	// Generate an embedding for the query, retrieving the topK most similar chunks
	Query(ctx context.Context, query string, topK int) ([]Chunk, error)

	// Remove all chunks associated with the given document ID
	Delete(ctx context.Context, docID string) error
}
