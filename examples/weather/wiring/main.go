package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/agentic_blueprint/examples/weather/wiring/specs"
)

func main() {

	name := "weather"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Docker,
	)
}
