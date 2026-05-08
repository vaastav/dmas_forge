from __future__ import annotations

import json
import math
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pandas as pd

from .labels import short_container_name, short_service_name
from .topology import build_topology_rows

SPEC_ORDER = ["single", "http", "mcp", "a2a", "memory", "no_memory", "automatic", "agentic"]
PROFILE_ORDER = ["sequential", "long-sequential", "light", "heavy"]


@dataclass(frozen=True)
class BenchmarkRun:
    run_id: str
    run_dir: Path
    run_info: dict[str, Any]
    cases: pd.DataFrame
    expected_cases: pd.DataFrame
    requests: pd.DataFrame
    resources: pd.DataFrame
    spans: pd.DataFrame
    traces: pd.DataFrame
    errors: pd.DataFrame
    error_details: pd.DataFrame
    components: pd.DataFrame
    agent_metrics: pd.DataFrame
    agent_checks: pd.DataFrame
    topologies: pd.DataFrame


def load_run(results_root: Path, run_id: str) -> BenchmarkRun:
    run_dir = results_root / run_id
    if not run_dir.exists():
        raise FileNotFoundError(f"Run not found: {run_dir}")

    run_info = _read_json(run_dir / "run.json", {})
    case_rows: list[dict[str, Any]] = []
    request_rows: list[dict[str, Any]] = []
    resource_rows: list[dict[str, Any]] = []
    span_rows: list[dict[str, Any]] = []
    trace_rows: list[dict[str, Any]] = []
    component_rows: list[dict[str, Any]] = []
    agent_rows: list[dict[str, Any]] = []

    for case_dir in sorted(p for p in run_dir.iterdir() if p.is_dir()):
        summary = _read_json(case_dir / "summary.json", None)
        if not isinstance(summary, dict):
            continue
        case_name = case_dir.name
        case_base = {
            "case_name": case_name,
            "example": summary.get("example", ""),
            "spec": summary.get("spec", ""),
            "profile": summary.get("profile", ""),
        }
        case_rows.append(
            {
                **case_base,
                "requests": _num(summary.get("requests")),
                "successes": _num(summary.get("successes")),
                "errors": _num(summary.get("errors")),
                "success_rate": _safe_div(_num(summary.get("successes")), _num(summary.get("requests"))),
                "elapsed_ms": _num(summary.get("elapsed_ms")),
                "throughput_rps": _num(summary.get("throughput_rps")),
                "p50_ms": _num(summary.get("p50_ms")),
                "p95_ms": _num(summary.get("p95_ms")),
                "p99_ms": _num(summary.get("p99_ms")),
                "input_tokens": _num(summary.get("input_tokens")),
                "output_tokens": _num(summary.get("output_tokens")),
                "total_tokens": _num(summary.get("total_tokens")),
                "tokens_per_success": _safe_div(_num(summary.get("total_tokens")), _num(summary.get("successes"))),
                "cpu_avg_percent": _num(summary.get("cpu_avg_percent")),
                "cpu_max_percent": _num(summary.get("cpu_max_percent")),
                "memory_avg_bytes": _num(summary.get("memory_avg_bytes")),
                "memory_max_bytes": _num(summary.get("memory_max_bytes")),
                "trace_error": summary.get("trace_error", ""),
                "resource_error": summary.get("resource_error", ""),
                "status": _case_status(summary),
            }
        )
        for comp in summary.get("components", []) or []:
            component_rows.append(
                {
                    **case_base,
                    "component": comp.get("name", "unknown"),
                    "spans": _num(comp.get("spans")),
                    "duration_ms": _num(comp.get("duration_ms")),
                    "input_tokens": _num(comp.get("input_tokens")),
                    "output_tokens": _num(comp.get("output_tokens")),
                    "total_tokens": _num(comp.get("total_tokens")),
                }
            )
        for row in _read_jsonl(case_dir / "requests.jsonl"):
            error = str(row.get("error", "") or "")
            response_text = str(row.get("response_text", "") or "")
            request_rows.append(
                {
                    **case_base,
                    "sequence": _num(row.get("sequence")),
                    "status": _num(row.get("status")),
                    "ok": bool(row.get("ok")),
                    "latency_ms": _num(row.get("latency_ms")),
                    "response_bytes": _num(row.get("response_bytes")),
                    "error": error,
                    "response_text": response_text,
                    "error_category": "" if row.get("ok") else categorize_error(error, row.get("status")),
                    "error_reason": "" if row.get("ok") else error_reason(error, row.get("status"), response_text),
                    "url": row.get("url", ""),
                }
            )
        for row in _read_jsonl(case_dir / "resources.jsonl"):
            resource_rows.append(
                {
                    **case_base,
                    "timestamp": row.get("timestamp", ""),
                    "container_id": row.get("container_id", ""),
                    "container_name": row.get("container_name", ""),
                    "container_short": short_container_name(row.get("container_name", "")),
                    "cpu_percent": _num(row.get("cpu_percent")),
                    "memory_bytes": _num(row.get("memory_bytes")),
                    "memory_percent": _num(row.get("memory_percent")),
                }
            )
        for row in _read_jsonl(case_dir / "spans.jsonl"):
            tags = row.get("tags") if isinstance(row.get("tags"), dict) else {}
            service = row.get("service_name", "unknown") or "unknown"
            status_description = str(tags.get("otel.status_description", "") or "")
            is_error = bool(tags.get("error")) or str(tags.get("otel.status_code", "")).upper() == "ERROR"
            span_rows.append(
                {
                    **case_base,
                    "trace_id": row.get("trace_id", ""),
                    "span_id": row.get("span_id", ""),
                    "operation": row.get("operation_name", "unknown") or "unknown",
                    "service": service,
                    "service_short": short_service_name(service),
                    "start_time_us": _num(row.get("start_time")),
                    "duration_us": _num(row.get("duration")),
                    "duration_ms": _num(row.get("duration")) / 1000.0,
                    "llm_model": tags.get("llm.model", ""),
                    "llm_call_type": tags.get("llm.call_type", ""),
                    "llm_tool_count": _num(tags.get("llm.tool_count")),
                    "llm_tool_call_count": _num(tags.get("llm.tool_call_count")),
                    "input_tokens": _num(tags.get("llm.input_tokens")),
                    "output_tokens": _num(tags.get("llm.output_tokens")),
                    "total_tokens": _num(tags.get("llm.total_tokens")),
                    "is_error": is_error,
                    "status_code": tags.get("otel.status_code", ""),
                    "status_description": status_description,
                    "span_error_category": categorize_error(status_description) if is_error else "",
                    "span_error_reason": error_reason(status_description) if is_error else "",
                    "tool_name": tags.get("tool.name", ""),
                    "tool_round": _num(tags.get("tool.round")),
                    "max_tool_rounds_exhausted": bool(tags.get("llm.max_tool_rounds_exhausted")),
                    "parent_span_id": row.get("parent_span_id", ""),
                }
            )
        trace_payload = _read_json(case_dir / "traces.json", {})
        for trace in trace_payload.get("data", []) if isinstance(trace_payload, dict) else []:
            spans = trace.get("spans", []) or []
            starts = [_num(s.get("startTime")) for s in spans if isinstance(s, dict)]
            ends = [_num(s.get("startTime")) + _num(s.get("duration")) for s in spans if isinstance(s, dict)]
            trace_rows.append(
                {
                    **case_base,
                    "trace_id": trace.get("traceID", ""),
                    "span_count": len(spans),
                    "duration_ms": ((max(ends) - min(starts)) / 1000.0) if starts and ends else 0.0,
                    "process_count": len(trace.get("processes", {}) or {}),
                    "warnings": len(trace.get("warnings", []) or []),
                }
            )
        agent_rows.extend(_agent_metric_rows(case_base, trace_payload))

    cases = _order_cases(pd.DataFrame(case_rows))
    expected_cases = _expected_cases(run_info, cases)
    requests = _order_cases(pd.DataFrame(request_rows))
    resources = _prepare_resources(_order_cases(pd.DataFrame(resource_rows)))
    spans = _prepare_spans(_order_cases(pd.DataFrame(span_rows)))
    traces = _order_cases(pd.DataFrame(trace_rows))
    components = _order_cases(pd.DataFrame(component_rows))
    agent_metrics = _order_cases(pd.DataFrame(agent_rows))
    errors = _build_errors(requests)
    error_details = _build_error_details(requests)
    agent_checks = _build_agent_checks(cases, agent_metrics)
    topologies = build_topology_rows(expected_cases if not expected_cases.empty else cases)

    return BenchmarkRun(
        run_id=run_id,
        run_dir=run_dir,
        run_info=run_info,
        cases=cases,
        expected_cases=expected_cases,
        requests=requests,
        resources=resources,
        spans=spans,
        traces=traces,
        errors=errors,
        error_details=error_details,
        components=components,
        agent_metrics=agent_metrics,
        agent_checks=agent_checks,
        topologies=topologies,
    )


