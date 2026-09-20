# AGENTS.md — cli-llm

These instructions apply to the entire repository.

cli-llm is internal software with no public release yet. Keep every change
release-ready: stable interfaces, reproducible installs, no committed secrets,
and enough tests that publishing later does not require architectural cleanup.

For current behavior, trust executable code and tests over old planning prose.
`CHANGELOG.md` records milestone history; `docs/plans/` contains design history
and may describe work that was later changed, completed, or deferred.

## Runtime status

The repository intentionally contains three implementations with different
maturity and version numbers:

| Runtime | Version source | Status |
|---|---|---|
| Python | `pyproject.toml` (`0.3.0`) | Primary packaged/supported runtime for the shared CLI surface. |
| Go/Eino | `src-go/internal/cli/root.go` (`0.4.4`) | Active parity runtime, default target of the root installer, and home of `llm-session`. |
| Go session plugin | `src-go/internal/session/cli/version.go` (`0.4.4`) | Bundled Go-only plugin installed as `llm-session`. |
| Rust | `src-rs/Cargo.toml` (`0.1.0`) | Dormant prototype; buildable, but not on an active parity track. |

Do not assume a single repository-wide version. Update the source of truth for
the runtime being released, and keep Go main/plugin versions aligned when they
ship together.

Python remains the primary supported package until that policy is explicitly
changed. This does not conflict with the root multi-runtime installer defaulting
to Go: the root installer is a repository/developer installer, while Python is
still the mature packaging track.

## Non-negotiable engineering rules

- Keep changes scoped. Do not expand configuration schemas, add agentic modes,
  or activate Rust parity without an explicit requirement.
- Preserve Python/Go behavior for shared CLI features where practical. Session
  functionality is deliberately Go-only and should not be copied into Python by
  default.
- Keep provider integration behind the existing abstractions. New
  OpenAI-compatible endpoints should normally be configuration, not new provider
  branches.
- Protect automation output: diagnostics go to stderr; JSON and other
  machine-readable stdout must remain parseable.
- Preserve user configuration and session files. Installers must not overwrite
  an existing config; session JSONL is append-only.
- Add comments sparingly. Use public API docstrings/comments and explanations
  for genuinely non-obvious logic, not line-by-line narration.
- Never commit real API keys, tokens, private endpoints, generated binaries,
  session transcripts, PTY artifacts, caches, or local virtual environments.
- Update `README.md`, `CHANGELOG.md`, and this file when a user-visible behavior,
  runtime status, installation contract, or roadmap decision changes.

## Repository map

- `src/cli_llm/` — Python package and primary packaged CLI.
- `src-go/cmd/llm/` — Go CLI entrypoint.
- `src-go/internal/{cli,config,prompts,providers,render,runtime,tools,workflows}/`
  — Go parity implementation. Chat and toolcall use Eino compose chains.
- `src-go/cmd/llm-session/` and `src-go/internal/session/` — persistent Go-only
  session plugin.
- `src-go/cmd/tui-smoke/` — transcript-overlay smoke utility, not a production
  CLI entrypoint.
- `src-go/examples/chatmodel_agent/` — isolated Eino ADK learning example; it is
  not part of the product runtime.
- `src-rs/` — held Rust prototype.
- `tests/` — Python tests plus installer shell tests.
- `src-go/internal/session/acceptance/` — built-binary PTY/VT acceptance suite.
- `docs/plans/` — historical implementation plans, not current requirements.
- `install.sh` — multi-runtime repository installer.
- `scripts/install.sh` — Python-only deployment installer.

## Shared CLI contract

The shared built-in surface is:

- `chat` — default command when no command is supplied.
- `inspect` — inspect merged provider profiles.
- `provider models [name]` — list configured models.
- `toolcall` — one constrained tool-call request using enabled presets.
- `session` — dispatch to the external `llm-session` executable.

Unknown first arguments are resolved as cargo-style plugins by searching `PATH`
for `llm-<name>` before falling back to treating the arguments as a chat prompt.
Built-ins always win. The repository includes the `llm-session` plugin; the
framework also supports third-party executables in any language.

