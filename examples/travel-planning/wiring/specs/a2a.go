package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/jaeger"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/opentelemetry"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/a2a"
	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/travel-planning/workflow"
)

var A2A = cmdbuilder.SpecOption{
	Name:        "a2a",
	Description: "Deploys travel-planning agents in containers with A2A between agents and HTTP on the coordinator",
	Build:       makeA2ASpec,
}

func makeA2ASpec(spec wiring.WiringSpec) ([]string, error) {
	modelInfo, err := model.GetModelInfo()
	if err != nil {
		return []string{}, err
	}

	applyDockerDefaults := func(serviceName string, exposeHTTP bool) string {
		if exposeHTTP {
			http.Deploy(spec, serviceName)
		} else {
			a2a.Deploy(spec, serviceName)
		}
		proc := goproc.Deploy(spec, serviceName)
		opentelemetry.Logger(spec, proc)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	plannerLLM := openai_plugin.OpenAILLMAgent(spec, "planner_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	plannerService := workflow.Service[wf.TravelPlannerAgent](spec, "planner_service", plannerLLM)
	localLLM := openai_plugin.OpenAILLMAgent(spec, "local_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	localService := workflow.Service[wf.LocalAgent](spec, "local_service", localLLM)
	languageLLM := openai_plugin.OpenAILLMAgent(spec, "language_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	languageService := workflow.Service[wf.LanguageAgent](spec, "language_service", languageLLM)
	summaryLLM := openai_plugin.OpenAILLMAgent(spec, "summary_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	summaryService := workflow.Service[wf.TravelSummaryAgent](spec, "summary_service", summaryLLM)
	coordinatorLLM := openai_plugin.OpenAILLMAgent(spec, "coordinator_llm", modelInfo.URL, modelInfo.Key, modelInfo.Name, openai_plugin.AgentConfig{})
	coordinatorService := workflow.Service[wf.TravelCoordinator](spec, "coordinator_service", plannerService, localService, languageService, summaryService, coordinatorLLM)

	collector := jaeger.Collector(spec, "jaeger")
	for _, service := range []string{plannerService, localService, languageService, summaryService, coordinatorService} {
		opentelemetry.Instrument(spec, service, collector)
	}

	plannerContainer := applyDockerDefaults(plannerService, false)
	localContainer := applyDockerDefaults(localService, false)
	languageContainer := applyDockerDefaults(languageService, false)
	summaryContainer := applyDockerDefaults(summaryService, false)
	coordinatorContainer := applyDockerDefaults(coordinatorService, true)

	return []string{plannerContainer, localContainer, languageContainer, summaryContainer, coordinatorContainer}, nil
}
