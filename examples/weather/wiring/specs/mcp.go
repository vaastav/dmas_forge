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
	"github.com/vaastav/agentic_blueprint/ai_plugins/mcp"
	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/weather/workflow"
)

var MCP = cmdbuilder.SpecOption{
	Name:        "mcp",
	Description: "Deploys each agent in a separate container with mcp connecting the agents, uses OpenAI for providing the agents. The frontend weather agent is deployed with http.",
	Build:       makeMCPSpec,
}

func makeMCPSpec(spec wiring.WiringSpec) ([]string, error) {

	applyDockerDefaults := func(spec wiring.WiringSpec, serviceName string, isweather bool) string {
		if !isweather {
			mcp.Deploy(spec, serviceName)
		} else {
			http.Deploy(spec, serviceName)
		}
		proc := goproc.Deploy(spec, serviceName)
		opentelemetry.Logger(spec, proc)
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

	wagent := openai_plugin.OpenAILLMAgent(spec, "wagent", model_url, model_key, model_name, openai_plugin.AgentConfig{})
	weather_agent := workflow.Service[wf.WeatherAgent](spec, "wagent_service", wagent, disaster_agent)

	collector := jaeger.Collector(spec, "jaeger")
	opentelemetry.Instrument(spec, disaster_agent, collector)
	opentelemetry.Instrument(spec, weather_agent, collector)

	disaster_ctr := applyDockerDefaults(spec, disaster_agent, false)
	weather_ctr := applyDockerDefaults(spec, weather_agent, true)

	return []string{disaster_ctr, weather_ctr}, nil
}