def write_normalized_data(data: BenchmarkRun, out_dir: Path) -> None:
    data_dir = out_dir / "data"
    data_dir.mkdir(parents=True, exist_ok=True)
    tables = {
        "cases": data.cases,
        "expected_cases": data.expected_cases,
        "requests": data.requests,
        "resources": data.resources,
        "spans": data.spans,
        "traces": data.traces,
        "errors": data.errors,
        "error_details": data.error_details,
        "components": data.components,
        "agent_metrics": data.agent_metrics,
        "agent_checks": data.agent_checks,
        "topologies": data.topologies,
    }
    normalized: dict[str, Any] = {
        "run_id": data.run_id,
        "run_info": data.run_info,
        "tables": {},
    }
    for name, frame in tables.items():
        frame.to_csv(data_dir / f"{name}.csv", index=False)
        normalized["tables"][name] = _json_records(frame)
    (data_dir / "normalized.json").write_text(json.dumps(normalized, indent=2), encoding="utf-8")


def categorize_error(error: str, status: Any = None) -> str:
    text = (error or "").lower()
    if "client.timeout exceeded while awaiting headers" in text:
        return "client_timeout_headers"
    if "context deadline exceeded" in text:
        return "deadline_exceeded"
    if "context canceled" in text or "context cancelled" in text:
        return "context_canceled"
    if "error parsing response json" in text or "unexpected end of json" in text:
        return "response_parse_error"
    if "429" in text or "too many requests" in text or "rate-limit" in text or "rate limited" in text:
        return "rate_limit_429"
    if "statuscode was 500" in text:
        return "http_500"
    if "unexpected http status 500" in text:
        return "transport_500"
    if "orchestrator finished without producing a report" in text:
        return "workflow_no_report"
    if "a2aclient" in text or "a2a" in text:
        return "a2a_error"
    if "mcp" in text:
        return "mcp_error"
    if "llm call" in text:
        return "llm_error"
    if status and int(_num(status)) >= 500:
        return "server_5xx"
    if error:
        return "other_error"
    return "unknown_error"


