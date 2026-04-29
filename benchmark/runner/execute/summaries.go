package execute

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func summarizeCase(c CasePlan, results []requestResult, spans []map[string]any, resourceSamples []resourceSample, elapsed time.Duration) CaseSummary {
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

func summarizeResources(samples []resourceSample) (float64, float64, int64, int64) {
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

func PrintSummaryTable(summaries []CaseSummary) {
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

func LoadSummaries(runDir string) ([]CaseSummary, error) {
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
