# Benchmark

Simple Go benchmark runner for the examples.

Run from this directory:

```bash
go run main.go -h
go run main.go list
go run main.go build -examples weather,chat -specs single,memory -rebuild
go run main.go run -examples weather -specs single -profiles sequential
go run main.go summary -run <run-id>
go run main.go jaeger -run <run-id> -case weather-single-sequential
```

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