def error_reason(error: str, status: Any = None, response_text: str = "") -> str:
    text = str(error or response_text or "").strip()
    lower = text.lower()
    if "client.timeout exceeded while awaiting headers" in lower:
        return "Client timed out awaiting response headers"
    if "error reading response body" in lower and "context deadline exceeded" in lower:
        return "Response body read hit context deadline"
    if "context deadline exceeded" in lower:
        return "Context deadline exceeded"
    if "context canceled" in lower or "context cancelled" in lower:
        return "Context canceled"
    if "error parsing response json" in lower:
        return "Response JSON parse failed"
    if "unexpected http status" in lower:
        return text
    if text:
        return _compact_text(text, 160)
    status_num = int(_num(status))
    if status_num:
        return f"HTTP {status_num}"
    return "Unknown failure"


def _read_json(path: Path, default: Any) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return default


def _read_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    try:
        with path.open("r", encoding="utf-8") as handle:
            for line in handle:
                line = line.strip()
                if not line:
                    continue
                try:
                    item = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if isinstance(item, dict):
                    rows.append(item)
    except OSError:
        pass
    return rows


def _num(value: Any) -> float:
    if value is None or value == "":
        return 0.0
    try:
        number = float(value)
    except (TypeError, ValueError):
        return 0.0
    if math.isnan(number) or math.isinf(number):
        return 0.0
    return number


def _safe_div(numerator: float, denominator: float) -> float:
    return numerator / denominator if denominator else 0.0


def _case_status(summary: dict[str, Any]) -> str:
    requests = _num(summary.get("requests"))
    successes = _num(summary.get("successes"))
    errors = _num(summary.get("errors"))
    if requests and successes == 0:
        return "failed"
    if errors:
        return "partial"
    return "ok"


