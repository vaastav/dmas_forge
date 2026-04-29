package execute

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

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

func startResourceSampling(containers []string, interval time.Duration) func() ([]resourceSample, error) {
	if len(containers) == 0 {
		return func() ([]resourceSample, error) { return nil, nil }
	}
	if interval <= 0 {
		interval = time.Second
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	var samples []resourceSample
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

	return func() ([]resourceSample, error) {
		close(stop)
		<-done
		return samples, firstErr
	}
}

func collectResourceSamples(containers []string) ([]resourceSample, error) {
	args := append([]string{"stats", "--no-stream", "--format", "{{json .}}"}, containers...)
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker stats: %w: %s", err, strings.TrimSpace(string(output)))
	}

	now := time.Now()
	var samples []resourceSample
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row map[string]string
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse docker stats: %w", err)
		}
		samples = append(samples, resourceSample{
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
