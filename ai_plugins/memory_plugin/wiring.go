// Package memory_plugin provides wiring functions for memory-backed agents.
// Use MemoryStore[Impl] to create a memory store with any core.Memory implementation,
// and MemoryAgent to wrap an agent with LLM-driven memory capabilities.
package memory_plugin

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/vaastav/dmas_forge/ai_runtime/core"
)

// MemoryStore creates a memory store using the specified implementation.
// Impl must be a concrete type that implements [core.Memory].
//
// Example usage:
//
//	// Using the built-in in-memory store
//	memStore := memory_plugin.MemoryStore[*memory.InMemoryStore](spec, "my_memory")
//
//	// Using a custom Redis-backed store
//	memStore := memory_plugin.MemoryStore[*redis.RedisMemory](spec, "my_memory")
func MemoryStore[Impl core.Memory](spec wiring.WiringSpec, name string) string {
	backendName := name + ".memory_store"

	spec.Define(backendName, &MemoryStoreClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		return newMemoryStoreClient[Impl](name)
	})

	pointer.CreatePointer[*MemoryStoreClient](spec, name, backendName)

	return name
}

// MemoryAgent wraps an existing agent with LLM-driven memory capabilities.
// The agent transparently gains memory tools (store, recall, delete, list) that
// the LLM can use autonomously. The workflow code is completely unaware of memory.
//
// agentName must reference an already-defined agent in the spec.
// memoryName must reference an already-defined memory store in the spec.
func MemoryAgent(spec wiring.WiringSpec, name string, agentName string, memoryName string) string {
	backendName := name + ".memory_agent"

	spec.Define(backendName, &MemoryAgentClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		var innerAgent ir.IRNode
		if err := ns.Get(agentName, &innerAgent); err != nil {
			return nil, err
		}
		var memStore ir.IRNode
		if err := ns.Get(memoryName, &memStore); err != nil {
			return nil, err
		}
		return newMemoryAgentClient(name, innerAgent, memStore)
	})

	pointer.CreatePointer[*MemoryAgentClient](spec, name, backendName)

	return name
}
