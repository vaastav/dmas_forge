package main

import (
	"crypto/sha1"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const jaegerImage = "jaegertracing/all-in-one:1.75.0"

type Config struct {
	Mock     bool            `json:"mock"`
	Profiles []Profile       `json:"profiles"`
	Examples []ExampleConfig `json:"examples"`
}

type Profile struct {
	Name           string `json:"name"`
	Requests       int    `json:"requests"`
	Concurrency    int    `json:"concurrency"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ExampleConfig struct {
	Name          string    `json:"name"`
	Specs         []string  `json:"specs"`
	Route         string    `json:"route"`
	Request       string    `json:"request"`
	QueryFile     string    `json:"query_file"`
	EntrypointEnv string    `json:"entrypoint_env"`
	Params        []string  `json:"params"`
	Profiles      []Profile `json:"profiles"`
	BuildArgs     []string  `json:"build_args"`
}

type QueryRow map[string]string

type RequestResult struct {
	Example       string  `json:"example"`
	Spec          string  `json:"spec"`
	Profile       string  `json:"profile"`
	Sequence      int     `json:"sequence"`
	QueryID       string  `json:"query_id"`
	Status        int     `json:"status"`
	OK            bool    `json:"ok"`
	LatencyMS     float64 `json:"latency_ms"`
	ResponseBytes int     `json:"response_bytes"`
	Error         string  `json:"error"`
	ResponseText  string  `json:"response_text,omitempty"`
	URL           string  `json:"url"`
}

type ComponentSummary struct {
	Name         string  `json:"name"`
	Spans        int     `json:"spans"`
	DurationMS   float64 `json:"duration_ms"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
}

type ResourceSample struct {
	Timestamp     time.Time `json:"timestamp"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryBytes   int64     `json:"memory_bytes"`
	MemoryPercent float64   `json:"memory_percent"`
}

type CaseSummary struct {
	Example        string             `json:"example"`
	Spec           string             `json:"spec"`
	Profile        string             `json:"profile"`
	Requests       int                `json:"requests"`
	Successes      int                `json:"successes"`
	Errors         int                `json:"errors"`
	ElapsedMS      float64            `json:"elapsed_ms"`
	ThroughputRPS  float64            `json:"throughput_rps"`
	P50MS          float64            `json:"p50_ms"`
	P95MS          float64            `json:"p95_ms"`
	P99MS          float64            `json:"p99_ms"`
	InputTokens    int64              `json:"input_tokens"`
	OutputTokens   int64              `json:"output_tokens"`
	TotalTokens    int64              `json:"total_tokens"`
	TraceError     string             `json:"trace_error,omitempty"`
	ResourceError  string             `json:"resource_error,omitempty"`
	CPUAvgPercent  float64            `json:"cpu_avg_percent"`
	CPUMaxPercent  float64            `json:"cpu_max_percent"`
	MemoryAvgBytes int64              `json:"memory_avg_bytes"`
	MemoryMaxBytes int64              `json:"memory_max_bytes"`
	Components     []ComponentSummary `json:"components"`
}

type CasePlan struct {
	Example ExampleConfig
	Spec    string
	Profile Profile
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
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
	fmt.Fprintln(w, `DMAS Forge benchmark runner

Run this from the benchmark directory:
  go run main.go -h
  go run main.go list
  go run main.go build -examples weather,chat -specs single,memory -rebuild
  go run main.go run -examples weather -specs single,http
  go run main.go summary -run 20260429-120000
  go run main.go jaeger -run 20260429-120000 -case weather-single-sequential

Commands:
  list      Print configured examples, specs, routes, and profiles.
  build     Generate cached builds under cached_builds/<example>/<spec>.
  run       Build/reuse, start Docker Compose, run fixed-count load, save results.
  summary   Print a compact table from saved summary.json files.
  jaeger    Start Jaeger UI over one saved case's jaeger/ directory.

Common flags:
  -config     Config file. Default: config.json
  -examples   Comma-separated example names.
  -specs      Comma-separated spec names.
  -profiles   Comma-separated profile names.
  -rebuild    Regenerate cached builds.
`)
}

func commandList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("config", "config.json", "config file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run main.go list [-config config.json]")
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
	cfg, err := loadConfig(filepath.Join(benchDir, *configPath))
	if err != nil {
		return err
	}
	for _, ex := range cfg.Examples {
		fmt.Printf("%-20s specs=%-28s route=%s entry=%s query=%s\n", ex.Name, strings.Join(ex.Specs, ","), ex.Route, ex.EntrypointEnv, ex.QueryFile)
	}
	fmt.Println()
	for _, profile := range cfg.Profiles {
		fmt.Printf("profile %-12s requests=%d concurrency=%d timeout=%ds\n", profile.Name, profile.Requests, profile.Concurrency, profile.TimeoutSeconds)
	}
	return nil
}

func commandBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("config", "config.json", "config file")
	exampleFilter := fs.String("examples", "", "comma-separated example names")
	specFilter := fs.String("specs", "", "comma-separated spec names")
	rebuild := fs.Bool("rebuild", false, "regenerate cached builds")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run main.go build [-examples weather,chat] [-specs single,memory] [-rebuild]")
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
	cases := selectCases(cfg, splitCSV(*exampleFilter), splitCSV(*specFilter), nil)
	if len(cases) == 0 {
		return fmt.Errorf("no examples/specs matched")
	}
	modelFile := filepath.Join(benchDir, "model.json")
	seen := map[string]bool{}
	for _, c := range cases {
		key := c.Example.Name + "/" + c.Spec
		if seen[key] {
			continue
		}
		seen[key] = true
		buildDir := filepath.Join(benchDir, "cached_builds", c.Example.Name, c.Spec)
		fmt.Printf("building %s %s -> %s\n", c.Example.Name, c.Spec, buildDir)
		if err := buildDeployment(repoRoot, modelFile, c.Example, c.Spec, buildDir, os.Stdout, *rebuild); err != nil {
			return err
		}
	}
	return nil
}

func commandRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("config", "config.json", "config file")
	exampleFilter := fs.String("examples", "", "comma-separated example names")
	specFilter := fs.String("specs", "", "comma-separated spec names")
	profileFilter := fs.String("profiles", "", "comma-separated profile names")
	runID := fs.String("run-id", time.Now().Format("20060102-150405"), "result run id")
	rebuild := fs.Bool("rebuild", false, "regenerate cached builds")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run main.go run [-examples weather] [-specs single,http] [-profiles sequential] [-run-id local] [-rebuild]")
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
	cases := selectCases(cfg, splitCSV(*exampleFilter), splitCSV(*specFilter), splitCSV(*profileFilter))
	if len(cases) == 0 {
		return fmt.Errorf("no benchmark cases matched")
	}

	resultsRoot := filepath.Join(benchDir, "results", *runID)
	if err := os.MkdirAll(resultsRoot, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(resultsRoot, "run.json"), map[string]any{
		"run_id":   *runID,
		"started":  time.Now().Format(time.RFC3339),
		"mock":     cfg.Mock,
		"examples": splitCSV(*exampleFilter),
		"specs":    splitCSV(*specFilter),
		"profiles": splitCSV(*profileFilter),
		"config":   cfg,
	}); err != nil {
		return err
	}

	modelFile := filepath.Join(benchDir, "model.json")
	for i, c := range cases {
		caseName := cleanName(c.Example.Name + "-" + c.Spec + "-" + c.Profile.Name)
		caseDir := filepath.Join(resultsRoot, caseName)
		buildDir := filepath.Join(benchDir, "cached_builds", c.Example.Name, c.Spec)
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			return err
		}

		logFile, err := os.Create(filepath.Join(caseDir, "build.log"))
		if err != nil {
			return err
		}
		progressWriter := io.MultiWriter(os.Stdout, logFile)
		fmt.Fprintf(progressWriter, "[%d/%d] running %s/%s/%s\n", i+1, len(cases), c.Example.Name, c.Spec, c.Profile.Name)
		fmt.Fprintf(progressWriter, "docker/build logs: %s\n", filepath.Join(caseDir, "build.log"))

		err = runCase(repoRoot, benchDir, modelFile, resultsRoot, caseDir, buildDir, caseName, cfg.Mock, c, progressWriter, logFile, *rebuild)
		closeErr := logFile.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func runCase(repoRoot, benchDir, modelFile, resultsRoot, caseDir, buildDir, caseName string, mock bool, c CasePlan, progressWriter io.Writer, detailWriter io.Writer, rebuild bool) error {
	fmt.Fprintf(progressWriter, "building deployment files for %s\n", caseName)
	if err := buildDeployment(repoRoot, modelFile, c.Example, c.Spec, buildDir, detailWriter, rebuild); err != nil {
		return err
	}
	jaegerDir := filepath.Join(caseDir, "jaeger")
	if err := prepareJaegerDir(jaegerDir); err != nil {
		return err
	}
	overrideFile, err := writeComposeOverride(buildDir, jaegerDir, mock)
	if err != nil {
		return err
	}
	project := composeProjectName(filepath.Base(resultsRoot), caseName)
	stopInterruptCleanup := stopCaseOnInterrupt(buildDir, overrideFile, project, caseName, progressWriter, detailWriter)
	defer stopInterruptCleanup()

	fmt.Fprintf(progressWriter, "stopping old containers for %s\n", caseName)
	_ = composeDown(buildDir, overrideFile, project, detailWriter)
	defer func() {
		fmt.Fprintf(progressWriter, "stopping containers for %s\n", caseName)
		dumpComposeDiagnostics(buildDir, overrideFile, project, detailWriter)
		composeDown(buildDir, overrideFile, project, detailWriter)
	}()

	fmt.Fprintf(progressWriter, "building docker images for %s\n", caseName)
	if err := runCommand(composeCommand(buildDir, overrideFile, project, "build"), filepath.Join(buildDir, "docker"), detailWriter); err != nil {
		return err
	}
	fmt.Fprintf(progressWriter, "starting docker compose for %s\n", caseName)
	if err := runCommand(composeCommand(buildDir, overrideFile, project, "up", "-d"), filepath.Join(buildDir, "docker"), detailWriter); err != nil {
		return err
	}

	env := loadLocalEnv(filepath.Join(buildDir, ".local.env"))
	httpPort, err := discoverHTTPPort(env, c.Example)
	if err != nil {
		return err
	}
	jaegerPort, err := discoverJaegerPort(env)
	if err != nil {
		return err
	}
	if err := waitTCP("localhost", httpPort, 120*time.Second); err != nil {
		return err
	}
	if err := waitTCP("localhost", jaegerPort, 120*time.Second); err != nil {
		return err
	}

	rows, err := loadQueries(filepath.Join(benchDir, c.Example.QueryFile))
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("http://localhost:%d%s", httpPort, c.Example.Route)
	fmt.Fprintf(progressWriter, "testing load %s requests=%d concurrency=%d endpoint=%s\n", c.Profile.Name, c.Profile.Requests, c.Profile.Concurrency, endpoint)
	resourceContainers, resourceErr := listBenchmarkContainers(project)
	if resourceErr != nil {
		fmt.Fprintf(progressWriter, "resource sampling error: %v\n", resourceErr)
	}
	if resourceErr == nil && len(resourceContainers) == 0 {
		fmt.Fprintf(detailWriter, "resource sampling: no non-jaeger containers found for project %s\n", project)
	}
	stopResources := startResourceSampling(resourceContainers, time.Second)
	start := time.Now()
	results := runLoad(endpoint, c, rows)
	end := time.Now()
	resourceSamples, stopErr := stopResources()
	if resourceErr == nil {
		resourceErr = stopErr
	}
	if stopErr != nil {
		fmt.Fprintf(progressWriter, "resource sampling error: %v\n", stopErr)
	}
	if err := writeJSONL(filepath.Join(caseDir, "resources.jsonl"), resourceSamples); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(caseDir, "requests.jsonl"), results); err != nil {
		return err
	}

	fmt.Fprintf(progressWriter, "collecting traces for %s\n", caseName)
	time.Sleep(2 * time.Second)
	traces, traceErr := collectTraces(fmt.Sprintf("http://localhost:%d", jaegerPort), start.Add(-2*time.Second), end.Add(2*time.Second))
	if err := writeJSON(filepath.Join(caseDir, "traces.json"), map[string]any{"data": traces}); err != nil {
		return err
	}
	spans := flattenSpans(traces)
	if err := writeJSONL(filepath.Join(caseDir, "spans.jsonl"), spans); err != nil {
		return err
	}
	summary := summarizeCase(c, results, spans, resourceSamples, end.Sub(start))
	if traceErr != nil {
		summary.TraceError = traceErr.Error()
	}
	if resourceErr != nil {
		summary.ResourceError = resourceErr.Error()
	}
	if err := writeJSON(filepath.Join(caseDir, "summary.json"), summary); err != nil {
		return err
	}
	printCaseSummary(summary)
	return nil
}

func commandSummary(args []string) error {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	runID := fs.String("run", "", "run id under results")
	resultsDir := fs.String("results", "results", "results directory")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run main.go summary [-run run-id]")
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
	summaries, err := loadSummaries(filepath.Join(root, *runID))
	if err != nil {
		return err
	}
	printSummaryTable(summaries)
	return nil
}

func commandJaeger(args []string) error {
	fs := flag.NewFlagSet("jaeger", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	runID := fs.String("run", "", "run id under results")
	caseName := fs.String("case", "", "case directory name")
	port := fs.Int("port", 16686, "host port for Jaeger UI")
	resultsDir := fs.String("results", "results", "results directory")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run main.go jaeger -run run-id -case weather-single-sequential [-port 16686]")
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
	if err := prepareJaegerDir(jaegerDir); err != nil {
		return err
	}
	fmt.Printf("Jaeger UI: http://localhost:%d\n", *port)
	cmd := exec.Command(
		"docker", "run", "--rm",
		"--user", "0:0",
		"-p", fmt.Sprintf("%d:16686", *port),
		"-e", "SPAN_STORAGE_TYPE=badger",
		"-e", "BADGER_EPHEMERAL=false",
		"-e", "BADGER_DIRECTORY_VALUE=/badger/data",
		"-e", "BADGER_DIRECTORY_KEY=/badger/key",
		"-v", jaegerDir+":/badger",
		jaegerImage,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func repoPaths() (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	if filepath.Base(wd) == "benchmark" {
		return filepath.Dir(wd), wd, nil
	}
	return wd, filepath.Join(wd, "benchmark"), nil
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(b, &cfg)
	return cfg, err
}

func selectCases(cfg Config, examples, specs, profiles map[string]bool) []CasePlan {
	var cases []CasePlan
	for _, ex := range cfg.Examples {
		if len(examples) > 0 && !examples[ex.Name] {
			continue
		}
		exProfiles := cfg.Profiles
		if len(ex.Profiles) > 0 {
			exProfiles = ex.Profiles
		}
		for _, spec := range ex.Specs {
			if len(specs) > 0 && !specs[spec] {
				continue
			}
			for _, profile := range exProfiles {
				if len(profiles) > 0 && !profiles[profile.Name] {
					continue
				}
				cases = append(cases, CasePlan{Example: ex, Spec: spec, Profile: profile})
			}
		}
	}
	return cases
}

func splitCSV(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
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

func writeComposeOverride(buildDir, jaegerDir string, mock bool) (string, error) {
	composeFile := filepath.Join(buildDir, "docker", "docker-compose.yml")
	services, err := parseComposeServices(composeFile)
	if err != nil {
		return "", err
	}
	override := filepath.Join(buildDir, "docker", "benchmark.override.yml")
	var b strings.Builder
	b.WriteString("services:\n")
	b.WriteString("  jaeger_ctr:\n")
	b.WriteString(fmt.Sprintf("    image: %s\n", jaegerImage))
	b.WriteString("    user: \"0:0\"\n")
	b.WriteString("    environment:\n")
	b.WriteString("      SPAN_STORAGE_TYPE: badger\n")
	b.WriteString("      BADGER_EPHEMERAL: \"false\"\n")
	b.WriteString("      BADGER_DIRECTORY_VALUE: /badger/data\n")
	b.WriteString("      BADGER_DIRECTORY_KEY: /badger/key\n")
	b.WriteString("    volumes:\n")
	b.WriteString(fmt.Sprintf("      - \"%s:/badger\"\n", jaegerDir))
	if mock {
		for _, service := range services {
			if service == "jaeger_ctr" {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s:\n", service))
			b.WriteString("    environment:\n")
			b.WriteString("      DMAS_BENCH_MOCK: \"1\"\n")
		}
	}
	return override, os.WriteFile(override, []byte(b.String()), 0o644)
}

func prepareJaegerDir(jaegerDir string) error {
	for _, dir := range []string{jaegerDir, filepath.Join(jaegerDir, "data"), filepath.Join(jaegerDir, "key")} {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o777); err != nil {
			return err
		}
	}
	return nil
}

func parseComposeServices(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var services []string
	inServices := false
	for _, line := range strings.Split(string(b), "\n") {
		if line == "services:" {
			inServices = true
			continue
		}
		if !inServices {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			services = append(services, strings.TrimSuffix(strings.TrimSpace(line), ":"))
		}
	}
	return services, nil
}

func composeCommand(buildDir, overrideFile, project string, args ...string) []string {
	composeFile := filepath.Join(buildDir, "docker", "docker-compose.yml")
	cmd := []string{"docker", "compose", "-p", project}
	envFile := filepath.Join(buildDir, ".local.env")
	if _, err := os.Stat(envFile); err == nil {
		cmd = append(cmd, "--env-file", envFile)
	}
	cmd = append(cmd, "-f", composeFile, "-f", overrideFile)
	return append(cmd, args...)
}

func composeDown(buildDir, overrideFile, project string, logWriter io.Writer) error {
	args := composeCommand(buildDir, overrideFile, project, "down", "--remove-orphans")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = filepath.Join(buildDir, "docker")
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	_ = cmd.Run()
	return nil
}

func stopCaseOnInterrupt(buildDir, overrideFile, project, caseName string, progressWriter, logWriter io.Writer) func() {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-signals:
			fmt.Fprintf(progressWriter, "received interrupt; stopping containers for %s\n", caseName)
			composeDown(buildDir, overrideFile, project, logWriter)
			os.Exit(130)
		case <-done:
		}
	}()

	return func() {
		signal.Stop(signals)
		close(done)
	}
}

func dumpComposeDiagnostics(buildDir, overrideFile, project string, logWriter io.Writer) {
	fmt.Fprintln(logWriter, "\n--- docker compose ps ---")
	_ = runCommand(composeCommand(buildDir, overrideFile, project, "ps"), filepath.Join(buildDir, "docker"), logWriter)
	fmt.Fprintln(logWriter, "\n--- docker compose logs ---")
	_ = runCommand(composeCommand(buildDir, overrideFile, project, "logs", "--no-color"), filepath.Join(buildDir, "docker"), logWriter)
}

func composeProjectName(runID, caseName string) string {
	sum := sha1.Sum([]byte(runID + "/" + caseName))
	prefix := cleanName("benchmark-" + runID + "-" + caseName)
	if len(prefix) > 46 {
		prefix = prefix[:46]
	}
	return fmt.Sprintf("%s-%x", strings.TrimRight(prefix, "-"), sum[:4])
}

func listBenchmarkContainers(project string) ([]string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.ID}}\t{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var containers []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		id := strings.TrimSpace(parts[0])
		name := ""
		if len(parts) == 2 {
			name = strings.TrimSpace(parts[1])
		}
		if id == "" || strings.Contains(strings.ToLower(name), "jaeger_ctr") {
			continue
		}
		containers = append(containers, id)
	}
	return containers, nil
}

func startResourceSampling(containers []string, interval time.Duration) func() ([]ResourceSample, error) {
	if len(containers) == 0 {
		return func() ([]ResourceSample, error) { return nil, nil }
	}
	if interval <= 0 {
		interval = time.Second
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	var samples []ResourceSample
	var firstErr error

	capture := func() {
		rows, err := collectResourceSamples(containers)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		samples = append(samples, rows...)
	}

	go func() {
		defer close(done)
		capture()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				capture()
			}
		}
	}()

	return func() ([]ResourceSample, error) {
		close(stop)
		<-done
		return samples, firstErr
	}
}

func collectResourceSamples(containers []string) ([]ResourceSample, error) {
	args := append([]string{"stats", "--no-stream", "--format", "{{json .}}"}, containers...)
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker stats: %w: %s", err, strings.TrimSpace(string(output)))
	}

	now := time.Now()
	var samples []ResourceSample
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row map[string]string
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse docker stats: %w", err)
		}
		samples = append(samples, ResourceSample{
			Timestamp:     now,
			ContainerID:   strings.TrimSpace(row["ID"]),
			ContainerName: strings.TrimSpace(row["Name"]),
			CPUPercent:    parsePercent(row["CPUPerc"]),
			MemoryBytes:   parseMemoryBytes(row["MemUsage"]),
			MemoryPercent: parsePercent(row["MemPerc"]),
		})
	}
	return samples, nil
}

func parsePercent(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.ReplaceAll(value, ",", "")
	n, _ := strconv.ParseFloat(value, 64)
	return n
}

func parseMemoryBytes(value string) int64 {
	value = strings.TrimSpace(strings.Split(value, "/")[0])
	if value == "" {
		return 0
	}
	i := 0
	for i < len(value) && ((value[i] >= '0' && value[i] <= '9') || value[i] == '.') {
		i++
	}
	n, _ := strconv.ParseFloat(value[:i], 64)
	unit := strings.ToLower(strings.TrimSpace(value[i:]))
	multiplier := float64(1)
	switch unit {
	case "k", "kb", "kib":
		multiplier = 1024
	case "m", "mb", "mib":
		multiplier = 1024 * 1024
	case "g", "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "t", "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	}
	return int64(n * multiplier)
}

func runCommand(cmdArgs []string, dir string, writer io.Writer) error {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = dir
	cmd.Stdout = writer
	cmd.Stderr = writer
	return cmd.Run()
}

func loadLocalEnv(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		out[parts[0]] = parts[1]
	}
	return out
}

func discoverHTTPPort(env map[string]string, ex ExampleConfig) (int, error) {
	if ex.EntrypointEnv != "" {
		port := parsePort(env[ex.EntrypointEnv])
		if port == 0 {
			return 0, fmt.Errorf("%s not found in .local.env", ex.EntrypointEnv)
		}
		return port, nil
	}

	best := 0
	for key, value := range env {
		if strings.HasSuffix(key, "_HTTP_BIND_ADDR") {
			if port := parsePort(value); port > best {
				best = port
			}
		}
	}
	if best == 0 {
		return 0, fmt.Errorf("no *_HTTP_BIND_ADDR found in .local.env")
	}
	return best, nil
}

func discoverJaegerPort(env map[string]string) (int, error) {
	port := parsePort(env["JAEGER_UI_BIND_ADDR"])
	if port == 0 {
		return 0, fmt.Errorf("no JAEGER_UI_BIND_ADDR found in .local.env")
	}
	return port, nil
}

func parsePort(value string) int {
	parts := strings.Split(value, ":")
	if len(parts) == 0 {
		return 0
	}
	port, _ := strconv.Atoi(parts[len(parts)-1])
	return port
}

func waitTCP(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("%s:%d", host, port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", addr)
}

func loadQueries(path string) ([]QueryRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%s has no query rows", path)
	}
	headers := records[0]
	var rows []QueryRow
	for _, rec := range records[1:] {
		row := QueryRow{}
		for i, h := range headers {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func runLoad(endpoint string, c CasePlan, rows []QueryRow) []RequestResult {
	total := c.Profile.Requests
	concurrency := c.Profile.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	jobs := make(chan int)
	results := make([]RequestResult, 0, total)
	var mu sync.Mutex
	var wg sync.WaitGroup
	client := &http.Client{Timeout: time.Duration(c.Profile.TimeoutSeconds) * time.Second}

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seq := range jobs {
				row := rows[seq%len(rows)]
				reqURL := buildRequestURL(endpoint, c.Example, row)
				result := sendOne(client, reqURL, c, row, seq)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}
		}()
	}
	for seq := 0; seq < total; seq++ {
		jobs <- seq
	}
	close(jobs)
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Sequence < results[j].Sequence })
	return results
}

func buildRequestURL(endpoint string, ex ExampleConfig, row QueryRow) string {
	values := url.Values{}
	if ex.Request == "body" {
		body := map[string]any{}
		for key, value := range row {
			if key == "id" {
				continue
			}
			if strings.Contains(value, "|") {
				body[key] = strings.Split(value, "|")
			} else {
				body[key] = value
			}
		}
		b, _ := json.Marshal(body)
		values.Set("req", string(b))
	} else {
		keys := ex.Params
		if len(keys) == 0 {
			for key := range row {
				if key != "id" {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
		}
		for _, key := range keys {
			values.Set(key, row[key])
		}
	}
	if strings.Contains(endpoint, "?") {
		return endpoint + "&" + values.Encode()
	}
	return endpoint + "?" + values.Encode()
}

func sendOne(client *http.Client, reqURL string, c CasePlan, row QueryRow, seq int) RequestResult {
	start := time.Now()
	status := 0
	size := 0
	errText := ""
	responseText := ""
	resp, err := client.Get(reqURL)
	if err != nil {
		errText = err.Error()
	} else {
		status = resp.StatusCode
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		size = len(body)
		if readErr != nil {
			errText = readErr.Error()
		}
		if status < 200 || status >= 300 {
			responseText = trimResponseText(string(body))
			if errText == "" {
				errText = responseText
			}
		}
	}
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	return RequestResult{
		Example:       c.Example.Name,
		Spec:          c.Spec,
		Profile:       c.Profile.Name,
		Sequence:      seq,
		QueryID:       row["id"],
		Status:        status,
		OK:            status >= 200 && status < 300 && errText == "",
		LatencyMS:     latency,
		ResponseBytes: size,
		Error:         errText,
		ResponseText:  responseText,
		URL:           reqURL,
	}
}

func trimResponseText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 2048 {
		return value
	}
	return value[:2048] + "...<truncated>"
}

func collectTraces(baseURL string, start, end time.Time) ([]map[string]any, error) {
	deadline := time.Now().Add(30 * time.Second)
	var traces []map[string]any
	var lastErr error
	for time.Now().Before(deadline) {
		traces, lastErr = collectTracesOnce(baseURL, start, end)
		if lastErr == nil && len(traces) > 0 {
			return traces, nil
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no traces returned")
		}
		time.Sleep(time.Second)
	}
	return traces, lastErr
}

func collectTracesOnce(baseURL string, start, end time.Time) ([]map[string]any, error) {
	servicesPayload, err := fetchJSON(baseURL + "/api/services")
	if err != nil {
		return nil, err
	}
	var services []string
	for _, item := range asSlice(servicesPayload["data"]) {
		if s, ok := item.(string); ok {
			services = append(services, s)
		}
	}
	seen := map[string]bool{}
	var traces []map[string]any
	var lastErr error
	for _, service := range services {
		params := url.Values{}
		params.Set("service", service)
		params.Set("start", strconv.FormatInt(start.UnixMicro(), 10))
		params.Set("end", strconv.FormatInt(end.UnixMicro(), 10))
		params.Set("limit", "1000")
		payload, err := fetchJSON(baseURL + "/api/traces?" + params.Encode())
		if err != nil {
			lastErr = err
			continue
		}
		for _, raw := range asSlice(payload["data"]) {
			trace, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			traceID, _ := trace["traceID"].(string)
			if traceID == "" || seen[traceID] {
				continue
			}
			seen[traceID] = true
			traces = append(traces, trace)
		}
	}
	return traces, lastErr
}

func fetchJSON(fetchURL string) (map[string]any, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fetchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func flattenSpans(traces []map[string]any) []map[string]any {
	var rows []map[string]any
	for _, trace := range traces {
		traceID, _ := trace["traceID"].(string)
		processes := map[string]string{}
		if rawProcesses, ok := trace["processes"].(map[string]any); ok {
			for id, raw := range rawProcesses {
				if proc, ok := raw.(map[string]any); ok {
					processes[id], _ = proc["serviceName"].(string)
				}
			}
		}
		for _, rawSpan := range asSlice(trace["spans"]) {
			span, ok := rawSpan.(map[string]any)
			if !ok {
				continue
			}
			tags := tagsMap(span)
			processID, _ := span["processID"].(string)
			rows = append(rows, map[string]any{
				"trace_id":       traceID,
				"span_id":        span["spanID"],
				"operation_name": span["operationName"],
				"service_name":   processes[processID],
				"start_time":     span["startTime"],
				"duration":       span["duration"],
				"tags":           tags,
			})
		}
	}
	return rows
}

func summarizeCase(c CasePlan, results []RequestResult, spans []map[string]any, resourceSamples []ResourceSample, elapsed time.Duration) CaseSummary {
	var latencies []float64
	successes := 0
	for _, result := range results {
		if result.OK {
			successes++
			latencies = append(latencies, result.LatencyMS)
		}
	}
	sort.Float64s(latencies)

	components := map[string]*ComponentSummary{}
	var totalIn, totalOut, totalTokens int64
	for _, span := range spans {
		name, _ := span["operation_name"].(string)
		if name == "" {
			name = "unknown"
		}
		comp := components[name]
		if comp == nil {
			comp = &ComponentSummary{Name: name}
			components[name] = comp
		}
		comp.Spans++
		comp.DurationMS += anyFloat(span["duration"]) / 1000.0
		tags, _ := span["tags"].(map[string]any)
		in := anyInt(tags["llm.input_tokens"])
		out := anyInt(tags["llm.output_tokens"])
		total := anyInt(tags["llm.total_tokens"])
		comp.InputTokens += in
		comp.OutputTokens += out
		comp.TotalTokens += total
		totalIn += in
		totalOut += out
		totalTokens += total
	}
	var componentRows []ComponentSummary
	for _, comp := range components {
		componentRows = append(componentRows, *comp)
	}
	sort.Slice(componentRows, func(i, j int) bool { return componentRows[i].Name < componentRows[j].Name })

	elapsedSeconds := elapsed.Seconds()
	throughput := 0.0
	if elapsedSeconds > 0 {
		throughput = float64(len(results)) / elapsedSeconds
	}
	cpuAvg, cpuMax, memAvg, memMax := summarizeResources(resourceSamples)
	return CaseSummary{
		Example:        c.Example.Name,
		Spec:           c.Spec,
		Profile:        c.Profile.Name,
		Requests:       len(results),
		Successes:      successes,
		Errors:         len(results) - successes,
		ElapsedMS:      float64(elapsed.Microseconds()) / 1000.0,
		ThroughputRPS:  throughput,
		P50MS:          percentile(latencies, 50),
		P95MS:          percentile(latencies, 95),
		P99MS:          percentile(latencies, 99),
		InputTokens:    totalIn,
		OutputTokens:   totalOut,
		TotalTokens:    totalTokens,
		CPUAvgPercent:  cpuAvg,
		CPUMaxPercent:  cpuMax,
		MemoryAvgBytes: memAvg,
		MemoryMaxBytes: memMax,
		Components:     componentRows,
	}
}

func summarizeResources(samples []ResourceSample) (float64, float64, int64, int64) {
	if len(samples) == 0 {
		return 0, 0, 0, 0
	}

	type totalSample struct {
		cpuPercent float64
		memory     int64
	}

	totals := map[string]*totalSample{}
	for _, sample := range samples {
		timestamp := sample.Timestamp.Format(time.RFC3339Nano)
		total := totals[timestamp]
		if total == nil {
			total = &totalSample{}
			totals[timestamp] = total
		}
		total.cpuPercent += sample.CPUPercent
		total.memory += sample.MemoryBytes
	}

	var cpuSum, cpuMax float64
	var memorySum, memoryMax int64
	for _, total := range totals {
		cpuSum += total.cpuPercent
		if total.cpuPercent > cpuMax {
			cpuMax = total.cpuPercent
		}
		memorySum += total.memory
		if total.memory > memoryMax {
			memoryMax = total.memory
		}
	}

	count := float64(len(totals))
	return cpuSum / count, cpuMax, int64(float64(memorySum) / count), memoryMax
}

func printCaseSummary(s CaseSummary) {
	fmt.Printf("%s %s %s status=%s requests=%d ok=%d errors=%d throughput=%.2f/s p50=%.0fms p95=%.0fms p99=%.0fms tokens=%d trace=%s cpu_avg=%.2f cores cpu_max=%.2f cores mem_avg=%s mem_max=%s\n",
		s.Example, s.Spec, s.Profile, summaryStatus(s), s.Requests, s.Successes, s.Errors, s.ThroughputRPS, s.P50MS, s.P95MS, s.P99MS, s.TotalTokens, traceStatus(s), cpuCores(s.CPUAvgPercent), cpuCores(s.CPUMaxPercent), formatBytes(s.MemoryAvgBytes), formatBytes(s.MemoryMaxBytes))
	for _, comp := range s.Components {
		if comp.Name == "llm.call" || comp.TotalTokens > 0 || strings.HasPrefix(comp.Name, "tool.") || strings.Contains(comp.Name, "mcp") || strings.Contains(comp.Name, "kb.") {
			fmt.Printf("  %-24s %4d spans %9.0fms %8d tokens\n", comp.Name, comp.Spans, comp.DurationMS, comp.TotalTokens)
		}
	}
	if s.TraceError != "" {
		fmt.Printf("  trace error: %s\n", s.TraceError)
	}
	if s.ResourceError != "" {
		fmt.Printf("  resource error: %s\n", s.ResourceError)
	}
}

func printSummaryTable(summaries []CaseSummary) {
	fmt.Println("example spec profile status requests ok errors throughput p50 p95 p99 tokens trace cpu_avg_cores cpu_max_cores mem_avg mem_max")
	for _, s := range summaries {
		fmt.Printf("%s %s %s %s %d %d %d %.2f/s %.0fms %.0fms %.0fms %d %s %.2f %.2f %s %s\n",
			s.Example, s.Spec, s.Profile, summaryStatus(s), s.Requests, s.Successes, s.Errors, s.ThroughputRPS, s.P50MS, s.P95MS, s.P99MS, s.TotalTokens, traceStatus(s), cpuCores(s.CPUAvgPercent), cpuCores(s.CPUMaxPercent), formatBytes(s.MemoryAvgBytes), formatBytes(s.MemoryMaxBytes))
	}
}

func cpuCores(cpuPercent float64) float64 {
	return cpuPercent / 100.0
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0B"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%dB", bytes)
	}
	if value >= 100 {
		return fmt.Sprintf("%.0f%s", value, units[unit])
	}
	if value >= 10 {
		return fmt.Sprintf("%.1f%s", value, units[unit])
	}
	return fmt.Sprintf("%.2f%s", value, units[unit])
}

func summaryStatus(s CaseSummary) string {
	if s.Requests > 0 && s.Successes == 0 {
		return "failed"
	}
	if s.Errors > 0 {
		return "partial"
	}
	return "ok"
}

func traceStatus(s CaseSummary) string {
	if s.TraceError != "" {
		return "error"
	}
	return "ok"
}

func percentile(values []float64, pct float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) == 1 {
		return values[0]
	}
	idx := (float64(len(values)) - 1) * pct / 100.0
	lo := int(idx)
	hi := lo + 1
	if hi >= len(values) {
		return values[lo]
	}
	weight := idx - float64(lo)
	return values[lo]*(1-weight) + values[hi]*weight
}

func tagsMap(span map[string]any) map[string]any {
	out := map[string]any{}
	for _, raw := range asSlice(span["tags"]) {
		tag, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		key, _ := tag["key"].(string)
		if key != "" {
			out[key] = tag["value"]
		}
	}
	return out
}

func asSlice(value any) []any {
	if s, ok := value.([]any); ok {
		return s
	}
	return nil
}

func anyFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func anyInt(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func writeJSONL[T any](path string, rows []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func loadSummaries(runDir string) ([]CaseSummary, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}
	var summaries []CaseSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(runDir, entry.Name(), "summary.json")
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var summary CaseSummary
		if err := json.Unmarshal(b, &summary); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Example+summaries[i].Spec+summaries[i].Profile < summaries[j].Example+summaries[j].Spec+summaries[j].Profile
	})
	return summaries, nil
}

func latestRun(resultsRoot string) (string, error) {
	entries, err := os.ReadDir(resultsRoot)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no runs found under %s", resultsRoot)
	}
	sort.Strings(names)
	return names[len(names)-1], nil
}

func cleanName(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
