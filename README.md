# cli-llm (internal)

cli-llm is an internal command-line client for interacting with LLM providers.  
The project currently targets team workflows only; no public release is scheduled yet, but every change must keep the codebase publish-ready.

## Status
- **Current train**: `0.6.x – In-Session Resume`
- **Latest milestone**: `0.6.0` (in-session `/resume` switching, on top of resume history replay, resume picker on stderr, resume picker/continue, tolerant session loading, durable compaction, and model restoration)
- **Python** remains the primary supported implementation. `src-go/` now contains an Eino-based parity implementation for evaluation; Rust remains dormant until explicitly scheduled.

## Roadmap Snapshot

### 0.2.x – Internal Foundations ✅
- Document internal-only posture and roadmap (0.2.1).
- Move to a modern Python packaging layout with editable installs and helper scripts (0.2.2).
- Modularise the legacy single-file CLI and introduce the configuration loader & tooling baseline.

### 0.3.x – Extensibility & UX ✅
- Agents Context Toggle: `--agents-context` flag reads `./AGENTS.md` into system prompt.
- Renderer Upgrade: `rich` + `markdown-it-py` for syntax-highlighted code blocks and proper markdown rendering.
- Plugin Framework: cargo-style subcommand discovery via `llm-*` executables on PATH.
- Go/Eino parity track: `src-go/` implements the current command surface with Eino compose workflows for `chat` and constrained `toolcall`, plus an isolated ADK learning example.

### 0.4.x – Advanced Provider & Release Prep ✅
- Go-only `llm-session` plugin for persistent multi-turn sessions, checkpoints,
  branches, and resume.
- Richer output-control pipelines for automation.
- Extended provider/model metadata + configuration depth.
- Documentation + release rehearsal; keep interfaces aligned so future Rust work can plug in without rewrites.

### 0.5.x – Session Resume & Context Management ✅
- `llm-session --resume` picker with titles and recency, `--continue`/`-c`,
  `-r` alias, and truthful errors for missing sessions.
- Tolerant JSONL loading: skipped-line diagnostics, no 64 KiB line limit.
- Durable compaction wired into turns, with projection applied after reload and
  best-effort summary failures.
- Resume restores the recorded model, including cross-profile resolution with a
  warned fallback.
- Request retry, highlighted errors, and prompt continuation after a failed turn.
- Resumed sessions replay their branch history into the terminal main buffer
  before the prompt, applying the newest compaction summary first (0.5.1).
- `--replay` / `--no-replay` control the replay; it defaults to on for terminal
  output and stays off when stdout is redirected (0.5.1).
- The `--resume` picker and resume banner write to stderr so stdout carries only
  conversation output (0.5.1).

### 0.6.x – In-Session Resume ✅
- `/resume [name]` (alias `/r`) switches to another session without restarting
  `llm-session`, restoring its model and honoring the replay preference.

> The roadmap is intentionally iterative; adjust milestones as new evidence appears.

## Guiding Principles
- Prefer developer-friendly workflows (editable installs, reproducible environments).
- Enforce Pythonic architecture with clear typing and testability.
- Build experimental features on dedicated branches with tests before merging to main.

## Installation Guide
One-click installer (interactive):

```bash
./scripts/install.sh
```

The installer supports three targets:
1. `user` (default): installs a managed venv and links `llm` to `~/.local/bin`.
2. `venv`: installs directly into a uv virtual environment (default path: `<PWD>/.venv`).
3. `system`: installs a managed venv under `/opt/cli-llm/venv` and links `llm` to `/usr/local/bin`.

Non-interactive examples:

```bash
# Default method: user PATH (~/.local/bin)
./scripts/install.sh --mode user --yes

# Project/local venv scope
./scripts/install.sh --mode venv --venv-path .venv --yes

# System path (will use sudo if needed)
./scripts/install.sh --mode system --yes
```

Uninstall examples:

```bash
./scripts/install.sh --mode user --uninstall --yes
./scripts/install.sh --mode venv --venv-path .venv --uninstall --yes
./scripts/install.sh --mode system --uninstall --yes
```

If `~/.local/bin` is not on your `PATH`, add this to your shell profile:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

For repository builds that select a runtime implementation, use the root
installer. It builds exactly one target; Go is the default when no selector is
provided:

```bash
./install.sh
CLI_LLM_GO=1 ./install.sh
CLI_LLM_PY=1 ./install.sh
CLI_LLM_RUST=1 ./install.sh
```

`CLI_LLM_GO`, `CLI_LLM_PY`, and `CLI_LLM_RUST` are mutually exclusive. The
installer reports the selected target's tool version, detected toolchain, and
numbered build/install steps. Before creating the install directory or building,
it validates the selected target's source manifests and required local tools,
including minimum toolchain versions where declared. Package downloads remain
the responsibility of the selected build tool and the user's environment.

