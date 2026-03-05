package core

import "context"

// Memory provides a key-value long-term knowledge store for agents.
// Implementations must be safe for concurrent use.
type Memory interface {
	// Store saves a value under the given key. Overwrites if the key already exists.
	Store(ctx context.Context, key string, value string) error

	// Recall retrieves the value for the given key.
	// Returns a human-readable "not found" message (not an error) if the key does not exist,
	// so the LLM receives useful feedback.
	Recall(ctx context.Context, key string) (string, error)

	// Delete removes the value for the given key. No-op if the key does not exist.
	Delete(ctx context.Context, key string) error

	// List returns all keys currently stored in memory.
	List(ctx context.Context) ([]string, error)
}
