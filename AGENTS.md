# AGENT CONTEXT – cli-llm

This repository currently ships only to internal users (no public release yet).  
All work must nevertheless keep the codebase release-ready so that future publishing (PyPI, crates.io, etc.) requires zero or minimal refactoring.  
Critical thinking is encouraged: plans evolve when new evidence appears.

Comment style: less comment. remains comment only for some public function doc string, complicated logic explanation.

---

## High-Level Goals

- Maintain parallel Python + Go + Rust structure. **Keep the Python implementation as the primary supported runtime** until the Go/Eino parity track is explicitly promoted. Keep interfaces aligned so future Rust work can happen without major redesign.
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
| `0.4.2`  | Prefix slash-command completion with inline descriptions, multi-line Bubble Tea textarea editor, PTY/VT acceptance suite, main-buffer rendering, TTY status cleanup, Ctrl+C turn-cancel fix. |

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

12. **Agents Context Toggle** (0.3.0)  
    `--agents-context` / `-A` flag reads `./AGENTS.md` from cwd and appends it to the system prompt. Size guard (16 KB truncation), sanitization, graceful missing-file handling.

13. **Renderer Upgrade** (0.3.0)  
    Replaced naive ANSI-based rendering with `rich` and `markdown-it-py`. Syntax-highlighted code blocks, proper markdown rendering, preserved streaming and non-streaming APIs.

14. **Plugin Framework** (0.3.0)  
    Cargo-style subcommand discovery: unknown subcommands trigger a PATH search for `llm-<subcommand>` executables. If found, the process is replaced via `os.execvp`. Framework only — no bundled plugins.


15. **Go/Eino parity track** (0.3.0)  
   `src-go/` now contains an Eino-based implementation of the current command surface. The core runtime stays compose-first for deterministic `chat` and constrained `toolcall`; ADK is isolated to a learning example until a real agentic product mode is scheduled.


### 🔄 Partially Done / In Progress

- **CLI Options Overhaul** (2.D) — Flags exist (`--provider`, `--role`, `--model`, `--input-mode`, etc.) and have help text, but some legacy flags remain (`--localtest`), and flag naming hasn't been fully audited per the 2.D spec.
- **Go/Eino runtime promotion** — `src-go/` has parity coverage for config, prompts, provider factory, chat, toolcall, inspect/provider metadata, plugin dispatch, and an ADK spike. It still needs packaging/install integration and live-provider release rehearsal before replacing Python.

### ❌ Not Yet Started

- **Rust Parity** (timing TBD) — Align Python and Go abstractions so a Rust reimplementation can reuse the same mental model.

### 🎯 Roadmap Decisions

The following were discussed and decided:

| Item | Decision | Notes |
|------|----------|-------|
| Agents Context Toggle | **Do it** | Add `--agents-context` flag to inject `./AGENTS.md` as supplemental system context, with size/sanitize guards. |
| Plugins System | **Plan B — framework only** | Plugins register as direct subcommands (like `cargo-*`). Implement only the framework (discovery + dispatch); don't build actual plugins yet. |
| Renderer Upgrade | **High priority** | Integrate `rich`/`markdown-it` for syntax-highlighted output. Current plain rendering is too basic. |
| Output Automation Pipeline | **Not needed** | Existing `--json-output` is sufficient. No structured streaming format required. |
| Rust Parity | **Hold** | Keep `src-rs/` tree dormant. No active Rust work; revisit when there's a clear need. |
| Eino framework version | **High priority** | Use `src-go/` for a compose-first parity implementation. Keep ADK out of core UX until an agentic requirement is concrete. |
| Config Schema Extension | **Not needed** | Current config structure is fine. Keep code extensible but don't expand schema proactively. |
bc6|| Multi-Turn Session Mode | **0.4.0 goal (Go only)** | Build `llm-session` plugin with git-like checkpoint model, branching, context compression, and `@`/`#` references. Leverage Eino's graph/state for checkpoint state machine. |

### Next milestones (tentative)

