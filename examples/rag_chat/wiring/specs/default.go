package specs

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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

type ModelInfo struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Key            string `json:"key"`
	EmbeddingModel string `json:"embedding_model"`
}

var Automatic = cmdbuilder.SpecOption{
	Name:        "automatic",
	Description: "Deploys rag chat with auto-query, no tools - pure retrieval augmentation",
	Build:       makeAutomaticSpec,
}

var Agentic = cmdbuilder.SpecOption{
	Name:        "agentic",
	Description: "Deploys rag chat with no auto features - agent has full CRUD control over knowledge base",
	Build:       makeAgenticSpec,
}

var modelFile = flag.String("modfile", "model.json", "Specific model related information")

func makeAutomaticSpec(spec wiring.WiringSpec) ([]string, error) {
	model, err := readModelInfo()
	if err != nil {
		return nil, err
	}

	baseAgent, kb := defineRAGStack(spec, model)
	agent := rag_plugin.RAGAgent(spec, "agent", baseAgent, kb, ragruntime.RAGAgentConfig{
		ToolExposure: ragruntime.NoTools,
		AutoQuery:    true,
		TopK:         3,
	})

	chatCtr := deployChatService(spec, agent, kb, "*")
	return []string{chatCtr}, nil
}

func makeAgenticSpec(spec wiring.WiringSpec) ([]string, error) {
	model, err := readModelInfo()
	if err != nil {
		return nil, err
	}

	baseAgent, kb := defineRAGStack(spec, model)
	agent := rag_plugin.RAGAgent(spec, "agent", baseAgent, kb, ragruntime.RAGAgentConfig{
		ToolExposure: ragruntime.FullCRUD,
		AutoQuery:    false,
		TopK:         3,
	})

	chatCtr := deployChatService(spec, agent, kb, "")
	return []string{chatCtr}, nil
}

func defineRAGStack(spec wiring.WiringSpec, model ModelInfo) (string, string) {
	baseAgent := openai_plugin.OpenAILLMAgent(spec, "agent_base", model.URL, model.Key, model.Name, openai_plugin.AgentConfig{})
	vectorStoreName := rag_plugin.VectorStore[*vectorstore.InMemoryVectorStore](spec, "vector_store")
	kb := rag_plugin.OpenAIKnowledgeBase(spec, "knowledge_base", model.URL, model.Key, model.EmbeddingModel, vectorStoreName)
	return baseAgent, kb
}

func readModelInfo() (ModelInfo, error) {
	var model ModelInfo
	file, err := os.Open(*modelFile)
	if err != nil {
		return ModelInfo{}, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return ModelInfo{}, err
	}
	if err := json.Unmarshal(bytes, &model); err != nil {
		return ModelInfo{}, err
	}
	if strings.TrimSpace(model.EmbeddingModel) == "" {
		return ModelInfo{}, fmt.Errorf("embedding_model must be set in %s", *modelFile)
	}
	return model, nil
}

func deployChatService(spec wiring.WiringSpec, agent string, kb string, preIndexFiles string) string {
	chatService := workflow.Service[wf.ChatAgent](spec, "chat_service", agent, kb, preIndexFiles)
	http.Deploy(spec, chatService)
	goproc.Deploy(spec, chatService)
	return linuxcontainer.Deploy(spec, chatService)
}