def _order_cases(frame: pd.DataFrame) -> pd.DataFrame:
    if frame.empty:
        return frame
    frame = frame.copy()
    sort_cols = [col for col in ["example", "spec", "profile", "case_name"] if col in frame.columns]
    if not sort_cols:
        return frame.reset_index(drop=True)

    sort_frame = frame.copy()
    if "spec" in sort_frame:
        sort_frame["_spec_order"] = _ordered_sort_key(sort_frame["spec"], SPEC_ORDER)
    if "profile" in sort_frame:
        sort_frame["_profile_order"] = _ordered_sort_key(sort_frame["profile"], PROFILE_ORDER)

    by: list[str] = []
    for col in sort_cols:
        if col == "spec" and "_spec_order" in sort_frame:
            by.append("_spec_order")
        elif col == "profile" and "_profile_order" in sort_frame:
            by.append("_profile_order")
        by.append(col)
    return sort_frame.sort_values(by).drop(columns=["_spec_order", "_profile_order"], errors="ignore").reset_index(drop=True)


def _ordered_sort_key(values: pd.Series, ordered_values: list[str]) -> pd.Series:
    order = {value: index for index, value in enumerate(ordered_values)}
    fallback = len(order)
    return values.astype(str).map(lambda value: order.get(value, fallback))


def _expected_cases(run_info: dict[str, Any], cases: pd.DataFrame) -> pd.DataFrame:
    cfg = run_info.get("config", {}) if isinstance(run_info.get("config"), dict) else {}
    selected_examples = set(_selected_keys(run_info.get("examples")))
    selected_specs = set(_selected_keys(run_info.get("specs")))
    selected_profiles = set(_selected_keys(run_info.get("profiles")))
    global_profiles = cfg.get("profiles", []) if isinstance(cfg.get("profiles"), list) else []
    examples = cfg.get("examples", []) if isinstance(cfg.get("examples"), list) else []
    rows: list[dict[str, Any]] = []
    present = set(cases["case_name"].astype(str)) if "case_name" in cases else set()
    case_examples = set(cases["example"].astype(str)) if "example" in cases else set()
    case_specs = set(cases["spec"].astype(str)) if "spec" in cases else set()
    case_profiles = set(cases["profile"].astype(str)) if "profile" in cases else set()

    for ex in examples:
        if not isinstance(ex, dict):
            continue
        example = ex.get("name", "")
        if selected_examples and example not in selected_examples:
            continue
        specs = [s for s in ex.get("specs", []) if not selected_specs or s in selected_specs]
        profiles = ex.get("profiles") or global_profiles
        for spec in specs:
            for profile in profiles:
                profile_name = profile.get("name", "") if isinstance(profile, dict) else str(profile)
                if selected_profiles and profile_name not in selected_profiles:
                    continue
                case_name = f"{example}-{spec}-{profile_name}"
                rows.append(
                    {
                        "case_name": case_name,
                        "example": example,
                        "spec": spec,
                        "profile": profile_name,
                        "expected": True,
                        "configured": True,
                        "present": case_name in present,
                        "configured_mode": profile.get("mode", "") if isinstance(profile, dict) else "",
                        "configured_value": profile.get("value", 0) if isinstance(profile, dict) else 0,
                        "configured_concurrency": profile.get("concurrency", 0) if isinstance(profile, dict) else 0,
                        "timeout_seconds": profile.get("timeout_seconds", 0) if isinstance(profile, dict) else 0,
                    }
                )
    configured_names = {row["case_name"] for row in rows}
    comparison_examples = sorted(selected_examples or {row["example"] for row in rows} or case_examples)
    comparison_specs = [spec for spec in SPEC_ORDER if (not selected_specs or spec in selected_specs) and (spec in selected_specs or spec in case_specs)]
    profile_names = {p.get("name", "") for p in global_profiles if isinstance(p, dict)}
    profile_names.update(str(p) for p in selected_profiles)
    if not profile_names:
        profile_names.update(case_profiles)
    ordered_profile_names = [profile for profile in PROFILE_ORDER if profile in profile_names]
    ordered_profile_names.extend(sorted(profile_names - set(ordered_profile_names)))
    comparison_profiles = ordered_profile_names
    for example in comparison_examples:
        for spec in comparison_specs:
            for profile_name in comparison_profiles:
                case_name = f"{example}-{spec}-{profile_name}"
                if case_name in configured_names:
                    continue
                rows.append(
                    {
                        "case_name": case_name,
                        "example": example,
                        "spec": spec,
                        "profile": profile_name,
                        "expected": False,
                        "configured": False,
                        "present": case_name in present,
                        "configured_mode": "",
                        "configured_value": 0,
                        "configured_concurrency": 0,
                        "timeout_seconds": 0,
                    }
                )
    expected = pd.DataFrame(rows)
    if expected.empty and not cases.empty:
        expected = cases[["case_name", "example", "spec", "profile"]].copy()
        expected["expected"] = True
        expected["configured"] = True
        expected["present"] = True
    return _order_cases(expected)


