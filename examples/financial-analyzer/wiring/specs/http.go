package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
)

var HTTP = cmdbuilder.SpecOption{
	Name:        "http",
	Description: "Deploy each agent in a separate container with HTTP connecting the services",
	Build:       makeHTTPSpec,
}

func makeHTTPSpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineFinancialServices(spec)
	if err != nil {
		return []string{}, err
	}

	deployService := func(serviceName string) string {
		http.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	return []string{
		deployService(services.collectorService),
		deployService(services.evaluatorService),
		deployService(services.researchService),
		deployService(services.analystService),
		deployService(services.writerService),
		deployService(services.coordinatorService),
	}, nil
}
