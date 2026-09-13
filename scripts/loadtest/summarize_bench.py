#!/usr/bin/env python3
"""Print a compact summary table from one or more k6 --summary-export JSON files."""

from __future__ import annotations

import json
import sys
from pathlib import Path


def metric(data: dict, name: str) -> dict:
    # k6 --summary-export puts stats directly on the metric object (not under "values").
    return data.get("metrics", {}).get(name, {}) or {}


def fmt_ms(v) -> str:
    if v is None:
        return "-"
    return f"{float(v):.2f}ms"


def fmt_pct(v) -> str:
    if v is None:
        return "-"
    return f"{float(v) * 100:.2f}%"


def summarize(path: Path, label: str) -> tuple[str, str, str, str]:
    data = json.loads(path.read_text(encoding="utf-8"))
    duration = metric(data, "http_req_duration")
    checks = metric(data, "checks")
    p95 = duration.get("p(95)", duration.get("p95"))
    med = duration.get("med")
    # Prefer check failure rate: http_req_failed treats expected 429s as failures.
    if "value" in checks:
        fail_rate = 1.0 - float(checks["value"])
    else:
        failed = metric(data, "http_req_failed")
        fail_rate = failed.get("rate", failed.get("value"))
    return label, fmt_ms(p95), fmt_ms(med), fmt_pct(fail_rate)


def main() -> int:
    if len(sys.argv) < 2 or len(sys.argv) % 2 != 1:
        print(
            "usage: summarize_bench.py <label> <summary.json> [label summary.json ...]",
            file=sys.stderr,
        )
        return 2

    rows = []
    args = sys.argv[1:]
    for i in range(0, len(args), 2):
        label, path = args[i], Path(args[i + 1])
        rows.append(summarize(path, label))

    headers = ("run", "p95", "median", "check_fail")
    widths = [max(len(headers[i]), max(len(r[i]) for r in rows)) for i in range(4)]

    def fmt_row(cols: tuple[str, ...]) -> str:
        return "  ".join(c.ljust(widths[i]) for i, c in enumerate(cols))

    print()
    print("=== AutOps bench summary ===")
    print(fmt_row(headers))
    print(fmt_row(tuple("-" * w for w in widths)))
    for row in rows:
        print(fmt_row(row))
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
