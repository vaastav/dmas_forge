from __future__ import annotations

from pathlib import Path
from typing import Any
import os

import pandas as pd
from jinja2 import Environment, select_autoescape

from .data import BenchmarkRun, SPEC_ORDER
from .labels import example_case_label, example_label, example_sort_key


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
    cases = _with_iqr(data.cases, data.requests)
    expected = data.expected_cases.copy()
    partial = cases[cases["errors"] > 0] if not cases.empty else pd.DataFrame()
    failed = cases[(cases["requests"] > 0) & (cases["successes"] == 0)] if not cases.empty else pd.DataFrame()
    top_latency = cases.sort_values("p95_ms", ascending=False).head(10) if not cases.empty else pd.DataFrame()
    top_latency_records = _case_records(_sort_cases(top_latency, ["example", "spec", "profile"]))
    intra_spec_records = _records(_spec_comparisons(cases))
    inter_example_records = _records(_example_spec_comparisons(cases))
    examples = [
        {**example, "example_label": example.get("example_label", example_label(example.get("example", "")))}
        for example in plot_index.get("examples", [])
    ]
    return template.render(
        run_id=data.run_id,
        model_name=_model_name(data.run_info),
        run_info=data.run_info,
        plots=plot_index.get("sections", {}),
        case_plots=plot_index.get("cases", []),
        kpis=_run_kpis(cases, expected),
        partial=_sorted_case_records(partial, ["example", "spec", "profile"]),
        failed=_sorted_case_records(failed, ["example", "spec", "profile"]),
        top_latency=top_latency_records,
        top_latency_groups=_group_records(top_latency_records, "example"),
        agent_check_note=_agent_check_note(data.agent_checks),
        errors=_error_rows(data.errors),
        profiles=_profile_table(data.run_info, cases),
        intra_spec_comparisons=intra_spec_records,
        intra_spec_groups=_group_records(intra_spec_records, "example"),
        inter_example_comparisons=inter_example_records,
        inter_example_groups=_group_records(inter_example_records, "spec"),
        examples=examples,
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
    all_cases = _with_iqr(data.cases, data.requests)
    cases = all_cases[all_cases["example"].astype(str) == example].copy() if not all_cases.empty else pd.DataFrame()
    expected = data.expected_cases[data.expected_cases["example"].astype(str) == example].copy() if not data.expected_cases.empty else pd.DataFrame()
    partial = cases[cases["errors"] > 0] if not cases.empty else pd.DataFrame()
    errors = data.errors[data.errors["example"].astype(str) == example] if not data.errors.empty else pd.DataFrame()

    topology = [
        {"title": item["title"], "path": _rel(out_dir / item["path"], page_dir)}
        for item in plot_index.get("sections", {}).get("topology", [])
        if str(item["path"]).startswith(f"topology/{example}_")
    ]
    case_plots = []
    for case in plot_index.get("cases", []):
        if not str(case["case_name"]).startswith(example + "-"):
            continue
        case_plots.append(
            {
                "case_name": case["case_name"],
                "case_label": case.get("case_label", example_case_label(case["case_name"])),
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
        example_label=example_entry.get("example_label", example_label(example)),
        root_index=_rel(out_dir / "index.html", page_dir),
        css=_rel(out_dir / "assets" / "report.css", page_dir),
        kpis=_example_kpis(cases, expected),
        plots=example_plots,
        topology=topology,
        case_plots=case_plots,
        cases=_sorted_case_records(cases, ["spec", "profile"]),
        spec_comparisons=_records(_spec_comparisons(cases)),
        partial=_sorted_case_records(partial, ["spec", "profile"]),
        errors=_error_rows(errors),
    )


def _template_env() -> Environment:
    env = Environment(autoescape=select_autoescape(["html"]))
    env.globals["format_cell"] = format_cell
    env.globals["format_table_cell"] = format_table_cell
    env.globals["column_label"] = column_label
    env.globals["cell_class"] = cell_class
    return env


def _records(frame: pd.DataFrame) -> list[dict[str, Any]]:
    if frame.empty:
        return []
    out = frame.copy()
    out = _with_table_fields(out)
    for col in out.columns:
        if isinstance(out[col].dtype, pd.CategoricalDtype):
            out[col] = out[col].astype(str)
    out = out.where(pd.notnull(out), None)
    return out.to_dict(orient="records")


def _sorted_records(frame: pd.DataFrame, columns: list[str]) -> list[dict[str, Any]]:
    if frame.empty:
        return []
    return _records(frame.sort_values(columns))


def _group_records(rows: list[dict[str, Any]], key: str) -> list[dict[str, Any]]:
    groups: list[dict[str, Any]] = []
    by_title: dict[str, list[dict[str, Any]]] = {}
    for row in rows:
        raw_title = str(row.get(key) or "Unspecified")
        title = SPEC_LABELS.get(raw_title, raw_title) if key == "spec" else raw_title
        if title not in by_title:
            by_title[title] = []
            groups.append({"title": title, "rows": by_title[title]})
        by_title[title].append(row)
    return groups


def _case_records(frame: pd.DataFrame) -> list[dict[str, Any]]:
    return _records(_with_display_labels(frame))


def _sorted_case_records(frame: pd.DataFrame, columns: list[str]) -> list[dict[str, Any]]:
    if frame.empty:
        return []
    return _case_records(_sort_cases(frame, columns))


def _with_display_labels(frame: pd.DataFrame) -> pd.DataFrame:
    if frame.empty:
        return frame
    out = frame.copy()
    if "example" in out:
        out["example"] = out["example"].astype(str).map(example_label)
    if "case_name" in out:
        out["case_name"] = out["case_name"].astype(str).map(example_case_label)
    return out


def _with_table_fields(frame: pd.DataFrame) -> pd.DataFrame:
    out = frame.copy()
    if "spec" in out:
        out["protocol"] = out["spec"].astype(str).map(lambda value: SPEC_LABELS.get(value, value))
    if {"successes", "requests"}.issubset(out.columns):
        out["request_summary"] = [
            _request_summary(successes, requests)
            for successes, requests in zip(out["successes"], out["requests"], strict=False)
        ]
    return out


def _with_iqr(cases: pd.DataFrame, requests: pd.DataFrame) -> pd.DataFrame:
    if cases.empty:
        return cases.copy()
    out = cases.copy()
    out["iqr_ms"] = 0.0
    if requests.empty or not {"case_name", "ok", "latency_ms"}.issubset(requests.columns):
        return out
    successful = requests[requests["ok"].fillna(False)].copy()
    if successful.empty:
        return out
    latency = successful["latency_ms"].dropna()
    if latency.empty:
        return out
    iqr = successful.groupby("case_name", observed=True)["latency_ms"].quantile(0.75).sub(
        successful.groupby("case_name", observed=True)["latency_ms"].quantile(0.25),
        fill_value=0.0,
    )
    out["iqr_ms"] = out["case_name"].map(iqr).fillna(0.0).astype(float)
    return out


def _request_summary(successes: Any, requests: Any) -> str:
    return f"{_count(successes)} / {_count(requests)}"


def _count(value: Any) -> str:
    try:
        return f"{float(value):,.0f}"
    except (TypeError, ValueError):
        return "0"


def _sort_cases(frame: pd.DataFrame, columns: list[str]) -> pd.DataFrame:
    out = frame.copy()
    if "example" in out:
        out["_example_order"] = out["example"].astype(str).map(lambda value: example_sort_key(value)[0])
    if "spec" in out:
        order = {spec: index for index, spec in enumerate(SPEC_ORDER)}
        out["_spec_order"] = out["spec"].astype(str).map(lambda value: order.get(value, len(order)))
    sort_cols = ["_example_order" if col == "example" and "_example_order" in out else col for col in columns]
    sort_cols = ["_spec_order" if col == "spec" and "_spec_order" in out else col for col in sort_cols]
    return out.sort_values(sort_cols).drop(columns=["_example_order", "_spec_order"], errors="ignore")


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


def _agent_check_note(agent_checks: pd.DataFrame) -> str:
    if agent_checks.empty or "attribution_ok" not in agent_checks:
        return "Token check: no agent token check was available."
    failed = agent_checks[~agent_checks["attribution_ok"].fillna(False)]
    if failed.empty:
        return "Token check: all plotted agent tokens match the case totals."
    count = len(failed)
    return f"Token check: {count} case{'s' if count != 1 else ''} need review; see data/agent_checks.csv."


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
            scope = example_label(example) if profile_name in example_profiles else "default"
            _add_profile_row(rows_by_key, cases, example, profile_name, scope, mode, value, profile)
    for example, profile_name in sorted((example, profile) for example, profile in used if example not in known_examples):
        profile = global_profiles.get(profile_name) or {"name": profile_name}
        mode = str(profile.get("mode", ""))
        value = profile.get("value", 0)
        scope = "default" if profile_name in global_profiles else example_label(example)
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
    sorted_cases = _sort_cases(cases, ["example", "profile"])
    for (example, profile), group in sorted_cases.groupby(["example", "profile"], observed=True, sort=False):
        if group["spec"].nunique() < 2:
            continue
        for row in _sort_by_spec(group).itertuples(index=False):
            p95 = float(row.p95_ms)
            throughput = float(row.throughput_rps)
            rows.append(
                {
                    "example": example_label(example),
                    "profile": str(profile),
                    "spec": str(row.spec),
                    "requests": int(row.requests),
                    "successes": int(row.successes),
                    "success_rate": float(row.success_rate),
                    "p50_ms": float(row.p50_ms),
                    "p95_ms": p95,
                    "iqr_ms": float(getattr(row, "iqr_ms", 0.0)),
                    "throughput_rps": throughput,
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
        sorted_group = _sort_cases(group, ["example"])
        for row in sorted_group.itertuples(index=False):
            rows.append(
                {
                    "spec": str(spec),
                    "profile": str(profile),
                    "example": example_label(row.example),
                    "requests": int(row.requests),
                    "successes": int(row.successes),
                    "success_rate": float(row.success_rate),
                    "p50_ms": float(row.p50_ms),
                    "p95_ms": float(row.p95_ms),
                    "iqr_ms": float(getattr(row, "iqr_ms", 0.0)),
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
      <thead><tr>{% for col in columns %}<th>{{ column_label(col) }}</th>{% endfor %}</tr></thead>
      <tbody>
        {% for row in rows %}
        <tr>
          {% for col in columns %}
          <td class="{{ cell_class(col) }}">{{ format_table_cell(col, row.get(col), row) }}</td>
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

{% macro grouped_tables(groups, columns) -%}
  {% if groups %}
    {% for group in groups %}
    <div class="table-group">
      <h4>{{ group.title }}</h4>
      {{ table(group.rows, columns) }}
    </div>
    {% endfor %}
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
      <a href="#agents">Agent Metrics</a>
      <a href="#resources">Resources</a>
      <a href="#spans-per-trace">Spans per Trace</a>
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
        <a href="{{ example.path }}">{{ example.example_label }}</a>
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
      <p class="table-note muted">Requests are successful / total completed requests. Completed Req/min is completed request attempts per elapsed minute. E2E latency and IQR are computed over successful requests from request dispatch to final response.</p>
      {{ grouped_tables(top_latency_groups, ["protocol", "request_summary", "success_rate", "throughput_rps", "p50_ms", "p95_ms", "p99_ms", "iqr_ms"]) }}
    </section>

    <section id="spec-comparisons" class="section">
      <h2>Spec Comparisons</h2>
      <p class="subtitle">Intra-example rows compare specs within the same example and profile. Inter-example rows compare examples under the same spec and profile.</p>
      <p class="table-note muted">Requests are successful / total completed requests. Completed Req/min is completed request attempts per elapsed minute. E2E IQR is p75 - p25 over successful end-to-end request latencies.</p>
      <h3>Intra-Example</h3>
      {{ grouped_tables(intra_spec_groups, ["profile", "spec", "request_summary", "success_rate", "throughput_rps", "p50_ms", "p95_ms", "iqr_ms"]) }}
      <h3>Inter-Example</h3>
      {{ grouped_tables(inter_example_groups, ["profile", "example", "request_summary", "success_rate", "throughput_rps", "p50_ms", "p95_ms", "iqr_ms"]) }}
    </section>

    <section id="reliability" class="section">
      <h2>Reliability</h2>
      {{ gallery(plots.get("reliability", [])) }}
      <h3>Failed Cases</h3>
      {{ table(failed, ["case_name", "request_summary", "success_rate", "errors", "p95_ms"]) }}
      <h3>Partial Cases</h3>
      {{ table(partial, ["case_name", "request_summary", "success_rate", "errors", "p95_ms"]) }}
    </section>

    <section id="agents" class="section">
      <h2>Agent Metrics</h2>
      <p class="subtitle">Per-example charts show average agent seconds per request and average LLM token use per successful request.</p>
      {{ gallery(plots.get("agents", [])) }}
      <p class="footnote muted">{{ agent_check_note }}</p>
    </section>

    <section id="resources" class="section">
      <h2>Resources</h2>
      {{ gallery(plots.get("resources", [])) }}
    </section>

    <section id="spans-per-trace" class="section">
      <h2>Spans per Trace</h2>
      <p class="subtitle">Each example gets separate span-count distribution and protocol summary charts so protocol variance is visible without mixing unrelated workflows.</p>
      {{ gallery(plots.get("spans_per_trace", [])) }}
    </section>

    <section id="cases" class="section">
      <h2>Per-Case Detail</h2>
      <p class="subtitle">Each case includes request latency, resource timelines, and a longest-trace waterfall when trace spans were available.</p>
      <div class="case-grid">
        {% for case in case_plots %}
        <article class="case-card">
          <h3>{{ case.case_label }}</h3>
          {% for plot in case.plots %}
          <a href="{{ plot }}"><img src="{{ plot }}" alt="{{ case.case_label }} plot"></a>
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
        <a href="data/agent_metrics.csv">agent_metrics.csv</a>
        <a href="data/agent_checks.csv">agent_checks.csv</a>
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
      <thead><tr>{% for col in columns %}<th>{{ column_label(col) }}</th>{% endfor %}</tr></thead>
      <tbody>
        {% for row in rows %}
        <tr>{% for col in columns %}<td class="{{ cell_class(col) }}">{{ format_table_cell(col, row.get(col), row) }}</td>{% endfor %}</tr>
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
  <title>{{ example_label }} - {{ run_id }}</title>
  <link rel="stylesheet" href="{{ css }}">
</head>
<body>
  <header class="page-header">
    <div>
      <p class="eyebrow">Example View</p>
      <h1>{{ example_label }}</h1>
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
      <p class="table-note muted">Requests are successful / total completed requests. Completed Req/min is completed request attempts per elapsed minute. E2E latency and IQR are computed over successful requests from request dispatch to final response.</p>
      {{ table(spec_comparisons, ["profile", "spec", "request_summary", "success_rate", "throughput_rps", "p50_ms", "p95_ms", "iqr_ms"]) }}
      {{ table(cases, ["case_name", "request_summary", "success_rate", "throughput_rps", "p50_ms", "p95_ms", "p99_ms", "iqr_ms", "total_tokens"]) }}
      <h3>Partial or Failed Cases</h3>
      {{ table(partial, ["case_name", "request_summary", "success_rate", "errors", "p95_ms"]) }}
      <div class="case-grid">
        {% for case in case_plots %}
        <article class="case-card">
          <h3>{{ case.case_label }}</h3>
          {% for plot in case.plots %}
          <a href="{{ plot }}"><img src="{{ plot }}" alt="{{ case.case_label }} plot"></a>
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


COLUMN_LABELS = {
    "case_name": "Case",
    "category": "Error category",
    "count": "Requests",
    "example": "Example",
    "profile": "Profile",
    "protocol": "Protocol",
    "spec": "Spec",
    "scope": "Scope",
    "name": "Profile",
    "load_type": "Load",
    "target": "Target",
    "concurrency": "Concurrency",
    "timeout_seconds": "Timeout (s)",
    "cases": "Cases",
    "request_summary": "Requests",
    "requests": "Requests",
    "successes": "Successful",
    "errors": "Errors",
    "success_rate": "Success",
    "throughput_rps": "Completed Req/min",
    "p50_ms": "E2E p50",
    "p95_ms": "E2E p95",
    "p99_ms": "E2E p99",
    "iqr_ms": "E2E IQR",
    "total_tokens": "Tokens",
}

SPEC_LABELS = {
    "single": "Single",
    "http": "HTTP",
    "mcp": "MCP",
    "a2a": "A2A",
}

NUMERIC_COLUMNS = {
    "count",
    "concurrency",
    "timeout_seconds",
    "cases",
    "request_summary",
    "requests",
    "successes",
    "errors",
    "success_rate",
    "throughput_rps",
    "p50_ms",
    "p95_ms",
    "p99_ms",
    "iqr_ms",
    "total_tokens",
}


def column_label(column: str) -> str:
    return COLUMN_LABELS.get(column, column.replace("_", " ").title())


def cell_class(column: str) -> str:
    classes = []
    if column in NUMERIC_COLUMNS:
        classes.append("num")
    if column == "case_name":
        classes.append("case-name")
    return " ".join(classes)


def format_table_cell(column: str, value: Any, row: dict[str, Any] | None = None) -> str:
    if column == "request_summary" and not value and row:
        return _request_summary(row.get("successes"), row.get("requests"))
    if column == "success_rate":
        return _format_percent(value)
    if column in {"p50_ms", "p95_ms", "p99_ms", "iqr_ms"}:
        return _format_latency(value)
    if column == "throughput_rps":
        return _format_rate(value)
    if column == "total_tokens":
        return _format_compact_count(value)
    if column in {"count", "concurrency", "timeout_seconds", "cases", "requests", "successes", "errors"}:
        return _count(value)
    if column == "spec":
        return SPEC_LABELS.get(str(value), format_cell(value))
    return format_cell(value)


def _format_percent(value: Any) -> str:
    number = _as_float(value)
    if number is None:
        return ""
    return f"{number * 100:.1f}%"


def _format_rate(value: Any) -> str:
    number = _as_float(value)
    if number is None:
        return ""
    number *= 60.0
    if number == 0:
        return "0 req/min"
    if abs(number) < 0.1:
        return "<0.1 req/min"
    if abs(number) < 1:
        formatted = f"{number:.3f}".rstrip("0").rstrip(".")
        return f"{formatted} req/min"
    return f"{number:,.2f} req/min"


def _format_latency(value: Any) -> str:
    number = _as_float(value)
    if number is None:
        return ""
    return _format_latency_number(number)


def _format_latency_number(ms: float) -> str:
    seconds = ms / 1000.0
    return f"{seconds:,.1f} s"


def _format_compact_count(value: Any) -> str:
    number = _as_float(value)
    if number is None:
        return ""
    if abs(number) >= 1_000_000:
        return f"{number / 1_000_000:.1f}M"
    if abs(number) >= 10_000:
        return f"{number / 1_000:.1f}k"
    return f"{number:,.0f}"


def _as_float(value: Any) -> float | None:
    if value is None:
        return None
    try:
        if pd.isna(value):
            return None
        return float(value)
    except (TypeError, ValueError):
        return None


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
h4 {
  margin: 16px 0 8px;
  color: #334155;
  font-size: 14px;
  letter-spacing: .01em;
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
  margin: 8px 0 18px;
}
table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  min-width: 520px;
}
th, td {
  text-align: left;
  padding: 10px 12px;
  border-bottom: 1px solid #edf2f7;
  vertical-align: middle;
  white-space: nowrap;
}
th {
  background: #f1f5f9;
  color: #334155;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: .05em;
  position: sticky;
  top: 0;
  z-index: 1;
}
tbody tr:nth-child(even) { background: #fbfdff; }
tbody tr:hover { background: #f8fafc; }
tbody tr:last-child td { border-bottom: 0; }
td.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  font-feature-settings: "tnum";
}
td.case-name {
  min-width: 220px;
  white-space: normal;
  font-weight: 650;
  color: #1e293b;
}
.table-note {
  max-width: 920px;
  margin: -2px 0 10px;
  font-size: 13px;
}
.table-group + .table-group { margin-top: 18px; }
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
.footnote {
  border-top: 1px solid var(--line);
  font-size: 12px;
  margin: 12px 0 0;
  padding-top: 10px;
}
.muted { color: var(--muted); }

@media (max-width: 760px) {
  .page-header { display: block; }
  nav { justify-content: flex-start; margin-top: 18px; }
  .gallery { grid-template-columns: 1fr; }
  main { width: min(100vw - 20px, 1500px); }
}
"""
