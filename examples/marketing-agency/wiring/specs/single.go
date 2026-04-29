package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/opentelemetry"
)

var Single = cmdbuilder.SpecOption{
	Name:        "single",
	Description: "Deploy the full marketing workflow in a single container with an HTTP endpoint",
	Build:       makeSingleSpec,
}

func makeSingleSpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineMarketingServices(spec)
	if err != nil {
		return []string{}, err
	}
	instrumentMarketingServices(spec, services)

	http.Deploy(spec, services.coordinatorService)
	proc := goproc.Deploy(spec, services.coordinatorService)
	opentelemetry.Logger(spec, proc)
	ctr := linuxcontainer.Deploy(spec, services.coordinatorService)

	return []string{ctr}, nil
}
