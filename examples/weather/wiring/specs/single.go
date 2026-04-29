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

	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/weather/workflow"
)

var Single = cmdbuilder.SpecOption{
	Name:        "single",
	Description: "Deploys the full weather workflow in one container with an HTTP endpoint",
	Build:       makeSingleSpec,
}

func makeSingleSpec(spec wiring.WiringSpec) ([]string, error) {
	minfo, err := model.GetModelInfo()
	if err != nil {
		return []string{}, err
	}

	dagent := openai_plugin.OpenAILLMAgent(spec, "dagent", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	disasterAgent := workflow.Service[wf.DisasterAgent](spec, "dagent_service", dagent)

	wagent := openai_plugin.OpenAILLMAgent(spec, "wagent", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	weatherAgent := workflow.Service[wf.WeatherAgent](spec, "wagent_service", wagent, disasterAgent)

	collector := jaeger.Collector(spec, "jaeger")
	opentelemetry.Instrument(spec, disasterAgent, collector)
	opentelemetry.Instrument(spec, weatherAgent, collector)

	http.Deploy(spec, weatherAgent)
	proc := goproc.Deploy(spec, weatherAgent)
	opentelemetry.Logger(spec, proc)
	ctr := linuxcontainer.Deploy(spec, weatherAgent)

	return []string{ctr}, nil
}
