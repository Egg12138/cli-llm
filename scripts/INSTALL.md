# Installer Guide

`./scripts/install.sh` provides an interactive one-click install for `cli-llm`.

## Modes

- `user` (default)
  - venv: `~/.local/share/cli-llm/venv`
  - binary: `~/.local/bin/llm`
- `venv`
  - venv: `<PWD>/.venv`
  - binary lives in `<venv>/bin/llm`
- `system`
  - venv: `/opt/cli-llm/venv`
  - binary: `/usr/local/bin/llm`

## Common commands

```bash
# Interactive install (default: user mode)
./scripts/install.sh

# Non-interactive user install
./scripts/install.sh --mode user --yes

# Install to local/project venv
./scripts/install.sh --mode venv --venv-path .venv --yes

# Install to system path (uses sudo if required)
./scripts/install.sh --mode system --yes

# Uninstall
./scripts/install.sh --mode user --uninstall --yes
```

## Options

- `--mode <user|venv|system>`
- `--venv-path <path>`
- `--bindir <path>`
- `--yes`
- `--uninstall`
- `--help`
