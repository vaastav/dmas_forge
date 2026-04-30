package execute

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func loadQueries(path string) ([]queryRow, error) {
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
	var rows []queryRow
	for _, rec := range records[1:] {
		row := queryRow{}
		for i, h := range headers {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func runLoad(endpoint string, c CasePlan, rows []queryRow) []requestResult {
	concurrency := c.Profile.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	capacity := 0
	if c.Profile.Mode == "requests" {
		capacity = c.Profile.Value
	}
	jobs := make(chan int)
	results := make([]requestResult, 0, capacity)
	var mu sync.Mutex
	var wg sync.WaitGroup
	client := &http.Client{Timeout: time.Duration(c.Profile.TimeoutSeconds) * time.Second}
	finish := func() []requestResult {
		close(jobs)
		wg.Wait()
		sort.Slice(results, func(i, j int) bool { return results[i].Sequence < results[j].Sequence })
		return results
	}

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
	switch c.Profile.Mode {
	case "requests":
		for seq := 0; seq < c.Profile.Value; seq++ {
			jobs <- seq
		}
	case "timed":
		timer := time.NewTimer(time.Duration(c.Profile.Value) * time.Second)
		for seq := 0; ; seq++ {
			select {
			case jobs <- seq:
			case <-timer.C:
				return finish()
			}
		}
	}
	return finish()
}

func buildRequestURL(endpoint string, ex ExampleConfig, row queryRow) string {
	values := url.Values{}
	if ex.Request == "body" {
		body := map[string]any{}
		for key, value := range row {
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
				keys = append(keys, key)
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

func sendOne(client *http.Client, reqURL string, c CasePlan, row queryRow, seq int) requestResult {
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
	return requestResult{
		Example:       c.Example.Name,
		Spec:          c.Spec,
		Profile:       c.Profile.Name,
		Sequence:      seq,
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
