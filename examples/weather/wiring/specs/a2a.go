package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"
	"github.com/vaastav/agentic_blueprint/ai_plugins/a2a"
	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/weather/workflow"
)

var A2A = cmdbuilder.SpecOption{
	Name:        "a2a",
	Description: "Deploys each agent in a separate container with a2a connecting the agents, uses OpenAI for providing the agents",
	Build:       makeA2ASpec,
}

func makeA2ASpec(spec wiring.WiringSpec) ([]string, error) {

	applyDockerDefaults := func(spec wiring.WiringSpec, serviceName string) string {
		a2a.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	minfo, err := model.GetModelInfo()
	if err != nil {
		return []string{}, err
	}

	model_url := minfo.URL
	model_key := minfo.Key
	model_name := minfo.Name

	dagent := openai_plugin.OpenAILLMAgent(spec, "dagent", model_url, model_key, model_name, openai_plugin.AgentConfig{})
	disaster_agent := workflow.Service[wf.DisasterAgent](spec, "dagent_service", dagent)
	disaster_ctr := applyDockerDefaults(spec, disaster_agent)

	wagent := openai_plugin.OpenAILLMAgent(spec, "wagent", model_url, model_key, model_name, openai_plugin.AgentConfig{})
	weather_agent := workflow.Service[wf.WeatherAgent](spec, "wagent_service", wagent, disaster_agent)
	weather_ctr := applyDockerDefaults(spec, weather_agent)

	return []string{disaster_ctr, weather_ctr}, nil
}
