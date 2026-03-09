// Package memory_plugin provides a client-wrapper implementation of the [core.Memory] interface, along with a MemoryAgent that can wrap any agent to give it LLM-driven memory capabilities.
package memory_plugin

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
)

// MemoryStore registers an in-memory key-value store.
func MemoryStore(spec wiring.WiringSpec, name string) string {
	backendName := name + ".memory_store"

	spec.Define(backendName, &MemoryStoreClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		return newMemoryStoreClient(name)
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
