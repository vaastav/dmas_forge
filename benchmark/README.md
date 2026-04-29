# Benchmark

Small Go runner for comparing the DMAS Forge examples.

It answers a simple question:

> For the same example and prompts, how do different wiring specs behave?

The benchmark builds examples, runs them in Docker, sends HTTP requests, collects traces and resource usage, then writes a small summary.

## What It Measures

- Request success and errors
- Latency: p50, p95, p99
- Throughput: requests per second
- LLM token counts from traces
- Component spans, like `llm.call`, tools, MCP, and RAG work
- Docker CPU and memory usage

## What It Compares

Cases come from `config.json`.

Current examples:

- `weather`
- `travel-planning`
- `marketing-agency`
- `financial-analyzer`
- `chat`
- `rag_chat`

Each example has one or more specs, such as `single`, `http`, `mcp`, `a2a`, `memory`, `no_memory`, `automatic`, or `agentic`.

Profiles control load:

- `sequential`: low load, one request at a time
- `light`: small concurrent run
- `heavy`: larger concurrent run

## Quick Start

```bash
go run main.go list
go run main.go smoke -examples weather -specs single -profiles sequential
go run main.go run -examples weather -specs single -profiles sequential
go run main.go summary
```

## Common Commands

```bash
# Show configured examples, specs, query files, and profiles.
go run main.go list

# Generate deployments only.
go run main.go build -examples weather,chat -specs single,memory

# Run one request per selected case.
go run main.go smoke -examples weather -specs single,http

# Run the selected benchmark cases.
go run main.go run -examples weather -specs single -profiles sequential

# Print a saved run summary. Without -run, this uses the latest run.
go run main.go summary -run <run-id>

# Open Jaeger for one saved case.
go run main.go jaeger -run <run-id> -case weather-single-sequential
```

Use `-examples`, `-specs`, and `-profiles` to keep runs small while testing.

## How It Works

1. Reads cases from `config.json`.
2. Reads model settings from `model.json`.
3. Generates selected example/spec deployments into `.builds/`.
4. Starts one Docker Compose case at a time.
5. Adds Jaeger tracing for the run.
6. Sends requests from `queries/<example>.csv`.
7. Samples Docker CPU and memory while requests run.
8. Saves raw data and a summary under `results/`.
9. Stops containers before moving to the next case.

The runner cycles through query rows until the selected profile's request count is reached.

## Results

Generated builds are written here:

```text
benchmark/.builds/<example>/<spec>/
```

Runs are saved here:

```text
benchmark/results/<run-id>/
```

Each case gets its own folder:

```text
benchmark/results/<run-id>/<example>-<spec>-<profile>/
```

Important files:

```text
build.log        Docker and case logs
requests.jsonl   One row per HTTP request
resources.jsonl  Docker CPU and memory samples
traces.json      Raw Jaeger traces
spans.jsonl      Flattened spans
summary.json     Latency, errors, throughput, tokens, CPU, memory
jaeger/          Saved Jaeger storage
```

The run folder also includes `run.json`, which records the config and model name/url used for the run. The model key is not saved. Deployment generation logs are written to the run folder's top-level `build.log`.

## Config Files

- `config.json`: examples, specs, profiles, routes, query files, and mock mode
- `model.json`: model name, URL, key, and embedding model
- `queries/*.csv`: request inputs for each example

Set `"mock": true` in `config.json` to inject `DMAS_BENCH_MOCK=1` into benchmark containers.

That currently mocks:

- `marketing-agency` DuckDuckGo search: returns fixed search-result JSON
- `marketing-agency` image generation: writes a deterministic local JPEG instead of calling DALL-E
- `financial-analyzer` MCP tools: replaces external MCP servers with local `search_web` and `fetch_url` tools backed by fixed finance fixtures

## Model Access From Docker

Docker containers must be able to reach the model URL in `model.json`.

If the model is only reachable from the host, expose a forwarding port first:

```bash
socat TCP-LISTEN:30001,reuseaddr,fork,bind=0.0.0.0 TCP:<MODEL_HOST>:<MODEL_PORT>
```

Then use a Docker-reachable URL in `model.json`, for example:

```text
http://host.docker.internal:30001/v1
```

## Technical Notes

The benchmark relies on three measurement paths:

- HTTP request timing from the runner
- OpenTelemetry traces exported to Jaeger
- Docker CPU and memory samples from `docker stats`

The example wiring specs add a Jaeger collector and instrument the workflow services with OpenTelemetry. The generated containers export spans during each request, and the runner saves both raw traces and flattened span rows.

LLM calls are wrapped in `llm.call` spans. Those spans include model/provider attributes, tool-call counts, and token usage when the OpenAI-compatible response reports it. Tool use inside LLM loops is wrapped in `llm.tool_call` spans.

RAG code adds spans around knowledge-base work:

- `kb.index`
- `kb.query`
- `embedding.create`
- `rag.tool_call`

The runner summarizes spans by operation name. That is how `summary.json` can show total tokens and component-level timing for LLM calls, tools, MCP work, and RAG work.

Latency is measured outside the app by the runner's HTTP client, so it includes the full request path for the selected spec. That makes `single`, `http`, `mcp`, and `a2a` comparable from the caller's point of view.

CPU and memory are sampled once per second from the benchmark Docker Compose project. Jaeger is excluded from those resource totals so the numbers focus on the app containers.

Mock mode is controlled by `DMAS_BENCH_MOCK=1`. The runner copies that env var into each benchmark container, and the container code uses it as the switch for the mocked benchmark behavior listed above.
