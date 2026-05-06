package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/vaastav/dmas_forge/ai_plugins/a2a"
)

var A2A = cmdbuilder.SpecOption{
	Name:        "a2a",
	Description: "Deploy sub-agents over A2A while exposing the coordinator over HTTP",
	Build:       makeA2ASpec,
}

func makeA2ASpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineFinancialServices(spec)
	if err != nil {
		return []string{}, err
	}

	deployService := func(serviceName string, exposeHTTP bool) string {
		if exposeHTTP {
			http.Deploy(spec, serviceName)
		} else {
			a2a.Deploy(spec, serviceName)
		}
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	return []string{
		deployService(services.collectorService, false),
		deployService(services.evaluatorService, false),
		deployService(services.researchService, false),
		deployService(services.analystService, false),
		deployService(services.writerService, false),
		deployService(services.coordinatorService, true),
	}, nil
}
