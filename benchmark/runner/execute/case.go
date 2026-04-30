package execute

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type caseOptions struct {
	BenchDir       string
	ResultsRoot    string
	CaseDir        string
	BuildDir       string
	Mock           bool
	Case           CasePlan
	ProgressWriter io.Writer
	DetailWriter   io.Writer
}

type RunOptions struct {
	RepoRoot string
	BenchDir string
	RunID    string
	Mock     bool
	Cases    []CasePlan
	RunInfo  any
	Stdout   io.Writer
}

func Run(opts RunOptions) error {
	resultsRoot := filepath.Join(opts.BenchDir, "results", opts.RunID)
	if err := os.MkdirAll(resultsRoot, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(resultsRoot, "run.json"), opts.RunInfo); err != nil {
		return err
	}

	buildLogPath := filepath.Join(resultsRoot, "build.log")
	fmt.Fprintf(opts.Stdout, "generating deployment files\n")
	fmt.Fprintf(opts.Stdout, "build logs: %s\n", buildLogPath)
	buildLog, err := os.Create(buildLogPath)
	if err != nil {
		return err
	}
	buildErr := Build(BuildOptions{
		RepoRoot:  opts.RepoRoot,
		BenchDir:  opts.BenchDir,
		Cases:     opts.Cases,
		LogWriter: buildLog,
	})
	closeErr := buildLog.Close()
	if buildErr != nil {
		return buildErr
	}
	if closeErr != nil {
		return closeErr
	}

	for i, c := range opts.Cases {
		caseName := caseName(c)
		caseDir := filepath.Join(resultsRoot, caseName)
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			return err
		}

		logPath := filepath.Join(caseDir, "build.log")
		logFile, err := os.Create(logPath)
		if err != nil {
			return err
		}
		progressWriter := io.MultiWriter(opts.Stdout, logFile)
		fmt.Fprintf(progressWriter, "[%d/%d] running %s/%s/%s\n", i+1, len(opts.Cases), c.Example.Name, c.Spec, c.Profile.Name)
		fmt.Fprintf(progressWriter, "case logs: %s\n", logPath)

		runErr := runCase(caseOptions{
			BenchDir:       opts.BenchDir,
			ResultsRoot:    resultsRoot,
			CaseDir:        caseDir,
			BuildDir:       generatedBuildDir(opts.BenchDir, c),
			Mock:           opts.Mock,
			Case:           c,
			ProgressWriter: progressWriter,
			DetailWriter:   logFile,
		})
		closeErr := logFile.Close()
		if runErr != nil {
			return runErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func caseName(c CasePlan) string {
	return cleanName(c.Example.Name + "-" + c.Spec + "-" + c.Profile.Name)
}

func runCase(opts caseOptions) error {
	c := opts.Case
	caseName := caseName(c)

	fmt.Fprintf(opts.ProgressWriter, "using generated deployment files for %s\n", caseName)
	jaegerDir := filepath.Join(opts.CaseDir, "jaeger")
	if err := prepareJaegerDir(jaegerDir); err != nil {
		return err
	}
	overrideFile, err := writeComposeOverride(opts.BuildDir, jaegerDir, opts.Mock)
	if err != nil {
		return err
	}
	project := composeProjectName(filepath.Base(opts.ResultsRoot), caseName)
	stopInterruptCleanup := stopCaseOnInterrupt(opts.BuildDir, overrideFile, project, caseName, opts.ProgressWriter, opts.DetailWriter)
	defer stopInterruptCleanup()

	fmt.Fprintf(opts.ProgressWriter, "stopping old containers for %s\n", caseName)
	_ = composeDown(opts.BuildDir, overrideFile, project, opts.DetailWriter)
	defer func() {
		fmt.Fprintf(opts.ProgressWriter, "stopping containers for %s\n", caseName)
		dumpComposeDiagnostics(opts.BuildDir, overrideFile, project, opts.DetailWriter)
		composeDown(opts.BuildDir, overrideFile, project, opts.DetailWriter)
	}()

	fmt.Fprintf(opts.ProgressWriter, "building docker images for %s\n", caseName)
	if err := runCommand(composeCommand(opts.BuildDir, overrideFile, project, "build"), filepath.Join(opts.BuildDir, "docker"), opts.DetailWriter); err != nil {
		return err
	}
	fmt.Fprintf(opts.ProgressWriter, "starting docker compose for %s\n", caseName)
	if err := runCommand(composeCommand(opts.BuildDir, overrideFile, project, "up", "-d"), filepath.Join(opts.BuildDir, "docker"), opts.DetailWriter); err != nil {
		return err
	}

	env := loadLocalEnv(filepath.Join(opts.BuildDir, ".local.env"))
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
	settleWait := 15 * time.Second
	fmt.Fprintf(opts.ProgressWriter, "waiting %s for services to settle before testing %s\n", settleWait, caseName)
	time.Sleep(settleWait)

	rows, err := loadQueries(filepath.Join(opts.BenchDir, c.Example.QueryFile))
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("http://localhost:%d%s", httpPort, c.Example.Route)
	fmt.Fprintf(opts.ProgressWriter, "testing load %s mode=%s value=%d concurrency=%d endpoint=%s\n", c.Profile.Name, c.Profile.Mode, c.Profile.Value, c.Profile.Concurrency, endpoint)
	resourceContainers, resourceErr := listBenchmarkContainers(project)
	if resourceErr != nil {
		fmt.Fprintf(opts.ProgressWriter, "resource sampling error: %v\n", resourceErr)
	}
	if resourceErr == nil && len(resourceContainers) == 0 {
		fmt.Fprintf(opts.DetailWriter, "resource sampling: no non-jaeger containers found for project %s\n", project)
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
		fmt.Fprintf(opts.ProgressWriter, "resource sampling error: %v\n", stopErr)
	}
	if err := writeJSONL(filepath.Join(opts.CaseDir, "resources.jsonl"), resourceSamples); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(opts.CaseDir, "requests.jsonl"), results); err != nil {
		return err
	}

	fmt.Fprintf(opts.ProgressWriter, "collecting traces for %s\n", caseName)
	time.Sleep(2 * time.Second)
	traces, traceErr := collectTraces(fmt.Sprintf("http://localhost:%d", jaegerPort), start.Add(-2*time.Second), end.Add(2*time.Second))
	if err := writeJSON(filepath.Join(opts.CaseDir, "traces.json"), map[string]any{"data": traces}); err != nil {
		return err
	}
	spans := flattenSpans(traces)
	if err := writeJSONL(filepath.Join(opts.CaseDir, "spans.jsonl"), spans); err != nil {
		return err
	}
	summary := summarizeCase(c, results, spans, resourceSamples, end.Sub(start))
	if traceErr != nil {
		summary.TraceError = traceErr.Error()
	}
	if resourceErr != nil {
		summary.ResourceError = resourceErr.Error()
	}
	if err := writeJSON(filepath.Join(opts.CaseDir, "summary.json"), summary); err != nil {
		return err
	}
	printCaseSummary(summary)
	return nil
}
