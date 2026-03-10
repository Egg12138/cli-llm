# Update

## installation easier

Implemented one-click installer at `scripts/install.sh` with interactive prompts and non-interactive flags.

Supported installation targets:
1. uv venv scope (default: `<PWD>/.venv`) via `--mode venv`
2. user PATH (default: `~/.local/bin`) via `--mode user` (default mode)
3. system path (default: `/usr/local/bin`) via `--mode system`

Also supports uninstall flow: `--uninstall`.

