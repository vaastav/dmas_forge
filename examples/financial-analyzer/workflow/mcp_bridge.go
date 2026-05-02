package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const mcpBridgeTracerName = "github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow/mcp_bridge"

type MCPToolBridge struct {
	servers   []*mcpBridgeServer
	toolToIdx map[string]int
	tools     map[string]openai.ChatCompletionToolParam
	mock      bool
}

type mcpBridgeServer struct {
	url     string
	client  *mcp.Client
	session *mcp.ClientSession
	mu      sync.RWMutex
}

func NewMCPToolBridge(ctx context.Context, serverURLs []string) (*MCPToolBridge, error) {
	if os.Getenv("DMAS_BENCH_MOCK") == "1" {
		return newMockMCPToolBridge(), nil
	}

	b := &MCPToolBridge{
		servers:   make([]*mcpBridgeServer, 0, len(serverURLs)),
		toolToIdx: make(map[string]int),
		tools:     make(map[string]openai.ChatCompletionToolParam),
	}

	if len(serverURLs) == 0 {
		return nil, fmt.Errorf("at least one MCP server URL is required")
	}

	for _, rawURL := range serverURLs {
		url := strings.TrimSpace(rawURL)
		if url == "" {
			continue
		}
		tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(mcpBridgeTracerName)
		serverCtx, span := tracer.Start(ctx, "mcp.discovery",
			trace.WithAttributes(
				attribute.String("mcp.server_url", url),
				attribute.String("provider_mode", "external"),
			),
		)

		client := mcp.NewClient(&mcp.Implementation{
			Name:    "financial-analyzer-mcp-bridge",
			Version: "1.0.0",
		}, nil)

		server := &mcpBridgeServer{url: url, client: client}
		session, err := server.connect(serverCtx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			log.Printf("MCP bridge: failed to connect to server %s: %v", url, err)
			continue
		}

		result, err := session.ListTools(serverCtx, nil)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			log.Printf("MCP bridge: failed to list tools from %s: %v", url, err)
			_ = session.Close()
			continue
		}
		span.SetAttributes(attribute.Int("mcp.tool_count", len(result.Tools)))
		span.SetStatus(codes.Ok, "")
		span.End()

		idx := len(b.servers)
		server.session = session
		b.servers = append(b.servers, server)

		for _, tool := range result.Tools {
			if _, exists := b.toolToIdx[tool.Name]; exists {
				log.Printf("MCP bridge: tool %q already registered from another server; overriding with definition from %s", tool.Name, url)
			}
			openaiTool, err := mcpToolToOpenAI(tool)
			if err != nil {
				log.Printf("MCP bridge: skipping tool %q from %s: %v", tool.Name, url, err)
				continue
			}
			b.toolToIdx[tool.Name] = idx
			b.tools[tool.Name] = openaiTool
		}
	}

	if len(b.tools) == 0 {
		for _, server := range b.servers {
			if session := server.currentSession(); session != nil {
				_ = session.Close()
			}
		}
		return nil, fmt.Errorf("no tools discovered from any MCP server")
	}

	log.Printf("MCP bridge: connected to %d server(s), discovered %d tool(s)", len(b.servers), len(b.tools))
	return b, nil
}

func (b *MCPToolBridge) AddToolsToAgent(ctx context.Context, agent core.Agent) error {
	return agent.AddTools(ctx, b.tools)
}

func (b *MCPToolBridge) HandleToolCall(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(mcpBridgeTracerName)
	ctx, span := tracer.Start(ctx, "mcp.tool_call",
		trace.WithAttributes(
			attribute.String("tool.name", tc.Function.Name),
			attribute.String("provider_mode", b.providerMode()),
		),
	)
	defer span.End()

	if b.mock {
		out, err := b.handleMockToolCall(tc)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return "", err
		}
		span.SetStatus(codes.Ok, "")
		return out, nil
	}

	idx, ok := b.toolToIdx[tc.Function.Name]
	if !ok {
		span.SetStatus(codes.Error, "tool not found")
		return fmt.Sprintf("Error: tool %q not found in MCP bridge", tc.Function.Name), nil
	}
	span.SetAttributes(attribute.Int("mcp.session_index", idx))

	var args map[string]any
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return "", fmt.Errorf("parsing tool call arguments for %q: %w", tc.Function.Name, err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	result, err := b.callTool(ctx, idx, &mcp.CallToolParams{
		Name:      tc.Function.Name,
		Arguments: args,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Sprintf("Error calling MCP tool %q: %v", tc.Function.Name, err), nil
	}

	var parts []string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		} else {
			parts = append(parts, fmt.Sprintf("%v", content))
		}
	}

	output := strings.Join(parts, "\n")
	if result.IsError && output != "" {
		output = "Error from MCP server: " + output
		span.SetStatus(codes.Error, "mcp tool returned error")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.SetAttributes(attribute.Bool("mcp.tool_is_error", result.IsError))
	return output, nil
}

