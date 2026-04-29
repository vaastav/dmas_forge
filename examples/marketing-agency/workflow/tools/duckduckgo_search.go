package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const searchTracerName = "github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow/tools/search"

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func DuckDuckGoSearchTool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "duckduckgo_search",
			Description: openai.String("Search DuckDuckGo and return top web results for a query."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func DuckDuckGoSearchHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		mode := "real"
		if benchmarkMockEnabled() {
			mode = "mock"
		}
		tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(searchTracerName)
		ctx, span := tracer.Start(ctx, "tool.search",
			trace.WithAttributes(
				attribute.String("tool.name", "duckduckgo_search"),
				attribute.String("provider_mode", mode),
			),
		)
		defer span.End()

		if tc.Function.Name != "duckduckgo_search" {
			err := fmt.Errorf("unsupported tool: %s", tc.Function.Name)
			recordToolError(span, err)
			return "", err
		}

		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			recordToolError(span, err)
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		span.SetAttributes(attribute.String("search.query", args.Query))

		var results []SearchResult
		var err error
		if benchmarkMockEnabled() {
			results, err = mockSearch(args.Query)
		} else {
			results, err = performSearch(ctx, args.Query)
		}
		payload := map[string]interface{}{
			"query":   args.Query,
			"results": results,
		}
		if err != nil {
			payload["error"] = fmt.Sprintf("duckduckgo search failed: %v", err)
			recordToolError(span, err)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.SetAttributes(attribute.Int("search.result_count", len(results)))
		b, err := json.Marshal(payload)
		if err != nil {
			recordToolError(span, err)
			return "", fmt.Errorf("marshal search results: %w", err)
		}

		return string(b), nil
	}
}

func performSearch(ctx context.Context, query string) ([]SearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	encoded := url.QueryEscape(trimmed)
	searchURL := "https://duckduckgo.com/html/?q=" + encoded

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; dmas-forge-marketing-agent/1.0)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	results := extractDuckDuckGoResults(body, 10)
	return results, nil
}

var anchorRE = regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
var stripTagsRE = regexp.MustCompile(`<[^>]+>`)
var mockSlugRE = regexp.MustCompile(`[^a-z0-9]+`)

func extractDuckDuckGoResults(body string, maxResults int) []SearchResult {
	matches := anchorRE.FindAllStringSubmatch(body, -1)
	results := make([]SearchResult, 0, maxResults)
	for _, m := range matches {
		if len(m) < 3 || len(results) >= maxResults {
			break
		}
		href := strings.TrimSpace(html.UnescapeString(m[1]))
		title := strings.TrimSpace(html.UnescapeString(stripTagsRE.ReplaceAllString(m[2], "")))
		if href == "" || title == "" {
			continue
		}
		results = append(results, SearchResult{Title: title, URL: href, Snippet: ""})
	}
	return results
}

func mockSearch(query string) ([]SearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	slug := strings.Trim(mockSlugRE.ReplaceAllString(strings.ToLower(trimmed), "-"), "-")
	if slug == "" {
		slug = "brand"
	}
	topic := trimmed
	words := strings.Fields(trimmed)
	if len(words) > 4 {
		topic = strings.Join(words[:4], " ")
	}
	return []SearchResult{
		{Title: strings.Title(slug) + " Official Site", URL: "https://mock-search.example/brands/" + slug, Snippet: fmt.Sprintf("Mock result for %q covering brand positioning.", topic)},
		{Title: strings.Title(slug) + " Market Trends", URL: "https://mock-search.example/research/" + slug, Snippet: fmt.Sprintf("Deterministic market notes for %q.", topic)},
		{Title: strings.Title(slug) + " Creative References", URL: "https://mock-search.example/creative/" + slug, Snippet: fmt.Sprintf("Stable creative references for %q.", topic)},
	}, nil
}

func benchmarkMockEnabled() bool {
	return os.Getenv("DMAS_BENCH_MOCK") == "1"
}

func recordToolError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