**0.3.0** (current track)
- [x] Agents Context Toggle (`--agents-context`)
- [x] Renderer upgrade (rich/markdown-it integration)
- [x] Plugin framework (Plan B: subcommand registration)
- [x] refactor `src-go` with Eino compose workflows and an isolated ADK learning spike
- [ ] decide whether/when the Go runtime becomes an install target

**0.4.0** (active track — multi-turn session mode, Go only)
- [x] `llm-session` plugin binary skeleton (reuse existing plugin dispatch)
- [x] Session persistence format (JSONL tree, id/parentId branching)
- [x] Git-like checkpoint model: auto-checkpoint after each turn, manual `/checkpoint <name>`
- [x] Branch topology data model with persisted branch heads
- [x] `/switch <target>` behavior: switch to existing node/branch, or create a new branch
- [x] `/branches` — list all branches from precomputed topology
- [x] Context compression primitives: auto-trigger when token budget exceeded
- [x] Simplified session system prompt: no tool calls; branch/checkpoint/head/session state stays local and is not sent to the model
- [x] Auto session title generation from the first user message; fresh sessions no longer prompt for a session name
- [x] Go-only: Python `src/` unchanged
- [x] **High priority:** add `/help`; it must list every enabled slash command with usage and a concise description
- [x] **High priority:** add prefix-matching slash-command completion with inline descriptions; fuzzy and argument completion are out of scope for v1
- [x] **High priority / bug:** replace the current single-line input with a real multi-line editor that preserves spaces and UTF-8/CJK input; Enter submits, while Shift+Enter or Ctrl+J inserts a newline
- [x] **High priority / bug:** render complete multi-line assistant responses with preserved newlines, terminal-width wrapping, and native scrollback; long responses must not be clipped to one line or one screen
- [x] **High priority / bug:** make the thinking/stream-start spinner refresh in place on one logical terminal row and clear it before response content; spinner frames must never accumulate across the line
- [x] **High priority / verification:** exercise the built `llm-session` binary through a PTY, capture its real ANSI byte stream and reconstructed terminal screen, and assert user-visible behavior instead of inferring it only from `View()` or source code
- [x] **Ctrl+T transcript overlay foundation** (Bubble Tea + bubbles + lipgloss): scrollable branch-aware viewport, role/markdown highlighting, branch tree/list panel, and copy-selected-dialog-to-clipboard (OSC 52). This does not mean the main input/response TUI is release-ready. Design: `docs/plans/2026-06-22-llm-session-tui-design.md`. Decided 2026-06-22 — see "TUI Implementation" below.
- [ ] `/export` — dump current branch full history to text file
- [ ] `/new` — start a fresh session
- [ ] `@filename` file reference (with fuzzy completion) — inject content into context
- [ ] `#head` / `#checkpoint` reference — jump to, diff against, or branch from
- [ ] Bracketed paste mode: `[paste #1 +N lines]` markers → expand on submit
- [ ] Basic Eino Graph state machine for checkpoint transitions (not just linear chain)

---

## Guiding Principles

1. Keep changes scoped to the active milestone unless explicitly coordinated.
2. Ensure documentation (README / AGENTS / CHANGELOG) stays aligned.
3. Treat every internal build as if it might be published tomorrow.

---

## 0.4.0 Design Reference: `llm-session` Multi-Turn Agent Window

### Overview

`llm-session` is a new Go-only interactive session mode that treats conversation history as a **version-controlled graph** (git-like), not a linear chat. It leverages Eino's compose/Graph capabilities for checkpoint state machines, context compression, and branch management.

**Go only** — Python `src/` is unchanged for 0.4.0. The feature ships as an `llm-session` binary plugin via the existing cargo-style plugin dispatch.

**Session files:** stored in `~/.cli-llm/sessions/<name>.jsonl`. Each session is a single JSONL file, append-only (crash-safe).

**Launch modes:**
- `llm-session` (no flags) → start a fresh session immediately; the first user message triggers a lightweight title request, then the local store is silently renamed from a temporary internal filename.
- `llm-session --resume` → list existing sessions, user picks one. Resumes at the leaf entry.
- `llm-session --resume <name>` → resume specific session non-interactively.

