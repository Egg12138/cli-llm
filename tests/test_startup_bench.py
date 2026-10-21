"""Tests for the startup benchmark harness (scripts/benchmark_startup.py).

The harness measures built binaries, so these tests exercise the parts that are
cheap and deterministic offline: aggregation, regression comparison, the stub
provider, and one end-to-end timing scenario using a stand-in executable.
"""

from __future__ import annotations

import importlib.util
import json
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def _load_bench():
    spec = importlib.util.spec_from_file_location(
        "benchmark_startup", ROOT / "scripts" / "benchmark_startup.py"
    )
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module  # dataclasses resolve annotations via sys.modules
    spec.loader.exec_module(module)
    return module


bench = _load_bench()


def test_percentile_reports_median_and_tail() -> None:
    values = [10.0, 20.0, 30.0, 40.0, 50.0]
    assert bench._percentile(values, 0.50) == 30.0
    assert bench._percentile(values, 0.95) == 50.0
    stats = bench._stats(values)
    assert stats["runs"] == 5
    assert stats["min_ms"] == 10.0
    assert stats["max_ms"] == 50.0
    assert stats["p50_ms"] == 30.0


def test_regression_comparison_respects_tolerance_and_floor() -> None:
    baseline = {"metrics": {"chat.request": {"p50_ms": 100.0}}}

    def check(p50: float) -> list[str]:
        return bench.compare_with_baseline(
            {"chat.request": {"p50_ms": p50}},
            baseline,
            tolerance_pct=20.0,
            floor_ms=3.0,
        )

    assert check(110.0) == []  # within 20%
    assert check(101.5) == []  # above baseline but below the noise floor
    assert check(130.0)  # 30% and 30 ms: a real regression


def test_stub_provider_streams_marker_and_classifies_requests() -> None:
    stub = bench.StubServer()
    try:
        record = stub.state.begin()
        record.spawn = bench.time.monotonic()

        stream_body = json.dumps(
            {"stream": True, "messages": [{"role": "user", "content": bench.PROMPT}]}
        ).encode()
        request = urllib.request.Request(
            stub.base_url + "/chat/completions",
            data=stream_body,
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(request, timeout=5) as response:
            streamed = response.read()
        assert bench.MARKER.encode() in streamed
        assert b"[DONE]" in streamed

        title_body = json.dumps(
            {
                "stream": False,
                "messages": [
                    {"role": "user", "content": f"write a {bench.TITLE_HINT}"}
                ],
            }
        ).encode()
        request = urllib.request.Request(
            stub.base_url + "/chat/completions",
            data=title_body,
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(request, timeout=5) as response:
            titled = json.loads(response.read())

        assert titled["choices"][0]["message"]["content"] == "Bench Session"
        kinds = [kind for _, kind, _ in record.requests]
        assert kinds == ["chat", "title"]
        assert record.request is not None
    finally:
        stub.close()


def test_timing_scenario_measures_a_stand_in_binary() -> None:
    stub = bench.StubServer()
    try:
        environment = {"PATH": "/usr/bin:/bin", "OPENAI_API_KEY": "bench-key"}
        harness = bench.Bench(
            Path(sys.executable), Path(sys.executable), stub, environment, timeout=10
        )
        measured = harness.version()
        assert measured["llm.version"] > 0
    finally:
        stub.close()
