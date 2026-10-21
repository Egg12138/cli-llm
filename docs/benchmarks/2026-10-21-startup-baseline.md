# Startup performance baseline (Go runtimes)

cli-llm is a lightweight Q&A client, so startup latency is a feature: the
useful number is "time from pressing Enter to seeing the first token", not
throughput. `scripts/benchmark_startup.py` measures the part of that wait this
repository owns.

## What is measured

The bench points `llm` / `llm-session` at an in-process OpenAI-compatible stub
that answers instantly, so provider and network latency are excluded. Every
metric is a wall-clock duration starting at process spawn:

| metric | meaning |
|---|---|
| `llm.version` | `llm --version`: binary + Go runtime init floor |
| `llm.inspect` | `llm inspect`: config load and provider profile merge |
| `llm.session-dispatch` | `llm session --version`: PATH lookup + `exec` of the plugin |
| `chat.request` | `llm chat "..."`: spawn → provider request sent |
| `chat.first-token` | `llm chat "..."`: spawn → first streamed text on stdout |
| `chat.exit` | `llm chat "..."`: spawn → process exit |
| `session.request` | `llm-session --no-tui`: spawn → provider request sent |
| `session.first-token` | `llm-session --no-tui`: spawn → first streamed text |
| `session.exit` | `llm-session --no-tui`: spawn → process exit |
| `session.tui.first-paint` | `llm-session`: spawn → first terminal output |
| `session.tui.editor-ready` | `llm-session`: spawn → editor accepting input |
| `session.tui.request` | `llm-session`: spawn → provider request sent |
| `session.tui.first-token` | `llm-session`: spawn → first streamed text painted |

The TUI scenarios run the real binary on a PTY (`pty.fork`), answer the
terminal's background-color and cursor-position queries, and type the prompt as
soon as the editor reports `INSERT` — so the measurement includes the actual
Bubble Tea startup path, not a synthetic approximation.

## Running it

```bash
scripts/benchmark_startup.py                        # human table, current source
scripts/benchmark_startup.py --runs 30 --warmups 5  # more stable numbers
scripts/benchmark_startup.py --json /tmp/base.json  # store a baseline
scripts/benchmark_startup.py --baseline /tmp/base.json
scripts/benchmark_startup.py --ref /path/to/old/bin  # A/B two builds
```

`--ref DIR` expects `DIR/llm` and `DIR/llm-session` and interleaves the two
builds run-by-run so thermal/background drift hits both equally. It uses the
reference build as the regression baseline: a p50 regression larger than
`--max-regression` (default 20%) **and** `--regression-floor` (default 3 ms)
exits non-zero.

Baselines are machine-specific (CPU, page cache, WSL2/container overhead).
Commit numbers only for a named reference machine, or store them outside the
repository and refresh them deliberately.

## Reference run — 2026-10-21

Intel i7-14700, 28 threads, Go 1.24.13, Linux 6.18 (WSL2), x86_64, warm page
cache, 30 runs / 5 warmups, local instant provider.

Comparing the `0.5.0` release (commit `4efe021`) against `0.4.5` (commit
`049cf59`), built from their clean trees:

| metric | 0.4.5 p50 | 0.5.0 p50 | Δ |
|---|---:|---:|---:|
| `llm.version` | 14.4 | 14.5 | +0.3% |
| `llm.inspect` | 14.9 | 15.0 | +0.8% |
| `llm.session-dispatch` | 29.2 | 28.8 | −1.3% |
| `chat.request` | 15.1 | 15.0 | −0.7% |
| `chat.first-token` | 21.2 | 21.2 | −0.1% |
| `chat.exit` | 21.7 | 21.5 | −0.7% |
| `session.request` | **21.3** | **61.2** | **+187%** |
| `session.first-token` | **25.9** | **65.9** | **+154%** |
| `session.exit` | **26.4** | **67.5** | **+156%** |
| `session.tui.first-paint` | 16.8 | 16.4 | −2.3% |
| `session.tui.editor-ready` | 34.6 | 34.4 | −0.7% |
| `session.tui.request` | **43.0** | **84.7** | **+97%** |
| `session.tui.first-token` | **48.1** | **89.9** | **+87%** |

`llm chat` startup is unchanged. `llm-session` startup grew by a constant
~40 ms before the first provider request, in both the line REPL and the TUI.

### Cause

`0.5.0` wired compaction into every turn. `runtime.MaybeCompress` tokenizes the
projected history before deciding whether to compress, and
`sessiontoken.CountText` calls `tiktoken.GetEncoding("cl100k_base")`. In
`pkoukk/tiktoken-go@v0.1.8` the BPE ranks are cached but `GetEncoding` rebuilds
the `CoreBPE` on every call: it recompiles the cl100k regex with `regexp2` and
rebuilds a ~100k-entry decoder map. Measured in isolation:

| call | time |
|---|---:|
| first `CountText("", model)` in a process | ~41 ms |
| second `CountText("hello world", model)` | ~26 ms |

Because the check runs before the user message is appended, a brand-new session
with empty history still pays the full encoding build before the first request.

### Measured effect of skipping the empty history

A three-line guard (`if strings.TrimSpace(text) == "" { return false, nil }`
before `CountText`) restores parity in an A/B run of 20 runs / 4 warmups:

| metric | 0.4.5 p50 | 0.5.0 + guard p50 | Δ vs 0.4.5 |
|---|---:|---:|---:|
| `session.request` | 21.8 | 21.3 | −2.2% |
| `session.first-token` | 26.6 | 26.3 | −1.1% |
| `session.tui.request` | 42.7 | 42.4 | −0.8% |
| `session.tui.first-token` | 47.8 | 47.6 | −0.5% |

The guard is not in the `0.5.0` release; this document records the regression
and the verified fix measurement so the change can land with its own tests. The
same per-call `CoreBPE` rebuild also affects every compaction check on long
histories, so caching the encoder (or tokenizing only up to the threshold) is
the follow-up.

`llm-session --resume` replay is another startup path (resumed history is
re-rendered before the prompt); it is not covered by the scenarios above yet.

## Maintenance

Run the bench before and after any change that touches process startup: config
loading, prompt/role registry, provider factory, session store/projection,
compaction, or TUI initialization. For a release, run `--ref` against the
previous release build and attach the table to the release notes.