**Exit:** `/exit` or Ctrl+D. Auto-saves on exit.

**Compression:** auto-trigger at 200k tokens (`keepRecentTokens: 20000`). Uses the default model for both chat and compression (no separate model config in v1).

**Model:** no override — uses whatever `~/.cli-llm/config.toml` defaults to.


### TUI Implementation (decided 2026-06-22)

Full design: `docs/plans/2026-06-22-llm-session-tui-design.md`. Summary of the locked decisions:

**Terminal model — main buffer for chat, alt-screen only for the overlay.** Normal conversation stays in the terminal main buffer and appends like a shell command (native scrollback preserved). Only the Ctrl+T transcript overlay enters the alternate screen. This matches the existing terminal contract *and* matches how Claude Code (Ink, never enters alt-screen), Codex CLI (ratatui+crossterm, alt-screen only for pager overlays), and Gemini CLI (Ink, alt-buffer opt-in) all behave — verified by source inspection.

**`/switch` rendering — separator in main buffer, isolation in the overlay.** On `/switch`, the main buffer prints a `── switched to <branch> ──` separator and keeps appending; old branch lines remain in scrollback (like `git checkout` leaving prior terminal output intact). The requirement "after switch, unrelated conversation must not be shown" is enforced by the **Ctrl+T overlay**, which rebuilds from `graph.State.ReachableHistory()` and therefore shows only the current branch's reachable chain. No mainstream tool clears/redraws the main buffer on branch switch — it destroys native scrollback — so we don't either.

**Framework — Bubble Tea + bubbles + lipgloss.** Same ecosystem as the existing `glamour`/`lipgloss` transitive deps (already in the module cache; promote indirect → direct). Model-Update-View is pure-function, so `Update(msg)` + `View() string` snapshots fit the project's existing `[]KeyEvent` + `bytes.Buffer` headless test style. (Codex's `ratatui` is the Rust equivalent; this is the Go-idiomatic choice.)

**Overlay capabilities (v1):** scrollable viewport (reuses existing offset/maxOffset logic), user/assistant role coloring + glamour markdown, a branch tree/list panel (`ListBranches`, current branch marked, select-to-preview), and **copy selected dialog to the system clipboard via OSC 52** (`ansi.SetClipboard`, already a transitive dep — works over SSH, no `xclip`/`pbcopy`, and is a byte sequence so it stays `bytes.Buffer`-testable). Full-text search is deferred.

**Main input editor (release blocker):** the editor must accept ordinary spaces and arbitrary UTF-8 text, including Chinese, without dropping or joining characters. It must be width-aware and support visual wrapping. Enter submits the whole buffer; Shift+Enter and Ctrl+J insert a newline. Submitted text must retain its spaces and line breaks exactly when it is sent to the model and persisted.

**Main response rendering (release blocker):** streaming output must preserve model-provided newlines and wrap long lines to the current terminal width. A response may span any number of terminal rows and must remain available through native scrollback; it must never be truncated merely because it exceeds one row or the visible terminal height.

**Status indicator:** the `thinking` / `waiting for stream` / `streaming` lifecycle now uses one synchronously cleared TTY row. Frames render with carriage-return plus full-row erase, the ticker is stopped before response/error/cancellation output, and non-TTY output remains deterministic. Status text is never persisted as a session entry.

**Integration — new `internal/session/tui` package implementing the existing `repl.TranscriptOverlay` interface.** `repl.Loop` is unchanged; `runner.go` swaps `transcriptOverlay{}` for `tui.NewOverlay(...)`. The old pure-ANSI `internal/session/terminal` package is kept (its alt-screen ANSI assertions still apply) and can be retired later.

**Verification strategy:** pure Model tests remain useful for state transitions, bounds, branch isolation, and OSC 52 bytes, but they are not sufficient evidence of terminal behavior. Automated acceptance tests must build and launch the real `llm-session` executable under a PTY at fixed terminal sizes, drive it with real key byte sequences, use a deterministic mock streaming provider, and capture both the raw ANSI byte stream and screen frames reconstructed by a VT-compatible terminal emulator. On failure, retain the input trace, raw ANSI capture, and normalized screen snapshot as artifacts.