def _selected_keys(value: Any) -> list[str]:
    if isinstance(value, dict):
        return [str(k) for k, v in value.items() if bool(v)]
    if isinstance(value, list):
        return [str(v) for v in value]
    return []


def _prepare_resources(frame: pd.DataFrame) -> pd.DataFrame:
    if frame.empty:
        return frame
    frame = frame.copy()
    frame["timestamp_dt"] = pd.to_datetime(frame["timestamp"], errors="coerce", utc=True)
    frame["case_start"] = frame.groupby("case_name", observed=False)["timestamp_dt"].transform("min")
    frame["elapsed_s"] = (frame["timestamp_dt"] - frame["case_start"]).dt.total_seconds().fillna(0)
    return frame


def _prepare_spans(frame: pd.DataFrame) -> pd.DataFrame:
    if frame.empty:
        return frame
    frame = frame.copy()
    frame["start_ms"] = frame["start_time_us"] / 1000.0
    frame["end_ms"] = (frame["start_time_us"] + frame["duration_us"]) / 1000.0
    frame["trace_start_ms"] = frame.groupby(["case_name", "trace_id"], observed=False)["start_ms"].transform("min")
    frame["relative_start_ms"] = frame["start_ms"] - frame["trace_start_ms"]
    return frame


def _agent_metric_rows(case_base: dict[str, Any], trace_payload: Any) -> list[dict[str, Any]]:
    if not isinstance(trace_payload, dict):
        return []
    rows: dict[str, dict[str, Any]] = {}
    for trace in trace_payload.get("data", []) or []:
        if not isinstance(trace, dict):
            continue
        spans = [span for span in trace.get("spans", []) or [] if isinstance(span, dict)]
        spans_by_id = {str(span.get("spanID", "")): span for span in spans if span.get("spanID")}
        for span in spans:
            operation = str(span.get("operationName", "") or "")
            agent = _agent_name_from_operation(operation)
            if agent and "Server_" in operation:
                row = _ensure_agent_metric_row(rows, case_base, agent)
                row["agent_spans"] += 1
                row["duration_ms"] += _num(span.get("duration")) / 1000.0
            if operation == "llm.call":
                agent = _nearest_parent_agent(span, spans_by_id) or "Unattributed"
                tags = _trace_span_tags(span)
                row = _ensure_agent_metric_row(rows, case_base, agent)
                row["llm_calls"] += 1
                row["input_tokens"] += _num(tags.get("llm.input_tokens"))
                row["output_tokens"] += _num(tags.get("llm.output_tokens"))
                row["total_tokens"] += _num(tags.get("llm.total_tokens"))
    return sorted(rows.values(), key=lambda row: (row["agent"] == "Unattributed", str(row["agent"])))


def _ensure_agent_metric_row(rows: dict[str, dict[str, Any]], case_base: dict[str, Any], agent: str) -> dict[str, Any]:
    row = rows.get(agent)
    if row is None:
        row = {
            **case_base,
            "agent": agent,
            "agent_spans": 0,
            "duration_ms": 0.0,
            "llm_calls": 0,
            "input_tokens": 0.0,
            "output_tokens": 0.0,
            "total_tokens": 0.0,
        }
        rows[agent] = row
    return row


def _nearest_parent_agent(span: dict[str, Any], spans_by_id: dict[str, dict[str, Any]]) -> str:
    seen: set[str] = set()
    parent = _parent_span_id(span)
    while parent and parent not in seen:
        seen.add(parent)
        current = spans_by_id.get(parent)
        if not current:
            break
        operation = str(current.get("operationName", "") or "")
        agent = _agent_name_from_operation(operation)
        if agent:
            return agent
        parent = _parent_span_id(current)
    return ""


