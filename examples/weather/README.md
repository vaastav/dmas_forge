# Weather Agent Example

A multi-agent application consisting of two agents that collaborate to produce weather reports with disaster risk assessments.

## Architecture

The application has two workflow services deployed in separate containers:

- **WeatherAgent** (`workflow/WeatherAgent.go`) -- accepts a location query, uses a `get_weather` tool to fetch weather data, then forwards the result to the DisasterAgent. Registers its own tool and tool handler on the `core.Agent` interface.
- **DisasterAgent** (`workflow/DisasterAgent.go`) -- receives a weather report and assesses the likelihood of natural disasters. Uses a plain `LLMCall` (no tools).

Inter-agent communication is transparent to the workflow code. The wiring spec determines the protocol:

| Wiring spec | Inter-agent protocol | File |
|---|---|---|
| `http` (default) | HTTP | `wiring/specs/default.go` |
| `a2a` | Agent-to-Agent (A2A) | `wiring/specs/a2a.go` |
| `mcp` | Model Context Protocol (MCP) | `wiring/specs/mcp.go` |

## Setup

Edit `wiring/example_model.json` with your API key, model name, and URL:

```json
{
    "name": "gpt-3.5-turbo",
    "url": "https://api.openai.com/v1",
    "key": "your-api-key-here"
}
```

## Build and Run

```bash
cd examples/weather/wiring
go run main.go -w http -o build -modfile=./example_model.json
cd build/docker
docker compose build && docker compose up -d
```

To use a different wiring spec (e.g. A2A):

```bash
go run main.go -w a2a -o build -modfile=./example_model.json
```

## Usage

```bash
curl 'http://localhost:12346/Query?query=London,England%20Weather'
```

Expected output:

```json
{"Ret0":"The current weather in London, England is 30 degrees Celsius.\nThere is not enough information available to determine the likelihood of a natural disaster based solely on the temperature in London, England."}
```

The WeatherAgent calls its `get_weather` tool (which returns a hardcoded response for demonstration), then passes the weather report to the DisasterAgent for risk assessment. Both LLM calls and the inter-agent HTTP call happen transparently within a single user request.

## Notes

- The `get_weather` tool returns a hardcoded response (`"30 degrees Celsius"`) for demonstration purposes. Replace the tool handler in `WeatherAgent.go` to integrate a real weather API.
- Each agent runs in its own Docker container. The wiring spec controls how they communicate -- switching from HTTP to A2A or MCP requires only a wiring change, no workflow modifications.
