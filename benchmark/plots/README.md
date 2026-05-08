# Benchmark Plots

Python report generator for saved DMAS Forge benchmark runs.

It answers a follow-up question:

> After a benchmark run finishes, what happened across examples, specs, profiles, agents, traces, and resources?

The plot generator reads saved benchmark artifacts from `benchmark/results/`, normalizes them into reusable data files, then writes a static HTML report with charts, topology diagrams, and per-example drilldowns.

## What It Visualizes

- Request success, partial failures, and failed cases
- Latency: per-request points, CDFs, plus p50, p95, and p99 comparisons
- Throughput by example, spec, and profile
- Average agent time and LLM input/output token use by example and protocol
- Docker CPU and memory timelines
- Spans per trace distributions with per-protocol averages, medians, and IQRs
- Longest-trace waterfall charts for each case
- Example/spec topology diagrams with HTTP, MCP, A2A, LLM, tool, and OpenTelemetry edges

## What It Generates

Reports are written under:

```text
benchmark/plots/out/<run-id>/
```

Important files:

```text
index.html                  Main static report
assets/report.css           Report styling
data/normalized.json        All normalized tables in one JSON file
data/*.csv                  CSV exports for cases, requests, spans, traces, etc.
overview/                   Run-level summary charts
performance/                Throughput and latency comparisons
reliability/                Success/error and error taxonomy charts
agents/                     Per-example agent time and token charts
resources/                  CPU and memory charts
spans_per_trace/            Per-example spans-per-trace charts
topology/                   Example/spec topology SVGs
examples/<example>/         Per-example report pages
cases/<case>/               Per-case charts
```

The generator only reads `benchmark/results/`. It does not modify saved benchmark results.

## Quick Start

Run this from `benchmark/plots`:

```bash
uv run python -m benchplots --run 20260429-173215
```

Then open:

```text
benchmark/plots/out/20260429-173215/index.html
```

## Common Commands

```bash
# Generate a report for one saved run.
uv run python -m benchplots --run <run-id>

# Use a custom results directory.
uv run python -m benchplots --run <run-id> --results ../results

# Use a custom output directory.
uv run python -m benchplots --run <run-id> --out out

# Limit per-case longest-trace waterfall generation.
uv run python -m benchplots --run <run-id> --max-case-waterfalls 50
```

The default paths are relative to this folder:

```text
--results ../results
--out out
```

## How It Works

1. Loads `run.json` for the selected run.
2. Finds each case folder under `benchmark/results/<run-id>/`.
3. Reads `summary.json`, `requests.jsonl`, `resources.jsonl`, `spans.jsonl`, and `traces.json`.
4. Normalizes the raw files into tables for cases, requests, resources, spans, traces, errors, components, agent metrics, attribution checks, and topologies.
5. Writes JSON and CSV data exports.
6. Generates Matplotlib and Seaborn charts.
7. Generates NetworkX topology diagrams.
8. Renders a self-contained static HTML report with Jinja2.

Missing raw files are treated as unavailable data for that section. The report still renders the rest of the run.

## Example Views

The main report links to one page per example:

```text
examples/financial-analyzer/index.html
examples/marketing-agency/index.html
examples/travel-planning/index.html
examples/weather/index.html
```

These pages show only charts, topology diagrams, cases, and errors for that example. They are useful when the full run report is too broad.

## Topology Diagrams

Topology diagrams are generated from a small descriptor registry in the plotting code. They show the logical wiring for each example/spec pair:

- `single`: in-process service calls in one app container
- `http`: HTTP between services
- `mcp`: MCP between sub-agents, HTTP on the front/coordinator service
- `a2a`: A2A between sub-agents, HTTP on the front/coordinator service

OpenTelemetry edges are drawn as muted dashed lines, and Jaeger is shown as a lower-emphasis observability sink. LLM and tool edges use separate colors so protocol, model, tool, and telemetry paths are easier to distinguish.

## Technical Notes

This is a Python project managed by `uv`.

Key dependencies:

- `pandas` and `numpy` for normalization
- `matplotlib` and `seaborn` for charts
- `networkx` for topology diagrams
- `jinja2` for static HTML rendering

The project pins Python to 3.12 through `.python-version` and `pyproject.toml`.

Generated output under `out/` can be deleted and recreated at any time:

```bash
rm -rf out/<run-id>
uv run python -m benchplots --run <run-id>
```

## Verification

Useful checks after editing the generator:

```bash
uv run python -m benchplots --run 20260429-173215
uv run python -m compileall -q benchplots
```

For a complete report, check that:

- `index.html` exists
- all expected example pages exist
- `data/normalized.json` exists
- topology SVGs exist for each example/spec pair in the run
- report links point to files under the same `out/<run-id>/` folder
