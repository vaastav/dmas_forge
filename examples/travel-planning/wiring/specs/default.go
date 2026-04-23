package specs

import (
	"flag"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/travel-planning/workflow"
)

var Docker = cmdbuilder.SpecOption{
	Name:        "docker",
	Description: "Deploys travel-planning agents in containers with HTTP, using OpenAI models",
	Build:       makeDockerSpec,
}

var modelFile = flag.String("modfile", "model.json", "Specific model related information")

func makeDockerSpec(spec wiring.WiringSpec) ([]string, error) {
	modelInfo, err := model.GetModelInfo()
	if err != nil {
		return []string{}, err
	}

	applyDockerDefaults := func(serviceName string) string {
		http.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	plannerLLM := openai_plugin.OpenAILLMAgent(spec, "planner_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	plannerService := workflow.Service[wf.TravelPlannerAgent](spec, "planner_service", plannerLLM)
	plannerContainer := applyDockerDefaults(plannerService)

	localLLM := openai_plugin.OpenAILLMAgent(spec, "local_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	localService := workflow.Service[wf.LocalAgent](spec, "local_service", localLLM)
	localContainer := applyDockerDefaults(localService)

	languageLLM := openai_plugin.OpenAILLMAgent(spec, "language_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	languageService := workflow.Service[wf.LanguageAgent](spec, "language_service", languageLLM)
	languageContainer := applyDockerDefaults(languageService)

	summaryLLM := openai_plugin.OpenAILLMAgent(spec, "summary_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	summaryService := workflow.Service[wf.TravelSummaryAgent](spec, "summary_service", summaryLLM)
	summaryContainer := applyDockerDefaults(summaryService)

	coordinatorLLM := openai_plugin.OpenAILLMAgent(spec, "coordinator_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	coordinatorService := workflow.Service[wf.TravelCoordinator](spec, "coordinator_service", plannerService, localService, languageService, summaryService, coordinatorLLM)
	coordinatorContainer := applyDockerDefaults(coordinatorService)

	return []string{plannerContainer, localContainer, languageContainer, summaryContainer, coordinatorContainer}, nil
}
