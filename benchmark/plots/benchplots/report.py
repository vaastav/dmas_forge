from __future__ import annotations

from pathlib import Path
from typing import Any
import os

import pandas as pd
from jinja2 import Environment, select_autoescape

from .data import BenchmarkRun, SPEC_ORDER


def write_report(data: BenchmarkRun, plot_index: dict[str, Any], out_dir: Path) -> None:
    assets = out_dir / "assets"
    assets.mkdir(parents=True, exist_ok=True)
    (assets / "report.css").write_text(REPORT_CSS, encoding="utf-8")
    env = _template_env()
    html = _render(env, data, plot_index)
    (out_dir / "index.html").write_text(html, encoding="utf-8")
    for example in plot_index.get("examples", []):
        page_path = out_dir / example["path"]
        page_path.parent.mkdir(parents=True, exist_ok=True)
        page_path.write_text(_render_example(env, data, plot_index, example, out_dir, page_path.parent), encoding="utf-8")


def _render(env: Environment, data: BenchmarkRun, plot_index: dict[str, Any]) -> str:
    template = env.from_string(REPORT_TEMPLATE)
    cases = data.cases.copy()
    expected = data.expected_cases.copy()
    partial = cases[cases["errors"] > 0] if not cases.empty else pd.DataFrame()
    failed = cases[(cases["requests"] > 0) & (cases["successes"] == 0)] if not cases.empty else pd.DataFrame()
    top_latency = cases.sort_values("p95_ms", ascending=False).head(10) if not cases.empty else pd.DataFrame()
    top_tokens = cases.sort_values("total_tokens", ascending=False).head(10) if not cases.empty else pd.DataFrame()
    return template.render(
        run_id=data.run_id,
        model_name=_model_name(data.run_info),
        run_info=data.run_info,
        plots=plot_index.get("sections", {}),
        case_plots=plot_index.get("cases", []),
        kpis=_run_kpis(cases, expected),
        partial=_sorted_records(partial, ["example", "spec", "profile"]),
        failed=_sorted_records(failed, ["example", "spec", "profile"]),
        top_latency=_records(top_latency),
        top_tokens=_records(top_tokens),
        errors=_error_rows(data.errors),
        profiles=_profile_table(data.run_info, cases),
        intra_spec_comparisons=_records(_spec_comparisons(cases)),
        inter_example_comparisons=_records(_example_spec_comparisons(cases)),
        examples=plot_index.get("examples", []),
        generated_note="Generated from saved benchmark artifacts; benchmark/results was read-only.",
    )


def _render_example(
    env: Environment,
    data: BenchmarkRun,
    plot_index: dict[str, Any],
    example_entry: dict[str, Any],
    out_dir: Path,
    page_dir: Path,
) -> str:
    template = env.from_string(EXAMPLE_TEMPLATE)
    example = example_entry["example"]
    cases = data.cases[data.cases["example"].astype(str) == example].copy() if not data.cases.empty else pd.DataFrame()
    expected = data.expected_cases[data.expected_cases["example"].astype(str) == example].copy() if not data.expected_cases.empty else pd.DataFrame()
    partial = cases[cases["errors"] > 0] if not cases.empty else pd.DataFrame()
    errors = data.errors[data.errors["example"].astype(str) == example] if not data.errors.empty else pd.DataFrame()

    topology = [
        {"title": item["title"], "path": _rel(out_dir / item["path"], page_dir)}
        for item in plot_index.get("sections", {}).get("topology", [])
        if item["title"].startswith(example + " ")
    ]
    case_plots = []
    for case in plot_index.get("cases", []):
        if not str(case["case_name"]).startswith(example + "-"):
            continue
        case_plots.append(
            {
                "case_name": case["case_name"],
                "plots": [_rel(out_dir / plot, page_dir) for plot in case["plots"]],
            }
        )
    example_plots = [
        {"title": item["title"], "path": _rel(out_dir / item["path"], page_dir)}
        for item in example_entry.get("plots", [])
    ]
    return template.render(
        run_id=data.run_id,
        model_name=_model_name(data.run_info),
        example=example,
        root_index=_rel(out_dir / "index.html", page_dir),
        css=_rel(out_dir / "assets" / "report.css", page_dir),
        kpis=_example_kpis(cases, expected),
        plots=example_plots,
        topology=topology,
        case_plots=case_plots,
        cases=_sorted_records(cases, ["spec", "profile"]),
        spec_comparisons=_records(_spec_comparisons(cases)),
        partial=_sorted_records(partial, ["spec", "profile"]),
        errors=_error_rows(errors),
    )


