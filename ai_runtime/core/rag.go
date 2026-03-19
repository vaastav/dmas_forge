package core

import "context"

// Document represents a single document to be indexed in the knowledge base.
// This document should be split into chunks by the knowledge base implementation.
type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// Chunk represents a single chunk of a document, used for retrieval and generation.
// Each chunk has a score indicating its relevance to the query, and a source document ID.
type Chunk struct {
	Content     string
	Score       float64
	SourceDocID string
	Metadata    map[string]any
}

// KnowledgeBase defines the interface for RAG knowledge storage.
// Implementations handle document chunking, embedding generation, and similarity search.
//
// The typical workflow is:
//   - Index: Split document intochunks, generate embeddings, store in vector store
//   - Query: Generate query embedding, find similar chunks via vector search
//   - Delete: Remove all chunks associated with a document
//
// A KnowledgeBase differs from a raw VectorStore in that it manages the full
// pipeline of chunking documents and generating embeddings, while a VectorStore
// only handles raw vector operations.
//
// Additionally, this interface allows you to implement more advanced features such as:
//   - Hybrid search (vector + keyword search)
//   - Reranker (re-rank search results based on relevance)
//   - Metadata filtering
type KnowledgeBase interface {
	Index(ctx context.Context, doc Document) error
	Query(ctx context.Context, query string, topK int) ([]Chunk, error)
	Delete(ctx context.Context, docID string) error
}