func (b *MCPToolBridge) Close() error {
	var firstErr error
	for _, server := range b.servers {
		session := server.currentSession()
		if session != nil {
			if err := session.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *mcpBridgeServer) connect(ctx context.Context) (*mcp.ClientSession, error) {
	return s.client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: s.url}, nil)
}

func (s *mcpBridgeServer) currentSession() *mcp.ClientSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

func (s *mcpBridgeServer) replaceSession(ctx context.Context, stale *mcp.ClientSession) (*mcp.ClientSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != stale {
		return s.session, nil
	}
	session, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	s.session = session
	if session == nil {
		return nil, errors.New("mcp reconnect returned nil session")
	}
	if stale != nil {
		_ = stale.Close()
	}
	return session, nil
}

func (b *MCPToolBridge) callTool(ctx context.Context, idx int, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	server := b.servers[idx]
	session := server.currentSession()
	if session == nil {
		var err error
		session, err = server.replaceSession(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	result, err := session.CallTool(ctx, params)
	if err == nil {
		return result, nil
	}

	session, err = server.replaceSession(ctx, session)
	if err != nil {
		return nil, err
	}
	return session.CallTool(ctx, params)
}

func (b *MCPToolBridge) providerMode() string {
	if b.mock {
		return "mock"
	}
	return "external"
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

type mockCompanyProfile struct {
	Name      string
	Ticker    string
	Price     string
	MarketCap string
	Revenue   string
	NetIncome string
	Summary   string
}

var mockProfiles = []mockCompanyProfile{
	{Name: "Apple", Ticker: "AAPL", Price: "$212.47", MarketCap: "$3.2T", Revenue: "$391.0B trailing twelve months", NetIncome: "$96.9B trailing twelve months", Summary: "Apple combines large-scale consumer hardware with high-margin services revenue."},
	{Name: "Microsoft", Ticker: "MSFT", Price: "$468.12", MarketCap: "$3.5T", Revenue: "$261.8B trailing twelve months", NetIncome: "$96.5B trailing twelve months", Summary: "Microsoft combines enterprise software cash flows with cloud and AI platform growth."},
	{Name: "NVIDIA", Ticker: "NVDA", Price: "$128.44", MarketCap: "$3.1T", Revenue: "$130.5B trailing twelve months", NetIncome: "$72.9B trailing twelve months", Summary: "NVIDIA is driven by accelerated computing and data-center GPU demand."},
}

func mockFinancialSearch(query string) string {
	p := matchMockProfile(query)
	slug := strings.ToLower(strings.ReplaceAll(p.Name, " ", "-"))
	return fmt.Sprintf("1. %s overview\nURL: https://benchmark.mock/%s/overview\nSummary: %s\n\n2. %s financials\nURL: https://benchmark.mock/%s/financials\nSummary: Price %s, market cap %s, revenue %s, net income %s.\n\n3. %s outlook\nURL: https://benchmark.mock/%s/outlook\nSummary: Current operating context and investment watchpoints for %s.",
		p.Name, slug, p.Summary, p.Name, slug, p.Price, p.MarketCap, p.Revenue, p.NetIncome, p.Name, slug, p.Name)
}

func mockFinancialFetch(url string) string {
	p := matchMockProfile(url)
	return fmt.Sprintf("# %s mock source\n\n- Source: Benchmark Mock Finance Feed\n- Ticker: %s\n- Current price: %s\n- Market cap: %s\n- Revenue: %s\n- Net income: %s\n\n## Summary\n%s",
		p.Name, p.Ticker, p.Price, p.MarketCap, p.Revenue, p.NetIncome, p.Summary)
}

func matchMockProfile(value string) mockCompanyProfile {
	lower := strings.ToLower(value)
	for _, p := range mockProfiles {
		if strings.Contains(lower, strings.ToLower(p.Name)) || strings.Contains(lower, strings.ToLower(p.Ticker)) {
			return p
		}
	}
	return mockCompanyProfile{Name: "Example Company", Ticker: "EXM", Price: "$100.00", MarketCap: "$100B", Revenue: "$10B trailing twelve months", NetIncome: "$2B trailing twelve months", Summary: "Fallback mock profile used when the query does not match a benchmark fixture company."}
}

func mcpToolToOpenAI(tool *mcp.Tool) (openai.ChatCompletionToolParam, error) {
	params := openai.FunctionParameters{
		"type": "object",
	}
	if tool.InputSchema != nil {
		switch schema := tool.InputSchema.(type) {
		case map[string]any:
			for k, v := range schema {
				params[k] = v
			}
		default:
			encoded, err := json.Marshal(tool.InputSchema)
			if err != nil {
				return openai.ChatCompletionToolParam{}, fmt.Errorf("marshaling input schema for tool %q: %w", tool.Name, err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				return openai.ChatCompletionToolParam{}, fmt.Errorf("unmarshaling input schema for tool %q: %w", tool.Name, err)
			}
			for k, v := range decoded {
				params[k] = v
			}
		}
	}
	if _, ok := params["type"]; !ok {
		params["type"] = "object"
	}

	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  params,
		},
	}, nil
}
