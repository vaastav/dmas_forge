package main

import (
	"fmt"
	"os"

	benchmark "github.com/vaastav/agentic_blueprint/benchmark/runner"
)

func main() {
	if err := benchmark.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
