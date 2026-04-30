package benchmark

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vaastav/agentic_blueprint/benchmark/runner/execute"
)

func commandRun(args []string, smoke bool) error {
	commandName := "run"
	if smoke {
		commandName = "smoke"
	}
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("config", "config.json", "config file")
	exampleFilter := fs.String("examples", "", "comma-separated example names")
	specFilter := fs.String("specs", "", "comma-separated spec names")
	profileFilter := fs.String("profiles", "", "comma-separated profile names")
	runID := fs.String("run-id", time.Now().Format("20060102-150405"), "result run id")
	usage := fmt.Sprintf("Usage: go run benchmark/main.go %s [-examples weather] [-specs single,http] [-profiles sequential] [-run-id local]", commandName)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), usage)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, benchDir, err := repoPaths()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(filepath.Join(benchDir, *configPath))
	if err != nil {
		return err
	}
	modelInfo := map[string]any{}
	modelBytes, err := os.ReadFile(filepath.Join(benchDir, "model.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(modelBytes, &modelInfo); err != nil {
		return err
	}
	delete(modelInfo, "key")
	examples := splitCSV(*exampleFilter)
	specs := splitCSV(*specFilter)
	profiles := splitCSV(*profileFilter)
	cases := selectCases(cfg, examples, specs, profiles)
	if len(cases) == 0 {
		return fmt.Errorf("no benchmark cases matched")
	}
	if smoke {
		for i := range cases {
			cases[i].Profile.Mode = "requests"
			cases[i].Profile.Value = 1
			cases[i].Profile.Concurrency = 1
		}
	}

	return execute.Run(execute.RunOptions{
		RepoRoot: repoRoot,
		BenchDir: benchDir,
		RunID:    *runID,
		Mock:     cfg.Mock,
		Cases:    cases,
		Stdout:   os.Stdout,
		RunInfo: map[string]any{
			"run_id":   *runID,
			"started":  time.Now().Format(time.RFC3339),
			"mock":     cfg.Mock,
			"examples": examples,
			"specs":    specs,
			"profiles": profiles,
			"config":   cfg,
			"model":    modelInfo,
		},
	})
}
