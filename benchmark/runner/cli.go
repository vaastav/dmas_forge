package benchmark

import (
	"fmt"
	"io"
	"os"
)

func Run(args []string) error {
	if len(args) < 2 || args[1] == "-h" || args[1] == "--help" {
		printUsage(os.Stdout)
		return nil
	}

	switch args[1] {
	case "list":
		return commandList(args[2:])
	case "build":
		return commandBuild(args[2:])
	case "run":
		return commandRun(args[2:])
	case "summary":
		return commandSummary(args[2:])
	case "jaeger":
		return commandJaeger(args[2:])
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[1])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `DMAS Forge benchmark runner

Example Usage:
  go run main.go -h
  go run main.go list
  go run main.go build -examples weather,chat -specs single,memory
  go run main.go run -examples weather -specs single,http -profiles smoke
  go run main.go summary -run 20260429-120000
  go run main.go jaeger -run 20260429-120000 -case weather-single-sequential

Commands:
  list      Print configured examples, specs, query files, and profiles.
  build     Generate deployment output under .builds/<example>/<spec>.
  run       Generate builds, start Docker Compose, run fixed-count load, save results.
  summary   Print a compact table from saved summary.json files.
  jaeger    Start Jaeger UI over one saved case's jaeger/ directory.

Common flags:
  -config     Config file. Default: config.json
  -examples   Comma-separated example names.
  -specs      Comma-separated spec names.
  -profiles   Comma-separated profile names.
`)
}
