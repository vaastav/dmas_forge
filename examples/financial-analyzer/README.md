# Financial Analyzer

A multi-agent financial analysis example that is behaviorally close to the original [`mcp_financial_analyzer_benchmark`](https://github.com/sands-lab/maestro/tree/main/examples/mcp-agent/mcp_financial_analyzer_benchmark), but implemented with DMAS-Forge workflow services and split wiring.

## What It Does

Given a company name and mode, the example runs a coordinated set of agents:

1. `ResearchQualityController` calls the `DataCollector` and `DataEvaluator` in a refinement loop.
2. `FinancialAnalyzerCoordinator` drives the agent flow with LLM tool calls.
3. `FinancialAnalyst` runs in full mode to add investment analysis.
4. `ReportWriter` produces the final markdown report.

The `DataCollector` connects to user-supplied MCP servers for search and fetch capabilities, matching the reference benchmark's architecture.

The primary result is returned inline as structured JSON, so the output is directly usable even in containerized deployments.

## Key Differences From The Benchmark

| Aspect | Benchmark | This Example |
|---|---|---|
| Coordination | Python `Orchestrator` | Go coordinator with tool-dispatch to workflow services |
| Research loop | `EvaluatorOptimizerLLM` | Looser `ResearchQualityController` that mimics evaluator/optimizer behavior |
| Search stack | MCP search providers + fetch | User-supplied MCP servers discovered at startup via `ListTools` |
| Output contract | Writes report artifacts locally | Returns core analysis content inline over HTTP/MCP JSON |
| Deployment | Single Python app | `single`, `http`, `mcp`, and `a2a` DMAS-Forge specs |

## Architecture

Workflow services:

- `DataCollectorAgent` — connects to MCP servers for search/fetch tools
- `DataEvaluatorAgent`
- `ResearchQualityController`
- `FinancialAnalystAgent`
- `ReportWriterAgent`
- `FinancialAnalyzerCoordinator`

A `MCPToolBridge` connects to one or more MCP servers at startup, discovers their tools, and proxies LLM tool calls to the appropriate server.
Available wiring specs:

| Wiring spec | Behavior |
|---|---|
| `single` | one container, coordinator exposed over HTTP |
| `http` | one container per service, service-to-service over HTTP |
| `mcp` | one container per service, sub-agents over MCP, coordinator exposed over HTTP |
| `a2a` | one container per service, sub-agents over A2A, coordinator exposed over HTTP |

## Configuration

### Model config — [example_model.json](/home/abdo/dmas_forge/examples/financial-analyzer/wiring/example_model.json)

Edit this file to configure the LLM backend (same format as other DMAS-Forge examples):

```json
{
  "name": "gpt-5.4-nano",
  "url": "https://api.openai.com/v1",
  "key": "your-api-key-here"
}
```

### MCP servers — `-mcp-servers` flag

Comma-separated list of MCP server URLs passed at build time. The `DataCollector` connects to each URL, discovers available tools via the MCP `ListTools` protocol, and proxies LLM tool calls to the appropriate server. This matches the reference benchmark's architecture where search and fetch capabilities come from external MCP servers.

You need at least one MCP server that provides search and/or fetch tools. MCP servers can be **local** (downloaded and run on your machine) or **external** (hosted services accessed over the internet).

The `-mcp-servers` flag accepts one or more URLs:

```bash
# Single local MCP server providing both search and fetch
-mcp-servers=http://localhost:8080

# Separate local servers for search and fetch
-mcp-servers=http://localhost:8080,http://localhost:8081

# External Tavily MCP server (no local install needed)
-mcp-servers=https://mcp.tavily.com/mcp/?tavilyApiKey=<YOUR_API_KEY>
```

If multiple servers expose the same tool name, the last one wins.

This example assumes each external MCP server can handle concurrent or multiplexed requests on the long-lived session the bridge opens at startup.

### Company and mode

Company and mode are required when calling the coordinator.

- `company`: the company name or ticker symbol to analyze, for example `Apple` or `AAPL`.
- `mode`: either `sanity` for a faster lightweight snapshot or `full` for the complete research-and-analysis workflow.

## Build

```bash
cd examples/financial-analyzer/wiring

# Single-container deployment with one MCP server
go run main.go -w single -modfile=./example_model.json -mcp-servers=http://localhost:8080 -o build

# Or one of the Multi-container deployment options:
go run main.go -w http -modfile=./example_model.json -mcp-servers=http://localhost:8080,http://localhost:8081 -o build
go run main.go -w mcp -modfile=./example_model.json -mcp-servers=https://mcp.tavily.com/mcp/?tavilyApiKey=<YOUR_API_KEY> -o build
go run main.go -w a2a -modfile=./example_model.json -mcp-servers=http://localhost:8080 -o build
```

## Usage

1. Start your MCP server(s) before building/running. For example, obtain an API key from Tavily:

```bash
https://mcp.tavily.com/mcp/?tavilyApiKey=<YOUR_API_KEY>
```

2. Build and run the coordinator:

```bash
# After building with one of the commands above, run the generated container
cd build/docker
cp ../.local.env .env
docker compose build
docker compose up
```

3. Send a request to the coordinator's HTTP endpoint:

Example request:

```bash
curl --get 'http://localhost:12345/Analyze' \
  --data-urlencode 'company=Apple' \
  --data-urlencode 'mode=sanity'
```

> [!NOTE]
> The port shown above (`12345`) may change depending on your system. Run `docker ps` to see which localhost port the coordinator service is mapped to.

The response contains a `Ret0` object with:

- `report_markdown`
- `research_markdown`
- `analysis_markdown` (full mode)
- `mode`
- `company`
- `run_id`

## Modes

- `sanity`: one refinement budget, minimum `FAIR`, skips the analyst tool, produces a short snapshot.
- `full`: up to three refinements, minimum `GOOD`, runs the analyst tool, produces a more complete report.

## Notes

- The example is optimized for direct API consumption. File persistence is not part of the main contract.
- Search quality depends on which MCP servers you provide and the selected model.
- You must start at least one MCP server with search/fetch capabilities before building.
