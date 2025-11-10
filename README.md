# cli-llm (internal)

cli-llm is an internal command-line client for interacting with LLM providers.  
The project currently targets team workflows only; no public release is scheduled yet, but every change must keep the codebase publish-ready.

## Status
- **Current train**: `0.2.x – Internal Foundations`
- **Latest milestone**: `0.2.1 Freeze Roadmap Messaging` (in progress)
- **Python** is the only actively supported implementation today. Rust remains in the repo for future parity work.

## Roadmap Snapshot

### 0.2.x – Internal Foundations
- Document internal-only posture and roadmap (0.2.1).
- Move to a modern Python packaging layout with editable installs and helper scripts (0.2.2).
- Begin modularising the legacy single-file CLI and introduce the configuration loader & tooling baseline.

### 0.3.x – Extensibility & UX
- Role manager plus user-defined prompt loading.
- Provider abstraction with the latest OpenAI-compatible SDK integration.
- Optional AGENTS.md context ingestion, CLI option overhaul, renderer/output-mode redesign.

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
1. Ensure Python 3.9+ is available (use [`uv`](https://github.com/astral-sh/uv) or `pyenv` for isolation).
2. Clone the repository and install in editable mode:
   ```bash
   uv pip install -e .
   # or
   python -m pip install -e .
   ```
3. Verify the CLI:
   ```bash
   llm --help        # installed entrypoint
   python -m cli_llm # module form
   ```
4. Optional: install dev extras (`pip install -e .[dev]`) to get linting and testing tools.

## Development Guide
1. **Environment**
   - Create a virtual environment (`uv venv` / `python -m venv .venv`) and activate it.
   - Install with dev extras: `pip install -e .[dev]`.
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
