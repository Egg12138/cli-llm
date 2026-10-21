#!/usr/bin/env python3
"""Benchmark startup latency of the Go `llm` and `llm-session` binaries.

cli-llm is a fast, lightweight Q&A client, so "time from typing the command to
seeing an answer" is a product feature, not a micro-detail. This script isolates
the local half of that wait by pointing the CLI at an in-process
OpenAI-compatible stub that replies instantly. Network and provider latency are
therefore excluded, and what remains is overhead this repository owns:

    spawn -> HTTP request   binary init, config load, prompt build, HTTP connect
    spawn -> first token    the above plus the first streamed chunk being drawn

The reported numbers are wall-clock milliseconds per run, aggregated as
min / p50 / p95 / max over N runs after W warmups.

    scripts/benchmark_startup.py                      # human table
    scripts/benchmark_startup.py --json out.json      # machine-readable baseline
    scripts/benchmark_startup.py --baseline out.json  # fail on regressions
    scripts/benchmark_startup.py --ref /path/to/old-bins

`--ref` compares two built versions in one interleaved run (the reference
build supplies the baseline), which is the supported way to answer "did this
release make startup slower?".

The TUI scenario needs a PTY (it runs `llm-session` on a pseudo-terminal like a
real terminal); it is skipped with a warning when no PTY is available.
"""

from __future__ import annotations

import argparse
import fcntl
import json
import os
import platform
import pty
import selectors
import shutil
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import threading
import time
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Callable

ROOT = Path(__file__).resolve().parents[1]
SRC_GO = ROOT / "src-go"

MODEL = "bench-model"
MARKER = "benchok"
PROMPT = "benchmark startup latency probe"
TITLE_HINT = "descriptive title"

DEFAULT_RUNS = 12
DEFAULT_WARMUPS = 3
DEFAULT_TIMEOUT = 15.0

# Metrics that make up the reported table, in display order. Scenarios not
# selected with --scenario are simply absent from the output.
SCENARIOS = {
    "version": ["llm.version"],
    "inspect": ["llm.inspect"],
    "dispatch": ["llm.session-dispatch"],
    "chat": ["chat.request", "chat.first-token", "chat.exit"],
    "session": ["session.request", "session.first-token", "session.exit"],
    "tui": [
        "session.tui.first-paint",
        "session.tui.editor-ready",
        "session.tui.request",
        "session.tui.first-token",
    ],
}

METRIC_DESCRIPTIONS = {
    "llm.version": "llm --version: binary + runtime init floor",
    "llm.inspect": "llm inspect: config load and provider merge",
    "llm.session-dispatch": "llm session --version: PATH lookup + exec",
    "chat.request": "llm chat: spawn -> provider request",
    "chat.first-token": "llm chat: spawn -> first streamed text",
    "chat.exit": "llm chat: spawn -> process exit",
    "session.request": "llm-session --no-tui: spawn -> provider request",
    "session.first-token": "llm-session --no-tui: spawn -> first streamed text",
    "session.exit": "llm-session --no-tui: spawn -> process exit",
    "session.tui.first-paint": "llm-session TUI: spawn -> first terminal output",
    "session.tui.editor-ready": "llm-session TUI: spawn -> editor accepting input",
    "session.tui.request": "llm-session TUI: spawn -> provider request",
    "session.tui.first-token": "llm-session TUI: spawn -> first streamed text",
}


class BenchError(RuntimeError):
    """A scenario could not produce a trustworthy measurement."""


@dataclass
class RunRecord:
    """Timing hand-off between the bench process and the stub server thread."""

    spawn: float = 0.0
    request: float | None = None
    requests: list[tuple[float, str, bool]] = field(default_factory=list)


