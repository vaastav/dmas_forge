package workflow

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	openai "github.com/openai/openai-go"
)

type mockFinancialTemplate struct {
	Price           string
	DailyMove       string
	Volume          string
	Week52Range     string
	MarketCap       string
	Revenue         string
	NetIncome       string
	EPSActual       string
	EPSEstimate     string
	RevenueActual   string
	RevenueEstimate string
	EarningsQuarter string
	YoYGrowth       string
	PERatio         string
	Headlines       []string
	Summary         string
}

var mockFinancialTemplates = []mockFinancialTemplate{
	{Price: "$184.22", DailyMove: "+1.4%", Volume: "42.1M shares", Week52Range: "$132.10 - $199.80", MarketCap: "$2.8T", Revenue: "$381.6B trailing twelve months", NetIncome: "$91.4B trailing twelve months", EPSActual: "$2.18", EPSEstimate: "$2.05", RevenueActual: "$94.7B", RevenueEstimate: "$92.8B", EarningsQuarter: "Q1 FY2026", YoYGrowth: "+7.8%", PERatio: "29.4", Headlines: []string{"Shares rose after better-than-expected quarterly earnings", "Analysts highlighted resilient margins and cash flow"}, Summary: "Large-cap business with durable margins, strong free cash flow, and moderate growth expectations."},
	{Price: "$267.45", DailyMove: "-0.6%", Volume: "18.7M shares", Week52Range: "$211.35 - $289.90", MarketCap: "$640B", Revenue: "$112.3B trailing twelve months", NetIncome: "$24.8B trailing twelve months", EPSActual: "$1.74", EPSEstimate: "$1.69", RevenueActual: "$28.6B", RevenueEstimate: "$28.1B", EarningsQuarter: "Q4 FY2025", YoYGrowth: "+4.2%", PERatio: "24.7", Headlines: []string{"Management reiterated full-year revenue guidance", "New product cycle expected to support second-half demand"}, Summary: "Profitable operator with stable revenue, disciplined expenses, and steady capital returns."},
	{Price: "$96.38", DailyMove: "+2.2%", Volume: "63.4M shares", Week52Range: "$58.20 - $108.75", MarketCap: "$235B", Revenue: "$67.9B trailing twelve months", NetIncome: "$12.6B trailing twelve months", EPSActual: "$0.88", EPSEstimate: "$0.76", RevenueActual: "$17.4B", RevenueEstimate: "$16.8B", EarningsQuarter: "Q2 FY2026", YoYGrowth: "+18.5%", PERatio: "36.1", Headlines: []string{"Growth accelerated as enterprise demand improved", "Analysts raised price targets after margin expansion"}, Summary: "Growth-oriented company benefiting from stronger demand, operating leverage, and improving profitability."},
	{Price: "$412.10", DailyMove: "+0.3%", Volume: "9.8M shares", Week52Range: "$344.50 - $438.60", MarketCap: "$1.1T", Revenue: "$204.5B trailing twelve months", NetIncome: "$38.2B trailing twelve months", EPSActual: "$3.42", EPSEstimate: "$3.31", RevenueActual: "$52.2B", RevenueEstimate: "$51.6B", EarningsQuarter: "Q3 FY2025", YoYGrowth: "+9.1%", PERatio: "31.8", Headlines: []string{"Company announced expanded buyback authorization", "Recent results showed continued international growth"}, Summary: "Scaled global business with diversified revenue, consistent earnings, and healthy balance-sheet flexibility."},
}

func newMockMCPToolBridge() *MCPToolBridge {
	return &MCPToolBridge{
		mock: true,
		tools: map[string]openai.ChatCompletionToolParam{
			"search_web": mockMCPTool("search_web", "Return deterministic benchmark financial search results.", "query"),
			"fetch_url":  mockMCPTool("fetch_url", "Return deterministic benchmark page content.", "url"),
		},
		toolToIdx: map[string]int{},
	}
}

func mockMCPTool(name, description, required string) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(description),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					required: map[string]any{"type": "string"},
				},
				"required": []string{required},
			},
		},
	}
}

