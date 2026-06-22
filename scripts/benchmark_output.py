#!/usr/bin/env python3
"""Compare Python and Go markdown output rendering performance."""

from __future__ import annotations

import argparse
import io
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
SRC_GO = ROOT / "src-go"

MARKDOWN_SAMPLE = """# Output Benchmark

This paragraph includes **bold text**, `inline code`, and a short list.

- first item
- second item
- third item

```go
package main

import "fmt"

func main() {
\tfmt.Println("hello from cli-llm")
}
```
"""


@dataclass(frozen=True)
class BenchResult:
    name: str
    iterations: int
    ns_per_op: float
    extra: str = ""

    @property
    def ops_per_second(self) -> float:
        if self.ns_per_op == 0:
            return 0.0
        return 1_000_000_000 / self.ns_per_op


def benchmark_python(iterations: int) -> BenchResult:
    sys.path.insert(0, str(SRC))

    from rich.console import Console
    from rich.markdown import Markdown

    buffer = io.StringIO()
    console = Console(
        file=buffer,
        force_terminal=True,
        color_system="256",
        width=80,
    )

    for _ in range(max(1, iterations // 10)):
        buffer.seek(0)
        buffer.truncate(0)
        console.print(Markdown(MARKDOWN_SAMPLE))

    start = time.perf_counter_ns()
    for _ in range(iterations):
        buffer.seek(0)
        buffer.truncate(0)
        console.print(Markdown(MARKDOWN_SAMPLE))
    elapsed = time.perf_counter_ns() - start

    return BenchResult(
        name="python-rich",
        iterations=iterations,
        ns_per_op=elapsed / iterations,
    )


def benchmark_go(benchtime: str) -> BenchResult:
    go_cache = Path(os.environ.get("GOCACHE", "/tmp/cli-llm-go-build-cache"))
    go_cache.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["GOCACHE"] = str(go_cache)

    command = [
        "go",
        "test",
        "./internal/render",
        "-run",
        "^$",
        "-bench",
        "BenchmarkDefaultRenderMarkdown",
        "-benchmem",
        "-benchtime",
        benchtime,
    ]
    completed = subprocess.run(
        command,
        cwd=SRC_GO,
        env=env,
        check=True,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )

    pattern = re.compile(
        r"BenchmarkDefaultRenderMarkdown-\d+\s+"
        r"(?P<iterations>\d+)\s+"
        r"(?P<ns_per_op>[0-9.]+)\s+ns/op\s+"
        r"(?P<bytes_per_op>[0-9.]+)\s+B/op\s+"
        r"(?P<allocs_per_op>[0-9.]+)\s+allocs/op"
    )
    match = pattern.search(completed.stdout)
    if not match:
        raise RuntimeError(f"could not parse go benchmark output:\n{completed.stdout}")

    return BenchResult(
        name="go-glamour",
        iterations=int(match.group("iterations")),
        ns_per_op=float(match.group("ns_per_op")),
        extra=f"{match.group('bytes_per_op')} B/op, {match.group('allocs_per_op')} allocs/op",
    )


def print_results(results: list[BenchResult]) -> None:
    fastest = min(result.ns_per_op for result in results)
    print("Output renderer benchmark")
    print(f"sample_bytes={len(MARKDOWN_SAMPLE.encode('utf-8'))}")
    print()
    print(f"{'runtime':<14} {'iters':>10} {'ms/op':>12} {'ops/s':>12} {'relative':>10}  details")
    for result in results:
        relative = result.ns_per_op / fastest if fastest else 0.0
        print(
            f"{result.name:<14} "
            f"{result.iterations:>10} "
            f"{result.ns_per_op / 1_000_000:>12.3f} "
            f"{result.ops_per_second:>12.1f} "
            f"{relative:>9.2f}x  "
            f"{result.extra}"
        )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Benchmark local Python Rich output rendering against Go glamour rendering."
    )
    parser.add_argument(
        "--python-iterations",
        type=int,
        default=200,
        help="Number of Python render iterations.",
    )
    parser.add_argument(
        "--go-benchtime",
        default="1s",
        help="Go benchmark benchtime value passed to go test.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.python_iterations <= 0:
        print("--python-iterations must be positive", file=sys.stderr)
        return 2

    results = [
        benchmark_python(args.python_iterations),
        benchmark_go(args.go_benchtime),
    ]
    print_results(results)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