def _agent_name_from_operation(operation: str) -> str:
    if _is_internal_operation(operation):
        return ""
    match = re.match(r"^(.+?(?:Agent|Coordinator|Controller))(?:Client|Server)?_", operation)
    return match.group(1) if match else ""


def _is_internal_operation(operation: str) -> bool:
    return operation.startswith(("llm.", "mcp.", "tool.", "rag.", "kb.", "embedding."))


def _parent_span_id(span: dict[str, Any]) -> str:
    for ref in span.get("references", []) or []:
        if isinstance(ref, dict) and ref.get("refType") == "CHILD_OF":
            return str(ref.get("spanID", "") or "")
    return ""


def _trace_span_tags(span: dict[str, Any]) -> dict[str, Any]:
    tags: dict[str, Any] = {}
    for raw in span.get("tags", []) or []:
        if not isinstance(raw, dict):
            continue
        key = str(raw.get("key", "") or "")
        if key:
            tags[key] = raw.get("value")
    return tags


def _build_agent_checks(cases: pd.DataFrame, agent_metrics: pd.DataFrame) -> pd.DataFrame:
    columns = [
        "case_name",
        "example",
        "spec",
        "profile",
        "case_total_tokens",
        "accounted_tokens",
        "unattributed_tokens",
        "token_delta",
        "attribution_ok",
    ]
    if cases.empty:
        return pd.DataFrame(columns=columns)
    checks = cases[["case_name", "example", "spec", "profile", "total_tokens"]].copy()
    checks = checks.rename(columns={"total_tokens": "case_total_tokens"})
    if agent_metrics.empty:
        checks["accounted_tokens"] = 0.0
        checks["unattributed_tokens"] = 0.0
    else:
        totals = agent_metrics.groupby("case_name", observed=True)["total_tokens"].sum().reset_index(name="accounted_tokens")
        unattributed = (
            agent_metrics[agent_metrics["agent"].astype(str) == "Unattributed"]
            .groupby("case_name", observed=True)["total_tokens"]
            .sum()
            .reset_index(name="unattributed_tokens")
        )
        checks = checks.merge(totals, on="case_name", how="left")
        checks = checks.merge(unattributed, on="case_name", how="left")
        checks[["accounted_tokens", "unattributed_tokens"]] = checks[["accounted_tokens", "unattributed_tokens"]].fillna(0.0)
    checks["token_delta"] = checks["case_total_tokens"] - checks["accounted_tokens"]
    checks["attribution_ok"] = (checks["token_delta"].abs() < 0.5) & (checks["unattributed_tokens"].abs() < 0.5)
    return _order_cases(checks[columns])


def _build_errors(requests: pd.DataFrame) -> pd.DataFrame:
    if requests.empty:
        return pd.DataFrame()
    errors = requests[~requests["ok"]].copy()
    if errors.empty:
        return pd.DataFrame(columns=["case_name", "example", "spec", "profile", "error_category", "count"])
    grouped = (
        errors.groupby(["case_name", "example", "spec", "profile", "error_category"], observed=True)
        .size()
        .reset_index(name="count")
    )
    return _order_cases(grouped)


def _build_error_details(requests: pd.DataFrame) -> pd.DataFrame:
    columns = [
        "case_name",
        "example",
        "spec",
        "profile",
        "sequence",
        "status",
        "latency_ms",
        "response_bytes",
        "error_category",
        "error_reason",
        "error",
        "response_text",
        "url",
    ]
    if requests.empty:
        return pd.DataFrame(columns=columns)
    errors = requests[~requests["ok"]].copy()
    if errors.empty:
        return pd.DataFrame(columns=columns)
    return _order_cases(errors[[col for col in columns if col in errors.columns]])


def _compact_text(value: Any, limit: int) -> str:
    text = " ".join(str(value or "").split())
    if len(text) <= limit:
        return text
    return text[: max(0, limit - 14)].rstrip() + "...<truncated>"


def _json_records(frame: pd.DataFrame) -> list[dict[str, Any]]:
    if frame.empty:
        return []
    safe = frame.copy()
    for col in safe.columns:
        if pd.api.types.is_datetime64_any_dtype(safe[col]):
            safe[col] = safe[col].astype(str)
        if isinstance(safe[col].dtype, pd.CategoricalDtype):
            safe[col] = safe[col].astype(str)
    safe = safe.where(pd.notnull(safe), None)
    return safe.to_dict(orient="records")
