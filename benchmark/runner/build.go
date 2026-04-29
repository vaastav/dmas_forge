package benchmark

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vaastav/agentic_blueprint/benchmark/runner/execute"
)

func commandBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("config", "config.json", "config file")
	exampleFilter := fs.String("examples", "", "comma-separated example names")
	specFilter := fs.String("specs", "", "comma-separated spec names")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run benchmark/main.go build [-examples weather,chat] [-specs single,memory]")
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
	examples := splitCSV(*exampleFilter)
	specs := splitCSV(*specFilter)
	cases := selectCases(cfg, examples, specs, nil)
	if len(cases) == 0 {
		return fmt.Errorf("no examples/specs matched")
	}
	return execute.Build(execute.BuildOptions{
		RepoRoot:  repoRoot,
		BenchDir:  benchDir,
		Cases:     cases,
		LogWriter: os.Stdout,
	})
}