The PTY suite must cover `/help`, prefix completion, spaces, UTF-8/Chinese text, multi-line input through Shift+Enter or Ctrl+J, long multi-line streamed responses, in-place spinner refresh, cancellation, terminal resize, and clean restoration after the Ctrl+T overlay. Assertions must inspect captured user-visible output: no spinner trail, no lost whitespace or Unicode, no missing response lines, and no stale overlay/status rows. Before marking the TUI release-ready, also perform and record one live-provider smoke run in a real terminal. Source inspection or direct `View()` snapshots alone cannot close these bugs.


### v1 MVP Scope — Must Work End-to-End

Only these items need to work for v1 to ship. Everything else is deferred to v1.1 / v2.

| # | Item | Why it's critical |
|---|------|-------------------|
| 1 | **Session JSONL persistence** | The entire feature is a session file. Without it, nothing works. Append-only, `id`/`parentId` tree, entry type system. |
| 2 | **`llm-session` binary** | Plugin entry point. Handles `--resume` / `--resume <name>` / fresh start. Reads config, loads session, enters REPL. |
| 3 | **Chat REPL** | User types a message → sent to LLM → streaming response rendered → auto-checkpoint created → loop. No tool calls, no file refs, no paste markers. Just type and reply. |
| 4 | **Auto-checkpoint after each turn** | Every assistant response creates a `custom:checkpoint` entry with the response's hash as `returnTo`. This is the git-commit equivalent — the backbone of branching. |
| 5 | **`/switch <target>`** | Navigate to a hash (detached) or branch name. Unknown name → create new branch from current position. This is the core navigation primitive. |
| 6 | **`/branches`** | List branches from pre-computed topology. User-facing output should stay compact and omit raw `head_id` by default. |
| 7 | **`/checkpoint <name>`** | Labels the current node. Creates a named branch if one doesn't exist. |
| 8 | **`/exit` (save)** | Write session to disk and exit. |
| 9 | **Auto session titling** | Sub-request to LLM on first message, stores title in `session_info`. Makes `/resume` usable. |
| 10 | **Context compression** | When token count exceeds 200k, summarize old entries into a `compaction` entry. Keeps the session usable for long conversations. |
| 11 | **`/help`** | Lists every enabled slash command with its usage and concise description so the REPL is discoverable without external docs. |
| 12 | **Slash command completion** | Prefix matching for command names, with inline descriptions. Fuzzy matching and argument completion are deferred. |
| 13 | **Multi-line UTF-8 input editor** | Preserves spaces and Chinese/other Unicode; Enter submits and Shift+Enter or Ctrl+J inserts a newline. |
| 14 | **Complete response rendering** | Preserves model newlines, wraps to terminal width, and emits the entire streamed response into native scrollback. |
| 15 | **Thinking status** | Shows one in-place status row while waiting and removes it cleanly before response, error, or cancellation output. |
| 16 | **Real terminal-output verification** | PTY tests capture the built binary's raw ANSI stream and reconstructed screen; a recorded live-provider terminal smoke test is required before release-ready status. |

**Explicitly deferred:**
- `/export` (can copy the JSONL file manually for now)
- `/new` (exit + start fresh works)
- `/rename` (minor UX polish)
- `@filename` references (v2 with tool calls)
- `#head` / `#checkpoint` references (v2)
- Bracketed paste mode (v2)
- Branch merge (v2)
- Raw head IDs in `/branches` default output; hashes remain internal unless a command explicitly needs one

---

### Data Model

