from __future__ import annotations

from pathlib import Path
from typing import Any
import os
import shutil

import pandas as pd
from jinja2 import Environment, FileSystemLoader, select_autoescape

from .data import BenchmarkRun, SPEC_ORDER
from .labels import example_case_label, example_label, example_sort_key


def write_report(data: BenchmarkRun, plot_index: dict[str, Any], out_dir: Path) -> None:
    assets = out_dir / "assets"
    assets.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(Path(__file__).resolve().parent / "static" / "report.css", assets / "report.css")
    env = _template_env()
    html = _render(env, data, plot_index)
    (out_dir / "index.html").write_text(html, encoding="utf-8")
    for example in plot_index.get("examples", []):
        page_path = out_dir / example["path"]
        page_path.parent.mkdir(parents=True, exist_ok=True)
        page_path.write_text(_render_example(env, data, plot_index, example, out_dir, page_path.parent), encoding="utf-8")


def _render(env: Environment, data: BenchmarkRun, plot_index: dict[str, Any]) -> str:
    template = env.get_template("report.html")
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
        case_details=_case_details(cases, plot_index.get("cases", [])),
        kpis=_run_kpis(cases, expected),
        partial=_sorted_case_records(partial, ["example", "spec", "profile"]),
        failed=_sorted_case_records(failed, ["example", "spec", "profile"]),
        top_latency=top_latency_records,
        top_latency_groups=_group_records(top_latency_records, "example"),
        topology_groups=_topology_groups(plot_index.get("sections", {}).get("topology", [])),
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
    template = env.get_template("example.html")
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
    example_case_plots = []
    for case in plot_index.get("cases", []):
        if not str(case["case_name"]).startswith(example + "-"):
            continue
        example_case_plots.append(
            {
                "case_name": case["case_name"],
                "case_label": case.get("case_label", example_case_label(case["case_name"])),
                "plots": [_plot_item(plot, out_dir, page_dir) for plot in case["plots"]],
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
        case_plots=example_case_plots,
        case_details=_case_details(cases, example_case_plots),
        cases=_sorted_case_records(cases, ["spec", "profile"]),
        spec_comparisons=_records(_spec_comparisons(cases)),
        partial=_sorted_case_records(partial, ["spec", "profile"]),
        errors=_error_rows(errors),
    )


def _template_env() -> Environment:
    env = Environment(
        loader=FileSystemLoader(Path(__file__).resolve().parent / "templates"),
        autoescape=select_autoescape(["html"]),
        trim_blocks=True,
        lstrip_blocks=True,
    )
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


def _topology_groups(items: list[dict[str, Any]]) -> list[dict[str, Any]]:
    groups: list[dict[str, Any]] = []
    by_example: dict[str, dict[str, Any]] = {}
    for item in items:
        plot = _plot_item(item)
        raw_example = item.get("example") or _topology_example_from_path(plot["path"])
        key = str(raw_example or item.get("example_label") or "topology")
        group = by_example.get(key)
        if group is None:
            group = {
                "title": str(item.get("example_label") or example_label(key)),
                "anchor": f"topology-{_slug(key)}",
                "plots": [],
            }
            by_example[key] = group
            groups.append(group)
        group["plots"].append(plot)
    return groups


def _topology_example_from_path(path: str) -> str | None:
    stem = Path(path).stem
    for spec in sorted(SPEC_LABELS, key=len, reverse=True):
        suffix = f"_{_slug(spec)}"
        if stem.endswith(suffix):
            return stem[: -len(suffix)]
    return None


def _case_details(cases: pd.DataFrame, case_plots: list[dict[str, Any]]) -> list[dict[str, Any]]:
    plot_by_case = {str(case.get("case_name", "")): case for case in case_plots}
    details: list[dict[str, Any]] = []
    for row in _case_table_rows(cases):
        case_name = str(row.get("raw_case_name", ""))
        plot_entry = plot_by_case.get(case_name, {})
        plots = [_plot_item(plot) for plot in plot_entry.get("plots", [])]
        details.append(
            {
                **row,
                "anchor": _case_anchor(case_name),
                "plots": plots,
            }
        )
    return details


def _case_table_rows(cases: pd.DataFrame) -> list[dict[str, Any]]:
    if cases.empty:
        return []
    rows: list[dict[str, Any]] = []
    frame = _sort_cases(cases, ["example", "spec", "profile"])
    for row in _records(_with_table_fields(frame)):
        case_name = str(row.get("case_name", ""))
        example = str(row.get("example", ""))
        rows.append(
            {
                **row,
                "raw_case_name": case_name,
                "case_name": example_case_label(case_name),
                "example": example_label(example),
            }
        )
    return rows


def _plot_item(plot: Any, out_dir: Path | None = None, page_dir: Path | None = None) -> dict[str, str]:
    if isinstance(plot, dict):
        title = str(plot.get("title") or _plot_title(plot.get("path", "")))
        path = str(plot.get("path", ""))
    else:
        path = str(plot)
        title = _plot_title(path)
    if out_dir is not None and page_dir is not None and path:
        path = _rel(out_dir / path, page_dir)
    return {"title": title, "path": path}


def _plot_title(path: Any) -> str:
    stem = Path(str(path)).stem.replace("_", " ").replace("-", " ").strip()
    return stem.title() if stem else "Plot"


def _case_anchor(case_name: str) -> str:
    slug = "".join(ch.lower() if ch.isalnum() else "-" for ch in case_name).strip("-")
    return f"case-{slug or 'detail'}"


def _slug(value: Any) -> str:
    safe = []
    for ch in str(value).lower():
        if ch.isalnum():
            safe.append(ch)
        elif ch in {"-", "_", "."}:
            safe.append(ch)
        else:
            safe.append("-")
    return "".join(safe).strip("-") or "item"


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
    "memory": "Memory",
    "no_memory": "No memory",
    "automatic": "Automatic",
    "agentic": "Agentic",
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
