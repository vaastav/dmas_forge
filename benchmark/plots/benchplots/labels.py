from __future__ import annotations

from typing import Any


EXAMPLE_LABELS = {
    "weather": "Weather Retrieval",
    "travel-planning": "Travel Planning",
    "marketing-agency": "Marketing Campaign",
    "financial-analyzer": "Financial Analysis",
    "agentic-hotel": "Hotel Booking",
}

EXAMPLE_ORDER = [
    "weather",
    "travel-planning",
    "marketing-agency",
    "financial-analyzer",
    "agentic-hotel",
]

CASE_PROFILE_SUFFIXES = (
    "-long-light",
    "-short-sequential",
    "-long-sequential",
    "-sequential",
    "-light",
    "-heavy",
    "-smoke",
)

CASE_SPEC_LABELS = {
    "single": "Single",
    "http": "HTTP",
    "mcp": "MCP",
    "a2a": "A2A",
}


def example_label(value: Any) -> str:
    text = str(value)
    return EXAMPLE_LABELS.get(text, text.replace("_", " ").replace("-", " ").title())


def example_sort_key(value: Any) -> tuple[int, str]:
    text = str(value)
    if text in EXAMPLE_ORDER:
        return EXAMPLE_ORDER.index(text), text
    return len(EXAMPLE_ORDER), text


def example_case_label(value: Any) -> str:
    text = str(value)
    for profile in CASE_PROFILE_SUFFIXES:
        if text.endswith(profile):
            text = text[: -len(profile)]
            break
    for spec, label in CASE_SPEC_LABELS.items():
        suffix = f"-{spec}"
        if text.endswith(suffix):
            return example_label(text[: -len(suffix)]) + " / " + label
    return example_label(text)


def example_case_sort_key(value: Any) -> tuple[int, str]:
    text = str(value)
    for profile in CASE_PROFILE_SUFFIXES:
        if text.endswith(profile):
            text = text[: -len(profile)]
            break
    for spec in CASE_SPEC_LABELS:
        suffix = f"-{spec}"
        if text.endswith(suffix):
            return example_sort_key(text[: -len(suffix)])
    return example_sort_key(text)