## Development Guide
1. **Environment**
   - Create a virtual environment (`uv venv` / `python -m venv .venv`) and activate it.
   - Install with dev extras: `uv pip install --python .venv/bin/python -e .[dev]` (or `pip install -e .[dev]`).
2. **Coding Standards**
   - Format with `black src`.
   - Lint with `ruff check src`.
   - Type-check with `mypy src`.
3. **Testing**
   - Run unit tests via `pytest`.
   - Run Go parity tests via `cd src-go && go test ./...`.
   - Compare Python/Go output rendering via `python scripts/benchmark_output.py`.
   - Measure `llm` / `llm-session` startup latency via `python scripts/benchmark_startup.py` (see `docs/benchmarks/`).
4. **Workflow**
   - Keep feature work scoped to the active roadmap milestone.
- Update `AGENTS.md` + `CHANGELOG.md` whenever behavior or plans change.
- Use feature branches for experiments; merge to `main` only after tests pass.

### Startup performance

cli-llm is meant to be a fast lightweight Q&A client, so startup latency is
part of the product. `scripts/benchmark_startup.py` measures spawn → provider
request and spawn → first streamed token for `llm chat`, `llm-session --no-tui`,
and the `llm-session` TUI against a local instant provider, so network latency
is excluded:

```bash
python scripts/benchmark_startup.py --runs 30 --warmups 5
python scripts/benchmark_startup.py --ref /path/to/previous-release-bins
```

`--ref` interleaves two builds and fails on a p50 regression beyond the
configured limits. Reference numbers, method, and the current `llm-session`
regression are in `docs/benchmarks/`.

## Configuration
cli-llm resolves configuration in this order: CLI flags > environment variables > `~/.cli-llm/config.toml` > built-in defaults.  
Environment overrides follow OpenAI-style naming (`OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`) plus `CLI_LLM_DEFAULT_ROLE` and `CLI_LLM_PROVIDER` for application-level settings.

Example `~/.cli-llm/config.toml` with multiple providers:

```toml
[defaults]
provider = "openai"
model = "gpt-4o-mini"
role = "coder"

[providers.openai]
api_key = "sk-openai"
api_endpoint = "https://api.openai.com/v1"
models = ["gpt-4o", "gpt-4o-mini"]

[providers.deepseek]
api_key = "sk-deepseek"
api_endpoint = "https://api.deepseek.com/v1"
models = ["deepseek-chat", "deepseek-coder"]
```

Select a provider via config, `CLI_LLM_PROVIDER`, or the `--provider` flag. Only the `openai` provider is wired today, but other profiles can be declared for forward compatibility.

### Provider discovery helpers
- `llm inspect` – show every loadable provider profile after merging defaults, config, and environment data.
- `llm provider models [name]` – print the models declared for a profile (defaults to the active provider when omitted). Use `--json` on either command for machine-readable output.

## Plugin Guide
cli-llm supports cargo-style plugins: any executable named `llm-<name>` on your `PATH` becomes a subcommand.

### Go session plugin

`llm-session` is the Go-only multi-turn session plugin. It is not installed by
the Python installer yet; build it manually from `src-go/`:

```bash
cd src-go
go build ./cmd/llm-session
```

Put the built `llm-session` binary on `PATH` to run either form:

```bash
llm-session
llm-session --resume          # or -r
llm-session --resume work
llm-session --resume --no-replay
llm-session --continue        # or -c: most recent session
llm session --resume work
```

Sessions are stored as append-only JSONL files in
`~/.cli-llm/sessions/<name>.jsonl`. With no name, `--resume` prints a numbered
list ordered by most recently used, showing each session's persisted title, and
prompts for a number or exact name/title; an empty line (or EOF) cancels and
exits without starting a session. The picker and the resume banner are
interface chrome and are written to stderr, so stdout carries only conversation
output. `--resume NAME` reports an error when that
session does not exist instead of opening an empty one. Malformed or torn JSONL
lines are skipped with a warning instead of making the session unresumable.
Resuming restores the model recorded in the session (switching provider profile
when that model belongs to another one); if the model is no longer configured,
the current model is used with a warning. A resumed conversation is replayed
into the terminal main buffer before the prompt appears, so previous turns are
visible without opening the transcript overlay; the replay follows the active
branch and starts with the newest compaction summary. Replay is skipped when
stdout is redirected, can be forced with `--replay`, and disabled with
`--no-replay`.

Use `/resume [name]` (or `/r`) to switch to another session without restarting
`llm-session`: it loads that session's history, restores its recorded model,
and follows the same replay preference; with no name it opens the resume
picker, and cancelling or a failed load leaves the current session running.
The other slash commands are `/branches`, `/switch <branch-or-hash>`,
`/checkpoint <name>`, and `/exit`.

Failed requests are retried up to three times without duplicating the user
message. A request that fails after part of the response was already streamed is
not retried, since that partial output is already visible. If all attempts fail,
the error is highlighted and the session returns to the prompt for the next
request. A failed turn persists the user message but no assistant message or
checkpoint.

