from __future__ import annotations

from collections import defaultdict
from typing import Any

import matplotlib.pyplot as plt
import networkx as nx
import numpy as np
from matplotlib.patches import Rectangle

from .labels import example_label
from .topology import protocol_for_spec


SPEC_LABELS = {
    "single": "Single-process",
    "http": "HTTP",
    "mcp": "MCP",
    "a2a": "A2A",
}


def draw_topology(example: str, spec: str, meta: dict[str, Any]) -> plt.Figure:
    graph = _build_graph(spec, meta)
    services = meta["services"]
    front = next((svc.key for svc in services if svc.role == "front"), services[0].key)
    pos = _layout(graph, front)

    fig_width = 23 if len(services) >= 5 else 20
    fig_height = max(12, 1.75 * len(services) + 5)
    fig, ax = plt.subplots(figsize=(fig_width, fig_height))
    ax.set_title(f"{example_label(example)}: {SPEC_LABELS.get(spec, spec.upper())} topology", loc="left", fontsize=17, pad=16, weight="bold")
    ax.axis("off")

    _draw_container_boxes(ax, graph, pos)
    _draw_nodes(graph, pos, ax)
    _draw_edges(graph, pos, ax)
    _draw_node_labels(graph, pos, ax)
    _draw_edge_labels(graph, pos, ax)
    _set_bounds(ax, pos)
    fig.tight_layout()
    return fig


def _build_graph(spec: str, meta: dict[str, Any]) -> nx.DiGraph:
    graph = nx.DiGraph()
    protocol = protocol_for_spec(spec)
    services = meta["services"]
    front = next((svc.key for svc in services if svc.role == "front"), services[0].key)

    graph.add_node("ingress", label=meta["entry"], kind="ingress", container="client")
    graph.add_node("jaeger", label="Jaeger", kind="observability", container="observability")
    for svc in services:
        container = "single app container" if spec == "single" else f"{svc.key} container"
        graph.add_node(svc.key, label=svc.label, kind="service", container=container)
        llm_node = f"{svc.key}_llm"
        graph.add_node(llm_node, label=svc.llm, kind="llm", container=container)
        graph.add_edge(svc.key, llm_node, label="LLM")
        graph.add_edge(svc.key, "jaeger", label="OTel")
    graph.add_edge("ingress", front, label="HTTP")
    for source, target, label in meta["edges"]:
        graph.add_edge(source, target, label=f"{protocol}: {label}")
    for source, target, label in meta["tools"]:
        graph.add_node(target, label=target, kind="tool", container="external/mock")
        graph.add_edge(source, target, label=label)
    return graph


def _layout(graph: nx.DiGraph, front: str) -> dict[str, tuple[float, float]]:
    services = [n for n, d in graph.nodes(data=True) if d["kind"] == "service"]
    workers = [n for n in services if n != front]
    pos: dict[str, tuple[float, float]] = {"ingress": (-9.2, 0), front: (-5.0, 0), "jaeger": (11.0, -5.3)}
    for node, y in zip(workers, np.linspace(5.7, -4.15, len(workers))):
        pos[node] = (2.35, float(y))
    for node, data in graph.nodes(data=True):
        if data["kind"] == "llm":
            sx, sy = pos.get(node.removesuffix("_llm"), (0, 0))
            pos[node] = (sx, sy - 1.45)
        elif data["kind"] == "tool":
            source = next((u for u, v in graph.in_edges(node)), front)
            sx, sy = pos.get(source, (0, 0))
            pos[node] = (sx + 4.75, sy + 1.15)
    return pos


def _draw_nodes(graph: nx.DiGraph, pos: dict[str, tuple[float, float]], ax: plt.Axes) -> None:
    styles = [
        ("ingress", "#1a365d", 1500, "s", 1.0),
        ("service", "#2f855a", 1850, "o", 1.0),
        ("llm", "#553c9a", 950, "o", 1.0),
        ("tool", "#c05621", 1050, "D", 1.0),
        ("observability", "#6b8aa3", 1200, "h", 0.38),
    ]
    for kind, color, size, shape, alpha in styles:
        nodes = [n for n, d in graph.nodes(data=True) if d["kind"] == kind]
        if nodes:
            nx.draw_networkx_nodes(
                graph,
                pos,
                nodelist=nodes,
                node_color=color,
                node_size=size,
                node_shape=shape,
                ax=ax,
                edgecolors="white",
                linewidths=1.8,
                alpha=alpha,
            )


def _draw_edges(graph: nx.DiGraph, pos: dict[str, tuple[float, float]], ax: plt.Axes) -> None:
    styles = [
        ("ingress", "#1a365d", "solid", 2.15, 0.95, "arc3,rad=0.03"),
        ("service", "#2d3748", "solid", 1.95, 0.88, "arc3,rad=0.10"),
        ("llm", "#6b46c1", "dotted", 1.45, 0.65, "arc3,rad=-0.05"),
        ("tool", "#dd6b20", "dashdot", 1.7, 0.78, "arc3,rad=0.12"),
        ("otel", "#5f7f95", "dashed", 1.15, 0.34, "arc3,rad=-0.18"),
    ]
    for group, color, style, width, alpha, connectionstyle in styles:
        edges = _edge_group(graph, group)
        if not edges:
            continue
        nx.draw_networkx_edges(
            graph,
            pos,
            edgelist=edges,
            ax=ax,
            arrows=True,
            arrowstyle="-|>",
            arrowsize=17 if group != "otel" else 13,
            width=width,
            style=style,
            alpha=alpha,
            edge_color=color,
            connectionstyle=connectionstyle,
        )