def _template_env() -> Environment:
    env = Environment(autoescape=select_autoescape(["html"]))
    env.globals["format_cell"] = format_cell
    return env


def _records(frame: pd.DataFrame) -> list[dict[str, Any]]:
    if frame.empty:
        return []
    out = frame.copy()
    for col in out.columns:
        if isinstance(out[col].dtype, pd.CategoricalDtype):
            out[col] = out[col].astype(str)
    out = out.where(pd.notnull(out), None)
    return out.to_dict(orient="records")


def _sorted_records(frame: pd.DataFrame, columns: list[str]) -> list[dict[str, Any]]:
    if frame.empty:
        return []
    return _records(frame.sort_values(columns))


def _run_kpis(cases: pd.DataFrame, expected: pd.DataFrame) -> dict[str, int | float]:
    requests = cases["requests"].sum() if not cases.empty else 0
    successes = cases["successes"].sum() if not cases.empty else 0
    return {
        "cases": int(len(cases)),
        "expected": int(len(expected)) if not expected.empty else int(len(cases)),
        "requests": int(requests),
        "successes": int(successes),
        "failures": int(cases["errors"].sum()) if not cases.empty else 0,
        "tokens": int(cases["total_tokens"].sum()) if not cases.empty else 0,
        "max_p95": float(cases["p95_ms"].max()) if not cases.empty else 0.0,
        "avg_success_rate": float(successes / requests) if requests else 0.0,
    }


def _example_kpis(cases: pd.DataFrame, expected: pd.DataFrame) -> dict[str, int | float]:
    requests = cases["requests"].sum() if not cases.empty else 0
    successes = cases["successes"].sum() if not cases.empty else 0
    return {
        "cases": int(len(cases)),
        "comparison_cells": int(len(expected)),
        "requests": int(requests),
        "successes": int(successes),
        "failures": int(cases["errors"].sum()) if not cases.empty else 0,
        "tokens": int(cases["total_tokens"].sum()) if not cases.empty else 0,
        "success_rate": float(successes / requests) if requests else 0.0,
    }


def _error_rows(errors: pd.DataFrame) -> list[dict[str, int | str]]:
    if errors.empty:
        return []
    counts = errors.groupby("error_category", observed=True)["count"].sum().sort_values(ascending=False)
    return [{"category": k, "count": int(v)} for k, v in counts.items()]


def _rel(target: Path, start: Path) -> str:
    return os.path.relpath(target, start).replace(os.sep, "/")