Long sessions are compressed automatically once the reachable history reaches
about 200k tokens: the model writes a summary and roughly the most recent 20k
tokens are kept. The summary is an append-only `compaction` entry, applied
again after a restart, so resumed sessions keep the compact context instead of
replaying summarized history. A failed summary never fails the chat turn.

Normal session chat writes to the terminal main buffer so regular scrollback
keeps working. The transcript overlay path uses alternate screen mode and
restores the terminal when it exits; the current line reader recognizes the
Ctrl+T control character when it is delivered by the terminal input path.

TTY prompts start in Vim INSERT mode for backward-compatible direct typing.
Esc switches to NORMAL and `v` enters VISUAL; core motions, `cw`/`ce`,
delete/change/yank operators, `yy`, `p/P`, and undo are available. The active
mode is always shown in the prompt footer. Normal chat displays blue `You ›`
and green `Assistant ›` role labels with a blank line between message blocks;
slash-command results are dimmed so they remain visually separate from chat.
Pressing Enter on an incomplete slash command executes the selected completion,
which defaults to the first candidate (for example, `/ex` executes `/exit`).
While a request is being sent, waiting for its first response, or streaming an
answer, press Esc or Ctrl+C to cancel that turn and return to the next prompt.
Set any non-empty `NO_COLOR` value to disable these styles while preserving the
labels and spacing.

### Using plugins

Once a plugin is installed on `PATH`, invoke it as a direct subcommand:

```bash
llm my-plugin arg1 --flag
# → looks for `llm-my-plugin` on PATH, replaces the process via exec
```

Plugin subcommands are dispatched **before** the default `chat` routing — if you have `llm-deploy` installed, `llm deploy ...` calls it. Unknown subcommands with no matching plugin fall back to `chat` (the original prompt-routing behavior).

### Writing a plugin

A plugin is any executable file named `llm-<name>` on your `PATH`. It can be written in any language.

**Minimal example** (bash):

```bash
#!/usr/bin/env bash
# Save as ~/.local/bin/llm-hello, then chmod +x
echo "Hello from llm-hello plugin!"
echo "Args received: $*"
```

```bash
$ chmod +x ~/.local/bin/llm-hello
$ llm hello world --verbose
Hello from llm-hello plugin!
Args received: world --verbose
```

**Python example**:

```python
#!/usr/bin/env python3
"""llm-translate — translate text via any LLM backend."""
import sys
from cli_llm.config import ConfigLoader
from cli_llm.providers import ProviderRouter, ChatRequest

def main():
    text = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else sys.stdin.read().strip()
    if not text:
        print("Usage: llm translate <text>", file=sys.stderr)
        sys.exit(1)

    config = ConfigLoader().load()
    provider = ProviderRouter(config).resolve()

    request = ChatRequest(
        model=config.default_model,
        messages=[
            {"role": "system", "content": "Translate the user's input to Chinese. Output only the translation."},
            {"role": "user", "content": text},
        ],
        stream=False,
    )
    response = provider.create_chat(request)
    print(response.choices[0].message.content)

if __name__ == "__main__":
    main()
```

Install it:

```bash
chmod +x llm-translate
mv llm-translate ~/.local/bin/
llm translate "Hello, world!"
# → 你好，世界！
```

### Plugin requirements

| Requirement | Details |
|-------------|---------|
| **Naming** | Must be named `llm-<subcommand>` (e.g., `llm-translate`, `llm-review`) |
| **Location** | Must be on `PATH` (`~/.local/bin` is recommended) |
| **Executable** | Must have execute permission (`chmod +x`) |
| **Args** | Receives all arguments after the subcommand name verbatim |
| **I/O** | Inherits stdin/stdout/stderr from the parent process — plugins can be piped |

### Built-in subcommands

These are reserved and handled internally (no plugin dispatch):

| Command | Purpose |
|---------|---------|
| `chat` | Start a chat session (default when no subcommand given) |
| `inspect` | List configured provider profiles |
| `provider` | Inspect provider metadata and models |
| `toolcall` | Execute a single tool-call-oriented request |

Plugins named `llm-chat`, `llm-inspect`, `llm-provider`, or `llm-toolcall` are ignored — built-ins always take precedence.

## Repository Layout
- `src/cli_llm/` – Python CLI package (modernised in 0.2.x).
- `src-go/` – Go/Eino parity implementation and learning examples.
- `src-rs/` – Rust prototype (development resumes when the roadmap calls for it).
- `docs/plans/` – implementation plans for active migration work.
- `AGENTS.md` – Full plan + requirements for other agents and automations.

## Contributing
1. Keep changes scoped to the active milestone unless explicitly coordinated.
2. Ensure documentation (README/AGENTS/CHANGELOG) stays aligned.
3. Treat every internal build as if it might be published tomorrow.

Questions? Start with `AGENTS.md` for context, then open an issue or discussion in the repo. 

Use `llm --version` to confirm the CLI build matches the `pyproject.toml` version.