Session state is a JSONL tree (same structure as pi's session format):

```
┌────────┬───────────┬─────────────────────────┐
│ Entry  │ id (hash) │ preview / name           │
├────────┼───────────┼─────────────────────────┤
│ Node   │ a1b2c3d4  │ "Write a React compo…"   │  ← first ~16 chars of user prompt
│ Node   │ e5f6a7b8  │ "Can we use zod for…"     │
│ Branch │ main      │ (current head: e5f6a7b8) │  ← only branches have names
│ Node   │ c9d0e1f2  │ "Debug the schema…"       │
│ Branch │ explore   │ (current head: c9d0e1f2) │
└────────┴───────────┴─────────────────────────┘
```

**Key rules:**
- **Every entry has an `id`** — a content-derived hash (SHA-256 truncated to 12 hex chars, like git short hashes). This hash is the `entryID` and never changes once created.
- **Nodes have no names.** Their identity is their hash. In the tree view, each node is shown by its **preview**: the first ~16 characters of the user prompt that spawned it.
- **Branches are the only named entities.** A branch is a mutable pointer to a head entry ID. `/switch <branchname>` moves the head. `/checkpoint <name>` labels the current node (retroactively), but the name is stored as branch metadata, not as part of the node entry.
- **Tree navigation is parent-chain walking** (same pattern as pi-navigator): from any leaf, walk `parentId` links backward to root. No graph algorithms needed — the entry type system handles everything.

**Entry types (minimum viable set):**

| Entry type | id shape | data | purpose |
|------------|----------|------|---------|
| `message` (user) | hash | `{ role, content }` | User prompt. Drives node preview. |
| `message` (assistant) | hash | `{ role, content }` | LLM response. |
| `compaction` | hash | `{ summary, firstKeptEntryId, tokensBefore }` | Compressed old context. Same semantics as pi's `CompactionEntry`. |
| `branch_summary` | hash | `{ summary, fromId }` | Injected when `/switch` leaves a branch. Same as pi's `BranchSummaryEntry`. |
| `custom:checkpoint` | hash | `{ name?, returnTo }` | Anchor point for `/switch` and `/return`. Optional `name` labels the node for human reference. |
| `custom:session_info` | hash | `{ title, created, model }` | Session-level metadata. Written once at session creation. |

**Why this works (pi-navigator proof):** pi-navigator implements a full checkpoint/return/branch system with exactly 3 custom entry types (`checkpoint`, `task`, `task-done`) and parent-chain walking — no parallel processes, no graph algorithms, no subsystems. `llm-session` follows the same pattern: minimal entry types + tree walking + user-controlled navigation.


**Auto session titling:** On the first user message (before the first assistant response), `llm-session` sends a lightweight sub-request to the LLM asking it to generate a short descriptive session title (≤60 chars, single line). The title is stored in the `SessionInfoEntry` of the session JSONL. The user can override with `/name <title>`.
  - Rationale: unnamed sessions are hard to distinguish in `/resume`. Auto-titling from the first message gives immediate recognizability with zero user friction.
  - Implementation: a separate Eino ChatModel invocation with a minimal prompt, using the default model. Result is saved before the main conversation flow begins.


---

### Session System Prompt (Simplified, v1 — No Tool Calls)

Derived from pi coding-agent's system prompt template. Session mode v1 does not expose tool calls; all state-machine operations (branch, checkpoint, head hash, session filename/name) are handled locally by the Go binary and are not included in the chat model context.

```text
You are an expert conversation assistant operating inside llm-session, a
multi-turn session harness.

Guidelines:
- Be concise in your responses.

Current date: {current_date}
```

**Future (v2+):** When tool calls are added, the template grows to include:

```text
Available tools:
{tool list}

Tool call rules:
- Call at most one tool per turn.
- Use only the provided tools.
- Prefer read/grep/find/ls over bash for file inspection.
- If no tool is needed, answer normally without a tool call.
```

---

### Paste Handling (Bracketed Paste Mode)

Reference: pi TUI editor's paste implementation.

**Terminal layer:**
- Terminal wraps pasted content in `\x1b[200~` (start) and `\x1b[201~` (end) — this is the bracketed paste mode sequence.
- `llm-session` captures these sequences to distinguish paste from manual typing.

**Input layer:**
- On detecting `\x1b[200~`, enter paste mode: buffer all subsequent input in a `pasteBuffer` string.
- On `\x1b[201~`, exit paste mode.

**Processing:**
- If paste content is small (≤10 lines and ≤1000 chars): inline it directly into the editor buffer.
- If paste content is large: store the raw content in an in-memory `pastes: Map<number, string>`, and display a placeholder marker in the editor: `[paste #1 +123 lines]`
- The marker is rendered as an atomic unit (cannot edit inside it; Delete/Backspace removes the whole marker).

**On submit:**
- Before sending to LLM, `expandPasteMarkers(text)` walks the input text, finds all `[paste #N ...]` markers, and replaces them with the stored raw content.
- After submission, `pastes.clear()` to release memory.

**Session persistence:**
- The JSONL file stores markers as-is (`[paste #1 +123 lines]`) rather than raw content.
- On session restore, pastes map is empty — markers remain but are inert. The user sees markers representing the original paste but cannot expand them (they would need to re-paste).
- This is intentional: session files stay compact and don't bloat with large pasted content.

---

### Slash Commands (宁缺毋滥)

| Command | Description |
|---------|-------------|
| `/help` | List every enabled slash command, including aliases, usage, and a concise inline description. Help content and completion candidates must come from the same command registry so they cannot drift. |
| `/exit` | Save the current session and exit. |
| `/transcript` (`/t`) | Open the transcript view; Ctrl+T is the keyboard shortcut. |
| `/export [file]` **(v1.1)** | Export current branch to a human-readable text file. Default filename: `session-{name}-{branch}.txt` |
| `/new` **(v1.1)** | Start a fresh session. |
| `/branches` | List branches and parent/child relationships. Do not show raw `head_id` in the default output; reads pre-computed topology, not the full entry tree. |
| `/switch <target>` | Navigate to a target:
  - **Hash** (`a1b2c3d4`) → detach to that node (like `git checkout <hash>` — working tree is that commit, no branch).
  - **Branch name** (`main`, `explore`) → switch to that branch's head.
  - **Unknown name** → create a new branch forked from current position.
 |
| `/rename <branch> <new>` **(v1.1)** | Rename a branch. |
| `/checkpoint <name>` | Label the current node with a human-readable name. Creates a named branch from the current position. |

**Completion contract (v1):** completion is active only for the slash-command name at the beginning of the input. The candidate set is the enabled v1 commands above, excluding commands marked v1.1. Matching is prefix-based and preserves the user's current argument text. The menu shows command usage plus its description; a unique match can be completed directly. Fuzzy matching, command-history ranking, and argument completion are deferred.

---

### Eino's Role in Session Mode

Unlike the current `chatflow`/`toolcallflow` which use a simple 2-node linear chain (Lambda → ChatModel), session mode needs a **stateful Graph** that models checkpoint transitions:

```
  ┌──────────────────────────────────────┐
  │         Session State Machine        │
  │                                      │
  │  ┌──────────┐    ┌──────────────┐   │
  │  │ ChatModel │◄───│ Checkpoint   │   │
  │  │ (reply)   │───►│ Manager      │   │
  │  └──────────┘    └──────┬───────┘   │
  │                         │           │
  │  ┌──────────────────────▼───────┐   │
  │  │ Context Compression (Eino)   │   │
  │  │ - Summarize old segments     │   │
  │  │ - Store CompactionEntry      │   │
  │  └──────────────────────────────┘   │
  │                         │           │
  │  ┌──────────────────────▼───────┐   │
  │  │ Branch / Switch / Fork       │   │
  │  │ (Eino state transition)     │   │
  │  └──────────────────────────────┘   │
  └──────────────────────────────────────┘
```

Eino capabilities used (beyond current linear compose):
- **`compose.Graph` with state** — manage the session lifecycle as a state machine with explicit transitions between states (chatting, compressed, branched, switched).
- **Multiple ChatModel nodes** — one for the main conversation, a second for compression summarization (different model/prompt).
- **Conditional branching in the graph** — e.g., "if token budget exceeded → route through compressor node before returning response".
- **Streaming through the graph** — user input → optional compression → checkpoint creation → chat model → response render. All as a single compiled graph.
- The graph edges encode the session's checkpoint transition rules, making the flow explicit and testable.

This is the first real use of Eino's graph capabilities beyond the minimal linear chain — precisely what makes Go the right home for this feature.
