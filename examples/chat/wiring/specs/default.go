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

var Memory = cmdbuilder.SpecOption{
	Name:        "memory",
	Description: "Deploys the chat agent in a container with http, uses OpenAI with memory enabled",
	Build:       makeMemorySpec,
}

var NoMemory = cmdbuilder.SpecOption{
	Name:        "no_memory",
	Description: "Deploys the chat agent in a container with http, uses OpenAI without memory",
	Build:       makeNoMemorySpec,
}

var model_file = flag.String("modfile", "model.json", "Specific model related information")

func makeMemorySpec(spec wiring.WiringSpec) ([]string, error) {
	minfo, err := readModelInfo()
	if err != nil {
		return []string{}, err
	}

	memStore := memory_plugin.MemoryStore[*memory.InMemoryStore](spec, "chat_memory")
	baseAgent := openai_plugin.OpenAILLMAgent(spec, "agent_base", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	agent := memory_plugin.MemoryAgent(spec, "agent", baseAgent, memStore)

	chatCtr := deployChatService(spec, agent)
	return []string{chatCtr}, nil
}

func makeNoMemorySpec(spec wiring.WiringSpec) ([]string, error) {
	minfo, err := readModelInfo()
	if err != nil {
		return []string{}, err
	}

	agent := openai_plugin.OpenAILLMAgent(spec, "agent", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})

	chatCtr := deployChatService(spec, agent)
	return []string{chatCtr}, nil
}

func readModelInfo() (ModelInfo, error) {
	var minfo ModelInfo
	file, err := os.Open(*model_file)
	if err != nil {
		return ModelInfo{}, err
	}
	defer file.Close()

	all_bytes, err := io.ReadAll(file)
	if err != nil {
		return ModelInfo{}, err
	}
	err = json.Unmarshal(all_bytes, &minfo)
	if err != nil {
		return ModelInfo{}, err
	}

	return minfo, nil
}

func deployChatService(spec wiring.WiringSpec, agent string) string {
	chatService := workflow.Service[wf.ChatAgent](spec, "chat_service", agent)
	http.Deploy(spec, chatService)
	goproc.Deploy(spec, chatService)
	return linuxcontainer.Deploy(spec, chatService)
}
