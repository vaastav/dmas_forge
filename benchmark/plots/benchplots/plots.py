from __future__ import annotations

from collections import defaultdict
from pathlib import Path
import re
from typing import Any

import matplotlib

matplotlib.use("Agg")

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns
from matplotlib.lines import Line2D
from matplotlib.patches import Patch
from matplotlib.ticker import FuncFormatter, PercentFormatter

from .data import BenchmarkRun, write_normalized_data
from .labels import example_case_label, example_label, example_sort_key, short_service_name
from .topology import EXAMPLES
from .topology_draw import draw_topology

PLOT_DIRS = [
    "overview",
    "performance",
    "reliability",
    "agents",
    "resources",
    "spans_per_trace",
    "topology",
    "examples",
    "cases",
    "assets",
    "data",
]

SPEC_DISPLAY_ORDER = ["single", "http", "mcp", "a2a"]
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
SPEC_COLORS = {
    "Single": "#0072B2",
    "HTTP": "#E69F00",
    "MCP": "#009E73",
    "A2A": "#CC79A7",
    "Memory": "#56B4E9",
    "No memory": "#D55E00",
    "Automatic": "#999999",
    "Agentic": "#F0E442",
}
OUTCOME_LABELS = {True: "success", False: "failed / timeout"}
OUTCOME_COLORS = {"success": "#0072B2", "failed / timeout": "#D55E00"}
PERCENTILE_LABELS = {"p50_ms": "p50", "p95_ms": "p95", "p99_ms": "p99"}
LATENCY_FILL_ALPHA = 0.7
METRIC_LABELS = {
    "cpu_avg_percent": "average CPU",
    "cpu_max_percent": "max CPU",
    "memory_avg_mib": "average memory",
    "memory_max_mib": "max memory",
}
ERROR_LABELS = {
    "server_5xx": "Server error / timeout",
    "other_error": "Other error",
    "rate_limit_429": "Rate limit",
    "http_500": "HTTP 500",
    "transport_500": "Transport 500",
    "workflow_no_report": "No workflow report",
    "a2a_error": "A2A error",
    "mcp_error": "MCP error",
    "llm_error": "LLM error",
}


def generate_plots(data: BenchmarkRun, out_dir: Path, max_case_waterfalls: int = 200) -> dict[str, Any]:
    _set_style()
    out_dir.mkdir(parents=True, exist_ok=True)
    for name in PLOT_DIRS:
        (out_dir / name).mkdir(parents=True, exist_ok=True)
    write_normalized_data(data, out_dir)

    index: dict[str, Any] = {"sections": defaultdict(list), "cases": [], "_out_dir": out_dir}
    _overview_plots(data, out_dir, index)
    _performance_plots(data, out_dir, index)
    _reliability_plots(data, out_dir, index)
    _resource_plots(data, out_dir, index)
    _topology_plots(data, out_dir, index)
    _example_plots(data, out_dir, index)
    _case_plots(data, out_dir, index, max_case_waterfalls)
    index["sections"] = dict(index["sections"])
    index.pop("_out_dir", None)
    return index


def _overview_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = data.cases
    expected = data.expected_cases
    if cases.empty:
        _placeholder(out_dir / "overview" / "no_cases.png", "No case summaries found")
        return

    merged = expected.merge(
        cases[["case_name", "success_rate", "requests", "successes", "errors"]],
        on="case_name",
        how="left",
    )
    merged["success_rate"] = merged["success_rate"].where(merged["present"], np.nan)
    include_profile = cases["profile"].astype(str).nunique(dropna=True) > 1 if "profile" in cases else False
    merged = _with_display_columns(merged, include_profile=include_profile)
    matrix = merged.pivot_table(
        index="example_display",
        columns="case_axis",
        values="success_rate",
        aggfunc="first",
        observed=True,
    )
    matrix = matrix.reindex(columns=[col for col in _axis_order(merged) if col in matrix.columns])
    matrix = matrix.reindex(index=[row for row in _example_axis_order(merged) if row in matrix.index])
    annot = matrix.map(lambda v: "NA" if pd.isna(v) else f"{v:.0%}")
    fig, ax = plt.subplots(figsize=(16, max(4, 0.52 * len(matrix.index) + 2)))
    cmap = sns.color_palette("RdYlGn", as_cmap=True)
    cmap.set_bad("#e5e7eb")
    sns.heatmap(
        matrix,
        ax=ax,
        vmin=0,
        vmax=1,
        cmap=cmap,
        linewidths=0.7,
        linecolor="white",
        annot=annot,
        fmt="",
        cbar_kws={"label": "success rate"},
    )
    ax.set_title("Success Rate by Example and Protocol", loc="left", pad=16)
    ax.set_xlabel("")
    ax.set_ylabel("")
    _save(fig, out_dir / "overview" / "success_rate_heatmap.png", index, "overview", "Success-rate heatmap")


