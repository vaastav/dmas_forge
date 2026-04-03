package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	"github.com/vaastav/agentic_blueprint/ai_plugins/rag_plugin"
	ragruntime "github.com/vaastav/agentic_blueprint/ai_runtime/plugins/rag"
	"github.com/vaastav/agentic_blueprint/ai_runtime/plugins/vectorstore"
	wf "github.com/vaastav/agentic_blueprint/examples/rag_chat/workflow"
)

var Automatic = cmdbuilder.SpecOption{
	Name:        "automatic",
	Description: "Deploys rag chat with auto-query, no tools - pure retrieval augmentation",
	Build:       makeAutomaticSpec,
}

func makeAutomaticSpec(spec wiring.WiringSpec) ([]string, error) {
	model, err := readModelInfo()
	if err != nil {
		return nil, err
	}

	baseAgent := openai_plugin.OpenAILLMAgent(spec, "agent_base", model.URL, model.Key, model.Name, openai_plugin.AgentConfig{})
	vectorStoreName := rag_plugin.VectorStore[*vectorstore.InMemoryVectorStore](spec, "vector_store")
	kb := rag_plugin.OpenAIKnowledgeBase(spec, "knowledge_base", model.URL, model.Key, model.EmbeddingModel, vectorStoreName)
	agent := rag_plugin.RAGAgent(spec, "agent", baseAgent, kb, ragruntime.RAGAgentConfig{
		ToolExposure: ragruntime.NoTools,
		AutoQuery:    true,
		TopK:         3,
	})

	chatService := workflow.Service[wf.ChatAgent](spec, "chat_service", agent, kb, "*")
	http.Deploy(spec, chatService)
	goproc.Deploy(spec, chatService)
	chatCtr := linuxcontainer.Deploy(spec, chatService)
	return []string{chatCtr}, nil
}
