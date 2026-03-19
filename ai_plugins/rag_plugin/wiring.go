package rag_plugin

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	ragruntime "github.com/vaastav/agentic_blueprint/ai_runtime/plugins/rag"
)

type ToolExposure = ragruntime.ToolExposure

const (
	NoTools    = ragruntime.NoTools
	SearchOnly = ragruntime.SearchOnly
	FullCRUD   = ragruntime.FullCRUD
)

type RAGAgentConfig = ragruntime.RAGAgentConfig

func KnowledgeBase[Impl core.KnowledgeBase](spec wiring.WiringSpec, name string) string {
	backendName := name + ".knowledge_base"

	spec.Define(backendName, &KnowledgeBaseClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		return newKnowledgeBaseClient[Impl](name)
	})

	pointer.CreatePointer[*KnowledgeBaseClient](spec, name, backendName)
	return name
}

func VectorStore[Impl core.VectorStore](spec wiring.WiringSpec, name string) string {
	backendName := name + ".vector_store"

	spec.Define(backendName, &VectorStoreClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		return newVectorStoreClient[Impl](name)
	})

	pointer.CreatePointer[*VectorStoreClient](spec, name, backendName)
	return name
}

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

func RAGAgent(spec wiring.WiringSpec, name string, baseAgent string, kb string, config RAGAgentConfig) string {
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
		return newRAGAgentClient(name, innerAgent, knowledgeBase, config.ToolExposure, config.AutoQuery, config.TopK)
	})

	pointer.CreatePointer[*RAGAgentClient](spec, name, backendName)
	return name
}