def _profile_table(run_info: dict[str, Any], cases: pd.DataFrame) -> list[dict[str, Any]]:
    cfg = run_info.get("config", {}) if isinstance(run_info.get("config"), dict) else {}
    used = set()
    if not cases.empty:
        used = {(str(row.example), str(row.profile)) for row in cases[["example", "profile"]].drop_duplicates().itertuples(index=False)}
    if not used:
        return []

    global_profiles = {
        str(profile.get("name", "")): profile
        for profile in cfg.get("profiles", [])
        if isinstance(profile, dict)
    }
    rows_by_key: dict[tuple[Any, ...], dict[str, Any]] = {}
    known_examples: set[str] = set()
    for ex in cfg.get("examples", []) if isinstance(cfg.get("examples"), list) else []:
        if not isinstance(ex, dict):
            continue
        example = str(ex.get("name", ""))
        known_examples.add(example)
        raw_example_profiles = ex.get("profiles") if isinstance(ex.get("profiles"), list) else []
        example_profiles = {
            str(profile.get("name", "")): profile
            for profile in raw_example_profiles
            if isinstance(profile, dict)
        }
        for used_example, profile_name in sorted(used):
            if used_example != example:
                continue
            profile = example_profiles.get(profile_name) or global_profiles.get(profile_name) or {"name": profile_name}
            mode = str(profile.get("mode", ""))
            value = profile.get("value", 0)
            scope = example if profile_name in example_profiles else "default"
            _add_profile_row(rows_by_key, cases, example, profile_name, scope, mode, value, profile)
    for example, profile_name in sorted((example, profile) for example, profile in used if example not in known_examples):
        profile = global_profiles.get(profile_name) or {"name": profile_name}
        mode = str(profile.get("mode", ""))
        value = profile.get("value", 0)
        scope = "default" if profile_name in global_profiles else example
        _add_profile_row(rows_by_key, cases, example, profile_name, scope, mode, value, profile)
    return list(rows_by_key.values())


def _add_profile_row(
    rows_by_key: dict[tuple[Any, ...], dict[str, Any]],
    cases: pd.DataFrame,
    example: str,
    profile_name: str,
    scope: str,
    mode: str,
    value: Any,
    profile: dict[str, Any],
) -> None:
    key = (
        scope,
        profile_name,
        mode,
        str(value),
        str(profile.get("concurrency", "")),
        str(profile.get("timeout_seconds", "")),
    )
    row = rows_by_key.setdefault(
        key,
        {
            "scope": scope,
            "name": profile_name,
            "load_type": _load_type(mode),
            "target": _load_target(mode, value),
            "concurrency": profile.get("concurrency", ""),
            "timeout_seconds": profile.get("timeout_seconds", ""),
            "cases": 0,
        },
    )
    row["cases"] += int(
        (
            (cases["example"].astype(str) == example)
            & (cases["profile"].astype(str) == profile_name)
        ).sum()
    )


def _load_type(mode: str) -> str:
    if mode == "timed":
        return "timed duration"
    if mode == "requests":
        return "fixed request count"
    return mode or "unknown"


def _load_target(mode: str, value: Any) -> str:
    if mode == "timed":
        return f"{format_cell(value)} seconds"
    if mode == "requests":
        return f"{format_cell(value)} requests"
    return format_cell(value)


def _spec_comparisons(cases: pd.DataFrame) -> pd.DataFrame:
    if cases.empty:
        return pd.DataFrame()
    rows: list[dict[str, Any]] = []
    for (example, profile), group in cases.groupby(["example", "profile"], observed=True, sort=True):
        if group["spec"].nunique() < 2:
            continue
        fastest_p95 = float(group["p95_ms"].min())
        best_throughput = float(group["throughput_rps"].max())
        for row in _sort_by_spec(group).itertuples(index=False):
            p95 = float(row.p95_ms)
            throughput = float(row.throughput_rps)
            rows.append(
                {
                    "example": str(example),
                    "profile": str(profile),
                    "spec": str(row.spec),
                    "requests": int(row.requests),
                    "success_rate": float(row.success_rate),
                    "p95_ms": p95,
                    "p95_vs_fastest_ms": p95 - fastest_p95,
                    "throughput_rps": throughput,
                    "throughput_vs_best_pct": ((throughput / best_throughput) - 1.0) * 100 if best_throughput else 0.0,
                }
            )
    return pd.DataFrame(rows)