class StubState:
    """Serial run bookkeeping for the stub: one RunRecord is active at a time."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._current: RunRecord | None = None

    def begin(self) -> RunRecord:
        record = RunRecord()
        with self._lock:
            self._current = record
        return record

    def note_request(self, kind: str, stream: bool) -> None:
        now = time.monotonic()
        with self._lock:
            record = self._current
            if record is None:
                return
            record.requests.append((now, kind, stream))
            if kind == "chat" and record.request is None:
                record.request = now


def _json_response(wfile, payload: dict, *, event_stream: bool = False) -> None:
    body = json.dumps(payload, separators=(",", ":")).encode()
    if event_stream:
        body = b"data: " + body + b"\n\n"
    wfile.write(body)
    wfile.flush()


class _Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.0"  # closing the socket delimits the SSE body
    stub: StubState

    def log_message(self, *args: object) -> None:
        _ = args

    def do_POST(self) -> None:
        try:
            length = int(self.headers.get("Content-Length") or 0)
            raw = self.rfile.read(length) if length else b"{}"
            body = json.loads(raw or b"{}")
        except (ValueError, json.JSONDecodeError):
            self.send_error(400, "invalid request body")
            return

        # The session's first turn issues a title request before the chat
        # request; only the latter measures the path the user waits on.
        kind = "title" if TITLE_HINT in raw.decode("utf-8", "replace") else "chat"
        stream = bool(body.get("stream"))
        self.stub.note_request(kind, stream)

        self.send_response(200)
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        try:
            if kind == "chat" and stream:
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                self._write_chunk({"content": MARKER})
                self._write_chunk({}, finish_reason="stop")
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()
            else:
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                _json_response(
                    self.wfile,
                    self._completion(MARKER if kind == "chat" else "Bench Session"),
                )
        except (BrokenPipeError, ConnectionResetError):
            # The client is allowed to cancel (Ctrl+C); nothing to report.
            pass

    def _write_chunk(self, delta: dict, finish_reason: str | None = None) -> None:
        _json_response(
            self.wfile,
            {
                "id": "chunk",
                "object": "chat.completion.chunk",
                "created": 1,
                "model": MODEL,
                "choices": [
                    {"index": 0, "delta": delta, "finish_reason": finish_reason},
                ],
            },
            event_stream=True,
        )

    @staticmethod
    def _completion(content: str) -> dict:
        return {
            "id": "completion",
            "object": "chat.completion",
            "created": 1,
            "model": MODEL,
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": content},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        }


class StubServer:
    """Minimal instant OpenAI-compatible endpoint on an ephemeral local port."""

    def __init__(self) -> None:
        self.state = StubState()
        handler = type("BoundHandler", (_Handler,), {"stub": self.state})
        self._server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
        self._server.daemon_threads = True
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)
        self._thread.start()

    @property
    def base_url(self) -> str:
        host, port = self._server.server_address[:2]
        return f"http://{host}:{port}/v1"

    def close(self) -> None:
        self._server.shutdown()
        self._server.server_close()
        self._thread.join(timeout=5)


@dataclass
class Capture:
    """Result of reading a child process's output until exit or deadline."""

    first_output: float | None
    marker: float | None
    tail: bytes


def _read_until_exit(stream, deadline: float, marker: str | None) -> Capture:
    """Drain ``stream`` until EOF or ``deadline``, timing marker appearance.

    Reading continues after the marker is seen so a chatty child never blocks on
    a full pipe; the loop still stops at EOF, which is what makes `exit`
    metrics meaningful.
    """
    fd = stream.fileno()
    marker_bytes = marker.encode() if marker else None
    selector = selectors.DefaultSelector()
    selector.register(fd, selectors.EVENT_READ)
    buffer = bytearray()
    first: float | None = None
    seen: float | None = None
    try:
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                break
            if not selector.select(min(remaining, 0.05)):
                continue
            try:
                chunk = os.read(fd, 65536)
            except OSError:
                break
            if not chunk:
                break
            now = time.monotonic()
            if first is None:
                first = now
            buffer.extend(chunk)
            if seen is None and marker_bytes is not None and marker_bytes in buffer:
                seen = now
            if len(buffer) > 1 << 20:
                # Keep memory bounded but never lose the tail we report on failure.
                del buffer[: len(buffer) - 65536]
    finally:
        selector.close()
    return Capture(first, seen, bytes(buffer[-4096:]))


