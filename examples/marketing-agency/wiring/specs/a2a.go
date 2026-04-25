package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/vaastav/agentic_blueprint/ai_plugins/a2a"
)

var A2A = cmdbuilder.SpecOption{
	Name:        "a2a",
	Description: "Deploys each marketing agent in a separate container with A2A connecting the agents, uses OpenAI for providing the agents. The coordinator agent is deployed with HTTP.",
	Build:       makeA2ASpec,
}

func makeA2ASpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineMarketingServices(spec)
	if err != nil {
		return []string{}, err
	}

	deployWithA2A := func(serviceName string) string {
		a2a.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	domainCtr := deployWithA2A(services.domainService)
	websiteCtr := deployWithA2A(services.websiteService)
	marketingCtr := deployWithA2A(services.marketingService)
	logoCtr := deployWithA2A(services.logoService)

	http.Deploy(spec, services.coordinatorService)
	goproc.Deploy(spec, services.coordinatorService)
	coordinatorCtr := linuxcontainer.Deploy(spec, services.coordinatorService)

	return []string{domainCtr, websiteCtr, marketingCtr, logoCtr, coordinatorCtr}, nil
}