def _example_spec_comparisons(cases: pd.DataFrame) -> pd.DataFrame:
    if cases.empty:
        return pd.DataFrame()
    rows: list[dict[str, Any]] = []
    for (spec, profile), group in cases.groupby(["spec", "profile"], observed=True, sort=True):
        if group["example"].nunique() < 2:
            continue
        fastest_p95 = float(group["p95_ms"].min())
        for row in group.sort_values(["example"]).itertuples(index=False):
            rows.append(
                {
                    "spec": str(spec),
                    "profile": str(profile),
                    "example": str(row.example),
                    "requests": int(row.requests),
                    "success_rate": float(row.success_rate),
                    "p95_ms": float(row.p95_ms),
                    "p95_vs_fastest_ms": float(row.p95_ms) - fastest_p95,
                    "throughput_rps": float(row.throughput_rps),
                }
            )
    return pd.DataFrame(rows)


def _sort_by_spec(frame: pd.DataFrame) -> pd.DataFrame:
    order = {spec: index for index, spec in enumerate(SPEC_ORDER)}
    return (
        frame.assign(_spec_order=frame["spec"].astype(str).map(lambda value: order.get(value, len(order))))
        .sort_values(["_spec_order", "spec"])
        .drop(columns=["_spec_order"])
    )


def _model_name(run_info: dict[str, Any]) -> str:
    model = run_info.get("model")
    if isinstance(model, dict):
        name = model.get("name")
        if name:
            return str(name)
    if isinstance(model, str) and model:
        return model
    return "unknown"


