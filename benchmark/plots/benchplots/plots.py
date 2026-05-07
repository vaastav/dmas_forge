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
from matplotlib.ticker import PercentFormatter

from .data import BenchmarkRun, write_normalized_data
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
    merged["column"] = merged["spec"].astype(str) + "\n" + merged["profile"].astype(str)
    matrix = merged.pivot_table(
        index="example",
        columns="column",
        values="success_rate",
        aggfunc="first",
        observed=True,
    )
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
    ax.set_title("Success Rate by Example, Spec, and Profile", loc="left", pad=16)
    ax.set_xlabel("")
    ax.set_ylabel("")
    _save(fig, out_dir / "overview" / "success_rate_heatmap.png", index, "overview", "Success-rate heatmap")


def _performance_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _actual(data.cases)
    requests = _actual(data.requests)
    if cases.empty:
        return

    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(data=cases, x="example", y="throughput_rps", hue="spec", ax=ax, palette="muted", errorbar=None)
    ax.set_title("Throughput by Example and Spec", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("requests / second")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Spec")
    _save(fig, out_dir / "performance" / "throughput_by_spec.png", index, "performance", "Throughput by spec")

    latency = cases.melt(
        id_vars=["case_name", "example", "spec", "profile"],
        value_vars=["p50_ms", "p95_ms", "p99_ms"],
        var_name="percentile",
        value_name="latency_ms",
    )
    fig, ax = plt.subplots(figsize=(14, 7))
    sns.barplot(data=latency, x="example", y="latency_ms", hue="percentile", ax=ax, palette="rocket", errorbar=None)
    ax.set_title("Latency Percentiles Across Examples", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("latency (ms)")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Percentile")
    _save(fig, out_dir / "performance" / "latency_percentiles.png", index, "performance", "Latency percentiles")

    p95 = cases.copy()
    p95["column"] = p95["spec"].astype(str) + "\n" + p95["profile"].astype(str)
    matrix = p95.pivot_table(index="example", columns="column", values="p95_ms", aggfunc="mean", observed=True)
    fig, ax = plt.subplots(figsize=(16, max(4, 0.52 * len(matrix.index) + 2)))
    annot = matrix.map(lambda v: "" if pd.isna(v) else f"{v:,.0f}")
    sns.heatmap(
        matrix,
        ax=ax,
        cmap="RdYlGn_r",
        linewidths=0.7,
        linecolor="white",
        annot=annot,
        fmt="",
        cbar_kws={"label": "p95 ms (green is faster, red is slower)"},
    )
    ax.set_title("P95 Latency Heatmap", loc="left", pad=16)
    ax.set_xlabel("")
    ax.set_ylabel("")
    _save(fig, out_dir / "performance" / "p95_latency_heatmap.png", index, "performance", "P95 latency heatmap")

    if not requests.empty:
        fig = _draw_latency_cdf(
            requests,
            title="Request Latency CDF by Example",
            group_col="example",
            legend_title="Example",
        )
        _save(
            fig,
            out_dir / "performance" / "request_latency_cdf_by_example.png",
            index,
            "performance",
            "Request latency CDF by example",
        )

        for example, frame in requests.groupby("example", observed=True):
            fig, ax = plt.subplots(figsize=(14, 6))
            frame = frame.copy()
            frame["case_axis"] = frame["spec"].astype(str) + "\n" + frame["profile"].astype(str)
            sns.stripplot(
                data=frame,
                x="case_axis",
                y="latency_ms",
                hue="ok",
                ax=ax,
                palette={True: "#2f855a", False: "#c53030"},
                dodge=True,
                alpha=0.75,
                size=4,
            )
            ax.set_title(f"{example}: Per-Request Latency", loc="left", pad=14)
            ax.set_xlabel("")
            ax.set_ylabel("latency (ms)")
            ax.tick_params(axis="x", rotation=35)
            _outside_legend(ax, "OK")
            _save(
                fig,
                out_dir / "performance" / f"request_latency_{_slug(example)}.png",
                index,
                "performance",
                f"{example} request latency",
            )
            fig = _draw_latency_distribution(
                frame,
                title=f"{example}: Request Latency Distribution",
                x_col="case_axis",
            )
            _save(
                fig,
                out_dir / "performance" / f"request_latency_range_{_slug(example)}.png",
                index,
                "performance",
                f"{example} request latency range",
            )
            fig = _draw_latency_cdf(
                frame,
                title=f"{example}: Request Latency CDF",
                group_col="case_axis",
                legend_title="Spec / profile",
            )
            _save(
                fig,
                out_dir / "performance" / f"request_latency_cdf_{_slug(example)}.png",
                index,
                "performance",
                f"{example} request latency CDF",
            )


def _reliability_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _actual(data.cases)
    if cases.empty:
        return
    plot = cases[["case_name", "example", "spec", "profile", "successes", "errors"]].copy()
    plot = plot.sort_values(["example", "spec", "profile"])
    fig, ax = plt.subplots(figsize=(16, max(7, len(plot) * 0.23)))
    y = np.arange(len(plot))
    ax.barh(y, plot["successes"], color="#2f855a", label="successes")
    ax.barh(y, plot["errors"], left=plot["successes"], color="#c53030", label="errors")
    ax.set_yticks(y)
    ax.set_yticklabels(plot["case_name"], fontsize=8)
    ax.invert_yaxis()
    ax.set_title("Successes and Errors by Case", loc="left", pad=14)
    ax.set_xlabel("request count")
    ax.legend(loc="lower right")
    _save(fig, out_dir / "reliability" / "success_error_stacked.png", index, "reliability", "Success and error stack")

    errors = data.errors
    if not errors.empty:
        by_category = errors.groupby("error_category", observed=True)["count"].sum().reset_index()
        by_category = by_category.sort_values("count", ascending=False)
        fig, ax = plt.subplots(figsize=(11, 5.5))
        sns.barplot(data=by_category, y="error_category", x="count", ax=ax, palette="flare", hue="error_category", legend=False)
        ax.set_title("Error Taxonomy", loc="left", pad=14)
        ax.set_xlabel("failed requests")
        ax.set_ylabel("")
        _save(fig, out_dir / "reliability" / "error_taxonomy.png", index, "reliability", "Error taxonomy")

        by_spec = errors.groupby(["example", "spec"], observed=True)["count"].sum().reset_index()
        fig, ax = plt.subplots(figsize=(11, 5.5))
        sns.barplot(data=by_spec, x="example", y="count", hue="spec", ax=ax, palette="Reds", errorbar=None)
        ax.set_title("Failures by Example and Spec", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("failed requests")
        ax.tick_params(axis="x", rotation=15)
        _outside_legend(ax, "Spec")
        _save(fig, out_dir / "reliability" / "failures_by_spec.png", index, "reliability", "Failures by spec")


def _token_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _actual(data.cases)
    if cases.empty:
        return
    token_cases = cases.sort_values("total_tokens", ascending=False).copy()
    fig, ax = plt.subplots(figsize=(15, max(7, len(token_cases) * 0.22)))
    y = np.arange(len(token_cases))
    ax.barh(y, token_cases["input_tokens"], color="#3182ce", label="input")
    ax.barh(y, token_cases["output_tokens"], left=token_cases["input_tokens"], color="#805ad5", label="output")
    ax.set_yticks(y)
    ax.set_yticklabels(token_cases["case_name"], fontsize=8)
    ax.invert_yaxis()
    ax.set_title("Input and Output Tokens by Case", loc="left", pad=14)
    ax.set_xlabel("tokens")
    ax.legend(loc="lower right")
    _save(fig, out_dir / "tokens" / "tokens_by_case.png", index, "tokens", "Tokens by case")

    fig, ax = plt.subplots(figsize=(13, 6))
    plot = cases[cases["tokens_per_success"] > 0].copy()
    sns.barplot(data=plot, x="example", y="tokens_per_success", hue="spec", ax=ax, palette="Purples", errorbar=None)
    ax.set_title("Tokens per Successful Request", loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("tokens / successful request")
    ax.tick_params(axis="x", rotation=15)
    _outside_legend(ax, "Spec")
    _save(fig, out_dir / "tokens" / "tokens_per_success.png", index, "tokens", "Tokens per success")

    components = data.components
    if not components.empty:
        comp_tokens = components.groupby("component", observed=True)["total_tokens"].sum().reset_index()
        comp_tokens = comp_tokens.sort_values("total_tokens", ascending=False).head(25)
        fig, ax = plt.subplots(figsize=(11, max(5, len(comp_tokens) * 0.32)))
        sns.barplot(data=comp_tokens, y="component", x="total_tokens", ax=ax, palette="viridis", hue="component", legend=False)
        ax.set_title("Top Components by Token Usage", loc="left", pad=14)
        ax.set_xlabel("tokens")
        ax.set_ylabel("")
        _save(fig, out_dir / "tokens" / "component_tokens.png", index, "tokens", "Component token usage")

        comp_duration = components.groupby("component", observed=True)["duration_ms"].sum().reset_index()
        comp_duration = comp_duration.sort_values("duration_ms", ascending=False).head(25)
        fig, ax = plt.subplots(figsize=(11, max(5, len(comp_duration) * 0.32)))
        sns.barplot(data=comp_duration, y="component", x="duration_ms", ax=ax, palette="crest", hue="component", legend=False)
        ax.set_title("Top Components by Span Duration", loc="left", pad=14)
        ax.set_xlabel("aggregate duration (ms)")
        ax.set_ylabel("")
        _save(fig, out_dir / "tokens" / "component_duration.png", index, "tokens", "Component duration")

    spans = data.spans
    if not spans.empty:
        ops = spans[spans["operation"].astype(str).str.contains("llm|tool", case=False, regex=True)]
        if not ops.empty:
            counts = ops.groupby(["example", "operation"], observed=True).size().reset_index(name="spans")
            fig, ax = plt.subplots(figsize=(13, 6))
            sns.barplot(data=counts, x="example", y="spans", hue="operation", ax=ax, palette="tab20", errorbar=None)
            ax.set_title("LLM and Tool Span Counts", loc="left", pad=14)
            ax.set_xlabel("")
            ax.set_ylabel("span count")
            ax.tick_params(axis="x", rotation=15)
            _outside_legend(ax, "Operation")
            _save(fig, out_dir / "tokens" / "llm_tool_spans.png", index, "tokens", "LLM and tool spans")


def _resource_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _actual(data.cases)
    resources = data.resources
    if cases.empty:
        return
    resource_metrics = cases.melt(
        id_vars=["case_name", "example", "spec", "profile"],
        value_vars=["cpu_avg_percent", "cpu_max_percent"],
        var_name="metric",
        value_name="cpu_percent",
    )
    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(data=resource_metrics, x="example", y="cpu_percent", hue="metric", ax=ax, palette="YlGnBu", errorbar=None)
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
        id_vars=["case_name", "example", "spec", "profile"],
        value_vars=["memory_avg_mib", "memory_max_mib"],
        var_name="metric",
        value_name="memory_mib",
    )
    fig, ax = plt.subplots(figsize=(13, 6))
    sns.barplot(data=mem_metrics, x="example", y="memory_mib", hue="metric", ax=ax, palette="BuPu", errorbar=None)
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
        fig, axes = plt.subplots(2, 1, figsize=(14, 8), sharex=True)
        sns.lineplot(data=plot, x="elapsed_s", y="cpu_percent", hue="case_name", ax=axes[0], legend=False)
        sns.lineplot(data=plot, x="elapsed_s", y="memory_mib", hue="case_name", ax=axes[1])
        axes[0].set_title("Resource Timelines for Highest-Memory Cases", loc="left", pad=14)
        axes[0].set_ylabel("CPU percent")
        axes[1].set_ylabel("MiB")
        axes[1].set_xlabel("elapsed seconds")
        _outside_legend(axes[1], "Case")
        _save(fig, out_dir / "resources" / "resource_timelines_top_memory.png", index, "resources", "Top memory timelines")


def _trace_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    traces = data.traces
    spans = data.spans
    cases = _actual(data.cases)
    if cases.empty:
        return
    trace_counts = traces.groupby("case_name", observed=True).size().reset_index(name="trace_count") if not traces.empty else pd.DataFrame(columns=["case_name", "trace_count"])
    plot = cases.merge(trace_counts, on="case_name", how="left").fillna({"trace_count": 0})
    fig, ax = plt.subplots(figsize=(15, max(7, len(plot) * 0.22)))
    y = np.arange(len(plot))
    ax.barh(y, plot["requests"], color="#cbd5e0", label="requests")
    ax.barh(y, plot["trace_count"], color="#2b6cb0", label="traces")
    ax.set_yticks(y)
    ax.set_yticklabels(plot["case_name"], fontsize=8)
    ax.invert_yaxis()
    ax.set_title("Trace Count vs Request Count", loc="left", pad=14)
    ax.set_xlabel("count")
    ax.legend(loc="lower right")
    _save(fig, out_dir / "traces" / "trace_coverage.png", index, "traces", "Trace coverage")

    if not traces.empty:
        fig, ax = plt.subplots(figsize=(10, 5.5))
        sns.histplot(data=traces, x="span_count", hue="example", multiple="stack", ax=ax, palette="tab10", bins=20)
        ax.set_title("Spans per Trace Distribution", loc="left", pad=14)
        ax.set_xlabel("spans per trace")
        ax.set_ylabel("trace count")
        _save(fig, out_dir / "traces" / "spans_per_trace.png", index, "traces", "Spans per trace")

    if not spans.empty:
        op_service = spans.groupby(["operation", "service"], observed=True).size().reset_index(name="spans")
        op_service = op_service.sort_values("spans", ascending=False).head(80)
        matrix = op_service.pivot_table(index="operation", columns="service", values="spans", aggfunc="sum", fill_value=0, observed=True)
        fig, ax = plt.subplots(figsize=(14, max(6, 0.28 * len(matrix.index))))
        sns.heatmap(matrix, ax=ax, cmap="Blues", linewidths=0.4, linecolor="white", cbar_kws={"label": "spans"})
        ax.set_title("Operation by Service Span Heatmap", loc="left", pad=14)
        ax.set_xlabel("service")
        ax.set_ylabel("operation")
        _save(fig, out_dir / "traces" / "operation_service_heatmap.png", index, "traces", "Operation/service heatmap")


def _topology_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = data.expected_cases if not data.expected_cases.empty else data.cases
    if cases.empty:
        return
    for item in cases[["example", "spec"]].drop_duplicates().itertuples(index=False):
        example = str(item.example)
        spec = str(item.spec)
        meta = EXAMPLES.get(example)
        if not meta:
            continue
        fig = draw_topology(example, spec, meta)
        path = out_dir / "topology" / f"{_slug(example)}_{_slug(spec)}.svg"
        _save(fig, path, index, "topology", f"{example} {spec} topology")


def _example_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any]) -> None:
    cases = _actual(data.cases)
    requests = _actual(data.requests)
    components = _actual(data.components)
    expected = _actual(data.expected_cases)
    if cases.empty:
        return

    examples: list[dict[str, Any]] = []
    expected_by_example = _groups(expected, "example")
    requests_by_example = _groups(requests, "example")
    components_by_example = _groups(components, "example")
    empty = pd.DataFrame()
    for example, ex_cases in cases.groupby("example", observed=True):
        example = str(example)
        slug = _slug(example)
        example_dir = out_dir / "examples" / slug
        example_dir.mkdir(parents=True, exist_ok=True)
        entry = {"example": example, "slug": slug, "path": f"examples/{slug}/index.html", "plots": []}

        ex_expected = expected_by_example.get(example, ex_cases)
        merged = ex_expected.merge(
            ex_cases[["case_name", "success_rate"]],
            on="case_name",
            how="left",
        )
        merged["success_rate"] = merged["success_rate"].where(merged["present"], np.nan) if "present" in merged else merged["success_rate"]
        merged["column"] = merged["spec"].astype(str) + "\n" + merged["profile"].astype(str)
        matrix = merged.pivot_table(index="example", columns="column", values="success_rate", aggfunc="first", observed=True)
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
            cbar_kws={"label": "success rate (red is bad, green is good)"},
        )
        ax.set_title(f"{example}: Success Rate", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("")
        entry["plots"].append({"title": "Success-rate heatmap", "path": _save(fig, example_dir / "success_rate_heatmap.png", index)})

        latency = ex_cases.melt(
            id_vars=["case_name", "spec", "profile"],
            value_vars=["p50_ms", "p95_ms", "p99_ms"],
            var_name="percentile",
            value_name="latency_ms",
        )
        latency["case_axis"] = latency["spec"].astype(str) + "\n" + latency["profile"].astype(str)
        fig, ax = plt.subplots(figsize=(14, 6))
        sns.barplot(data=latency, x="case_axis", y="latency_ms", hue="percentile", ax=ax, palette="rocket", errorbar=None)
        ax.set_title(f"{example}: Latency Percentiles", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("latency (ms)")
        ax.tick_params(axis="x", rotation=35)
        _outside_legend(ax, "Percentile")
        entry["plots"].append({"title": "Latency percentiles", "path": _save(fig, example_dir / "latency_percentiles.png", index)})

        fig, ax = plt.subplots(figsize=(14, 6))
        ex_cases = ex_cases.copy()
        ex_cases["case_axis"] = ex_cases["spec"].astype(str) + "\n" + ex_cases["profile"].astype(str)
        sns.barplot(data=ex_cases, x="case_axis", y="throughput_rps", hue="spec", ax=ax, palette="muted", errorbar=None)
        ax.set_title(f"{example}: Throughput", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("requests / second")
        ax.tick_params(axis="x", rotation=35)
        _outside_legend(ax, "Spec")
        entry["plots"].append({"title": "Throughput", "path": _save(fig, example_dir / "throughput.png", index)})

        token_plot = ex_cases.sort_values("total_tokens", ascending=False)
        fig, ax = plt.subplots(figsize=(14, 6))
        sns.barplot(data=token_plot, x="case_axis", y="total_tokens", hue="profile", ax=ax, palette="Purples", errorbar=None)
        ax.set_title(f"{example}: Total Tokens", loc="left", pad=14)
        ax.set_xlabel("")
        ax.set_ylabel("tokens")
        ax.tick_params(axis="x", rotation=35)
        _outside_legend(ax, "Profile")
        entry["plots"].append({"title": "Total tokens", "path": _save(fig, example_dir / "tokens.png", index)})

        ex_requests = requests_by_example.get(example, empty)
        if not ex_requests.empty:
            ex_requests = ex_requests.copy()
            ex_requests["case_axis"] = ex_requests["spec"].astype(str) + "\n" + ex_requests["profile"].astype(str)
            fig, ax = plt.subplots(figsize=(14, 6))
            sns.stripplot(
                data=ex_requests,
                x="case_axis",
                y="latency_ms",
                hue="ok",
                ax=ax,
                palette={True: "#2f855a", False: "#c53030"},
                dodge=True,
                alpha=0.75,
                size=4,
            )
            ax.set_title(f"{example}: Per-Request Latency", loc="left", pad=14)
            ax.set_xlabel("")
            ax.set_ylabel("latency (ms)")
            ax.tick_params(axis="x", rotation=35)
            _outside_legend(ax, "OK")
            entry["plots"].append({"title": "Per-request latency", "path": _save(fig, example_dir / "request_latency.png", index)})

            fig = _draw_latency_distribution(
                ex_requests,
                title=f"{example}: Request Latency Distribution",
                x_col="case_axis",
            )
            entry["plots"].append({"title": "Request latency range", "path": _save(fig, example_dir / "request_latency_range.png", index)})

            fig = _draw_latency_cdf(
                ex_requests,
                title=f"{example}: Request Latency CDF",
                group_col="case_axis",
                legend_title="Spec / profile",
            )
            entry["plots"].append({"title": "Request latency CDF", "path": _save(fig, example_dir / "request_latency_cdf.png", index)})

        ex_components = components_by_example.get(example, empty)
        if not ex_components.empty:
            comp = ex_components.groupby("component", observed=True).agg(duration_ms=("duration_ms", "sum"), total_tokens=("total_tokens", "sum")).reset_index()
            comp = comp.sort_values("duration_ms", ascending=False).head(18)
            fig, ax = plt.subplots(figsize=(11, max(5, len(comp) * 0.35)))
            sns.barplot(data=comp, y="component", x="duration_ms", ax=ax, palette="crest", hue="component", legend=False)
            ax.set_title(f"{example}: Component Duration", loc="left", pad=14)
            ax.set_xlabel("aggregate duration (ms)")
            ax.set_ylabel("")
            entry["plots"].append({"title": "Component duration", "path": _save(fig, example_dir / "component_duration.png", index)})

        examples.append(entry)

    index["examples"] = examples


def _case_plots(data: BenchmarkRun, out_dir: Path, index: dict[str, Any], max_case_waterfalls: int) -> None:
    cases = _actual(data.cases)
    requests = data.requests
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
        case_entry: dict[str, Any] = {"case_name": case, "plots": []}

        req = requests_by_case.get(case, empty)
        if not req.empty:
            req = req.copy()
            fig, ax = plt.subplots(figsize=(9, 4.8))
            sns.scatterplot(data=req, x="sequence", y="latency_ms", hue="ok", ax=ax, palette={True: "#2f855a", False: "#c53030"}, s=48)
            ax.plot(req["sequence"], req["latency_ms"], color="#a0aec0", linewidth=1, alpha=0.6)
            ax.set_title(f"{case}: Request Latency", loc="left", pad=12)
            ax.set_xlabel("request sequence")
            ax.set_ylabel("latency (ms)")
            _outside_legend(ax, "OK")
            case_entry["plots"].append(_save(fig, case_dir / "request_latency.png", index))

            req["outcome"] = req["ok"].map({True: "success", False: "failed"}).fillna("unknown")
            fig = _draw_latency_cdf(
                req,
                title=f"{case}: Request Latency CDF",
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
            axes[0].set_title(f"{case}: Resource Timeline", loc="left", pad=12)
            axes[0].set_ylabel("CPU percent")
            axes[1].set_ylabel("MiB")
            axes[1].set_xlabel("elapsed seconds")
            _outside_legend(axes[1], "Container")
            case_entry["plots"].append(_save(fig, case_dir / "resources.png", index))

        comp = components_by_case.get(case, empty)
        if not comp.empty:
            comp = comp.sort_values("duration_ms", ascending=False).head(18)
            fig, ax = plt.subplots(figsize=(9, max(4, len(comp) * 0.32)))
            sns.barplot(data=comp, y="component", x="duration_ms", ax=ax, palette="crest", hue="component", legend=False)
            ax.set_title(f"{case}: Component Duration", loc="left", pad=12)
            ax.set_xlabel("duration (ms)")
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
    frame["label"] = frame["operation"].astype(str) + "\n" + frame["service"].astype(str)
    frame = frame.tail(40) if len(frame) > 40 else frame
    fig, ax = plt.subplots(figsize=(12, max(5, len(frame) * 0.3)))
    colors = sns.color_palette("tab20", n_colors=max(1, frame["service"].nunique()))
    service_colors = {svc: colors[i % len(colors)] for i, svc in enumerate(frame["service"].unique())}
    y = np.arange(len(frame))
    for i, row in enumerate(frame.itertuples(index=False)):
        ax.barh(i, row.duration_ms, left=row.relative_start_ms, color=service_colors[row.service], edgecolor="white", height=0.72)
    ax.set_yticks(y)
    ax.set_yticklabels(frame["label"], fontsize=7)
    ax.invert_yaxis()
    ax.set_title(f"{case}: Longest Trace Waterfall", loc="left", pad=12)
    ax.set_xlabel("trace-relative time (ms)")
    ax.set_ylabel("")
    return fig


def _draw_latency_distribution(frame: pd.DataFrame, title: str, x_col: str) -> plt.Figure:
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
        ax.vlines(i, row["whisker_low"], row["whisker_high"], color="#334155", linewidth=1.6)
        ax.add_patch(
            plt.Rectangle(
                (i - width / 2, row["q1"]),
                width,
                max(row["q3"] - row["q1"], 0.0001),
                facecolor="#63b3ed",
                edgecolor="#1e3a8a",
                linewidth=1.2,
                alpha=0.9,
            )
        )
        ax.hlines(row["median"], i - width / 2, i + width / 2, color="#7c2d12", linewidth=2)

    if outlier_rows:
        outliers = pd.DataFrame(outlier_rows)
        positions = {label: i for i, label in enumerate(labels)}
        jitter = np.linspace(-0.14, 0.14, num=len(outliers)) if len(outliers) > 1 else np.array([0.0])
        xs = [positions[str(row[x_col])] + jitter[idx % len(jitter)] for idx, row in outliers.iterrows()]
        ax.scatter(xs, outliers["latency_ms"], color="#c53030", s=22, alpha=0.75, label="outlier")

    ax.set_xticks(x)
    ax.set_xticklabels(labels, rotation=35, ha="right")
    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("")
    ax.set_ylabel("latency (ms)")
    if outlier_rows:
        ax.legend(loc="upper right", frameon=False)
    return fig


def _draw_latency_cdf(
    frame: pd.DataFrame,
    title: str,
    group_col: str | None = None,
    legend_title: str = "Group",
) -> plt.Figure:
    fig, ax = plt.subplots(figsize=(14, 6))
    groups = list(frame.groupby(group_col, observed=True, sort=False)) if group_col else [("all requests", frame)]
    colors = sns.color_palette("tab20", n_colors=max(1, len(groups)))
    plotted = False

    for idx, (label, group) in enumerate(groups):
        latencies = group["latency_ms"].dropna().astype(float)
        if latencies.empty:
            continue
        x = np.sort(latencies.to_numpy())
        y = np.arange(1, len(x) + 1) / len(x)
        ax.step(x, y, where="post", linewidth=2, color=colors[idx], label=str(label))
        plotted = True

    if not plotted:
        ax.text(0.5, 0.5, "No request latencies found", ha="center", va="center", fontsize=14)
        ax.axis("off")
        return fig

    ax.set_title(title, loc="left", pad=14)
    ax.set_xlabel("latency (ms)")
    ax.set_ylabel("requests <= latency")
    ax.set_ylim(0, 1.01)
    ax.yaxis.set_major_formatter(PercentFormatter(1.0))
    if len(groups) > 1:
        _outside_legend(ax, legend_title)
    return fig


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
