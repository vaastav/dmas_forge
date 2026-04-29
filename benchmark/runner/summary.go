package benchmark

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vaastav/agentic_blueprint/benchmark/runner/execute"
)

func commandSummary(args []string) error {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	runID := fs.String("run", "", "run id under results")
	resultsDir := fs.String("results", "results", "results directory")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run ./benchmark summary [-run run-id]")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
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
	summaries, err := execute.LoadSummaries(filepath.Join(root, *runID))
	if err != nil {
		return err
	}
	execute.PrintSummaryTable(summaries)
	return nil
}
