# AGENT CONTEXT – cli-llm

This repository currently ships only to internal users (no public release yet).  
All work must nevertheless keep the codebase release-ready so that future publishing (PyPI, crates.io, etc.) requires zero or minimal refactoring.  
Critical thinking is encouraged: plans evolve when new evidence appears.

Comment style: less comment. remains comment only for some public function doc string, complicated logic explanation.

---

## High-Level Goals

- Maintain parallel Python + Rust structure, yet **keep the Python implementation as the primary focus until we explicitly schedule Rust work**. Keep interfaces aligned so a future Rust push can happen without major redesign.
- Prioritise developer experience: editable installs, modern packaging, fast iteration, reproducible environments.
- Keep code Pythonic with strong typing, modular architecture, and solid test coverage so experimental features graduate safely.

---

## Actual Release History

| Tag      | What shipped                                                                 |
|----------|------------------------------------------------------------------------------|
| `0.2.0`  | Code formatting cleanup, role system improvements, JSON output, pre-uv era.  |
| `0.2.2`  | Reconstructed codebase, uv migration, initial modularisation.                |
| `0.2.3`  | Layered configuration loader (CLI > env > `~/.cli-llm/config.toml` > defaults), `--version` flag, sample config, CHANGELOG introduced. |
| `0.2.4`  | `inspect` subcommand, toolcall service (presets + streaming + safe stdout), installer script (`install.sh`), raw-mode input handling (three modes), expanded test coverage (9 test files), provider dispatch thin layer. |

> No public PyPI release — all tags are internal milestones.

---

## Feature Completion Status

### ✅ Done (0.2.x · Internal Foundations)

1. **Freeze Roadmap Messaging** (0.2.1–0.2.3)  
   README + CHANGELOG document the internal-only posture and roadmap.

2. **Modern Packaging & Editable Install** (0.2.2–0.2.3)  
   `pyproject.toml` + `src/` layout, uv migration, `pip install -e .` / `uv pip install -e .` workflow documented.

3. **Python CLI Modularisation** (0.2.2–0.2.3)  
   Monolithic `cli` broken into dedicated modules: `cli`, `config`, `providers`, `renderers`, `services`, `toolcalls`, `utils`.

4. **Configuration Loader** (0.2.3)  
   Layered resolution (CLI flags > env vars > `~/.cli-llm/config.toml` > built-in defaults). Multi-provider profiles, sample config, legacy path fallback (`~/.cli_llm/`).

5. **Provider Abstraction / OpenAI-compatible SDK** (0.2.3–0.2.4)  
   `ProviderRouter` with dispatch, `OpenAIProvider` with streaming, multiple provider profiles in config. `inspect` and `provider models` CLI commands for merged metadata. Config-driven provider addition (no code changes for new OpenAI-compatible endpoints).

6. **Role Manager & Prompt Loading** (0.2.0+)  
   Embedded `coder` / `normal` profiles; optional `system_prompts.json` override; `--role` CLI flag.

7. **Renderer & Output Modes** (0.2.0+)  
   Response streaming, JSON output, code-only output via `--output-codes`. Plain terminal rendering.

8. **Testing & Tooling Baseline** (0.2.4)  
   9 test files covering config loader, provider routing, renderers, chat service, tool calls, CLI commands, version helper. `ruff`, `black`, `mypy` configured in `pyproject.toml`.

9. **Tool Call / Function Calling** (0.2.4)  
   Toolcall service with preset system, streaming delta parsing, safe stdout execution.

10. **Raw-Mode Input Handling** (0.2.4)  
    Three input modes via `--input-mode`:
    - `prompt` (default): prompt_toolkit with history, raw mode, multi-line via Alt+Enter
    - `editor`: opens `$EDITOR` for composing long messages
    - `stdin`: multi-line until EOF (Ctrl+D)

11. **One-Click Installer** (0.2.4)  
    `install.sh` — standalone script that installs Python package (editable), creates `~/.local/bin/llm` entry point, sets up `~/.config/cli-llm/config.toml`, optionally builds Rust binary.

### 🔄 Partially Done / In Progress

- **CLI Options Overhaul** (2.D) — Flags exist (`--provider`, `--role`, `--model`, `--input-mode`, etc.) and have help text, but some legacy flags remain (`--localtest`), and flag naming hasn't been fully audited per the 2.D spec.
- **Plugins / Subcommand isolation** — Toolcall service provides a `toolcall` subcommand with presets (PS1-like), but no true plugin loading mechanism yet.

### ❌ Not Yet Started

- **Agents Context Toggle** (2.CD) — `--agents-context` flag to read `./AGENTS.md` as supplemental system context.
- **Output-Control enhancements** (0.4.x) — Structured response objects, streaming-friendly formatting for automation pipelines.
- **Extended Provider & Model Metadata** (0.4.x) — Richer config schema (capabilities, defaults per-provider), deeper model list introspection.
- **Documentation & Release Readiness** (0.4.x) — `docs/` directory, release rehearsal, formal publishing prep.
- **Rust Parity** (timing TBD) — Align Python abstractions so a Rust reimplementation can reuse the same mental model.

### 🎯 Roadmap Decisions (2025-06-18)

The following were discussed and decided:

| Item | Decision | Notes |
|------|----------|-------|
| Agents Context Toggle | **Do it** | Add `--agents-context` flag to inject `./AGENTS.md` as supplemental system context, with size/sanitize guards. |
| Plugins System | **Plan B — framework only** | Plugins register as direct subcommands (like `cargo-*`). Implement only the framework (discovery + dispatch); don't build actual plugins yet. |
| Renderer Upgrade | **High priority** | Integrate `rich`/`markdown-it` for syntax-highlighted output. Current plain rendering is too basic. |
| Output Automation Pipeline | **Not needed** | Existing `--json-output` is sufficient. No structured streaming format required. |
| Rust Parity | **Hold** | Keep `src-rs/` tree dormant. No active Rust work; revisit when there's a clear need. |
| Config Schema Extension | **Not needed** | Current config structure is fine. Keep code extensible but don't expand schema proactively. |

### Next milestones (tentative)

**0.3.0** (current track)
- [ ] Agents Context Toggle (`--agents-context`)
- [ ] Renderer upgrade (rich/markdown-it integration)
- [ ] Plugin framework (Plan B: subcommand registration)

---

## Repository Layout

```
install.sh              — One-click installer
pyproject.toml          — Python build config + dependencies
src/cli_llm/            — Python CLI package
  cli.py                — Click-based CLI entry point
  config.py             — Layered config loader
  prompts.py            — Role/prompt management
  providers/            — Provider abstraction + router
  renderers/            — Output rendering
  services/             — Chat service + input handler
  toolcalls/            — Tool call / function calling
  utils/                — ANSI colors, misc helpers
src-rs/                 — Rust prototype (dormant)
tests/                  — pytest test suite
```

## Guiding Principles

1. Keep changes scoped to the active milestone unless explicitly coordinated.
2. Ensure documentation (README / AGENTS / CHANGELOG) stays aligned.
3. Treat every internal build as if it might be published tomorrow.