REPORT_TEMPLATE = """{% macro gallery(items) -%}
  {% if items %}
  <div class="gallery">
    {% for item in items %}
    <figure>
      <a href="{{ item.path }}"><img src="{{ item.path }}" alt="{{ item.title }}"></a>
      <figcaption>{{ item.title }}</figcaption>
    </figure>
    {% endfor %}
  </div>
  {% else %}
  <p class="muted">No plots available for this section.</p>
  {% endif %}
{%- endmacro %}

{% macro table(rows, columns) -%}
  {% if rows %}
  <div class="table-wrap">
    <table>
      <thead><tr>{% for col in columns %}<th>{{ col }}</th>{% endfor %}</tr></thead>
      <tbody>
        {% for row in rows %}
        <tr>
          {% for col in columns %}
          <td>{{ format_cell(row.get(col)) }}</td>
          {% endfor %}
        </tr>
        {% endfor %}
      </tbody>
    </table>
  </div>
  {% else %}
  <p class="muted">No rows.</p>
  {% endif %}
{%- endmacro %}
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Benchmark Plots {{ run_id }}</title>
  <link rel="stylesheet" href="assets/report.css">
</head>
<body>
  <header class="page-header">
    <div>
      <p class="eyebrow">DMAS Forge Benchmark Report</p>
      <h1>Run {{ run_id }}</h1>
      <p class="model-line">Model: {{ model_name }}</p>
      <p class="subtitle">{{ generated_note }}</p>
    </div>
    <nav>
      <a href="#overview">Overview</a>
      <a href="#topology">Topology</a>
      <a href="#performance">Performance</a>
      <a href="#spec-comparisons">Spec Comparisons</a>
      <a href="#reliability">Reliability</a>
      <a href="#tokens">Tokens</a>
      <a href="#resources">Resources</a>
      <a href="#traces">Traces</a>
      <a href="#cases">Cases</a>
    </nav>
  </header>

  <main>
    <section id="overview" class="section">
      <h2>Overview</h2>
      <div class="kpi-grid">
        <div><span>{{ kpis.cases }}</span><label>present cases</label></div>
        <div><span>{{ kpis.expected }}</span><label>comparison cells</label></div>
        <div><span>{{ kpis.requests }}</span><label>requests</label></div>
        <div><span>{{ kpis.successes }}</span><label>successes</label></div>
        <div><span>{{ kpis.failures }}</span><label>failures</label></div>
        <div><span>{{ "%.1f%%"|format(kpis.avg_success_rate * 100) }}</span><label>success rate</label></div>
        <div><span>{{ "{:,}".format(kpis.tokens) }}</span><label>tokens</label></div>
        <div><span>{{ "%.0f ms"|format(kpis.max_p95) }}</span><label>max p95</label></div>
      </div>
      {{ gallery(plots.get("overview", [])) }}

      <div class="table-pair">
        <div>
          <h3>Profiles</h3>
          <p class="muted">Fixed request count profiles stop after the target request count; timed duration profiles keep issuing requests until the target duration elapses.</p>
          {{ table(profiles, ["scope", "name", "load_type", "target", "concurrency", "timeout_seconds", "cases"]) }}
        </div>
        <div>
          <h3>Error Categories</h3>
          {{ table(errors, ["category", "count"]) }}
        </div>
      </div>

      <h3>Example Views</h3>
      <div class="example-links">
        {% for example in examples %}
        <a href="{{ example.path }}">{{ example.example }}</a>
        {% endfor %}
      </div>
    </section>

    <section id="topology" class="section">
      <h2>Topology</h2>
      {{ gallery(plots.get("topology", [])) }}
    </section>

    <section id="performance" class="section">
      <h2>Performance</h2>
      {{ gallery(plots.get("performance", [])) }}
      <h3>Highest p95 Latency</h3>
      {{ table(top_latency, ["case_name", "successes", "errors", "throughput_rps", "p50_ms", "p95_ms", "p99_ms"]) }}
    </section>

    <section id="spec-comparisons" class="section">
      <h2>Spec Comparisons</h2>
      <p class="subtitle">Intra-example rows compare specs within the same example and profile. Inter-example rows compare examples under the same spec and profile.</p>
      <h3>Intra-Example</h3>
      {{ table(intra_spec_comparisons, ["example", "profile", "spec", "requests", "success_rate", "p95_ms", "p95_vs_fastest_ms", "throughput_rps", "throughput_vs_best_pct"]) }}
      <h3>Inter-Example</h3>
      {{ table(inter_example_comparisons, ["spec", "profile", "example", "requests", "success_rate", "p95_ms", "p95_vs_fastest_ms", "throughput_rps"]) }}
    </section>

    <section id="reliability" class="section">
      <h2>Reliability</h2>
      {{ gallery(plots.get("reliability", [])) }}
      <h3>Failed Cases</h3>
      {{ table(failed, ["case_name", "requests", "successes", "errors", "p95_ms"]) }}
      <h3>Partial Cases</h3>
      {{ table(partial, ["case_name", "requests", "successes", "errors", "p95_ms"]) }}
    </section>

    <section id="tokens" class="section">
      <h2>Tokens and Components</h2>
      {{ gallery(plots.get("tokens", [])) }}
      <h3>Highest Token Cases</h3>
      {{ table(top_tokens, ["case_name", "successes", "errors", "input_tokens", "output_tokens", "total_tokens", "tokens_per_success"]) }}
    </section>

    <section id="resources" class="section">
      <h2>Resources</h2>
      {{ gallery(plots.get("resources", [])) }}
    </section>

    <section id="traces" class="section">
      <h2>Traces</h2>
      {{ gallery(plots.get("traces", [])) }}
    </section>

    <section id="cases" class="section">
      <h2>Per-Case Detail</h2>
      <p class="subtitle">Each case includes request latency, resource timelines, component duration, and a longest-trace waterfall when trace spans were available.</p>
      <div class="case-grid">
        {% for case in case_plots %}
        <article class="case-card">
          <h3>{{ case.case_name }}</h3>
          {% for plot in case.plots %}
          <a href="{{ plot }}"><img src="{{ plot }}" alt="{{ case.case_name }} plot"></a>
          {% endfor %}
        </article>
        {% endfor %}
      </div>
    </section>

    <section class="section">
      <h2>Data Exports</h2>
      <div class="links">
        <a href="data/normalized.json">normalized.json</a>
        <a href="data/cases.csv">cases.csv</a>
        <a href="data/requests.csv">requests.csv</a>
        <a href="data/resources.csv">resources.csv</a>
        <a href="data/spans.csv">spans.csv</a>
        <a href="data/traces.csv">traces.csv</a>
      </div>
    </section>
  </main>
</body>
</html>
"""


