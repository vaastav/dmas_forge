from __future__ import annotations

from collections import defaultdict
from pathlib import Path
from typing import Any

import matplotlib

matplotlib.use("Agg")

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns
from matplotlib.ticker import FuncFormatter, PercentFormatter

from .data import BenchmarkRun, write_normalized_data
from .labels import example_case_label, example_label, example_sort_key
from .topology import EXAMPLES
from .topology_draw import draw_topology

PLOT_DIRS = [
    "overview",
    "performance",
    "reliability",
    "tokens",
    "resources",
    "traces",
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
    _token_plots(data, out_dir, index)
    _resource_plots(data, out_dir, index)
    _trace_plots(data, out_dir, index)
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

    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(
        data=cases,
        x="example_display",
        y="throughput_rps",
        hue="spec_display",
        hue_order=_spec_hue_order(cases),
        ax=ax,
        palette=_spec_palette(cases),
        errorbar=None,
        order=_example_axis_order(cases),
    )
    ax.set_title("Throughput by Example and Protocol", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("requests / second")
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

    p95 = cases.copy()
    scale, label, fmt = _latency_scale(p95["p95_ms"])
    p95["p95_display"] = p95["p95_ms"] / scale
    matrix = p95.pivot_table(index="example_display", columns="case_axis", values="p95_display", aggfunc="mean", observed=True)
    matrix = matrix.reindex(columns=[col for col in _axis_order(p95) if col in matrix.columns])
    matrix = matrix.reindex(index=[row for row in _example_axis_order(p95) if row in matrix.index])
    fig, ax = plt.subplots(figsize=(16, max(4, 0.52 * len(matrix.index) + 2)))
    annot = matrix.map(lambda v: "" if pd.isna(v) else fmt.format(v))
    sns.heatmap(
        matrix,
        ax=ax,
        cmap="YlOrRd",
        linewidths=0.7,
        linecolor="white",
        annot=annot,
        fmt="",
        cbar_kws={"label": label.replace("latency", "p95 latency")},
    )
    ax.set_title("P95 End-to-End Latency by Example and Protocol", loc="left", pad=16)
    ax.set_xlabel("")
    ax.set_ylabel("")
    _save(fig, out_dir / "performance" / "p95_latency_heatmap.png", index, "performance", "P95 latency heatmap")

    if not requests.empty:
        fig = _draw_latency_cdf(
            requests,
            title="End-to-End Request Latency CDF by Example",
            group_col="example_display",
            legend_title="Example",
            legend_loc="lower right",
        )
        _save(
            fig,
            out_dir / "performance" / "request_latency_cdf_by_example.png",
            index,
            "performance",
            "Request latency CDF by example",
        )

        for example, frame in requests.groupby("example", observed=True, sort=False):
            example_display = example_label(example)
            fig, ax = plt.subplots(figsize=(14, 6))
            frame = frame.copy()
            frame, latency_label, _fmt = _scaled_latency(frame)
            frame["outcome"] = frame["ok"].map(OUTCOME_LABELS).fillna("unknown")
            sns.stripplot(
                data=frame,
                x="case_axis",
                y="latency",
                hue="outcome",
                ax=ax,
                palette=OUTCOME_COLORS,
                dodge=True,
                alpha=0.75,
                size=4,
                order=_axis_order(frame),
            )
            ax.set_title(f"{example_display}: End-to-End Request Latency", loc="left", pad=14)
            ax.set_xlabel("")
            ax.set_ylabel(latency_label)
            ax.tick_params(axis="x", rotation=0)
            _outside_legend(ax, "Outcome")
            _save(
                fig,
                out_dir / "performance" / f"request_latency_{_slug(example)}.png",
                index,
                "performance",
                f"{example_display} request latency",
            )
            fig = _draw_latency_distribution(
                frame,
                title=f"{example_display}: End-to-End Request Latency Distribution",
                x_col="case_axis",
            )
            _save(
                fig,
                out_dir / "performance" / f"request_latency_range_{_slug(example)}.png",
                index,
                "performance",
                f"{example_display} request latency range",
            )
            fig = _draw_latency_cdf(
                frame,
                title=f"{example_display}: End-to-End Request Latency CDF",
                group_col="case_axis",
                legend_title="Protocol",
            )
            _save(
                fig,
                out_dir / "performance" / f"request_latency_cdf_{_slug(example)}.png",
                index,
                "performance",
                f"{example_display} request latency CDF",
            )


def _reliability_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    if cases.empty:
        return
    plot = cases[["case_name", "example", "spec", "profile", "successes", "errors"]].copy()
    plot = _with_display_columns(plot)
    fig, ax = plt.subplots(figsize=(16, max(7, len(plot) * 0.23)))
    y = np.arange(len(plot))
    ax.barh(y, plot["successes"], color=OUTCOME_COLORS["success"], label="successes")
    ax.barh(y, plot["errors"], left=plot["successes"], color=OUTCOME_COLORS["failed / timeout"], label="failed / timeout")
    ax.set_yticks(y)
    ax.set_yticklabels(plot["case_name"].map(_short_case_label), fontsize=8)
    ax.invert_yaxis()
    ax.set_title("Request Outcomes by Example and Protocol", loc="left", pad=14)
    ax.set_xlabel("request count")
    max_requests = float((plot["successes"] + plot["errors"]).max())
    ax.set_xlim(0, max(1.0, max_requests * 1.08))
    _outside_legend(ax, "Outcome")
    _save(fig, out_dir / "reliability" / "success_error_stacked.png", index, "reliability", "Success and error stack")

    errors = data.errors
    if not errors.empty:
        by_category = errors.groupby("error_category", observed=True)["count"].sum().reset_index()
        by_category = by_category.sort_values("count", ascending=False)
        by_category["error_label"] = by_category["error_category"].map(_error_label)
        fig, ax = plt.subplots(figsize=(11, 5.5))
        sns.barplot(data=by_category, y="error_label", x="count", ax=ax, color=OUTCOME_COLORS["failed / timeout"])
        ax.bar_label(ax.containers[0], padding=3)
        ax.set_title("Failure Categories", loc="left", pad=14)
        ax.set_xlabel("failed requests")
        ax.set_ylabel("")
        _save(fig, out_dir / "reliability" / "error_taxonomy.png", index, "reliability", "Error taxonomy")

        by_spec = errors.groupby(["example", "spec"], observed=True)["count"].sum().reset_index()
        by_spec = _with_display_columns(by_spec)
        fig, ax = plt.subplots(figsize=(11, 5.5))
        sns.barplot(
            data=by_spec,
            x="example_display",
            y="count",
            hue="spec_display",
            hue_order=_spec_hue_order(by_spec),
            ax=ax,
            palette=_spec_palette(by_spec),
            errorbar=None,
            order=_example_axis_order(by_spec),
        )
        ax.set_title("Failures by Example and Protocol", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("failed requests")
        ax.tick_params(axis="x", rotation=15)
        _outside_legend(ax, "Protocol")
        _save(fig, out_dir / "reliability" / "failures_by_spec.png", index, "reliability", "Failures by spec")


def _token_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    if cases.empty:
        return
    token_cases = cases.sort_values("total_tokens", ascending=False).copy()
    fig, ax = plt.subplots(figsize=(15, max(7, len(token_cases) * 0.22)))
    y = np.arange(len(token_cases))
    ax.barh(y, token_cases["input_tokens"], color="#3182ce", label="input")
    ax.barh(y, token_cases["output_tokens"], left=token_cases["input_tokens"], color="#805ad5", label="output")
    ax.set_yticks(y)
    ax.set_yticklabels(token_cases["case_name"].map(_short_case_label), fontsize=8)
    ax.invert_yaxis()
    ax.set_title("Input and Output Tokens by Case", loc="left", pad=14)
    ax.set_xlabel("tokens (K)")
    _format_count_axis(ax, 1_000, "K")
    ax.legend(loc="lower right", frameon=False)
    _save(fig, out_dir / "tokens" / "tokens_by_case.png", index, "tokens", "Tokens by case")

    fig, ax = plt.subplots(figsize=(13, 6))
    plot = cases[cases["tokens_per_success"] > 0].copy()
    plot["tokens_per_success_k"] = plot["tokens_per_success"] / 1_000
    sns.barplot(
        data=plot,
        x="example_display",
        y="tokens_per_success_k",
        hue="spec_display",
        hue_order=_spec_hue_order(plot),
        ax=ax,
        palette=_spec_palette(plot),
        errorbar=None,
        order=_example_axis_order(plot),
    )
    ax.set_title("Tokens per Successful Request", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("K tokens / successful request")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Protocol")
    _save(fig, out_dir / "tokens" / "tokens_per_success.png", index, "tokens", "Tokens per success")

    components = data.components
    if not components.empty:
        comp_tokens = components.groupby("component", observed=True)["total_tokens"].sum().reset_index()
        comp_tokens = comp_tokens.sort_values("total_tokens", ascending=False).head(25)
        comp_tokens["component_label"] = comp_tokens["component"].map(_short_component)
        comp_tokens["total_tokens_k"] = comp_tokens["total_tokens"] / 1_000
        fig, ax = plt.subplots(figsize=(11, max(5, len(comp_tokens) * 0.32)))
        sns.barplot(data=comp_tokens, y="component_label", x="total_tokens_k", ax=ax, color="#4C78A8")
        ax.set_title("Top Components by Token Usage", loc="left", pad=14)
        ax.set_xlabel("tokens (K)")
        ax.set_ylabel("")
        _save(fig, out_dir / "tokens" / "component_tokens.png", index, "tokens", "Component token usage")

        comp_duration = components.groupby("component", observed=True)["duration_ms"].sum().reset_index()
        comp_duration = comp_duration.sort_values("duration_ms", ascending=False).head(25)
        comp_duration["component_label"] = comp_duration["component"].map(_short_component)
        comp_duration["duration_s"] = comp_duration["duration_ms"] / 1_000
        fig, ax = plt.subplots(figsize=(11, max(5, len(comp_duration) * 0.32)))
        sns.barplot(data=comp_duration, y="component_label", x="duration_s", ax=ax, color="#72B7B2")
        ax.set_title("Top Components by Span Duration", loc="left", pad=14)
        ax.set_xlabel("aggregate span duration (s)")
        ax.set_ylabel("")
        _save(fig, out_dir / "tokens" / "component_duration.png", index, "tokens", "Component duration")

    spans = data.spans
    if not spans.empty:
        ops = spans[spans["operation"].astype(str).str.contains("llm|tool", case=False, regex=True)]
        if not ops.empty:
            counts = ops.groupby(["example", "operation"], observed=True).size().reset_index(name="spans")
            counts = _with_display_columns(counts)
            counts["operation_label"] = counts["operation"].map(_short_component)
            fig, ax = plt.subplots(figsize=(13, 6))
            sns.barplot(data=counts, x="example_display", y="spans", hue="operation_label", ax=ax, palette="tab20", errorbar=None, order=_example_axis_order(counts))
            ax.set_title("LLM and Tool Span Counts", loc="left", pad=14)
            ax.set_xlabel("")
            ax.set_ylabel("span count")
            ax.tick_params(axis="x", rotation=15)
            _outside_legend(ax, "Operation")
            _save(fig, out_dir / "tokens" / "llm_tool_spans.png", index, "tokens", "LLM and tool spans")


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


def _trace_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    traces = data.traces
    spans = data.spans
    cases = _with_display_columns(_actual(data.cases))
    if cases.empty:
        return
    trace_counts = traces.groupby("case_name", observed=True).size().reset_index(name="trace_count") if not traces.empty else pd.DataFrame(columns=["case_name", "trace_count"])
    plot = cases.merge(trace_counts, on="case_name", how="left").fillna({"trace_count": 0})
    fig, ax = plt.subplots(figsize=(15, max(7, len(plot) * 0.22)))
    y = np.arange(len(plot))
    ax.barh(y, plot["requests"], color="#cbd5e0", label="requests")
    ax.barh(y, plot["trace_count"], color="#0072B2", label="traces")
    ax.set_yticks(y)
    ax.set_yticklabels(plot["case_name"].map(_short_case_label), fontsize=8)
    ax.invert_yaxis()
    ax.set_title("Trace Count vs Request Count", loc="left", pad=14)
    ax.set_xlabel("count")
    ax.legend(loc="lower right", frameon=False)
    _save(fig, out_dir / "traces" / "trace_coverage.png", index, "traces", "Trace coverage")

    if not traces.empty:
        trace_plot = _with_display_columns(traces)
        fig, ax = plt.subplots(figsize=(10, 5.5))
        sns.histplot(
            data=trace_plot,
            x="span_count",
            hue="example_display",
            hue_order=_example_axis_order(trace_plot),
            multiple="stack",
            ax=ax,
            palette="tab10",
            bins=20,
        )
        ax.set_title("Spans per Trace Distribution", loc="left", pad=14)
        ax.set_xlabel("spans per trace")
        ax.set_ylabel("trace count")
        _save(fig, out_dir / "traces" / "spans_per_trace.png", index, "traces", "Spans per trace")

    if not spans.empty:
        span_plot = spans.copy()
        span_plot["operation_label"] = span_plot["operation"].map(_short_component)
        span_plot["service_label"] = span_plot["service"].map(_short_service)
        op_service = span_plot.groupby(["operation_label", "service_label"], observed=True).size().reset_index(name="spans")
        op_service = op_service.sort_values("spans", ascending=False).head(45)
        matrix = op_service.pivot_table(index="operation_label", columns="service_label", values="spans", aggfunc="sum", fill_value=0, observed=True)
        fig, ax = plt.subplots(figsize=(14, max(6, 0.34 * len(matrix.index))))
        sns.heatmap(matrix, ax=ax, cmap="Blues", linewidths=0.4, linecolor="white", cbar_kws={"label": "spans"})
        ax.set_title("Operation by Service Span Heatmap", loc="left", pad=14)
        ax.set_xlabel("service")
        ax.set_ylabel("operation")
        _save(fig, out_dir / "traces" / "operation_service_heatmap.png", index, "traces", "Operation/service heatmap")


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
        fig = draw_topology(example, spec, meta)
        path = out_dir / "topology" / f"{_slug(example)}_{_slug(spec)}.svg"
        _save(fig, path, index, "topology", f"{example_label(example)} {spec} topology")


def _example_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _with_display_columns(_actual(data.cases))
    requests = _with_display_columns(_actual(data.requests))
    components = _actual(data.components)
    expected = _with_display_columns(_actual(data.expected_cases))
    if cases.empty:
        return

    examples: list[dict[str, Any]] = []
    expected_by_example = _groups(expected, "example")
    requests_by_example = _groups(requests, "example")
    components_by_example = _groups(components, "example")
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
        sns.barplot(data=ex_cases, x="case_axis", y="throughput_rps", hue="spec_display", hue_order=_spec_hue_order(ex_cases), ax=ax, palette=_spec_palette(ex_cases), errorbar=None, order=_axis_order(ex_cases))
        ax.set_title(f"{example_display}: Throughput", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("requests / second")
        ax.tick_params(axis="x", rotation=0)
        _outside_legend(ax, "Protocol")
        entry["plots"].append({"title": "Throughput", "path": _save(fig, example_dir / "throughput.png", index)})

        token_plot = ex_cases.sort_values("total_tokens", ascending=False)
        fig, ax = plt.subplots(figsize=(14, 6))
        token_plot["total_tokens_k"] = token_plot["total_tokens"] / 1_000
        sns.barplot(data=token_plot, x="case_axis", y="total_tokens_k", hue="spec_display", hue_order=_spec_hue_order(token_plot), ax=ax, palette=_spec_palette(token_plot), errorbar=None, order=_axis_order(token_plot), legend=False)
        ax.set_title(f"{example_display}: Total Tokens", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("tokens (K)")
        ax.tick_params(axis="x", rotation=0)
        entry["plots"].append({"title": "Total tokens", "path": _save(fig, example_dir / "tokens.png", index)})

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
            entry["plots"].append({"title": "Request latency range", "path": _save(fig, example_dir / "request_latency_range.png", index)})

            fig = _draw_latency_cdf(
                ex_requests,
                title=f"{example_display}: End-to-End Request Latency CDF",
                group_col="case_axis",
                legend_title="Protocol",
            )
            entry["plots"].append({"title": "Request latency CDF", "path": _save(fig, example_dir / "request_latency_cdf.png", index)})

        ex_components = components_by_example.get(example, empty)
        if not ex_components.empty:
            comp = ex_components.groupby("component", observed=True).agg(duration_ms=("duration_ms", "sum"), total_tokens=("total_tokens", "sum")).reset_index()
            comp = comp.sort_values("duration_ms", ascending=False).head(18)
            comp["component_label"] = comp["component"].map(_short_component)
            comp["duration_s"] = comp["duration_ms"] / 1_000
            fig, ax = plt.subplots(figsize=(11, max(5, len(comp) * 0.35)))
            sns.barplot(data=comp, y="component_label", x="duration_s", ax=ax, color="#72B7B2")
            ax.set_title(f"{example_display}: Component Duration", loc="left", pad=14)
            ax.set_xlabel("aggregate span duration (s)")
            ax.set_ylabel("")
            entry["plots"].append({"title": "Component duration", "path": _save(fig, example_dir / "component_duration.png", index)})

        examples.append(entry)

    index["examples"] = examples


def _case_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any], max_case_waterfalls: int) -> None:
    cases = _with_display_columns(_actual(data.cases))
    requests = _with_display_columns(data.requests)
    resources = data.resources
    spans = data.spans
    components = data.components
    waterfalls = 0
    requests_by_case = _groups(requests, "case_name")
    resources_by_case = _groups(resources, "case_name")
    spans_by_case = _groups(spans, "case_name")
    components_by_case = _groups(components, "case_name")
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
            case_entry["plots"].append(_save(fig, case_dir / "request_latency.png", index))

            fig = _draw_latency_cdf(
                req,
                title=f"{_short_case_label(case)}: Request Latency CDF",
                group_col="outcome",
                legend_title="Outcome",
            )
            case_entry["plots"].append(_save(fig, case_dir / "request_latency_cdf.png", index))

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
            case_entry["plots"].append(_save(fig, case_dir / "resources.png", index))

        comp = components_by_case.get(case, empty)
        if not comp.empty:
            comp = comp.sort_values("duration_ms", ascending=False).head(18)
            comp = comp.copy()
            comp["component_label"] = comp["component"].map(_short_component)
            comp["duration_s"] = comp["duration_ms"] / 1_000
            fig, ax = plt.subplots(figsize=(9, max(4, len(comp) * 0.32)))
            sns.barplot(data=comp, y="component_label", x="duration_s", ax=ax, color="#72B7B2")
            ax.set_title(f"{_short_case_label(case)}: Component Duration", loc="left", pad=12)
            ax.set_xlabel("duration (s)")
            ax.set_ylabel("")
            case_entry["plots"].append(_save(fig, case_dir / "components.png", index))

        span_case = spans_by_case.get(case, empty)
        if not span_case.empty and waterfalls < max_case_waterfalls:
            fig = _draw_longest_waterfall(case, span_case)
            if fig is not None:
                waterfalls += 1
                case_entry["plots"].append(_save(fig, case_dir / "longest_trace_waterfall.png", index))

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
    frame["service_label"] = frame["service"].map(_short_service)
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


def _draw_latency_distribution(frame: pd.DataFrame, title: str, x_col: str) -> plt.Figure:
    scale, y_label, fmt = _latency_scale(frame["latency_ms"] if "latency_ms" in frame else pd.Series(dtype=float))
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

    fig, ax = plt.subplots(figsize=(14, 6))
    if not stats:
        ax.text(0.5, 0.5, "No request latencies found", ha="center", va="center", fontsize=14)
        ax.axis("off")
        return fig

    labels = [row[x_col] for row in stats]
    x = np.arange(len(labels))
    width = 0.42
    for i, row in enumerate(stats):
        ax.vlines(i, row["whisker_low"] / scale, row["whisker_high"] / scale, color="#334155", linewidth=1.6)
        ax.add_patch(
            plt.Rectangle(
                (i - width / 2, row["q1"] / scale),
                width,
                max((row["q3"] - row["q1"]) / scale, 0.0001),
                facecolor="#63b3ed",
                edgecolor="#1e3a8a",
                linewidth=1.2,
                alpha=0.9,
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
        ax.legend(loc="upper right", frameon=False)
    return fig


def _draw_latency_cdf(
    frame: pd.DataFrame,
    title: str,
    group_col: str | None = None,
    legend_title: str = "Group",
    legend_loc: str = "outside",
) -> plt.Figure:
    fig, ax = plt.subplots(figsize=(14, 6))
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


def _short_service(value: Any) -> str:
    text = str(value)
    text = text.replace("unknown_service:", "")
    text = text.removesuffix("_proc")
    text = text.replace("_service", "")
    return text.replace("_", " ")


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