def _performance_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    requests = _with_display_columns(_actual(data.requests))
    if cases.empty:
        return
    cases["throughput_per_min"] = cases["throughput_rps"] * 60.0

    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(
        data=cases,
        x="example_display",
        y="throughput_per_min",
        hue="spec_display",
        hue_order=_spec_hue_order(cases),
        ax=ax,
        palette=_spec_palette(cases),
        errorbar=None,
        order=_example_axis_order(cases),
    )
    ax.set_title("Throughput by Example and Protocol", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("requests / min")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Protocol")
    _save(fig, out_dir / "performance" / "throughput_by_spec.png", index, "performance", "Throughput by spec")

    latency = cases.melt(
        id_vars=["case_name", "example", "example_display", "spec", "profile", "spec_display", "case_axis"],
        value_vars=["p50_ms", "p95_ms", "p99_ms"],
        var_name="percentile",
        value_name="latency_ms",
    )
    latency["percentile"] = latency["percentile"].map(PERCENTILE_LABELS)
    scale, latency_label, _fmt = _latency_scale(latency["latency_ms"])
    latency["latency"] = latency["latency_ms"] / scale
    examples = list(dict.fromkeys(latency["example"].astype(str).tolist()))
    ncols = min(2, max(1, len(examples)))
    nrows = int(np.ceil(len(examples) / ncols))
    fig, axes = plt.subplots(nrows, ncols, figsize=(15, max(5, 4.2 * nrows)), squeeze=False)
    for ax, example in zip(axes.flat, examples):
        frame = latency[latency["example"].astype(str) == example].copy()
        sns.barplot(
            data=frame,
            x="case_axis",
            y="latency",
            hue="percentile",
            hue_order=["p50", "p95", "p99"],
            ax=ax,
            palette=["#5B2A52", "#B8325B", "#E68A6A"],
            errorbar=None,
            order=_axis_order(frame),
        )
        ax.set_title(example_label(example), loc="left", pad=10)
        ax.set_xlabel("")
        ax.set_ylabel(latency_label)
        ax.tick_params(axis="x", rotation=0)
        if ax.get_legend() is not None:
            ax.get_legend().remove()
    for ax in axes.flat[len(examples):]:
        ax.axis("off")
    handles, labels = axes.flat[0].get_legend_handles_labels()
    if handles:
        fig.legend(handles, labels, title="Percentile", loc="center left", bbox_to_anchor=(1.01, 0.5), frameon=False)
    fig.suptitle("End-to-End Latency Percentiles by Protocol", x=0.02, ha="left", fontweight="bold")
    _save(fig, out_dir / "performance" / "latency_percentiles.png", index, "performance", "Latency percentiles")

    if not requests.empty:
        fig = _draw_request_latency_by_example(requests, "End-to-End Request Latency CDF by Example", "cdf")
        _save(
            fig,
            out_dir / "performance" / "request_latency_cdf_by_example.png",
            index,
            "performance",
            "Request latency CDF by example",
        )

        fig = _draw_request_latency_by_example(requests, "End-to-End Request Latency Distribution by Example", "distribution")
        _save(
            fig,
            out_dir / "performance" / "request_latency_distribution_by_example.png",
            index,
            "performance",
            "Request latency distribution by example",
        )

        fig = _draw_request_latency_by_example(requests, "End-to-End Request Latency Violin Plots by Example", "violin")
        _save(
            fig,
            out_dir / "performance" / "request_latency_violin_by_example.png",
            index,
            "performance",
            "Request latency violin plots by example",
        )


def _reliability_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    if cases.empty:
        return
    plot = cases[["case_name", "example", "example_display", "spec", "profile", "case_axis", "requests", "errors"]].copy()
    plot["failure_rate"] = np.where(plot["requests"].astype(float) > 0, plot["errors"].astype(float) / plot["requests"].astype(float), np.nan)
    matrix = plot.pivot_table(index="example_display", columns="case_axis", values="failure_rate", aggfunc="mean", observed=True)
    matrix = matrix.reindex(columns=[col for col in _axis_order(plot) if col in matrix.columns])
    matrix = matrix.reindex(index=[row for row in _example_axis_order(plot) if row in matrix.index])
    annot_lookup = plot.set_index(["example_display", "case_axis"])[["errors", "requests"]].to_dict("index")
    annot = matrix.copy().astype(object)
    for row in matrix.index:
        for col in matrix.columns:
            values = annot_lookup.get((row, col), {})
            if not values or pd.isna(matrix.loc[row, col]):
                annot.loc[row, col] = "NA"
            else:
                annot.loc[row, col] = f"{int(values['errors'])}/{int(values['requests'])}"
    fig, ax = plt.subplots(figsize=(16, max(4, 0.52 * len(matrix.index) + 2)))
    cmap = sns.color_palette("Reds", as_cmap=True)
    cmap.set_bad("#e5e7eb")
    finite_values = matrix.to_numpy()[np.isfinite(matrix.to_numpy())]
    vmax = max(0.01, float(finite_values.max()) if finite_values.size else 0.01)
    sns.heatmap(matrix, ax=ax, vmin=0, vmax=vmax, cmap=cmap, linewidths=0.7, linecolor="white", annot=annot, fmt="", cbar_kws={"label": "failure rate"})
    ax.set_title("Request Failures by Example and Protocol", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("")
    _save(fig, out_dir / "reliability" / "failure_rate_heatmap.png", index, "failures", "Request failure heatmap")

    errors = data.errors
    if not errors.empty:
        by_category = errors.groupby("error_category", observed=True)["count"].sum().reset_index()
        by_category = by_category.sort_values("count", ascending=False)
        by_category["error_label"] = by_category["error_category"].map(_error_label)
        fig, ax = plt.subplots(figsize=(11, 5.5))
        sns.barplot(data=by_category, y="error_label", x="count", ax=ax, color=OUTCOME_COLORS["failed / timeout"])
        ax.bar_label(ax.containers[0], padding=3)
        ax.set_title("Request Failure Categories", loc="left", pad=14)
        ax.set_xlabel("failed requests")
        ax.set_ylabel("")
        _save(fig, out_dir / "reliability" / "request_failure_categories.png", index, "failures", "Request failure categories")

    spans = data.spans
    if not spans.empty and "is_error" in spans:
        span_errors = spans[spans["is_error"].fillna(False)].copy()
        if not span_errors.empty:
            failing_cases = set(data.error_details["case_name"].astype(str)) if not data.error_details.empty and "case_name" in data.error_details else set()
            span_errors["outcome"] = np.where(span_errors["case_name"].astype(str).isin(failing_cases), "Request Failed", "Error Recovered")
            span_errors["workflow_service"] = span_errors["example"].map(example_label) + " / " + span_errors["service_short"].astype(str)
            by_service = span_errors.groupby(["workflow_service", "outcome"], observed=True).size().reset_index(name="span_errors")
            by_service = by_service.sort_values("span_errors", ascending=False)
            fig, ax = plt.subplots(figsize=(13, max(4.5, 0.48 * by_service["workflow_service"].nunique() + 2)))
            palette = {"Request Failed": "#D55E00", "Error Recovered": "#7A8EA4"}
            order = by_service.groupby("workflow_service", observed=True)["span_errors"].sum().sort_values(ascending=False).index.tolist()
            sns.barplot(data=by_service, y="workflow_service", x="span_errors", hue="outcome", order=order, ax=ax, palette=palette, errorbar=None)
            ax.set_title("Application Span Errors by Workflow and Service", loc="left", pad=14)
            ax.set_xlabel("span errors")
            ax.set_ylabel("")
            _outside_legend(ax, "Outcome")
            _save(fig, out_dir / "reliability" / "span_errors_by_workflow_service.png", index, "failures", "Span errors by workflow and service")


def _resource_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    resources = data.resources
    if cases.empty:
        return
    resource_metrics = cases.melt(
        id_vars=["case_name", "example", "example_display", "spec", "profile"],
        value_vars=["cpu_avg_percent", "cpu_max_percent"],
        var_name="metric",
        value_name="cpu_percent",
    )
    resource_metrics["metric_label"] = resource_metrics["metric"].map(METRIC_LABELS)
    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(data=resource_metrics, x="example_display", y="cpu_percent", hue="metric_label", ax=ax, palette=["#9ecae1", "#3182bd"], errorbar=None, order=_example_axis_order(resource_metrics))
    ax.set_title("CPU Usage Summary", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("CPU percent")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Metric")
    _save(fig, out_dir / "resources" / "cpu_summary.png", index, "resources", "CPU summary")

    mem = cases.copy()
    mem["memory_avg_mib"] = mem["memory_avg_bytes"] / 1024 / 1024
    mem["memory_max_mib"] = mem["memory_max_bytes"] / 1024 / 1024
    mem_metrics = mem.melt(
        id_vars=["case_name", "example", "example_display", "spec", "profile"],
        value_vars=["memory_avg_mib", "memory_max_mib"],
        var_name="metric",
        value_name="memory_mib",
    )
    mem_metrics["metric_label"] = mem_metrics["metric"].map(METRIC_LABELS)
    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(data=mem_metrics, x="example_display", y="memory_mib", hue="metric_label", ax=ax, palette=["#bcbddc", "#756bb1"], errorbar=None, order=_example_axis_order(mem_metrics))
    ax.set_title("Memory Usage Summary", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("MiB")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Metric")
    _save(fig, out_dir / "resources" / "memory_summary.png", index, "resources", "Memory summary")

    if not resources.empty:
        top_cases = cases.sort_values("memory_max_bytes", ascending=False).head(12)["case_name"].tolist()
        plot = resources[resources["case_name"].isin(top_cases)].copy()
        plot = plot.groupby(["case_name", "elapsed_s"], observed=True, as_index=False).agg(
            cpu_percent=("cpu_percent", "sum"),
            memory_bytes=("memory_bytes", "sum"),
        )
        plot["memory_mib"] = plot["memory_bytes"] / 1024 / 1024
        plot["case_label"] = plot["case_name"].map(_short_case_label)
        fig, axes = plt.subplots(2, 1, figsize=(14, 8), sharex=True)
        sns.lineplot(data=plot, x="elapsed_s", y="cpu_percent", hue="case_label", ax=axes[0], legend=False, linewidth=1.4, alpha=0.65)
        sns.lineplot(data=plot, x="elapsed_s", y="memory_mib", hue="case_label", ax=axes[1], linewidth=1.4, alpha=0.75)
        axes[0].set_title("Resource Timelines for Highest-Memory Cases", loc="left", pad=14)
        axes[0].set_ylabel("CPU percent")
        axes[1].set_ylabel("MiB")
        axes[1].set_xlabel("elapsed seconds")
        _outside_legend(axes[1], "Case")
        _save(fig, out_dir / "resources" / "resource_timelines_top_memory.png", index, "resources", "Top memory timelines")


def _topology_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = data.expected_cases if not data.expected_cases.empty else data.cases
    if cases.empty:
        return
    pairs = cases[["example", "spec"]].drop_duplicates().copy()
    pairs = _sort_for_display(pairs)
    for item in pairs.itertuples(index=False):
        example = str(item.example)
        spec = str(item.spec)
        meta = EXAMPLES.get(example)
        if not meta:
            continue
        example_display = example_label(example)
        fig = draw_topology(example, spec, meta)
        path = out_dir / "topology" / f"{_slug(example)}_{_slug(spec)}.svg"
        _save(fig, path, index, "topology", f"{example_display} {spec} topology")
        index["sections"]["topology"][-1].update(
            {
                "example": example,
                "example_label": example_display,
            }
        )


def _example_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    requests = _with_display_columns(_actual(data.requests))
    agent_metrics = _with_display_columns(_actual(data.agent_metrics))
    traces = _with_display_columns(_actual(data.traces))
    expected = _with_display_columns(_actual(data.expected_cases))
    if cases.empty:
        return

    examples: list[dict[str, Any]] = []
    expected_by_example = _groups(expected, "example")
    requests_by_example = _groups(requests, "example")
    agents_by_example = _groups(agent_metrics, "example")
    traces_by_example = _groups(traces, "example")
    empty = pd.DataFrame()
    for example, ex_cases in cases.groupby("example", observed=True, sort=False):
        example = str(example)
        example_display = example_label(example)
        slug = _slug(example)
        example_dir = out_dir / "examples" / slug
        example_dir.mkdir(parents=True, exist_ok=True)
        entry = {"example": example, "example_label": example_display, "slug": slug, "path": f"examples/{slug}/index.html", "plots": []}

        ex_expected = expected_by_example.get(example, ex_cases)
        merged = ex_expected.merge(
            ex_cases[["case_name", "success_rate"]],
            on="case_name",
            how="left",
        )
        merged["success_rate"] = merged["success_rate"].where(merged["present"], np.nan) if "present" in merged else merged["success_rate"]
        include_profile = ex_cases["profile"].astype(str).nunique(dropna=True) > 1 if "profile" in ex_cases else False
        merged = _with_display_columns(merged, include_profile=include_profile)
        matrix = merged.pivot_table(index="example_display", columns="case_axis", values="success_rate", aggfunc="first", observed=True)
        matrix = matrix.reindex(columns=[col for col in _axis_order(merged) if col in matrix.columns])
        fig, ax = plt.subplots(figsize=(13, 3.6))
        cmap = sns.color_palette("RdYlGn", as_cmap=True)
        cmap.set_bad("#e5e7eb")
        annot = matrix.map(lambda v: "NA" if pd.isna(v) else f"{v:.0%}")
        sns.heatmap(
            matrix,
            ax=ax,
            vmin=0,
            vmax=1,
            cmap=cmap,
            linewidths=0.8,
            linecolor="white",
            annot=annot,
            fmt="",
            cbar_kws={"label": "success rate"},
        )
        ax.set_title(f"{example_display}: Success Rate by Protocol", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("")
        entry["plots"].append({"title": "Success-rate heatmap", "path": _save(fig, example_dir / "success_rate_heatmap.png", index)})

        latency = ex_cases.melt(
            id_vars=["case_name", "spec", "profile", "spec_display", "case_axis"],
            value_vars=["p50_ms", "p95_ms", "p99_ms"],
            var_name="percentile",
            value_name="latency_ms",
        )
        latency["percentile"] = latency["percentile"].map(PERCENTILE_LABELS)
        latency, latency_label, _fmt = _scaled_latency(latency)
        fig, ax = plt.subplots(figsize=(14, 6))
        sns.barplot(data=latency, x="case_axis", y="latency", hue="percentile", hue_order=["p50", "p95", "p99"], ax=ax, palette=["#5B2A52", "#B8325B", "#E68A6A"], errorbar=None, order=_axis_order(latency))
        ax.set_title(f"{example_display}: End-to-End Latency Percentiles", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel(latency_label)
        ax.tick_params(axis="x", rotation=0)
        _outside_legend(ax, "Percentile")
        entry["plots"].append({"title": "Latency percentiles", "path": _save(fig, example_dir / "latency_percentiles.png", index)})

        fig, ax = plt.subplots(figsize=(14, 6))
        ex_cases = ex_cases.copy()
        ex_cases["throughput_per_min"] = ex_cases["throughput_rps"] * 60.0
        sns.barplot(data=ex_cases, x="case_axis", y="throughput_per_min", hue="spec_display", hue_order=_spec_hue_order(ex_cases), ax=ax, palette=_spec_palette(ex_cases), errorbar=None, order=_axis_order(ex_cases))
        ax.set_title(f"{example_display}: Throughput", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("requests / min")
        ax.tick_params(axis="x", rotation=0)
        _outside_legend(ax, "Protocol")
        entry["plots"].append({"title": "Throughput", "path": _save(fig, example_dir / "throughput.png", index)})

        ex_agents = agents_by_example.get(example, empty)
        if not ex_agents.empty:
            agent_plot = _prepare_agent_metric_plot(ex_agents, ex_cases)
            fig = _draw_agent_time(agent_plot, f"{example_display}: Average Agent Time per Request")
            path = _save(fig, out_dir / "agents" / f"{slug}_agent_time.png", index, "agents", f"{example_display} agent time")
            entry["plots"].append({"title": "Average agent time per request", "path": path})

            fig = _draw_agent_tokens(agent_plot, f"{example_display}: Average Agent Token Use per Successful Request")
            path = _save(fig, out_dir / "agents" / f"{slug}_agent_tokens.png", index, "agents", f"{example_display} agent tokens")
            entry["plots"].append({"title": "Average agent input/output tokens", "path": path})

        ex_traces = traces_by_example.get(example, empty)
        if not ex_traces.empty:
            fig = _draw_spans_per_trace_distribution(ex_traces, f"{example_display}: Spans per Trace Distribution")
            path = _save(fig, out_dir / "spans_per_trace" / f"{slug}_spans_per_trace_distribution.png", index, "spans_per_trace", f"{example_display} spans per trace distribution")
            entry["plots"].append({"title": "Spans per trace distribution", "path": path})

            fig = _draw_spans_per_trace_summary(ex_traces, f"{example_display}: Spans per Trace by Protocol")
            path = _save(fig, out_dir / "spans_per_trace" / f"{slug}_spans_per_trace_summary.png", index, "spans_per_trace", f"{example_display} spans per trace summary")
            entry["plots"].append({"title": "Spans per trace by protocol", "path": path})

        ex_requests = requests_by_example.get(example, empty)
        if not ex_requests.empty:
            ex_requests = ex_requests.copy()
            ex_requests, latency_label, _fmt = _scaled_latency(ex_requests)
            ex_requests["outcome"] = ex_requests["ok"].map(OUTCOME_LABELS).fillna("unknown")
            fig, ax = plt.subplots(figsize=(14, 6))
            sns.stripplot(
                data=ex_requests,
                x="case_axis",
                y="latency",
                hue="outcome",
                ax=ax,
                palette=OUTCOME_COLORS,
                dodge=True,
                alpha=0.75,
                size=4,
                order=_axis_order(ex_requests),
            )
            ax.set_title(f"{example_display}: End-to-End Request Latency", loc="left", pad=14)
            ax.set_xlabel("")
            ax.set_ylabel(latency_label)
            ax.tick_params(axis="x", rotation=0)
            _outside_legend(ax, "Outcome")
            entry["plots"].append({"title": "Per-request latency", "path": _save(fig, example_dir / "request_latency.png", index)})

            fig = _draw_latency_distribution(
                ex_requests,
                title=f"{example_display}: End-to-End Request Latency Distribution",
                x_col="case_axis",
            )
            entry["plots"].append({"title": "Request latency distribution", "path": _save(fig, example_dir / "request_latency_range.png", index)})

            fig = _draw_latency_violin(
                ex_requests,
                title=f"{example_display}: End-to-End Request Latency Violin Plot",
                x_col="case_axis",
            )
            entry["plots"].append({"title": "Request latency violin plot", "path": _save(fig, example_dir / "request_latency_violin.png", index)})

            fig = _draw_latency_cdf(
                ex_requests,
                title=f"{example_display}: End-to-End Request Latency CDF",
                group_col="case_axis",
                legend_title="Protocol",
            )
            entry["plots"].append({"title": "Request latency CDF", "path": _save(fig, example_dir / "request_latency_cdf.png", index)})

        examples.append(entry)

    index["examples"] = examples


def _prepare_agent_metric_plot(agent_metrics: pd.DataFrame, cases: pd.DataFrame) -> pd.DataFrame:
    counts = cases[["case_name", "requests", "successes"]].copy()
    out = agent_metrics.merge(counts, on="case_name", how="left")
    request_denom = out["requests"].where(out["requests"] > 0, np.nan)
    success_denom = out["successes"].where(out["successes"] > 0, out["requests"])
    success_denom = success_denom.where(success_denom > 0, np.nan)
    out["duration_s_per_request"] = (out["duration_ms"] / 1_000.0 / request_denom).fillna(0.0)
    out["input_tokens_per_success"] = (out["input_tokens"] / success_denom).fillna(0.0)
    out["output_tokens_per_success"] = (out["output_tokens"] / success_denom).fillna(0.0)
    out["total_tokens_per_success"] = (out["total_tokens"] / success_denom).fillna(0.0)
    out["agent_label"] = out["agent"].map(_short_agent)
    return out


def _draw_agent_time(plot: pd.DataFrame, title: str) -> plt.Figure:
    plot = plot[plot["duration_s_per_request"] > 0].copy()
    if plot.empty:
        return _blank_figure("No traced agent duration found")
    agent_order = plot.groupby("agent_label", observed=True)["duration_s_per_request"].mean().sort_values(ascending=False).index.tolist()
    fig, ax = plt.subplots(figsize=(13, max(5.0, 0.52 * len(agent_order) + 2.0)))
    sns.barplot(
        data=plot,
        y="agent_label",
        x="duration_s_per_request",
        hue="spec_display",
        hue_order=_spec_hue_order(plot),
        order=agent_order,
        ax=ax,
        palette=_spec_palette(plot),
        errorbar=None,
    )
    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("average seconds / request")
    ax.set_ylabel("")
    ax.grid(axis="x", alpha=0.35)
    ax.grid(axis="y", visible=False)
    _outside_legend(ax, "Protocol")
    sns.despine(ax=ax, left=True, bottom=False)
    return fig


def _draw_agent_tokens(plot: pd.DataFrame, title: str) -> plt.Figure:
    plot = plot[plot["total_tokens_per_success"] > 0].copy()
    if plot.empty:
        return _blank_figure("No attributed agent tokens found")
    plot = (
        plot.groupby(["spec", "spec_display", "agent_label"], observed=True)
        .agg(
            input_tokens_per_success=("input_tokens_per_success", "mean"),
            output_tokens_per_success=("output_tokens_per_success", "mean"),
            total_tokens_per_success=("total_tokens_per_success", "mean"),
        )
        .reset_index()
    )
    spec_labels = _spec_hue_order(plot)
    agent_order = plot.groupby("agent_label", observed=True)["total_tokens_per_success"].mean().sort_values(ascending=False).index.tolist()
    max_tokens = float(plot["total_tokens_per_success"].max())
    ncols = max(1, len(spec_labels))
    fig, axes = plt.subplots(
        1,
        ncols,
        figsize=(max(9.5, 4.5 * ncols), max(5.2, 0.5 * len(agent_order) + 2.2)),
        sharey=True,
        squeeze=False,
    )
    y = np.arange(len(agent_order))
    for index, spec_label in enumerate(spec_labels):
        ax = axes[0][index]
        frame = plot[plot["spec_display"] == spec_label].set_index("agent_label").reindex(agent_order)
        input_tokens = frame["input_tokens_per_success"].fillna(0.0).to_numpy()
        output_tokens = frame["output_tokens_per_success"].fillna(0.0).to_numpy()
        ax.barh(y, input_tokens, color="#2563eb", alpha=0.88)
        ax.barh(y, output_tokens, left=input_tokens, color="#7c3aed", alpha=0.86)
        ax.set_title(spec_label, loc="left", pad=10)
        ax.set_yticks(y)
        if index == 0:
            ax.set_yticklabels(agent_order)
        else:
            ax.tick_params(axis="y", labelleft=False)
        ax.grid(axis="x", alpha=0.35)
        ax.grid(axis="y", visible=False)
        ax.set_xlabel("average tokens / successful request")
        if max_tokens >= 1_000:
            _format_count_axis(ax, 1_000, "K")
            ax.set_xlabel("average K tokens / successful request")
        ax.set_xlim(0, max(max_tokens * 1.1, 1.0))
        sns.despine(ax=ax, left=index != 0, bottom=False)
    axes[0][0].invert_yaxis()
    fig.suptitle(title, x=0.02, ha="left", fontweight="bold")
    fig.legend(
        handles=[Patch(color="#2563eb", label="input"), Patch(color="#7c3aed", label="output")],
        loc="upper right",
        frameon=False,
        ncols=2,
    )
    return fig


def _draw_spans_per_trace_distribution(traces: pd.DataFrame, title: str) -> plt.Figure:
    traces = traces[traces["span_count"] > 0].copy()
    if traces.empty:
        return _blank_figure("No trace spans found")
    spec_labels = _spec_hue_order(traces)
    palette = _spec_palette(traces)
    fig, ax = plt.subplots(figsize=(10.5, 5.8))

    for spec_label in spec_labels:
        values = np.sort(traces[traces["spec_display"] == spec_label]["span_count"].to_numpy(dtype=float))
        if len(values) == 0:
            continue
        y = np.arange(1, len(values) + 1) / len(values)
        ax.step(values, y, where="post", color=palette.get(spec_label, "#666666"), linewidth=2.2, label=f"{spec_label} (n={len(values)})")
    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("spans per trace")
    ax.set_ylabel("traces <= span count")
    ax.yaxis.set_major_formatter(PercentFormatter(1.0))
    ax.set_ylim(0, 1.01)
    ax.grid(alpha=0.35)
    ax.legend(loc="lower right", frameon=True, framealpha=0.9, facecolor="white")
    sns.despine(ax=ax)
    return fig


def _draw_spans_per_trace_summary(traces: pd.DataFrame, title: str) -> plt.Figure:
    traces = traces[traces["span_count"] > 0].copy()
    if traces.empty:
        return _blank_figure("No trace spans found")
    spec_labels = _spec_hue_order(traces)
    palette = _spec_palette(traces)
    fig, ax = plt.subplots(figsize=(10.5, 5.2))
    sns.boxplot(
        data=traces,
        y="spec_display",
        x="span_count",
        order=spec_labels,
        hue="spec_display",
        hue_order=spec_labels,
        palette=palette,
        ax=ax,
        width=0.48,
        linewidth=1.4,
        fliersize=3,
        legend=False,
    )
    summary_max = float(traces["span_count"].max())
    pad = max(1.0, summary_max * 0.025)
    for pos, spec_label in enumerate(spec_labels):
        values = traces[traces["spec_display"] == spec_label]["span_count"].to_numpy(dtype=float)
        if len(values) == 0:
            continue
        avg = float(np.mean(values))
        ax.plot(avg, pos, marker="D", markerfacecolor="white", markeredgecolor="#1f2937", markeredgewidth=1.5, markersize=6, zorder=4)
        ax.text(summary_max + pad, pos, f"n={len(values)}", va="center", fontsize=9, color="#475569")
    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("spans per trace")
    ax.set_ylabel("")
    ax.set_xlim(0, summary_max * 1.1 + pad * 3)
    ax.grid(axis="x", alpha=0.35)
    ax.grid(axis="y", visible=False)
    ax.legend(
        handles=[
            Line2D([0], [0], marker="D", color="#334155", markerfacecolor="white", linewidth=0, label="average"),
        ],
        loc="upper center",
        bbox_to_anchor=(0.5, -0.12),
        frameon=False,
    )
    sns.despine(ax=ax)
    return fig


def _case_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any], max_case_waterfalls: int) -> None:
    cases = _with_display_columns(_actual(data.cases))
    requests = _with_display_columns(data.requests)
    resources = data.resources
    spans = data.spans
    waterfalls = 0
    requests_by_case = _groups(requests, "case_name")
    resources_by_case = _groups(resources, "case_name")
    spans_by_case = _groups(spans, "case_name")
    empty = pd.DataFrame()
    for case in cases["case_name"].astype(str).tolist():
        case_dir = out_dir / "cases" / _slug(case)
        case_dir.mkdir(parents=True, exist_ok=True)
        case_entry: dict[str, Any] = {"case_name": case, "case_label": _short_case_label(case), "plots": []}

        req = requests_by_case.get(case, empty)
        if not req.empty:
            req = req.copy()
            req, latency_label, _fmt = _scaled_latency(req)
            req["outcome"] = req["ok"].map(OUTCOME_LABELS).fillna("unknown")
            fig, ax = plt.subplots(figsize=(9, 4.8))
            sns.scatterplot(data=req, x="sequence", y="latency", hue="outcome", ax=ax, palette=OUTCOME_COLORS, s=48)
            ax.plot(req["sequence"], req["latency"], color="#a0aec0", linewidth=1, alpha=0.6)
            ax.set_title(f"{_short_case_label(case)}: Request Latency", loc="left", pad=12)
            ax.set_xlabel("request sequence")
            ax.set_ylabel(latency_label)
            _outside_legend(ax, "Outcome")
            case_entry["plots"].append({"title": "Request latency", "path": _save(fig, case_dir / "request_latency.png", index)})

            fig = _draw_latency_cdf(
                req,
                title=f"{_short_case_label(case)}: Request Latency CDF",
                group_col="outcome",
                legend_title="Outcome",
            )
            case_entry["plots"].append({"title": "Latency CDF", "path": _save(fig, case_dir / "request_latency_cdf.png", index)})

        res = resources_by_case.get(case, empty)
        if not res.empty:
            res = res.copy()
            res["memory_mib"] = res["memory_bytes"] / 1024 / 1024
            fig, axes = plt.subplots(2, 1, figsize=(9, 6), sharex=True)
            sns.lineplot(data=res, x="elapsed_s", y="cpu_percent", hue="container_short", ax=axes[0], legend=False)
            sns.lineplot(data=res, x="elapsed_s", y="memory_mib", hue="container_short", ax=axes[1])
            axes[0].set_title(f"{_short_case_label(case)}: Resource Timeline", loc="left", pad=12)
            axes[0].set_ylabel("CPU percent")
            axes[1].set_ylabel("MiB")
            axes[1].set_xlabel("elapsed seconds")
            _outside_legend(axes[1], "Container")
            case_entry["plots"].append({"title": "Resource timeline", "path": _save(fig, case_dir / "resources.png", index)})

        span_case = spans_by_case.get(case, empty)
        if not span_case.empty and waterfalls < max_case_waterfalls:
            fig = _draw_longest_waterfall(case, span_case)
            if fig is not None:
                waterfalls += 1
                case_entry["plots"].append({"title": "Longest trace waterfall", "path": _save(fig, case_dir / "longest_trace_waterfall.png", index)})

        index["cases"].append(case_entry)


def _draw_longest_waterfall(case: str, spans: pd.DataFrame) -> plt.Figure | None:
    if spans.empty:
        return None
    trace_windows = spans.groupby("trace_id", observed=True).agg(start=("start_ms", "min"), end=("end_ms", "max"), spans=("span_id", "count")).reset_index()
    trace_windows["duration_ms"] = trace_windows["end"] - trace_windows["start"]
    trace_id = trace_windows.sort_values(["duration_ms", "spans"], ascending=False).iloc[0]["trace_id"]
    frame = spans[spans["trace_id"] == trace_id].copy().sort_values("relative_start_ms")
    if frame.empty:
        return None
    frame["operation_label"] = frame["operation"].map(_short_component)
    frame["service_label"] = frame["service"].map(short_service_name)
    frame["label"] = frame["operation_label"].astype(str) + "\n" + frame["service_label"].astype(str)
    frame = frame.tail(40) if len(frame) > 40 else frame
    fig, ax = plt.subplots(figsize=(12, max(5, len(frame) * 0.3)))
    colors = sns.color_palette("tab20", n_colors=max(1, frame["service_label"].nunique()))
    service_colors = {svc: colors[i % len(colors)] for i, svc in enumerate(frame["service_label"].unique())}
    y = np.arange(len(frame))
    for i, row in enumerate(frame.itertuples(index=False)):
        ax.barh(i, row.duration_ms / 1000.0, left=row.relative_start_ms / 1000.0, color=service_colors[row.service_label], edgecolor="white", height=0.72)
    ax.set_yticks(y)
    ax.set_yticklabels(frame["label"], fontsize=7)
    ax.invert_yaxis()
    ax.set_title(f"{_short_case_label(case)}: Longest Trace Waterfall", loc="left", pad=12)
    ax.set_xlabel("trace-relative time (s)")
    ax.set_ylabel("")
    return fig


def _draw_request_latency_by_example(requests: pd.DataFrame, title: str, plot_kind: str) -> plt.Figure:
    examples = list(dict.fromkeys(requests["example"].astype(str).tolist()))
    ncols = min(2, max(1, len(examples)))
    nrows = int(np.ceil(len(examples) / ncols))
    fig, axes = plt.subplots(nrows, ncols, figsize=(15, max(5, 4.2 * nrows)), squeeze=False)

    for ax, example in zip(axes.flat, examples):
        frame = requests[requests["example"].astype(str) == example]
        plot_title = example_label(example)
        if plot_kind == "cdf":
            _draw_latency_cdf(frame, title=plot_title, group_col="case_axis", legend_title="Protocol", ax=ax)
        elif plot_kind == "distribution":
            _draw_latency_distribution(frame, title=plot_title, x_col="case_axis", ax=ax)
        elif plot_kind == "violin":
            _draw_latency_violin(frame, title=plot_title, x_col="case_axis", ax=ax)
        else:
            raise ValueError(f"unknown latency plot kind: {plot_kind}")

    for ax in axes.flat[len(examples):]:
        ax.axis("off")
    fig.suptitle(title, x=0.02, ha="left", fontweight="bold")
    return fig


def _latency_palette(labels: list[str]) -> dict[str, Any]:
    fallback_colors = sns.color_palette("tab10", n_colors=max(1, len(labels)))
    return {label: SPEC_COLORS.get(label.split("\n", 1)[0], fallback_colors[idx]) for idx, label in enumerate(labels)}


def _draw_latency_distribution(frame: pd.DataFrame, title: str, x_col: str, ax: plt.Axes | None = None) -> plt.Figure:
    scale, y_label, _fmt = _latency_scale(frame["latency_ms"] if "latency_ms" in frame else pd.Series(dtype=float))
    stats: list[dict[str, Any]] = []
    outlier_rows: list[dict[str, Any]] = []
    for case_axis, group in frame.groupby(x_col, observed=True, sort=False):
        latencies = group["latency_ms"].dropna().astype(float)
        if latencies.empty:
            continue
        q1 = float(latencies.quantile(0.25))
        median = float(latencies.quantile(0.50))
        q3 = float(latencies.quantile(0.75))
        iqr = q3 - q1
        lower_fence = q1 - 1.5 * iqr
        upper_fence = q3 + 1.5 * iqr
        inlier = latencies[(latencies >= lower_fence) & (latencies <= upper_fence)]
        whisker_low = float(inlier.min()) if not inlier.empty else float(latencies.min())
        whisker_high = float(inlier.max()) if not inlier.empty else float(latencies.max())
        stats.append(
            {
                x_col: str(case_axis),
                "q1": q1,
                "median": median,
                "q3": q3,
                "whisker_low": whisker_low,
                "whisker_high": whisker_high,
            }
        )
        for value in latencies[(latencies < whisker_low) | (latencies > whisker_high)]:
            outlier_rows.append({x_col: str(case_axis), "latency_ms": float(value)})

    if ax is None:
        fig, ax = plt.subplots(figsize=(14, 6))
    else:
        fig = ax.figure
    if not stats:
        ax.text(0.5, 0.5, "No request latencies found", ha="center", va="center", fontsize=14)
        ax.axis("off")
        return fig

    labels = [row[x_col] for row in stats]
    x = np.arange(len(labels))
    width = 0.42
    palette = _latency_palette(labels)
    for i, row in enumerate(stats):
        label = row[x_col]
        ax.vlines(i, row["whisker_low"] / scale, row["whisker_high"] / scale, color="#334155", linewidth=1.6)
        ax.add_patch(
            plt.Rectangle(
                (i - width / 2, row["q1"] / scale),
                width,
                max((row["q3"] - row["q1"]) / scale, 0.0001),
                facecolor=palette.get(label, "#63b3ed"),
                edgecolor="#1e3a8a",
                linewidth=1.2,
                alpha=LATENCY_FILL_ALPHA,
            )
        )
        ax.hlines(row["median"] / scale, i - width / 2, i + width / 2, color="#7c2d12", linewidth=2)

    if outlier_rows:
        outliers = pd.DataFrame(outlier_rows)
        positions = {label: i for i, label in enumerate(labels)}
        jitter = np.linspace(-0.14, 0.14, num=len(outliers)) if len(outliers) > 1 else np.array([0.0])
        xs = [positions[str(row[x_col])] + jitter[idx % len(jitter)] for idx, row in outliers.iterrows()]
        ax.scatter(xs, outliers["latency_ms"] / scale, color="#D55E00", s=22, alpha=0.75, label="outlier")

    ax.set_xticks(x)
    ax.set_xticklabels(labels, rotation=35, ha="right")
    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel(y_label)
    if outlier_rows:
        ax.legend(loc="lower right", frameon=False)
    return fig


def _draw_latency_violin(frame: pd.DataFrame, title: str, x_col: str, ax: plt.Axes | None = None) -> plt.Figure:
    if ax is None:
        fig, ax = plt.subplots(figsize=(14, 6))
    else:
        fig = ax.figure

    if "latency_ms" not in frame or x_col not in frame:
        ax.text(0.5, 0.5, "No request latencies found", ha="center", va="center", fontsize=14)
        ax.axis("off")
        return fig

    plot = frame[[x_col, "latency_ms"]].dropna().copy()
    if plot.empty:
        ax.text(0.5, 0.5, "No request latencies found", ha="center", va="center", fontsize=14)
        ax.axis("off")
        return fig

    scale, y_label, _fmt = _latency_scale(plot["latency_ms"])
    plot[x_col] = plot[x_col].astype(str)
    plot["latency"] = plot["latency_ms"].astype(float) / scale
    order = _axis_order(plot, x_col)
    palette = _latency_palette(order)
    sns.violinplot(
        data=plot,
        x=x_col,
        y="latency",
        order=order,
        hue=x_col,
        palette=palette,
        ax=ax,
        inner="box",
        cut=0,
        dodge=False,
        legend=False,
        linewidth=1.2,
    )
    for collection in ax.collections:
        collection.set_alpha(LATENCY_FILL_ALPHA)
    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel(y_label)
    ax.tick_params(axis="x", rotation=35)
    for label in ax.get_xticklabels():
        label.set_horizontalalignment("right")
    return fig


def _draw_latency_cdf(
    frame: pd.DataFrame,
    title: str,
    group_col: str | None = None,
    legend_title: str = "Group",
    legend_loc: str = "lower right",
    ax: plt.Axes | None = None,
) -> plt.Figure:
    if ax is None:
        fig, ax = plt.subplots(figsize=(14, 6))
    else:
        fig = ax.figure
    scale, x_label, _fmt = _latency_scale(frame["latency_ms"] if "latency_ms" in frame else pd.Series(dtype=float))
    groups = list(frame.groupby(group_col, observed=True, sort=False)) if group_col else [("all requests", frame)]
    fallback_colors = sns.color_palette("tab10", n_colors=max(1, len(groups)))
    plotted = False

    for idx, (label, group) in enumerate(groups):
        latencies = group["latency_ms"].dropna().astype(float)
        if latencies.empty:
            continue
        x = np.sort(latencies.to_numpy() / scale)
        y = np.arange(1, len(x) + 1) / len(x)
        color = OUTCOME_COLORS.get(str(label), SPEC_COLORS.get(str(label).split("\n", 1)[0], fallback_colors[idx]))
        ax.step(x, y, where="post", linewidth=2.2, color=color, label=_counts_label(label, group))
        plotted = True

    if not plotted:
        ax.text(0.5, 0.5, "No request latencies found", ha="center", va="center", fontsize=14)
        ax.axis("off")
        return fig

    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel(x_label)
    ax.set_ylabel("requests <= latency")
    ax.set_ylim(0, 1.01)
    ax.yaxis.set_major_formatter(PercentFormatter(1.0))
    if len(groups) > 1:
        if legend_loc == "outside":
            _outside_legend(ax, legend_title)
        else:
            ax.legend(title=legend_title, loc=legend_loc, frameon=True, framealpha=0.9, facecolor="white")
    return fig


def _with_display_columns(frame: pd.DataFrame, include_profile: bool | None = None) -> pd.DataFrame:
    if frame.empty:
        return frame.copy()
    out = frame.copy()
    if "example" in out:
        out["example_display"] = out["example"].astype(str).map(example_label)
    if "spec" in out:
        out["spec_display"] = out["spec"].astype(str).map(_spec_label)
    if "profile" in out:
        if include_profile is None:
            include_profile = out["profile"].astype(str).nunique(dropna=True) > 1
        profile = out["profile"].astype(str).map(_profile_label)
        if "spec_display" in out:
            out["case_axis"] = out["spec_display"] + ("\n" + profile if include_profile else "")
        else:
            out["case_axis"] = profile
    if "example_display" in out and "spec_display" in out:
        out["case_label"] = out["example_display"].astype(str) + " / " + out["spec_display"]
    return _sort_for_display(out)


def _sort_for_display(frame: pd.DataFrame) -> pd.DataFrame:
    if frame.empty:
        return frame
    out = frame.copy()
    sort_cols: list[str] = []
    if "example" in out:
        out["_example_order"] = out["example"].astype(str).map(lambda value: example_sort_key(value)[0])
        sort_cols.append("_example_order")
        sort_cols.append("example")
    if "spec" in out:
        out["_spec_order"] = out["spec"].astype(str).map(_spec_order)
        sort_cols.append("_spec_order")
    if "profile" in out:
        sort_cols.append("profile")
    if sort_cols:
        out = out.sort_values(sort_cols)
    return out.drop(columns=["_example_order", "_spec_order"], errors="ignore")


def _axis_order(frame: pd.DataFrame, column: str = "case_axis") -> list[str]:
    if frame.empty or column not in frame:
        return []
    ordered = _sort_for_display(frame).drop_duplicates([column])
    return [str(value) for value in ordered[column].tolist()]


def _example_axis_order(frame: pd.DataFrame) -> list[str]:
    if frame.empty or "example_display" not in frame:
        return []
    ordered = _sort_for_display(frame).drop_duplicates(["example_display"])
    return [str(value) for value in ordered["example_display"].tolist()]


def _spec_hue_order(frame: pd.DataFrame) -> list[str]:
    if frame.empty or "spec_display" not in frame:
        return []
    labels = frame[["spec", "spec_display"]].drop_duplicates().sort_values("spec", key=lambda s: s.astype(str).map(_spec_order))
    return [str(value) for value in labels["spec_display"].tolist()]


def _spec_palette(frame: pd.DataFrame | None = None) -> dict[str, str]:
    labels = _spec_hue_order(frame) if frame is not None else list(SPEC_COLORS)
    return {label: SPEC_COLORS.get(label, "#666666") for label in labels}


def _spec_label(value: Any) -> str:
    text = str(value)
    return SPEC_LABELS.get(text, text.replace("_", " ").replace("-", " ").title())


def _profile_label(value: Any) -> str:
    return str(value).replace("_", "-")


def _spec_order(value: Any) -> int:
    text = str(value)
    if text in SPEC_DISPLAY_ORDER:
        return SPEC_DISPLAY_ORDER.index(text)
    return len(SPEC_DISPLAY_ORDER) + sorted(SPEC_LABELS).index(text) if text in SPEC_LABELS else len(SPEC_DISPLAY_ORDER) + 99


def _latency_scale(values: pd.Series | list[Any] | np.ndarray) -> tuple[float, str, str]:
    return 1_000.0, "latency (s)", "{:.1f}"


def _scaled_latency(frame: pd.DataFrame, column: str = "latency_ms", target: str = "latency") -> tuple[pd.DataFrame, str, str]:
    out = frame.copy()
    scale, label, fmt = _latency_scale(out[column] if column in out else pd.Series(dtype=float))
    out[target] = out[column] / scale if column in out else []
    return out, label, fmt


def _format_latency(value: Any, scale: float, fmt: str) -> str:
    if pd.isna(value):
        return ""
    return fmt.format(float(value) / scale)


def _format_count_axis(ax: plt.Axes, divisor: float, suffix: str) -> None:
    ax.xaxis.set_major_formatter(FuncFormatter(lambda value, _pos: f"{value / divisor:g}{suffix}"))


def _format_y_count_axis(ax: plt.Axes, divisor: float, suffix: str) -> None:
    ax.yaxis.set_major_formatter(FuncFormatter(lambda value, _pos: f"{value / divisor:g}{suffix}"))


def _short_case_label(value: Any) -> str:
    return example_case_label(value)


def _short_component(value: Any) -> str:
    text = str(value)
    text = text.replace("AgentClient_", " client: ")
    text = text.replace("AgentServer_", " server: ")
    text = text.replace("CoordinatorServer_", " coordinator: ")
    text = text.replace("FinancialAnalyzer", "Financial analyzer")
    text = text.replace("ResearchQualityController", "Research QC")
    text = text.replace("MarketingCoordinator", "Marketing")
    text = text.replace("TravelCoordinator", "Travel")
    text = text.replace("WeatherAgent", "Weather")
    text = text.replace("llm.call", "LLM call")
    text = text.replace("llm.tool_call", "LLM tool call")
    text = text.replace("mcp.tool_call", "MCP tool call")
    text = text.replace("tool.", "tool: ")
    text = text.replace("_", " ")
    return " ".join(text.split())


def _short_agent(value: Any) -> str:
    text = str(value)
    if text == "Unattributed":
        return text
    return re.sub(r"(?<!^)(?=[A-Z])", " ", text).replace(" Q C", " QC")


def _error_label(value: Any) -> str:
    text = str(value)
    return ERROR_LABELS.get(text, text.replace("_", " ").title())


def _counts_label(label: Any, group: pd.DataFrame) -> str:
    count = len(group)
    if "ok" not in group:
        return f"{label} (n={count})"
    errors = int((~group["ok"].fillna(False).astype(bool)).sum())
    if errors:
        return f"{label} (n={count}, errors={errors})"
    return f"{label} (n={count})"


def _save(
    fig: plt.Figure,
    path: Path,
    index: dict[str, Any] | None = None,
    section: str | None = None,
    title: str | None = None,
) -> str:
    path.parent.mkdir(parents=True, exist_ok=True)
    fig.tight_layout()
    fig.savefig(path, dpi=180, bbox_inches="tight")
    plt.close(fig)
    rel = _relative_plot_path(path, index)
    if index is not None and section is not None and title is not None:
        index["sections"][section].append({"title": title, "path": rel.replace("\\", "/")})
    return rel.replace("\\", "/")


def _relative_plot_path(path: Path, index: dict[str, Any] | None) -> str:
    out_dir = index.get("_out_dir") if index is not None else None
    if isinstance(out_dir, Path):
        try:
            return path.resolve().relative_to(out_dir.resolve()).as_posix()
        except ValueError:
            pass
    return path.name


def _placeholder(path: Path, text: str) -> None:
    fig, ax = plt.subplots(figsize=(8, 3))
    ax.text(0.5, 0.5, text, ha="center", va="center", fontsize=14)
    ax.axis("off")
    fig.savefig(path, dpi=160, bbox_inches="tight")
    plt.close(fig)


def _blank_figure(text: str) -> plt.Figure:
    fig, ax = plt.subplots(figsize=(8, 3))
    ax.text(0.5, 0.5, text, ha="center", va="center", fontsize=14)
    ax.axis("off")
    return fig


def _actual(frame: pd.DataFrame) -> pd.DataFrame:
    if frame.empty:
        return frame
    out = frame.copy()
    for col in ["spec", "profile"]:
        if col in out.columns:
            out[col] = out[col].astype(str)
    return out


def _groups(frame: pd.DataFrame, column: str) -> dict[str, pd.DataFrame]:
    if frame.empty or column not in frame:
        return {}
    return {str(key): group for key, group in frame.groupby(column, observed=True, sort=False)}


def _set_style() -> None:
    sns.set_theme(
        style="whitegrid",
        context="notebook",
        rc={
            "figure.facecolor": "white",
            "axes.facecolor": "white",
            "axes.edgecolor": "#d9dee8",
            "grid.color": "#eef2f7",
            "axes.titleweight": "bold",
            "axes.titlesize": 14,
            "axes.labelsize": 12,
            "legend.title_fontsize": 11,
            "legend.fontsize": 10,
            "font.family": "DejaVu Sans",
        },
    )


def _outside_legend(ax: plt.Axes, title: str) -> None:
    handles, labels = ax.get_legend_handles_labels()
    if handles:
        ax.legend(handles, labels, title=title, loc="center left", bbox_to_anchor=(1.01, 0.5), frameon=False)


def _slug(value: str) -> str:
    safe = []
    for ch in str(value).lower():
        if ch.isalnum():
            safe.append(ch)
        elif ch in {"-", "_", "."}:
            safe.append(ch)
        else:
            safe.append("-")
    return "".join(safe).strip("-") or "item"