EXAMPLE_TEMPLATE = """{% macro gallery(items) -%}
  {% if items %}
  <div class="gallery">
    {% for item in items %}
    <figure>
      <a href="{{ item.path }}"><img src="{{ item.path }}" alt="{{ item.title }}"></a>
      <figcaption>{{ item.title }}</figcaption>
    </figure>
    {% endfor %}
  </div>
  {% else %}
  <p class="muted">No plots available.</p>
  {% endif %}
{%- endmacro %}

{% macro table(rows, columns) -%}
  {% if rows %}
  <div class="table-wrap">
    <table>
      <thead><tr>{% for col in columns %}<th>{{ col }}</th>{% endfor %}</tr></thead>
      <tbody>
        {% for row in rows %}
        <tr>{% for col in columns %}<td>{{ format_cell(row.get(col)) }}</td>{% endfor %}</tr>
        {% endfor %}
      </tbody>
    </table>
  </div>
  {% else %}
  <p class="muted">No rows.</p>
  {% endif %}
{%- endmacro %}
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ example }} - {{ run_id }}</title>
  <link rel="stylesheet" href="{{ css }}">
</head>
<body>
  <header class="page-header">
    <div>
      <p class="eyebrow">Example View</p>
      <h1>{{ example }}</h1>
      <p class="model-line">Model: {{ model_name }}</p>
      <p class="subtitle">Run {{ run_id }}</p>
    </div>
    <nav><a href="{{ root_index }}">Full report</a></nav>
  </header>
  <main>
    <section class="section">
      <h2>Example Summary</h2>
      <div class="kpi-grid">
        <div><span>{{ kpis.cases }}</span><label>present cases</label></div>
        <div><span>{{ kpis.comparison_cells }}</span><label>comparison cells</label></div>
        <div><span>{{ kpis.requests }}</span><label>requests</label></div>
        <div><span>{{ kpis.successes }}</span><label>successes</label></div>
        <div><span>{{ kpis.failures }}</span><label>failures</label></div>
        <div><span>{{ "%.1f%%"|format(kpis.success_rate * 100) }}</span><label>success rate</label></div>
        <div><span>{{ "{:,}".format(kpis.tokens) }}</span><label>tokens</label></div>
      </div>
      {{ gallery(plots) }}
      <h3>Error Categories</h3>
      {{ table(errors, ["category", "count"]) }}
    </section>
    <section class="section">
      <h2>Topology</h2>
      {{ gallery(topology) }}
    </section>
    <section class="section">
      <h2>Cases</h2>
      <h3>Spec Comparison</h3>
      {{ table(spec_comparisons, ["profile", "spec", "requests", "success_rate", "p95_ms", "p95_vs_fastest_ms", "throughput_rps", "throughput_vs_best_pct"]) }}
      {{ table(cases, ["case_name", "requests", "successes", "errors", "throughput_rps", "p50_ms", "p95_ms", "p99_ms", "total_tokens"]) }}
      <h3>Partial or Failed Cases</h3>
      {{ table(partial, ["case_name", "requests", "successes", "errors", "p95_ms"]) }}
      <div class="case-grid">
        {% for case in case_plots %}
        <article class="case-card">
          <h3>{{ case.case_name }}</h3>
          {% for plot in case.plots %}
          <a href="{{ plot }}"><img src="{{ plot }}" alt="{{ case.case_name }} plot"></a>
          {% endfor %}
        </article>
        {% endfor %}
      </div>
    </section>
  </main>
</body>
</html>
"""


def format_cell(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, float):
        if abs(value) >= 1000:
            return f"{value:,.0f}"
        if abs(value) >= 10:
            return f"{value:,.1f}"
        return f"{value:,.3f}".rstrip("0").rstrip(".")
    return str(value)


