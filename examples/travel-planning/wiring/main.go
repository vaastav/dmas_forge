package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/agentic_blueprint/examples/travel-planning/wiring/specs"
)

func main() {
	name := "travel-planning"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Single,
		specs.HTTP,
		specs.MCP,
		specs.A2A,
	)
}
