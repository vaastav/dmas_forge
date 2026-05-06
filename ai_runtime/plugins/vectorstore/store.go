// Package vectorstore provides in-memory vector storage implementations.
// The InMemoryVectorStore is suitable for development and testing, but
// production deployments should use a persistent vector database.
//
// # Wiring Spec Usage
//
//	rag_plugin.VectorStore[*vectorstore.InMemoryVectorStore](spec, "my_vector_store")
//
// This creates an InMemoryVectorStore named "my_vector_store". The store
// can then be used as the backing storage for a KnowledgeBase implementation.
//
// The InMemoryVectorStore is safe for concurrent use as it uses mutexes when
// accessing the underlying vector map.
package vectorstore

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/vaastav/dmas_forge/ai_runtime/core"
)

type storedVector struct {
	vector   []float64
	metadata map[string]any
}

// InMemoryVectorStore implements core.VectorStore using a thread-safe
// in-memory map. Uses cosine similarity for similarity search.
type InMemoryVectorStore struct {
	mu      sync.RWMutex
	vectors map[string]storedVector
}

func NewInMemoryVectorStore(ctx context.Context) (*InMemoryVectorStore, error) {
	return &InMemoryVectorStore{vectors: make(map[string]storedVector)}, nil
}

func (s *InMemoryVectorStore) Store(ctx context.Context, id string, vector []float64, metadata map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// this overwrites any existing vector with the same ID
	s.vectors[id] = storedVector{
		vector:   append([]float64(nil), vector...),
		metadata: copyMetadata(metadata),
	}
	return nil
}

func (s *InMemoryVectorStore) Query(ctx context.Context, vector []float64, topK int) ([]core.VectorMatch, error) {
	if topK <= 0 || len(vector) == 0 {
		return []core.VectorMatch{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryNorm := magnitude(vector)
	if queryNorm == 0 {
		return []core.VectorMatch{}, nil
	}

	matches := make([]core.VectorMatch, 0, len(s.vectors))
	for id, stored := range s.vectors {
		score, ok := cosineSimilarity(vector, queryNorm, stored.vector)
		if !ok {
			continue
		}
		matches = append(matches, core.VectorMatch{
			ID:       id,
			Score:    score,
			Metadata: copyMetadata(stored.metadata),
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Score > matches[j].Score
	})

	if topK > len(matches) {
		topK = len(matches)
	}
	return matches[:topK], nil
}

func (s *InMemoryVectorStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vectors, id)
	return nil
}

func cosineSimilarity(query []float64, queryNorm float64, candidate []float64) (float64, bool) {
	if len(query) == 0 || len(query) != len(candidate) {
		return 0, false
	}

	candidateNorm := magnitude(candidate)
	if candidateNorm == 0 {
		return 0, false
	}

	dot := 0.0
	for i := range query {
		dot += query[i] * candidate[i]
	}
	return dot / (queryNorm * candidateNorm), true
}

func magnitude(vector []float64) float64 {
	sum := 0.0
	for _, value := range vector {
		sum += value * value
	}
	return math.Sqrt(sum)
}

func copyMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = deepCopyAny(value)
	}
	return cloned
}

func deepCopyAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return copyMetadata(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = deepCopyAny(item)
		}
		return cloned
	default:
		return typed
	}
}
