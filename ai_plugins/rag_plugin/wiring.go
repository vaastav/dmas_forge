// Package rag_plugin provides Blueprint IR nodes and wiring functions for
// RAG (Retrieval-Augmented Generation) capabilities.
//
// # Wiring Spec Usage
//
// The package provides several wiring functions for different RAG components:
//
//	// Create a vector store (required for OpenAIKnowledgeBase)
//	rag_plugin.VectorStore[vectorstore.InMemoryVectorStore](spec, "my_vector_store")
//
//	// Create an OpenAI-backed knowledge base
//	rag_plugin.OpenAIKnowledgeBase(spec, "my_kb", "https://api.openai.com", "api-key", "text-embedding-3-small", "my_vector_store")
//
//	// Create a RAG-enabled agent
//	rag_plugin.RAGAgent(spec, "my_agent", "base_agent", "my_kb", rag.RAGAgentConfig{
//	    ToolExposure: rag.SearchOnly,
//	    AutoQuery:    true,
//	    TopK:         5,
//	})
//
// # Custom KnowledgeBase and VectorStore Implementations
//
// To use a custom KnowledgeBase implementation:
//
//	rag_plugin.KnowledgeBase[MyCustomKB](spec, "my_kb")
//	rag_plugin.VectorStore[MyCustomVS](spec, "my_vector_store")
//
// The implementation types (MyCustomKB, MyCustomVS) must satisfy their respective core.KnowledgeBase and core.VectorStore interfaces.
//
// # Tool Exposure Modes
//
//   - NoTools: No RAG tools exposed.
//   - SearchOnly: Exposes read-only tools to the knowledge base.
//   - FullCRUD: Exposes read and write tools to the knowledge base.
package rag_plugin

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	ragruntime "github.com/vaastav/agentic_blueprint/ai_runtime/plugins/rag"
)

// KnowledgeBase creates a Blueprint service node for a custom KnowledgeBase
// implementation.
func KnowledgeBase[Impl core.KnowledgeBase](spec wiring.WiringSpec, name string) string {
	backendName := name + ".knowledge_base"

	spec.Define(backendName, &KnowledgeBaseClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		return newKnowledgeBaseClient[Impl](name)
	})

	pointer.CreatePointer[*KnowledgeBaseClient](spec, name, backendName)
	return name
}

// VectorStore creates a Blueprint service node for a custom VectorStore
// implementation.
func VectorStore[Impl core.VectorStore](spec wiring.WiringSpec, name string) string {
	backendName := name + ".vector_store"

	spec.Define(backendName, &VectorStoreClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		return newVectorStoreClient[Impl](name)
	})

	pointer.CreatePointer[*VectorStoreClient](spec, name, backendName)
	return name
}

// OpenAIKnowledgeBase creates a Blueprint service node for an OpenAI-backed
// knowledge base. vectorStoreName must refer to a previously created
// VectorStore service.
func OpenAIKnowledgeBase(spec wiring.WiringSpec, name string, openaiURL string, apiKey string, embeddingModel string, vectorStoreName string) string {
	backendName := name + ".openai_knowledge_base"

	spec.Define(backendName, &OpenAIKnowledgeBaseClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		var vectorStore ir.IRNode
		if err := ns.Get(vectorStoreName, &vectorStore); err != nil {
			return nil, err
		}
		return newOpenAIKnowledgeBaseClient(name, openaiURL, apiKey, embeddingModel, vectorStore)
	})

	pointer.CreatePointer[*OpenAIKnowledgeBaseClient](spec, name, backendName)
	return name
}

// RAGAgent creates a Blueprint service node that wraps an existing agent with
// RAG capabilities. baseAgent and kb must refer to previously created
// core.Agent and KnowledgeBase services, respectively.
func RAGAgent(spec wiring.WiringSpec, name string, baseAgent string, kb string, config ragruntime.RAGAgentConfig) string {
	backendName := name + ".rag_agent"

	spec.Define(backendName, &RAGAgentClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		var innerAgent ir.IRNode
		if err := ns.Get(baseAgent, &innerAgent); err != nil {
			return nil, err
		}
		var knowledgeBase ir.IRNode
		if err := ns.Get(kb, &knowledgeBase); err != nil {
			return nil, err
		}
		return newRAGAgentClient(name, innerAgent, knowledgeBase, config)
	})

	pointer.CreatePointer[*RAGAgentClient](spec, name, backendName)
	return name
}
