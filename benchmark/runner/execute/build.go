package execute

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type BuildOptions struct {
	RepoRoot  string
	BenchDir  string
	Cases     []CasePlan
	Rebuild   bool
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
		buildDir := filepath.Join(opts.BenchDir, "cached_builds", c.Example.Name, c.Spec)
		fmt.Fprintf(opts.LogWriter, "building %s %s -> %s\n", c.Example.Name, c.Spec, buildDir)
		if err := buildDeployment(opts.RepoRoot, modelFile, c.Example, c.Spec, buildDir, opts.LogWriter, opts.Rebuild); err != nil {
			return err
		}
	}
	return nil
}

func buildDeployment(repoRoot, modelFile string, ex ExampleConfig, spec string, outDir string, logWriter io.Writer, rebuild bool) error {
	if _, err := os.Stat(filepath.Join(outDir, "docker", "docker-compose.yml")); err == nil && !rebuild {
		fmt.Fprintf(logWriter, "using cached build %s\n", outDir)
		return pinGeneratedOTelDeps(outDir, logWriter)
	}
	if rebuild {
		if err := os.RemoveAll(outDir); err != nil {
			return err
		}
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
	return pinGeneratedOTelDeps(outDir, logWriter)
}

func pinGeneratedOTelDeps(outDir string, logWriter io.Writer) error {
	var modFiles []string
	err := filepath.WalkDir(filepath.Join(outDir, "docker"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(string(b), "module blueprint/goproc/") {
			modFiles = append(modFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, modFile := range modFiles {
		modDir := filepath.Dir(modFile)
		fmt.Fprintf(logWriter, "pinning otel dependencies in %s\n", modFile)
		if err := runCommand([]string{
			"go", "mod", "edit",
			"-require=go.opentelemetry.io/otel@v1.26.0",
			"-require=go.opentelemetry.io/otel/metric@v1.26.0",
			"-require=go.opentelemetry.io/otel/trace@v1.26.0",
			"-require=go.opentelemetry.io/otel/sdk@v1.26.0",
			"-require=go.opentelemetry.io/otel/sdk/metric@v1.26.0",
			"-require=go.opentelemetry.io/otel/exporters/stdout/stdoutmetric@v1.26.0",
			"-require=go.opentelemetry.io/otel/exporters/stdout/stdouttrace@v1.26.0",
		}, modDir, logWriter); err != nil {
			return err
		}
		if err := runCommand([]string{"go", "mod", "tidy"}, modDir, logWriter); err != nil {
			return err
		}
	}
	return nil
}