The Go root command additionally owns aliases:

| Alias | Command |
|---|---|
| `s` | `session` |
| `c` | `chat` |
| `i` | `inspect` |
| `p` | `provider` |
| `t` | `toolcall` |

Python does not currently expose this alias registry. Do not claim cross-runtime
alias parity without implementing and testing it.

## Configuration and providers

Configuration precedence is:

1. CLI overrides
2. environment variables
3. `~/.cli-llm/config.toml`
4. runtime defaults

Supported environment overrides are `OPENAI_API_KEY`, `OPENAI_BASE_URL`,
`OPENAI_MODEL`, `CLI_LLM_DEFAULT_ROLE`, and `CLI_LLM_PROVIDER`.

Both active runtimes use an OpenAI-compatible transport. Multiple named profiles
can be declared under `[providers.<name>]`; only the OpenAI-compatible provider
implementation is wired. Python and Go built-in defaults currently differ, so
tests and documentation must not silently assume identical fallback provider or
model values.

`--agents-context` / `-A` appends sanitized `./AGENTS.md` content to the system
prompt with a 16 KiB guard. Role prompts are packaged from
`system_prompts.json`; keep the Python and Go role names/aliases compatible when
changing them.

## Installers

There are two distinct installers. Do not merge their responsibilities casually.

### Root multi-runtime installer

`./install.sh` selects exactly one runtime target and installs `llm` into
`~/.local/bin`:

```bash
./install.sh                 # Go is the default
CLI_LLM_GO=1 ./install.sh
CLI_LLM_PY=1 ./install.sh
CLI_LLM_RUST=1 ./install.sh
```

`CLI_LLM_GO`, `CLI_LLM_PY`, and `CLI_LLM_RUST` are mutually exclusive truthy
selectors. For backward compatibility, `CLI_LLM_GO=n` with no other selector
chooses Python.

After target selection, the installer checks only required local prerequisites:
source manifests, toolchain executables, declared minimum versions, and the
Python package manager. Do not add eager `go mod download`, `cargo fetch`, or
Python dependency-resolution probes; package downloads belong to the selected
build tool and the user's environment. Dependency checks must finish before the
installer creates or replaces the target executable.

The root installer discovers plugins separately and may offer to build
`llm-session`. It creates configuration under
`${XDG_CONFIG_HOME:-~/.config}/cli-llm` and links `~/.cli-llm` there when that
legacy/runtime path does not already exist. Runtime config loading itself uses
`~/.cli-llm/config.toml`.

### Python deployment installer

`./scripts/install.sh` installs only the Python package and supports `user`,
`venv`, and `system` modes plus uninstall. Its smoke test is
`tests/test_installer_smoke.sh`.

## `llm-session` current contract

`llm-session` is a Go-only multi-turn harness built around an append-only,
parent-linked JSONL history. Files live at
`~/.cli-llm/sessions/<name>.jsonl`.

Launch modes:

```bash
llm-session                   # fresh temporary name; first turn auto-titles and renames
llm-session --resume          # list/pick an existing session
llm-session --resume NAME     # resume by filename stem
llm-session --no-tui          # line-oriented input path
llm session ...               # same plugin through root dispatch
```

Entry IDs are immutable SHA-256-derived 12-character hashes. Implemented entry
types are `message`, `custom:checkpoint`, `custom:session_info`, `compaction`,
and `branch_summary`. Branches are reconstructed from named checkpoint entries;
nodes themselves do not have mutable names.

Every successful chat turn persists the user message, streamed assistant
message, an automatic checkpoint, and the current branch pointer. The first
turn uses the configured model for a short title request and renames the
temporary session file.

Enabled slash commands come from one registry shared by help and completion:

| Command | Behavior |
|---|---|
| `/help` | List enabled commands and descriptions. |
| `/exit` | Exit; entries have already been appended as they were created. |
| `/transcript`, `/t` | Open the transcript overlay. |
| `/branches` | List branch topology without raw head IDs. |
| `/switch <target>` | Switch to a branch/hash, or create an unknown branch name. |
| `/checkpoint <name>` | Persist a named pointer at the current position. |

