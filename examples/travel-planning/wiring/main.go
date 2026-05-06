package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/dmas_forge/examples/travel-planning/wiring/specs"
)

func main() {
	name := "travel-planning"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Docker,
	)
}
