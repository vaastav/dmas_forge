package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/dmas_forge/examples/financial-analyzer/wiring/specs"
)

func main() {
	name := "financial-analyzer"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Single,
		specs.HTTP,
		specs.MCP,
		specs.A2A,
	)
}
