from __future__ import annotations

import argparse
from pathlib import Path

from .data import load_run
from .plots import generate_plots
from .report import write_report


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate plots for a saved benchmark run.")
    parser.add_argument("--run", required=True, dest="run_id", help="Run id under benchmark/results")
    parser.add_argument(
        "--results",
        default="../results",
        help="Results root, relative to benchmark/plots by default",
    )
    parser.add_argument(
        "--out",
        default="out",
        help="Output root, relative to benchmark/plots by default",
    )
    parser.add_argument(
        "--max-case-waterfalls",
        type=int,
        default=200,
        help="Maximum per-case waterfall charts to generate",
    )
    args = parser.parse_args()

    plots_root = Path(__file__).resolve().parents[1]
    results_root = Path(args.results)
    if not results_root.is_absolute():
        results_root = (plots_root / results_root).resolve()
    out_root = Path(args.out)
    if not out_root.is_absolute():
        out_root = (plots_root / out_root).resolve()
    out_dir = out_root / args.run_id

    data = load_run(results_root=results_root, run_id=args.run_id)
    plot_index = generate_plots(data, out_dir, max_case_waterfalls=args.max_case_waterfalls)
    write_report(data, plot_index, out_dir)

    print(f"Generated report: {out_dir / 'index.html'}")


if __name__ == "__main__":
    main()
