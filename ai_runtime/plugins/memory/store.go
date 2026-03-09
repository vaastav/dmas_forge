package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

// InMemoryStore implements core.Memory using a thread-safe in-memory map.
type InMemoryStore struct {
	data map[string]string
	mu   sync.RWMutex
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore(ctx context.Context) (*InMemoryStore, error) {
	return &InMemoryStore{
		data: make(map[string]string),
	}, nil
}

func (s *InMemoryStore) Store(ctx context.Context, key string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *InMemoryStore) Recall(ctx context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	if !ok {
		return "", core.ErrKeyNotFound
	}
	return value, nil
}

func (s *InMemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *InMemoryStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}
