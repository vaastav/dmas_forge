package execute

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type BuildOptions struct {
	RepoRoot  string
	BenchDir  string
	Cases     []CasePlan
	LogWriter io.Writer
}

func Build(opts BuildOptions) error {
	modelFile := filepath.Join(opts.BenchDir, "model.json")
	seen := map[string]bool{}
	for _, c := range opts.Cases {
		key := c.Example.Name + "/" + c.Spec
		if seen[key] {
			continue
		}
		seen[key] = true
		outDir := generatedBuildDir(opts.BenchDir, c)
		fmt.Fprintf(opts.LogWriter, "generating %s %s -> %s\n", c.Example.Name, c.Spec, outDir)
		if err := generateDeployment(opts.RepoRoot, modelFile, c.Example, c.Spec, outDir, opts.LogWriter); err != nil {
			return err
		}
	}
	return nil
}

func generatedBuildDir(benchDir string, c CasePlan) string {
	return filepath.Join(benchDir, ".builds", c.Example.Name, c.Spec)
}

func generateDeployment(repoRoot, modelFile string, ex ExampleConfig, spec string, outDir string, logWriter io.Writer) error {
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outDir), 0o755); err != nil {
		return err
	}
	wiringDir := filepath.Join(repoRoot, "examples", ex.Name, "wiring")
	args := []string{"run", "main.go", "-w", spec, "-o", outDir, "-modfile=" + modelFile}
	args = append(args, ex.BuildArgs...)
	if err := runCommand(append([]string{"go"}, args...), wiringDir, logWriter); err != nil {
		return err
	}
	patchScript := filepath.Join(repoRoot, "benchmark", "patch-generated-otel-deps.sh")
	return runCommand([]string{"bash", patchScript, outDir}, repoRoot, logWriter)
}
