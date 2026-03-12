package core

import (
	"context"
	"errors"
)

// ErrKeyNotFound is returned by Memory.Recall when the requested key does not exist.
// This is so the handler can provide the agent with feedback that the key was not found.
var ErrKeyNotFound = errors.New("key not found")

// Memory provides a key-value long-term knowledge store for agents.
// Implementations must be safe for concurrent use.
type Memory interface {
	// Store saves a value under the given key. Overwrites if the key already exists.
	Store(ctx context.Context, key string, value string) error

	// Recall retrieves the value for the given key.
	// Returns ErrKeyNotFound if the key does not exist.
	Recall(ctx context.Context, key string) (string, error)

	// Delete removes the value for the given key. No-op if the key does not exist.
	Delete(ctx context.Context, key string) error

	// List returns all keys currently stored in memory.
	List(ctx context.Context) ([]string, error)
}
