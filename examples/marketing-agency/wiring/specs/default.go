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

	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow"
)

type ModelInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Key  string `json:"key"`
}

var Docker = cmdbuilder.SpecOption{
	Name:        "docker",
	Description: "Deploy marketing agency coordinator as HTTP service in Docker",
	Build:       makeDockerSpec,
}

var modelFile = flag.String("modfile", "model.json", "Specific model related information")

func makeDockerSpec(spec wiring.WiringSpec) ([]string, error) {
	model, err := readModelInfo()
	if err != nil {
		return []string{}, err
	}

	domainAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"domain_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	websiteAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"website_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	marketingAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"marketing_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	logoAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"logo_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	coordinatorCore := openai_plugin.OpenAILLMAgent(
		spec,
		"coordinator_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)

	domainService := workflow.Service[wf.DomainAgent](spec, "domain_service", domainAgentCore)
	websiteService := workflow.Service[wf.WebsiteAgent](spec, "website_service", websiteAgentCore)
	marketingService := workflow.Service[wf.MarketingAgent](spec, "marketing_service", marketingAgentCore)
	logoService := workflow.Service[wf.LogoAgent](spec, "logo_service", logoAgentCore, model.Key, model.URL)
	coordinatorService := workflow.Service[wf.MarketingCoordinator](
		spec,
		"coordinator_service",
		coordinatorCore,
		domainService,
		websiteService,
		marketingService,
		logoService,
	)

	http.Deploy(spec, coordinatorService)
	goproc.Deploy(spec, coordinatorService)

	ctr := linuxcontainer.Deploy(spec, coordinatorService)
	return []string{ctr}, nil
}

func readModelInfo() (ModelInfo, error) {
	var minfo ModelInfo
	file, err := os.Open(*modelFile)
	if err != nil {
		return ModelInfo{}, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return ModelInfo{}, err
	}

	if err := json.Unmarshal(b, &minfo); err != nil {
		return ModelInfo{}, err
	}

	return minfo, nil
}
