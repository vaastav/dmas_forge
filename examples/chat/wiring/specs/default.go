package specs

import (
	"encoding/json"
	"flag"
	"io"
	"os"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/memory_plugin"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	"github.com/vaastav/agentic_blueprint/ai_runtime/plugins/memory"
	wf "github.com/vaastav/agentic_blueprint/examples/chat/workflow"
)

type ModelInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Key  string `json:"key"`
}

var Docker = cmdbuilder.SpecOption{
	Name:        "docker",
	Description: "Deploys the chat agent in a container with http, uses OpenAI with memory enabled",
	Build:       makeDockerSpec,
}

var model_file = flag.String("modfile", "model.json", "Specific model related information")

func makeDockerSpec(spec wiring.WiringSpec) ([]string, error) {

	applyDockerDefaults := func(spec wiring.WiringSpec, serviceName string) string {
		http.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	var minfo ModelInfo
	file, err := os.Open(*model_file)
	if err != nil {
		return []string{}, err
	}
	defer file.Close()

	all_bytes, err := io.ReadAll(file)
	if err != nil {
		return []string{}, err
	}
	err = json.Unmarshal(all_bytes, &minfo)
	if err != nil {
		return []string{}, err
	}

	model_url := minfo.URL
	model_key := minfo.Key
	model_name := minfo.Name

	// Create memory store
	memStore := memory_plugin.MemoryStore[*memory.InMemoryStore](spec, "chat_memory")

	// Create base LLM agent, then wrap with memory
	baseAgent := openai_plugin.OpenAILLMAgent(spec, "agent_base", model_url, model_key, model_name, openai_plugin.AgentConfig{})
	agent := memory_plugin.MemoryAgent(spec, "agent", baseAgent, memStore)

	// Register workflow service -- workflow only sees core.Agent
	chatService := workflow.Service[wf.ChatAgent](spec, "chat_service", agent)
	chatCtr := applyDockerDefaults(spec, chatService)

	return []string{chatCtr}, nil
}
