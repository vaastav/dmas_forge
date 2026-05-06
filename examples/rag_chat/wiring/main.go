package main

import (
	"github.com/blueprint-uservices/blueprint/plugins/cmdbuilder"
	"github.com/vaastav/dmas_forge/examples/rag_chat/wiring/specs"
)

func main() {
	name := "rag_chat"
	cmdbuilder.MakeAndExecute(
		name,
		specs.Automatic,
		specs.Agentic,
	)
}
