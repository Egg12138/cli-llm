# Changelog

All notable changes to this project will be documented in this file.  
The format roughly follows [Keep a Changelog](https://keepachangelog.com/) and uses semantic versioning when we are ready for public releases.  
Until then, entries describe internal milestones so the team can track progress.

## [Unreleased]

### Added
- `llm-session` TTY prompts now provide INSERT, NORMAL, and VISUAL Vim modes with core motions, `cw`/`ce`, delete/change/yank operators, an unnamed register with `p`/`P`, and undo. Fresh prompts start in INSERT mode for compatibility.
- Unit and built-binary PTY coverage for modal editing, Unicode/CJK word motion, visual selection, yank/paste, provider input, and JSONL persistence.
- Isolated target-dispatch coverage for the root multi-runtime installer.

### Changed
- The root installer now treats `CLI_LLM_GO`, `CLI_LLM_PY`, and `CLI_LLM_RUST` as mutually exclusive target selectors. It builds only the selected runtime and reports that target's tool version, toolchain, and numbered build steps.
- After target selection, the root installer now completes a target-specific prerequisite check before creating the install directory or starting a build. It validates source manifests, required local tools, and declared minimum toolchain versions without pre-downloading package dependencies.

### Fixed
- Go-only installation no longer performs or prints Python installation work, and installer banners no longer label a package version as the installer version.
- The root installer's completion banner now emits terminal styling correctly instead of printing literal `\e` sequences.

## [0.4.3] – 2026-07-24

### Added
- Root-level alias registry: subcommands gain single-letter shortcuts ("s" → session, "c" → chat, "i" → inspect, "p" → provider, "t" → toolcall). Aliases resolve before dispatch so plugins and builtins require no changes.

## [0.4.2] – 2026-07-24

### Added
- Registry-driven `/help` and prefix slash-command completion with inline usage descriptions.
- Inline Bubbles textarea editor with exact spaces/UTF-8 preservation, Ctrl+J multi-line input, resize handling, and main-buffer operation.
- Built-binary PTY/VT acceptance suite covering help, completion, CJK and multi-line input, long streamed replies, resize, cancellation, status cleanup, and Ctrl+T overlay restoration; failure runs retain non-secret ANSI/screen artifacts.

### Changed
- Normal `llm-session` chat now uses the terminal main buffer; only the transcript overlay enters the alternate screen.
- Streamed assistant content is written byte-for-byte in arrival order and receives a display-only final newline only when needed.

### Fixed
- TTY status frames now replace one logical row and synchronously stop before response, error, or cancellation output.
- Ctrl+C cancels only the active model turn, prints `cancelled`, and returns to the REPL.
- Long and multi-line assistant replies are no longer clipped by a fixed-height response viewport.

## [0.4.1] – 2026-06-23

### Added
- `llm-session --version` flag with ldflags-injectable version (`0.4.1`).
- `/t` / `/transcript` REPL command to open the TUI overlay (Ctrl+T is intercepted by POSIX terminal line discipline and never reaches the app).
- Status reporter in `runtime` (`StatusReporter` interface, `StatusThinking`/`StatusWaitingStream`/`StatusStreaming` lifecycle).
- TTY spinner (`TTYReporter`) with braille frames and deterministic plain-text fallback (`PlainReporter`).
- Semantic color tokens (`AdaptiveColor`) for role styles, with `NO_COLOR` env var support.
- Minimum terminal size check in the TUI overlay (`60×8` minimum).
### Changed
- `/branches` output no longer displays raw head IDs — shows only branch names and parent topology.
- `Esc` in the TUI branch panel returns to scroll mode instead of quitting.
- Footer hint in TUI overlay shows `Tab` (branches) and `y` (copy) alongside nav keys.
- Install script (`install.sh`) injects version via `-X ldflags` for both main Go binary and plugin builds.
### Fixed
- Ctrl+T raw byte detection relaxed from `==` to `strings.Contains` for terminals that pass `\x14` through.
- `Esc` in branch panel quits overlay instead of returning to scroll mode.

## [0.4.0] – Advanced Provider & Release Prep *(internal)*
### Added
- `src-go/` Eino parity implementation covering layered config, prompt/AGENTS handling, OpenAI-compatible provider factory, compose-based `chat`, constrained `toolcall`, inspect/provider metadata commands, and cargo-style plugin dispatch.
- Go-only `llm-session` plugin binary with append-only JSONL sessions under `~/.cli-llm/sessions`, auto checkpoints, branch switching, resume modes, auto titling, context compression primitives, normal-buffer REPL behavior, and Ctrl+T transcript overlay support.
- Isolated `src-go/examples/chatmodel_agent` ADK learning spike with a deterministic local tool example.
- Go parity documentation at `src-go/README.md` and implementation plan at `docs/plans/2026-06-22-eino-reimplementation.md`.

### Changed
- Plugin dispatch resolution in the Go implementation now lives in `internal/plugins` and is covered by dedicated tests.

## [0.3.0] – Extensibility & UX *(internal)*
### Added
- `--agents-context` / `-A` flag on `chat` command — reads `./AGENTS.md` from cwd and appends to system prompt. Includes size guard (16 KB truncation), sanitization, and graceful missing-file handling.
- Plugin framework with cargo-style subcommand discovery — unknown subcommands trigger a PATH search for `llm-<subcommand>` executables, dispatching via `os.execvp`.
- Comprehensive test coverage for agents context (6 tests) and plugin framework (3 tests).
### Changed
- Response renderer upgraded from naive ANSI-based output to `rich` + `markdown-it-py`. Code blocks now receive proper syntax highlighting; markdown formatting is handled natively. Public API preserved (`highlight_code_blocks`, `process_streamed_chunk`, `process_unstreamed_chunk`).
### Fixed
- Pre-existing config loader test (`test_loader_reads_legacy_config_path`) repaired after `LEGACY_CONFIG_PATH` constant was removed.
- Plugin dispatch no longer breaks default-to-chat routing — unknown subcommands without a matching plugin fall back to `chat` instead of erroring.

## [0.2.5] – Toolcall & Installer *(internal)*
### Added
- `providers` and `provider models` CLI subcommands to inspect merged provider metadata and model lists, plus accompanying tests and docs.
- One-click installer script at `scripts/install.sh` with interactive setup and non-interactive flags for `user` (default), `venv`, and `system` install modes.
- Installer smoke test script `tests/test_installer_smoke.sh` covering user-mode install/reinstall/uninstall and venv-mode install/uninstall.
### Changed
- README installation docs now use `./scripts/install.sh` as the primary workflow with mode-specific examples and PATH troubleshooting guidance.
### Fixed
- Config loader now detects legacy `~/.cli_llm/config.toml` so existing installs no longer lose API keys when env vars are unset.

## [0.2.3] – Configuration Loader *(internal)*
### Added
- Documented the internal-only status and roadmap snapshot in `README.md`.
- Introduced this changelog to capture work in the 0.2.x cycle.
- Layered configuration loader with CLI/env/user-file/default precedence plus tests, including multi-provider support and a sample `config.toml`.
- CLI `--version/-V` flag that reflects the pyproject version regardless of how the package was installed.
- Regression tests for the version helper to ensure local installs stay in sync.
### Changed
- Refactored the CLI into modular components (`config`, `providers`, `renderers`, `services`) and shrank the `cli` entry to wiring only.

## [0.2.1] – Freeze Roadmap Messaging *(in progress, internal)*
- Define messaging tasks for README/CHANGELOG to reflect the internal-only posture and roadmap.
- Migrate package management to uv 
- Update README, gitignore
