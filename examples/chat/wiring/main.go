package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/dmas_forge/examples/chat/wiring/specs"
)

func main() {
	name := "chat"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Memory,
		specs.NoMemory,
	)
}
