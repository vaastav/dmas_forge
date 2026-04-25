package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/vaastav/agentic_blueprint/ai_plugins/mcp"
)

var MCP = cmdbuilder.SpecOption{
	Name:        "mcp",
	Description: "Deploys each marketing agent in a separate container with MCP connecting the agents, uses OpenAI for providing the agents. The coordinator agent is deployed with HTTP.",
	Build:       makeMCPSpec,
}

func makeMCPSpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineMarketingServices(spec)
	if err != nil {
		return []string{}, err
	}

	deployWithMCP := func(serviceName string) string {
		mcp.Deploy(spec, serviceName)
		goproc.Deploy(spec, serviceName)
		return linuxcontainer.Deploy(spec, serviceName)
	}

	domainCtr := deployWithMCP(services.domainService)
	websiteCtr := deployWithMCP(services.websiteService)
	marketingCtr := deployWithMCP(services.marketingService)
	logoCtr := deployWithMCP(services.logoService)

	http.Deploy(spec, services.coordinatorService)
	goproc.Deploy(spec, services.coordinatorService)
	coordinatorCtr := linuxcontainer.Deploy(spec, services.coordinatorService)

	return []string{domainCtr, websiteCtr, marketingCtr, logoCtr, coordinatorCtr}, nil
}
