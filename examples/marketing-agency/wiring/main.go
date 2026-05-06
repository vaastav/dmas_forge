package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/dmas_forge/examples/marketing-agency/wiring/specs"
)

func main() {
	name := "marketing-agency"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Single,
		specs.HTTP,
		specs.MCP,
		specs.A2A,
	)
}
