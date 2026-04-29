package execute

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const jaegerImage = "jaegertracing/all-in-one:1.75.0"

func StartJaegerUI(jaegerDir string, port int, stdout, stderr io.Writer) error {
	if err := prepareJaegerDir(jaegerDir); err != nil {
		return err
	}
	cmd := exec.Command(
		"docker", "run", "--rm",
		"--user", "0:0",
		"-p", fmt.Sprintf("%d:16686", port),
		"-e", "SPAN_STORAGE_TYPE=badger",
		"-e", "BADGER_EPHEMERAL=false",
		"-e", "BADGER_DIRECTORY_VALUE=/badger/data",
		"-e", "BADGER_DIRECTORY_KEY=/badger/key",
		"-v", jaegerDir+":/badger",
		jaegerImage,
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
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
