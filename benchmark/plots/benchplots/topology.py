from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import pandas as pd


@dataclass(frozen=True)
class Service:
    key: str
    label: str
    role: str
    llm: str


EXAMPLES: dict[str, dict[str, Any]] = {
    "weather": {
        "entry": "Weather API",
        "services": [
            Service("weather", "WeatherAgent", "front", "wagent"),
            Service("disaster", "DisasterAgent", "worker", "dagent"),
        ],
        "edges": [("weather", "disaster", "query")],
        "tools": [("weather", "get_weather", "mock weather tool")],
    },
    "travel-planning": {
        "entry": "Travel Plan API",
        "services": [
            Service("coordinator", "TravelCoordinator", "front", "coordinator_llm"),
            Service("planner", "TravelPlannerAgent", "worker", "planner_llm"),
            Service("local", "LocalAgent", "worker", "local_llm"),
            Service("language", "LanguageAgent", "worker", "language_llm"),
            Service("summary", "TravelSummaryAgent", "worker", "summary_llm"),
        ],
        "edges": [
            ("coordinator", "planner", "plan"),
            ("coordinator", "local", "local"),
            ("coordinator", "language", "language"),
            ("coordinator", "summary", "summary"),
        ],
        "tools": [],
    },
    "marketing-agency": {
        "entry": "Campaign API",
        "services": [
            Service("coordinator", "MarketingCoordinator", "front", "coordinator_core"),
            Service("domain", "DomainAgent", "worker", "domain_agent_core"),
            Service("website", "WebsiteAgent", "worker", "website_agent_core"),
            Service("marketing", "MarketingAgent", "worker", "marketing_agent_core"),
            Service("logo", "LogoAgent", "worker", "logo_agent_core"),
        ],
        "edges": [
            ("coordinator", "domain", "domain"),
            ("coordinator", "website", "website"),
            ("coordinator", "marketing", "marketing"),
            ("coordinator", "logo", "logo"),
        ],
        "tools": [
            ("domain", "duckduckgo_search", "mock search"),
            ("logo", "image_generation", "mock image"),
        ],
    },
    "financial-analyzer": {
        "entry": "Analyze API",
        "services": [
            Service("coordinator", "FinancialAnalyzerCoordinator", "front", "coordinator_core"),
            Service("research", "ResearchQualityController", "orchestrator", "researcher_core"),
            Service("collector", "DataCollectorAgent", "worker", "collector_core"),
            Service("evaluator", "DataEvaluatorAgent", "worker", "evaluator_core"),
            Service("analyst", "FinancialAnalystAgent", "worker", "analyst_core"),
            Service("writer", "ReportWriterAgent", "worker", "writer_core"),
        ],
        "edges": [
            ("coordinator", "research", "research"),
            ("coordinator", "analyst", "analysis"),
            ("coordinator", "writer", "report"),
            ("research", "collector", "collect"),
            ("research", "evaluator", "evaluate"),
        ],
        "tools": [
            ("collector", "search_web", "mock finance search"),
            ("collector", "fetch_url", "mock finance fetch"),
        ],
    },
}


def build_topology_rows(cases: pd.DataFrame) -> pd.DataFrame:
    rows: list[dict[str, Any]] = []
    if cases.empty:
        return pd.DataFrame(rows)
    unique = cases[["example", "spec"]].drop_duplicates()
    for row in unique.itertuples(index=False):
        example = str(row.example)
        spec = str(row.spec)
        meta = EXAMPLES.get(example)
        if not meta:
            continue
        protocol = protocol_for_spec(spec)
        for service in meta["services"]:
            rows.append(
                {
                    "example": example,
                    "spec": spec,
                    "kind": "service",
                    "source": service.key,
                    "target": "",
                    "label": service.label,
                    "role": service.role,
                    "protocol": "",
                    "container": container_for_service(spec, service.key),
                    "llm": service.llm,
                }
            )
        for source, target, label in meta["edges"]:
            rows.append(
                {
                    "example": example,
                    "spec": spec,
                    "kind": "edge",
                    "source": source,
                    "target": target,
                    "label": label,
                    "role": "",
                    "protocol": protocol,
                    "container": "",
                    "llm": "",
                }
            )
        for source, tool, label in meta["tools"]:
            rows.append(
                {
                    "example": example,
                    "spec": spec,
                    "kind": "tool",
                    "source": source,
                    "target": tool,
                    "label": label,
                    "role": "mock_tool",
                    "protocol": "tool",
                    "container": "external/mock",
                    "llm": "",
                }
            )
    return pd.DataFrame(rows)


def protocol_for_spec(spec: str) -> str:
    if spec == "single":
        return "in-process"
    if spec == "http":
        return "HTTP"
    if spec == "mcp":
        return "MCP"
    if spec == "a2a":
        return "A2A"
    return spec.upper()


def container_for_service(spec: str, service_key: str) -> str:
    if spec == "single":
        return "single app container"
    return f"{service_key} container"