def _draw_node_labels(graph: nx.DiGraph, pos: dict[str, tuple[float, float]], ax: plt.Axes) -> None:
    label_pos = {}
    for node, (x, y) in pos.items():
        kind = graph.nodes[node]["kind"]
        if kind == "llm":
            label_pos[node] = (x, y - 0.46)
        elif kind == "tool":
            label_pos[node] = (x, y + 0.48)
        elif kind == "observability":
            label_pos[node] = (x, y - 0.52)
        else:
            label_pos[node] = (x, y + 0.58)
    nx.draw_networkx_labels(
        graph,
        label_pos,
        labels={n: _wrap_label(d["label"], 18) for n, d in graph.nodes(data=True)},
        ax=ax,
        font_size=9,
        font_color="#172033",
        bbox={"boxstyle": "round,pad=0.28", "fc": "white", "ec": "#cbd5e0", "alpha": 0.95},
    )


def _draw_edge_labels(graph: nx.DiGraph, pos: dict[str, tuple[float, float]], ax: plt.Axes) -> None:
    for edges, color, alpha in [
        (_edge_group(graph, "otel"), "#5f7f95", 0.62),
        (_edge_group(graph, "llm"), "#4c1d95", 0.82),
        (_edge_group(graph, "tool"), "#9c4221", 0.86),
        (_edge_group(graph, "ingress"), "#1a365d", 0.9),
        (_edge_group(graph, "service"), "#2d3748", 0.88),
    ]:
        if edges:
            nx.draw_networkx_edge_labels(
                graph,
                pos,
                edge_labels={(u, v): _wrap_label(graph.edges[u, v]["label"], 18) for u, v in edges},
                ax=ax,
                font_size=8,
                font_color=color,
                rotate=False,
                bbox={"boxstyle": "round,pad=0.2", "fc": "white", "ec": "none", "alpha": alpha},
            )


def _draw_container_boxes(ax: plt.Axes, graph: nx.DiGraph, pos: dict[str, tuple[float, float]]) -> None:
    containers: dict[str, list[str]] = defaultdict(list)
    for node, data in graph.nodes(data=True):
        container = data.get("container")
        if container and container not in {"client", "observability", "external/mock"}:
            containers[container].append(node)
    for container, nodes in containers.items():
        xs = [pos[n][0] for n in nodes if n in pos]
        ys = [pos[n][1] for n in nodes if n in pos]
        if not xs or not ys:
            continue
        x0, x1 = min(xs) - 1.38, max(xs) + 1.38
        y0, y1 = min(ys) - 1.18, max(ys) + 1.18
        rect = Rectangle(
            (x0, y0),
            x1 - x0,
            y1 - y0,
            linewidth=1.1,
            edgecolor="#a0aec0",
            facecolor="#edf2f7",
            alpha=0.48,
            zorder=0,
        )
        ax.add_patch(rect)
        ax.text(x0 + 0.08, y1 - 0.2, container, fontsize=8, color="#4a5568", va="top", zorder=1)
    if "jaeger" in pos:
        x, y = pos["jaeger"]
        ax.add_patch(
            Rectangle(
                (x - 1.05, y - 0.7),
                2.1,
                1.4,
                linewidth=1.2,
                edgecolor="#5f7f95",
                facecolor="#dbeafe",
                alpha=0.25,
                linestyle="--",
                hatch="///",
                zorder=0,
            )
        )


def _edge_group(graph: nx.DiGraph, group: str) -> list[tuple[str, str]]:
    edges: list[tuple[str, str]] = []
    for u, v, data in graph.edges(data=True):
        label = str(data.get("label", ""))
        if group == "otel" and v == "jaeger":
            edges.append((u, v))
        elif group == "llm" and label == "LLM":
            edges.append((u, v))
        elif group == "tool" and graph.nodes[v].get("kind") == "tool":
            edges.append((u, v))
        elif group == "ingress" and u == "ingress":
            edges.append((u, v))
        elif group == "service" and u != "ingress" and v != "jaeger" and label != "LLM" and graph.nodes[v].get("kind") != "tool":
            edges.append((u, v))
    return edges


def _set_bounds(ax: plt.Axes, pos: dict[str, tuple[float, float]]) -> None:
    xs = [p[0] for p in pos.values()]
    ys = [p[1] for p in pos.values()]
    ax.set_xlim(min(xs) - 2.4, max(xs) + 2.5)
    ax.set_ylim(min(ys) - 1.9, max(ys) + 2.1)


def _wrap_label(value: str, width: int) -> str:
    text = str(value)
    if len(text) <= width:
        return text
    parts = text.replace("_", " _").replace("-", " -").split()
    lines: list[str] = []
    current = ""
    for part in parts:
        piece = part.replace(" _", "_").replace(" -", "-")
        candidate = piece if not current else f"{current} {piece}"
        if len(candidate) <= width:
            current = candidate
        else:
            if current:
                lines.append(current)
            current = piece
    if current:
        lines.append(current)
    return "\n".join(lines)