func (b *MCPToolBridge) handleMockToolCall(tc openai.ChatCompletionMessageToolCall) (string, error) {
	var args map[string]string
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", err
	}
	switch tc.Function.Name {
	case "search_web":
		return mockFinancialSearch(args["query"]), nil
	case "fetch_url":
		return mockFinancialFetch(args["url"]), nil
	default:
		return "", fmt.Errorf("unknown mock MCP tool: %s", tc.Function.Name)
	}
}

func mockFinancialSearch(query string) string {
	company := mockCompanyFromQuery(query)
	set := mockTemplateIndex(query)
	t := mockFinancialTemplates[set]
	dataURL := mockURL("financial-data", company, set)

	return fmt.Sprintf("1. %s benchmark financial data pack\nURL: %s\nSummary: Price %s, daily move %s, volume %s, 52-week range %s, market cap %s. %s EPS actual %s vs estimate %s; revenue actual %s vs estimate %s; year-over-year growth %s. P/E ratio %s. Recent headlines: %s; %s.",
		company, dataURL, t.Price, t.DailyMove, t.Volume, t.Week52Range, t.MarketCap,
		t.EarningsQuarter, t.EPSActual, t.EPSEstimate, t.RevenueActual, t.RevenueEstimate, t.YoYGrowth,
		t.PERatio, t.Headlines[0], t.Headlines[1])
}

func mockFinancialFetch(rawURL string) string {
	company, set := mockCompanyAndSetFromURL(rawURL)
	t := mockFinancialTemplates[set]
	ticker := mockTicker(company)

	return fmt.Sprintf(`# %s benchmark mock finance source

- Source: Benchmark Mock Finance Feed
- Ticker: %s
- Current price: %s
- Daily move: %s
- Trading volume: %s
- 52-week range: %s
- Market cap: %s
- Revenue: %s
- Net income: %s
- Latest earnings quarter: %s
- EPS actual vs estimate: %s vs %s
- Revenue actual vs estimate: %s vs %s
- Year-over-year growth: %s
- P/E ratio: %s

## Recent News
- %s. URL: https://benchmark.mock/news/%s/1
- %s. URL: https://benchmark.mock/news/%s/2

## Summary
%s`,
		company, ticker, t.Price, t.DailyMove, t.Volume, t.Week52Range, t.MarketCap, t.Revenue, t.NetIncome,
		t.EarningsQuarter, t.EPSActual, t.EPSEstimate, t.RevenueActual, t.RevenueEstimate, t.YoYGrowth, t.PERatio,
		t.Headlines[0], url.PathEscape(company), t.Headlines[1], url.PathEscape(company), t.Summary)
}

func mockURL(kind, company string, set int) string {
	q := url.Values{}
	q.Set("company", company)
	q.Set("set", strconv.Itoa(set))
	return "https://benchmark.mock/" + kind + "?" + q.Encode()
}

func mockCompanyAndSetFromURL(rawURL string) (string, int) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "Example Company", mockTemplateIndex(rawURL)
	}
	company := strings.TrimSpace(parsed.Query().Get("company"))
	if company == "" {
		company = "Example Company"
	}
	set, err := strconv.Atoi(parsed.Query().Get("set"))
	if err != nil || set < 0 {
		set = mockTemplateIndex(rawURL)
	}
	return company, set % len(mockFinancialTemplates)
}

func mockCompanyFromQuery(query string) string {
	for _, line := range strings.Split(query, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "target company:") {
			return cleanMockCompany(strings.TrimSpace(line[len("target company:"):]))
		}
	}
	for _, line := range strings.Split(query, "\n") {
		if company := cleanMockCompany(line); company != "" {
			return company
		}
	}
	return "Example Company"
}

func cleanMockCompany(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	words := strings.Fields(value)
	if len(words) > 4 {
		words = words[:4]
	}
	for i, word := range words {
		words[i] = strings.Trim(word, " .,;:!?()[]{}\"'")
	}
	company := strings.Join(words, " ")
	if company == "" {
		return "Example Company"
	}
	return company
}

func mockTicker(company string) string {
	var letters []rune
	for _, r := range company {
		if unicode.IsLetter(r) {
			letters = append(letters, unicode.ToUpper(r))
		}
		if len(letters) == 5 {
			break
		}
	}
	if len(letters) < 3 {
		return "EXM"
	}
	return string(letters)
}

func mockTemplateIndex(value string) int {
	sum := 0
	for _, r := range value {
		sum += int(r)
	}
	return sum % len(mockFinancialTemplates)
}
