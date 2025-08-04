package specs

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/weather/workflow"
)

type ModelInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Key  string `json:"key"`
}

var Docker = cmdbuilder.SpecOption{
	Name:        "docker",
	Description: "Deploys each agent in a separate container with http connecting the agents, uses OpenAI for providing the agents",
	Build:       makeDockerSpec,
}

var model_file = flag.String("outfile", "model.json", "Specific model related information")

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

	all_bytes, err := ioutil.ReadAll(file)
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

	dagent := openai_plugin.OpenAILLMAgent(spec, "dagent", model_url, model_key, model_name)
	disaster_agent := workflow.Service[wf.DisasterAgent](spec, "dagent_service", dagent)
	disaster_ctr := applyDockerDefaults(spec, disaster_agent)

	wagent := openai_plugin.OpenAILLMAgent(spec, "wagent", model_url, model_key, model_name)
	weather_agent := workflow.Service[wf.WeatherAgent](spec, "wagent_service", wagent, disaster_agent)
	weather_ctr := applyDockerDefaults(spec, weather_agent)

	return []string{disaster_ctr, weather_ctr}, nil
}
