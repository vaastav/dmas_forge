# Benchmark

Simple Go benchmark runner for the examples.

Run from the repository root:

```bash
go run benchmark/main.go -h
go run benchmark/main.go list
go run benchmark/main.go build -examples weather,chat -specs single,memory -rebuild
go run benchmark/main.go run -examples weather -specs single -profiles sequential
go run benchmark/main.go smoke -examples weather -specs single,http
go run benchmark/main.go summary -run <run-id>
go run benchmark/main.go jaeger -run <run-id> -case weather-single-sequential
```

Or from this directory, use `go run main.go <command>`.

Configure the model in `model.json`. Configure examples and request-count profiles in `config.json`.

Queries live in `queries/<example>.csv`. The runner cycles through those rows until the selected profile's request count is reached.

Generated builds go under `cached_builds/`. Saved outputs go under `results/<run-id>/<example>-<spec>-<profile>/`:

```text
build.log
requests.jsonl
traces.json
spans.jsonl
summary.json
jaeger/
```

Set `"mock": true` in `config.json` to inject `DMAS_BENCH_MOCK=1` into benchmark containers. This mocks web search, image generation, and financial-analyzer MCP tools.

Docker containers must be able to reach the model URL in `model.json`. If the model runs on a local/private host, expose a simple forwarding port first:

```bash
socat TCP-LISTEN:30001,reuseaddr,fork,bind=0.0.0.0 TCP:<MODEL_HOST>:<MODEL_PORT>
```

Then use a Docker-reachable URL in `model.json`, for example `http://host.docker.internal:30001/v1` or your host gateway IP.
