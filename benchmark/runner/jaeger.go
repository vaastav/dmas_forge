package benchmark

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vaastav/agentic_blueprint/benchmark/runner/execute"
)

func commandJaeger(args []string) error {
	fs := flag.NewFlagSet("jaeger", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	runID := fs.String("run", "", "run id under results")
	caseName := fs.String("case", "", "case directory name")
	port := fs.Int("port", 16686, "host port for Jaeger UI")
	resultsDir := fs.String("results", "results", "results directory")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run ./benchmark jaeger -run run-id -case weather-single-sequential [-port 16686]")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *caseName == "" {
		return fmt.Errorf("-case is required")
	}
	_, benchDir, err := repoPaths()
	if err != nil {
		return err
	}
	root := filepath.Join(benchDir, *resultsDir)
	if *runID == "" {
		*runID, err = latestRun(root)
		if err != nil {
			return err
		}
	}
	jaegerDir := filepath.Join(root, *runID, *caseName, "jaeger")
	if _, err := os.Stat(jaegerDir); err != nil {
		return err
	}
	fmt.Printf("Jaeger UI: http://localhost:%d\n", *port)
	return execute.StartJaegerUI(jaegerDir, *port, os.Stdout, os.Stderr)
}