def _set_window_size(fd: int, columns: int = 80, rows: int = 24) -> None:
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, columns, 0, 0))


def _kill(pid: int) -> None:
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


class Bench:
    def __init__(
        self,
        llm: Path,
        session: Path,
        stub: StubServer,
        env: dict[str, str],
        timeout: float,
    ):
        self.llm = llm
        self.session = session
        self.stub = stub
        self.env = env
        self.timeout = timeout

    def _timed(self, argv: list[str]) -> dict[str, float]:
        spawn = time.monotonic()
        result = subprocess.run(
            argv,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            env=self.env,
            timeout=self.timeout,
            check=False,
        )
        if result.returncode != 0:
            raise BenchError(
                f"{' '.join(argv)} exited {result.returncode}: "
                f"{result.stderr.decode('utf-8', 'replace').strip()[:400]}"
            )
        return {"elapsed": time.monotonic() - spawn}

    def version(self) -> dict[str, float]:
        return {
            "llm.version": self._timed([str(self.llm), "--version"])["elapsed"] * 1000
        }

    def inspect(self) -> dict[str, float]:
        return {
            "llm.inspect": self._timed([str(self.llm), "inspect"])["elapsed"] * 1000
        }

    def dispatch(self) -> dict[str, float]:
        return {
            "llm.session-dispatch": self._timed(
                [str(self.llm), "session", "--version"]
            )["elapsed"]
            * 1000
        }

    def _expect_request(self, record: RunRecord, scenario: str) -> float:
        if record.request is None:
            raise BenchError(
                f"{scenario}: the provider stub saw no chat request "
                f"(requests: {[(kind, stream) for _, kind, stream in record.requests]})"
            )
        return (record.request - record.spawn) * 1000

    def _expect_marker(
        self, capture: Capture, scenario: str, record: RunRecord
    ) -> float:
        if capture.marker is None:
            raise BenchError(
                f"{scenario}: response marker never reached the terminal "
                f"(requests: {[(kind, stream) for _, kind, stream in record.requests]}, "
                f"tail: {capture.tail.decode('utf-8', 'replace')!r})"
            )
        return (capture.marker - record.spawn) * 1000

    def chat(self) -> dict[str, float]:
        record = self.stub.state.begin()
        record.spawn = time.monotonic()
        proc = subprocess.Popen(
            [str(self.llm), "chat", PROMPT],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            env=self.env,
        )
        try:
            capture = _read_until_exit(proc.stdout, record.spawn + self.timeout, MARKER)
            proc.wait(timeout=self.timeout)
        finally:
            proc.stdout.close()
            if proc.poll() is None:
                proc.kill()
        return {
            "chat.request": self._expect_request(record, "llm chat"),
            "chat.first-token": self._expect_marker(capture, "llm chat", record),
            "chat.exit": (time.monotonic() - record.spawn) * 1000,
        }

    def session_no_tui(self) -> dict[str, float]:
        record = self.stub.state.begin()
        record.spawn = time.monotonic()
        proc = subprocess.Popen(
            [str(self.session), "--no-tui"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            env=self.env,
        )
        try:
            assert proc.stdin is not None
            proc.stdin.write(f"{PROMPT}\n/exit\n".encode())
            proc.stdin.flush()
            capture = _read_until_exit(proc.stdout, record.spawn + self.timeout, MARKER)
            proc.wait(timeout=self.timeout)
        finally:
            if proc.stdin is not None:
                proc.stdin.close()
            proc.stdout.close()
            if proc.poll() is None:
                proc.kill()
        return {
            "session.request": self._expect_request(record, "llm-session --no-tui"),
            "session.first-token": self._expect_marker(
                capture, "llm-session --no-tui", record
            ),
            "session.exit": (time.monotonic() - record.spawn) * 1000,
        }

    def session_tui(self) -> dict[str, float]:
        record = self.stub.state.begin()
        record.spawn = time.monotonic()
        pid, fd = pty.fork()
        if pid == 0:  # child
            try:
                _set_window_size(0)
                os.execve(str(self.session), [str(self.session)], self.env)
            except OSError:
                os._exit(127)

        deadline = record.spawn + self.timeout
        selector = selectors.DefaultSelector()
        selector.register(fd, selectors.EVENT_READ)
        buffer = bytearray()
        first_paint: float | None = None
        editor_ready: float | None = None
        marker_at: float | None = None
        typed_prompt = False
        typed_exit = False
        exited = False
        try:
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    break
                if not selector.select(min(remaining, 0.05)):
                    continue
                try:
                    chunk = os.read(fd, 65536)
                except OSError:  # EIO once the child side is gone
                    break
                if not chunk:
                    break
                now = time.monotonic()
                if first_paint is None:
                    first_paint = now
                buffer.extend(chunk)
                # termenv/bubbletea query the terminal for background color and
                # cursor position at startup. Without answers those queries time
                # out and, worse, swallow typed input as their reply.
                for _ in range(chunk.count(b"\x1b]11;?")):
                    os.write(fd, b"\x1b]11;rgb:0000/0000/0000\x1b\\")
                for _ in range(chunk.count(b"\x1b[6n")):
                    os.write(fd, b"\x1b[1;1R")
                if editor_ready is None and b"INSERT" in buffer:
                    editor_ready = now
                if not typed_prompt and editor_ready is not None:
                    os.write(fd, f"{PROMPT}\r".encode())
                    typed_prompt = True
                if marker_at is None and MARKER.encode() in buffer:
                    marker_at = now
                if marker_at is not None and not typed_exit:
                    os.write(fd, b"/exit\r")
                    typed_exit = True
                if len(buffer) > 1 << 20:
                    del buffer[: len(buffer) - 65536]
            if typed_exit:
                exited = self._wait_pid(pid, timeout=3.0)
        finally:
            selector.close()
            if not exited:
                _kill(pid)
                self._wait_pid(pid, timeout=3.0)
            os.close(fd)

        tail = bytes(buffer[-4096:])
        if first_paint is None:
            raise BenchError(
                f"llm-session TUI: no terminal output (tail: {tail.decode('utf-8', 'replace')!r})"
            )
        if editor_ready is None:
            raise BenchError(
                f"llm-session TUI: editor never became ready (tail: {tail.decode('utf-8', 'replace')!r})"
            )
        if marker_at is None:
            raise BenchError(
                f"llm-session TUI: response marker never painted (tail: {tail.decode('utf-8', 'replace')!r})"
            )
        return {
            "session.tui.first-paint": (first_paint - record.spawn) * 1000,
            "session.tui.editor-ready": (editor_ready - record.spawn) * 1000,
            "session.tui.request": self._expect_request(record, "llm-session TUI"),
            "session.tui.first-token": (marker_at - record.spawn) * 1000,
        }

    @staticmethod
    def _wait_pid(pid: int, timeout: float) -> bool:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            try:
                done, _ = os.waitpid(pid, os.WNOHANG)
            except ChildProcessError:
                return True
            if done == pid:
                return True
            time.sleep(0.01)
        return False


SCENARIO_FUNCS: dict[str, str] = {
    "version": "version",
    "inspect": "inspect",
    "dispatch": "dispatch",
    "chat": "chat",
    "session": "session_no_tui",
    "tui": "session_tui",
}


def _percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    if len(ordered) == 1:
        return ordered[0]
    index = min(len(ordered) - 1, max(0, round(fraction * (len(ordered) - 1))))
    return ordered[index]


def _stats(values: list[float]) -> dict[str, float]:
    return {
        "runs": len(values),
        "min_ms": min(values),
        "p50_ms": _percentile(values, 0.50),
        "p95_ms": _percentile(values, 0.95),
        "max_ms": max(values),
    }


def run_scenarios(
    bench: Bench,
    names: list[str],
    runs: int,
    warmups: int,
    verbose: bool,
    ref: Bench | None = None,
) -> tuple[dict[str, list[float]], dict[str, list[float]], list[str]]:
    """Measure each scenario, interleaving with ``ref`` when comparing builds.

    Alternating the two builds cancels slow drift (thermal, background load) that
    would otherwise bias whichever build ran second.
    """
    samples: dict[str, list[float]] = {
        metric: [] for name in names for metric in SCENARIOS[name]
    }
    ref_samples: dict[str, list[float]] = {
        metric: [] for name in names for metric in SCENARIOS[name]
    }
    notes: list[str] = []
    for name in names:
        func: Callable[[], dict[str, float]] = getattr(bench, SCENARIO_FUNCS[name])
        ref_func: Callable[[], dict[str, float]] | None = (
            getattr(ref, SCENARIO_FUNCS[name]) if ref else None
        )

        def warm(
            build: str, target: Callable[[], dict[str, float]], scenario: str
        ) -> bool:
            for index in range(warmups):
                try:
                    target()
                except BenchError as error:
                    print(
                        f"warning: {build} warmup for {scenario} failed: {error}",
                        file=sys.stderr,
                    )
                    return False
                if verbose:
                    print(
                        f"  warmup {index + 1}/{warmups} {build} {scenario}",
                        file=sys.stderr,
                    )
            return True

        primary_ok = warm("new", func, name)
        ref_ok = warm("ref", ref_func, name) if ref_func else False
        for index in range(runs):
            # Alternate which build goes first on each run.
            order: list[
                tuple[
                    str, Callable[[], dict[str, float]], dict[str, list[float]] | None
                ]
            ] = []
            if ref_func and ref_ok:
                order = [("ref", ref_func, ref_samples)]
            if primary_ok:
                order.append(("new", func, samples))
            if index % 2 == 1:
                order.reverse()
            failed = False
            for build, target, sink in order:
                try:
                    measured = target()
                except BenchError as error:
                    if name in {"tui", "session"}:
                        notes.append(f"{build} {name}: skipped after failure: {error}")
                        failed = True
                        break
                    raise
                if sink is not None:
                    for metric, value in measured.items():
                        sink[metric].append(value)
                if verbose:
                    detail = " ".join(
                        f"{metric}={value:.1f}ms" for metric, value in measured.items()
                    )
                    print(
                        f"  run {index + 1}/{runs} {build} {name}: {detail}",
                        file=sys.stderr,
                    )
            if failed:
                break
    return samples, ref_samples, notes


def build_binaries(destination: Path) -> tuple[Path, Path]:
    go = shutil.which("go")
    if not go:
        raise BenchError(
            "go toolchain not found; pass --llm/--llm-session or install Go"
        )
    for package, name in (("llm", "llm"), ("llm-session", "llm-session")):
        target = destination / name
        result = subprocess.run(
            [go, "build", "-o", str(target), f"./cmd/{package}"],
            cwd=SRC_GO,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
        )
        if result.returncode != 0:
            raise BenchError(
                f"go build ./cmd/{package} failed:\n{result.stdout.decode('utf-8', 'replace')}"
            )
    return destination / "llm", destination / "llm-session"


def load_baseline(path: Path) -> dict:
    try:
        return json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as error:
        raise BenchError(f"cannot read baseline {path}: {error}") from error


def compare_with_baseline(
    results: dict[str, dict[str, float]],
    baseline: dict,
    tolerance_pct: float,
    floor_ms: float,
) -> list[str]:
    regressions: list[str] = []
    metrics = baseline.get("metrics", {})
    for metric, stats in results.items():
        old = metrics.get(metric, {}).get("p50_ms")
        if old is None:
            continue
        new = stats["p50_ms"]
        allowed = old * (1 + tolerance_pct / 100)
        if new <= allowed or new - old < floor_ms:
            continue
        regressions.append(
            f"{metric}: p50 {new:.1f} ms vs baseline {old:.1f} ms "
            f"(+{new - old:.1f} ms, +{(new / old - 1) * 100:.0f}%; limit +{tolerance_pct:.0f}% and +{floor_ms:.0f} ms)"
        )
    return regressions


def print_table(results: dict[str, dict[str, float]], baseline: dict | None) -> None:
    headers = ["metric", "min", "p50", "p95", "max"]
    if baseline:
        headers.append("Δ p50")
    rows = []
    for metric, stats in results.items():
        row = [
            metric,
            f"{stats['min_ms']:.1f}",
            f"{stats['p50_ms']:.1f}",
            f"{stats['p95_ms']:.1f}",
            f"{stats['max_ms']:.1f}",
        ]
        if baseline:
            old = baseline.get("metrics", {}).get(metric, {}).get("p50_ms")
            row.append(
                "—" if old is None else f"{(stats['p50_ms'] / old - 1) * 100:+.1f}%"
            )
        rows.append(row)
    widths = [
        max(len(str(row[i])) for row in [headers] + rows) for i in range(len(headers))
    ]
    print()
    print("  ".join(header.ljust(widths[i]) for i, header in enumerate(headers)))
    print("  ".join("-" * width for width in widths))
    for row in rows:
        print("  ".join(str(value).ljust(widths[i]) for i, value in enumerate(row)))


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Benchmark startup latency of llm and llm-session against a local instant provider.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="\n".join(
            [
                "metrics:",
                *[f"  {name:<24}{desc}" for name, desc in METRIC_DESCRIPTIONS.items()],
            ]
        ),
    )
    parser.add_argument(
        "--llm", type=Path, help="path to a prebuilt llm binary (default: go build)"
    )
    parser.add_argument(
        "--llm-session", type=Path, help="path to a prebuilt llm-session binary"
    )
    parser.add_argument(
        "--ref",
        type=Path,
        metavar="DIR",
        help="compare against another build directory containing llm and llm-session (A/B mode)",
    )
    parser.add_argument(
        "--runs",
        type=int,
        default=DEFAULT_RUNS,
        help=f"measured runs per scenario (default {DEFAULT_RUNS})",
    )
    parser.add_argument(
        "--warmups",
        type=int,
        default=DEFAULT_WARMUPS,
        help=f"discarded runs per scenario (default {DEFAULT_WARMUPS})",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=DEFAULT_TIMEOUT,
        help="per-run timeout in seconds",
    )
    parser.add_argument(
        "--scenario",
        action="append",
        choices=sorted(SCENARIOS),
        help="only run the named scenario (repeatable)",
    )
    parser.add_argument(
        "--json", type=Path, help="write results as a baseline JSON file"
    )
    parser.add_argument(
        "--baseline", type=Path, help="compare against a baseline JSON file"
    )
    parser.add_argument(
        "--max-regression",
        type=float,
        default=20.0,
        help="max allowed p50 regression in percent (default 20)",
    )
    parser.add_argument(
        "--regression-floor",
        type=float,
        default=3.0,
        help="ignore regressions smaller than this in ms (default 3)",
    )
    parser.add_argument(
        "--strict-tui",
        action="store_true",
        help="fail instead of skipping when the TUI scenario fails",
    )
    parser.add_argument(
        "--verbose", action="store_true", help="print per-run timings to stderr"
    )
    args = parser.parse_args(argv)

    if args.runs < 1:
        parser.error("--runs must be at least 1")

    names = args.scenario or list(SCENARIOS)
    baseline = load_baseline(args.baseline) if args.baseline else None

    scratch: tempfile.TemporaryDirectory[str] | None = None
    try:
        llm, session = args.llm, args.llm_session
        if llm is None or session is None:
            scratch = tempfile.TemporaryDirectory(prefix="cli-llm-bench-")
            built_llm, built_session = build_binaries(Path(scratch.name))
            llm = llm or built_llm
            session = session or built_session
        if not llm.is_file():
            raise BenchError(f"llm binary not found: {llm}")
        if not session.is_file():
            raise BenchError(f"llm-session binary not found: {session}")
        if args.ref and args.baseline:
            raise BenchError(
                "--ref and --baseline are mutually exclusive; --ref supplies the comparison baseline"
            )

        home = tempfile.mkdtemp(prefix="cli-llm-bench-home-")
        stub = StubServer()

        def environment(binary: Path, home: str) -> dict[str, str]:
            return {
                "HOME": home,
                "PATH": os.pathsep.join(
                    [str(binary.parent), os.environ.get("PATH", "")]
                ),
                "TERM": "xterm-256color",
                "LANG": "C.UTF-8",
                "LC_ALL": "C.UTF-8",
                "TZ": "UTC",
                "OPENAI_API_KEY": "bench-key",
                "OPENAI_BASE_URL": stub.base_url,
                "OPENAI_MODEL": MODEL,
            }

        ref_bench: Bench | None = None
        if args.ref:
            ref_llm, ref_session = args.ref / "llm", args.ref / "llm-session"
            if not ref_llm.is_file() or not ref_session.is_file():
                raise BenchError(
                    f"--ref {args.ref} must contain llm and llm-session binaries"
                )
            ref_home = tempfile.mkdtemp(prefix="cli-llm-bench-ref-home-")
            ref_bench = Bench(
                ref_llm, ref_session, stub, environment(ref_llm, ref_home), args.timeout
            )
        else:
            ref_home = None
        try:
            bench = Bench(llm, session, stub, environment(llm, home), args.timeout)
            samples, ref_samples, notes = run_scenarios(
                bench, names, args.runs, args.warmups, args.verbose, ref_bench
            )
        finally:
            stub.close()
            shutil.rmtree(home, ignore_errors=True)
            if ref_home:
                shutil.rmtree(ref_home, ignore_errors=True)

        if args.strict_tui:
            fatal = [
                note for note in notes if note.startswith("tui:") or " tui:" in note
            ]
            if fatal:
                raise BenchError("; ".join(fatal))

        results = {
            metric: _stats(values) for metric, values in samples.items() if values
        }
        ref_results = {
            metric: _stats(values) for metric, values in ref_samples.items() if values
        }
        if not results:
            raise BenchError("no measurements collected")
        if ref_bench and not ref_results:
            raise BenchError("no reference measurements collected")

        print(
            f"Startup baseline — {llm} / {session.name} "
            f"({platform.system().lower()}/{platform.machine()}), "
            f"{args.runs} runs, {args.warmups} warmups"
        )
        print(
            f"local instant provider at {stub.base_url}; network and provider latency excluded"
        )
        if ref_bench:
            print(f"reference build: {ref_bench.llm} / {ref_bench.session.name}")
            baseline = {"metrics": ref_results}
        print_table(results, baseline)
        for note in notes:
            print(f"warning: {note}", file=sys.stderr)

        if args.json:
            payload = {
                "schema": 1,
                "platform": f"{platform.system().lower()}/{platform.machine()}",
                "runs": args.runs,
                "metrics": results,
            }
            if ref_results:
                payload["ref_metrics"] = ref_results
            args.json.parent.mkdir(parents=True, exist_ok=True)
            args.json.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
            print(f"\nwrote baseline to {args.json}")

        if baseline:
            regressions = compare_with_baseline(
                results, baseline, args.max_regression, args.regression_floor
            )
            if regressions:
                print("\nstartup regressions:", file=sys.stderr)
                for regression in regressions:
                    print(f"  {regression}", file=sys.stderr)
                return 1
            print("\nno startup regressions beyond the configured limits")
        return 0
    except BenchError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    finally:
        if scratch is not None:
            scratch.cleanup()


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
