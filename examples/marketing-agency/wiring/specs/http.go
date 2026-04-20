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
	Description: "Deploy each marketing agent in a separate container with HTTP between services",
	Build:       makeHTTPSpec,
}

func makeHTTPSpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineMarketingServices(spec)
	if err != nil {
		return []string{}, err
	}

	deployService := func(serviceName string) string {
		http.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	domainCtr := deployService(services.domainService)
	websiteCtr := deployService(services.websiteService)
	marketingCtr := deployService(services.marketingService)
	logoCtr := deployService(services.logoService)
	coordinatorCtr := deployService(services.coordinatorService)

	return []string{domainCtr, websiteCtr, marketingCtr, logoCtr, coordinatorCtr}, nil
}