REPORT_CSS = """
:root {
  color-scheme: light;
  --bg: #f6f8fb;
  --panel: #ffffff;
  --ink: #162033;
  --muted: #5c6b82;
  --line: #d9e1ee;
  --accent: #265f73;
  --accent-2: #6a4c93;
}

* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font: 14px/1.5 Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

.page-header {
  background: #0f172a;
  color: white;
  padding: 34px clamp(20px, 5vw, 72px);
  display: flex;
  justify-content: space-between;
  gap: 28px;
  align-items: flex-end;
  border-bottom: 5px solid #2c7a7b;
}
.page-header h1 {
  margin: 2px 0 8px;
  font-size: clamp(30px, 5vw, 54px);
  letter-spacing: 0;
}
.eyebrow {
  margin: 0;
  text-transform: uppercase;
  letter-spacing: .08em;
  font-size: 12px;
  color: #9ae6b4;
  font-weight: 700;
}
.subtitle { color: var(--muted); margin-top: 4px; }
.page-header .subtitle { color: #cbd5e1; }
.model-line {
  margin: 0 0 6px;
  color: #e2e8f0;
  font-weight: 700;
}
nav {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  max-width: 560px;
  justify-content: flex-end;
}
nav a, .links a {
  color: white;
  text-decoration: none;
  padding: 7px 10px;
  border: 1px solid rgba(255,255,255,.25);
  border-radius: 6px;
}

main {
  width: min(1500px, calc(100vw - 32px));
  margin: 28px auto 64px;
}
.section {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: clamp(18px, 3vw, 32px);
  margin: 22px 0;
  box-shadow: 0 12px 30px rgba(15, 23, 42, .05);
}
h2 {
  margin: 0 0 18px;
  font-size: 26px;
  letter-spacing: 0;
}
h3 {
  margin: 24px 0 10px;
  font-size: 17px;
}
.kpi-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin-bottom: 24px;
}
.kpi-grid div {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 14px;
  background: #fbfdff;
}
.kpi-grid span {
  display: block;
  font-size: 24px;
  font-weight: 800;
  color: var(--accent);
}
.kpi-grid label {
  display: block;
  color: var(--muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: .06em;
  margin-top: 2px;
}
.gallery {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
  gap: 18px;
  align-items: start;
}
figure {
  margin: 0;
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
  background: white;
}
figure img {
  display: block;
  width: 100%;
  height: auto;
}
figcaption {
  padding: 10px 12px;
  color: var(--muted);
  border-top: 1px solid var(--line);
  background: #fbfdff;
}
.table-pair {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 22px;
}
.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line);
  border-radius: 8px;
}
table {
  width: 100%;
  border-collapse: collapse;
  min-width: 520px;
}
th, td {
  text-align: left;
  padding: 8px 10px;
  border-bottom: 1px solid #edf2f7;
  vertical-align: top;
  white-space: nowrap;
}
th {
  background: #f1f5f9;
  color: #334155;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: .05em;
}
.case-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 18px;
}
.case-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 14px;
  background: #fbfdff;
}
.case-card h3 {
  margin-top: 0;
  font-size: 14px;
  word-break: break-word;
}
.case-card img {
  width: 100%;
  margin: 8px 0;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: white;
}
.links {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.example-links {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-top: 10px;
}
.example-links a {
  display: block;
  padding: 12px 14px;
  color: var(--accent);
  text-decoration: none;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fbfdff;
  font-weight: 700;
}
.links a {
  color: var(--accent);
  border-color: var(--line);
  background: #fbfdff;
}
.muted { color: var(--muted); }

@media (max-width: 760px) {
  .page-header { display: block; }
  nav { justify-content: flex-start; margin-top: 18px; }
  .gallery { grid-template-columns: 1fr; }
  main { width: min(100vw - 20px, 1500px); }
}
"""
