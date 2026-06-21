# cli-llm (internal)

cli-llm is an internal command-line client for interacting with LLM providers.  
The project currently targets team workflows only; no public release is scheduled yet, but every change must keep the codebase publish-ready.

## Status
- **Current train**: `0.3.x – Extensibility & UX`
- **Latest milestone**: `0.3.0` (Agents Context Toggle, Renderer Upgrade, Plugin Framework)
- **Python** is the only actively supported implementation today. Rust remains in the repo for future parity work.

## Roadmap Snapshot

### 0.2.x – Internal Foundations ✅
- Document internal-only posture and roadmap (0.2.1).
- Move to a modern Python packaging layout with editable installs and helper scripts (0.2.2).
- Modularise the legacy single-file CLI and introduce the configuration loader & tooling baseline.

### 0.3.x – Extensibility & UX ✅
- Agents Context Toggle: `--agents-context` flag reads `./AGENTS.md` into system prompt.
- Renderer Upgrade: `rich` + `markdown-it-py` for syntax-highlighted code blocks and proper markdown rendering.
- Plugin Framework: cargo-style subcommand discovery via `llm-*` executables on PATH.

### 0.4.x – Advanced Provider & Release Prep
- Richer output-control pipelines for automation.
- Extended provider/model metadata + configuration depth.
- Documentation + release rehearsal; keep interfaces aligned so future Rust work can plug in without rewrites.

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
4. **Workflow**
   - Keep feature work scoped to the active roadmap milestone.
- Update `AGENTS.md` + `CHANGELOG.md` whenever behavior or plans change.
- Use feature branches for experiments; merge to `main` only after tests pass.

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
- `llm providers` – show every loadable provider profile after merging defaults, config, and environment data.
- `llm provider models [name]` – print the models declared for a profile (defaults to the active provider when omitted). Use `--json` on either command for machine-readable output.

## Plugin Guide

cli-llm supports cargo-style plugins: any executable named `llm-<name>` on your `PATH` becomes a subcommand.

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
- `rust/` – Rust prototype (development resumes when the roadmap calls for it).
- `AGENTS.md` – Full plan + requirements for other agents and automations.

## Contributing
1. Keep changes scoped to the active milestone unless explicitly coordinated.
2. Ensure documentation (README/AGENTS/CHANGELOG) stays aligned.
3. Treat every internal build as if it might be published tomorrow.

Questions? Start with `AGENTS.md` for context, then open an issue or discussion in the repo. 

Use `llm --version` to confirm the CLI build matches the `pyproject.toml` version.
