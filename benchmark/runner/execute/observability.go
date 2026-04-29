package execute

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

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
