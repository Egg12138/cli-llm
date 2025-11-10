# Changelog

All notable changes to this project will be documented in this file.  
The format roughly follows [Keep a Changelog](https://keepachangelog.com/) and uses semantic versioning when we are ready for public releases.  
Until then, entries describe internal milestones so the team can track progress.

## [Unreleased]
### Added
- `providers` and `provider models` CLI subcommands to inspect merged provider metadata and model lists, plus accompanying tests and docs.
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
