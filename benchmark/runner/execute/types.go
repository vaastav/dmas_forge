package execute

import "time"

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

type queryRow map[string]string

type requestResult struct {
	Example       string  `json:"example"`
	Spec          string  `json:"spec"`
	Profile       string  `json:"profile"`
	Sequence      int     `json:"sequence"`
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

type resourceSample struct {
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
