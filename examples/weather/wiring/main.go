package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/agentic_blueprint/examples/weather/wiring/specs"
)

func main() {

	name := "weather"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Single,
		specs.Docker,
		specs.A2A,
		specs.MCP,
	)
}