Normal chat stays in the terminal main buffer to preserve native scrollback.
It renders bold blue `You ›` and bold green `Assistant ›` labels with a
blank line between message blocks. Slash-command results and completion lists
are dimmed; errors remain prominent. Any non-empty `NO_COLOR` value disables
color and dim styling without removing the labels or spacing.
Enter on an incomplete slash command applies and executes the selected
completion, defaulting to the first candidate; Tab expands it for editing.
The per-prompt Bubble Tea editor supports spaces, arbitrary UTF-8/CJK, visual
wrapping, Vim INSERT/NORMAL/VISUAL modes, and Ctrl+J for a newline. Enter submits
the complete buffer. Do not document Shift+Enter unless it is actually added and
covered by PTY tests.

During an active request, Esc and Ctrl+C cancel the current turn while sending,
waiting for the first response, or streaming. Cancellation restores the terminal
input mode before returning to the prompt; at an idle prompt, Esc retains its Vim
mode-switching behavior.

Only the Ctrl+T transcript overlay enters the alternate screen. It renders the
current branch's reachable history, offers a branch preview panel, and copies
the selected dialog with OSC 52. The production runner uses
`tui.EditorReader` + `repl.Loop` + `tui.Overlay`; the full-screen
`tui.SessionModel` is not the active production session path.

## Known gaps and deferred work

Be precise about these; types or unit tests alone do not make a feature
end-to-end complete.

- Compression primitives and the 200k/20k token constants exist and have unit
  tests, but `MaybeCompress` is not wired into `RunChatTurn`.
- `branch_summary` is modeled and accepted by context construction, but no
  production switch path creates branch summaries.
- Session navigation uses the local `graph.State` parent-chain model. It is not
  an Eino `compose.Graph` state machine.
- Bracketed-paste placeholders, `/export`, `/new`, `/model`, session deletion,
  `@filename`, `#head`/`#checkpoint` references, branch rename/merge, and search
  are not implemented.
- Session v1 has no model-visible tool calls.
- Rust parity remains on hold.
- Python still exposes the legacy `--localtest` flag, and CLI option naming has
  not had a final release audit.
- Python logging defaults to `/var/log/cli_llm.log`; a read-only filesystem can
  raise an `OSError` not covered by the current `PermissionError` fallback.
- A live-provider terminal smoke run is still required before claiming the
  session TUI is release-ready.

## Testing and verification

Use the smallest relevant checks during iteration, then the full runtime suite
for shared behavior.

Python:

```bash
uv venv
uv pip install --python .venv/bin/python -e '.[dev]'
.venv/bin/pytest -q
.venv/bin/ruff check src tests
.venv/bin/black --check src tests
.venv/bin/mypy src
```

Go:

```bash
cd src-go
go test ./...
go build ./cmd/llm ./cmd/llm-session
./scripts/test-session-pty.sh
```

Installers:

```bash
tests/test_root_installer_targets.sh
tests/test_installer_smoke.sh
```

For terminal behavior, unit snapshots are insufficient. Changes to input,
streaming, status rows, resizing, cancellation, overlays, branch isolation, or
terminal restoration require the built-binary PTY/VT suite. Keep failure
artifacts free of secrets. Live-provider checks are release rehearsal, not a
replacement for deterministic tests.

Installer tests must isolate `HOME`, config roots, install paths, and toolchains.
They must not overwrite the developer's real binary/config or require network
access merely to test dispatch and prerequisite checks.

## Change discipline

Before finishing a change:

1. Inspect the touched runtime's actual command/config path; do not copy claims
   from an old plan.
2. Add or update tests at the layer where behavior is observable.
3. Run formatting, static checks, and relevant runtime tests.
4. Update user-facing docs and `CHANGELOG.md` when behavior changed.
5. Report any skipped live/network/TTY validation explicitly.

Keep this file focused on current constraints and verified behavior. Put detailed
implementation proposals in `docs/plans/` and completed history in
`CHANGELOG.md`.
