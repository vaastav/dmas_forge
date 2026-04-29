package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/blueprint-uservices/blueprint/plugins/goproc"
	"github.com/blueprint-uservices/blueprint/plugins/http"
	"github.com/blueprint-uservices/blueprint/plugins/linuxcontainer"
	"github.com/blueprint-uservices/blueprint/plugins/opentelemetry"
	"github.com/vaastav/agentic_blueprint/ai_plugins/mcp"
)

var MCP = cmdbuilder.SpecOption{
	Name:        "mcp",
	Description: "Deploy sub-agents over MCP while exposing the coordinator over HTTP",
	Build:       makeMCPSpec,
}

func makeMCPSpec(spec wiring.WiringSpec) ([]string, error) {
	services, err := defineFinancialServices(spec)
	if err != nil {
		return []string{}, err
	}
	instrumentFinancialServices(spec, services)

	deployService := func(serviceName string, exposeHTTP bool) string {
		if exposeHTTP {
			http.Deploy(spec, serviceName)
		} else {
			mcp.Deploy(spec, serviceName)
		}
		proc := goproc.Deploy(spec, serviceName)
		opentelemetry.Logger(spec, proc)
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
