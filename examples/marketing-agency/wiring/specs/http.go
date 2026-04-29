package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/opentelemetry"
)

var HTTP = cmdbuilder.SpecOption{
	Name:        "http",
	Description: "Deploy each marketing agent in a separate container with HTTP between services",
	Build:       makeHTTPSpec,
}

func makeHTTPSpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineMarketingServices(spec)
	if err != nil {
		return []string{}, err
	}
	instrumentMarketingServices(spec, services)

	deployService := func(serviceName string) string {
		http.Deploy(spec, serviceName)
		proc := goproc.Deploy(spec, serviceName)
		opentelemetry.Logger(spec, proc)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	domainCtr := deployService(services.domainService)
	websiteCtr := deployService(services.websiteService)
	marketingCtr := deployService(services.marketingService)
	logoCtr := deployService(services.logoService)
	coordinatorCtr := deployService(services.coordinatorService)

	return []string{domainCtr, websiteCtr, marketingCtr, logoCtr, coordinatorCtr}, nil
}
