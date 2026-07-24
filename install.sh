#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────
#  cli-llm — install.sh
#  Single‑script installer for the cli-llm project.
#  Supports Python (primary), Go, and Rust (opt‑in) builds.
#
#  After installation the `llm` command is placed in
#  ~/.local/bin  so it is available on most Unix PATHs.
# ─────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

LOG_PREFIX="[install]"

# ── helpers ──────────────────────────────────────────────────

info() { printf "  %s %s\n" "$LOG_PREFIX" "$*"; }
ok() { printf "  %s \e[32m✓\e[0m %s\n" "$LOG_PREFIX" "$*"; }
warn() { printf "  %s \e[33m!\e[0m %s\n" "$LOG_PREFIX" "$*" >&2; }
fail() {
	printf "  %s \e[31m✗\e[0m %s\n" "$LOG_PREFIX" "$*" >&2
	exit 1
}

command_exists() { command -v "$1" &>/dev/null; }

# ── help ──────────────────────────────────────────────────────

show_help() {
	cat <<'EOF'
Usage: ./install.sh [OPTIONS]

Single‑script installer for the cli-llm project.
Supports Python (primary), Go, and Rust (opt‑in) builds.

After installation the `llm` command is placed in
~/.local/bin so it is available on most Unix PATHs.

Options:
  -h, --help    Show this help message and exit

Environment variables:
  CLI_LLM_GO=n    Skip the Go (EINO) binary build (Go is built by default)
  CLI_LLM_RUST=y  Build the Rust binary instead of using Python

Examples:
  ./install.sh
  CLI_LLM_GO=n ./install.sh
  CLI_LLM_RUST=y ./install.sh
EOF
	exit 0
}

# Parse --help / -h before anything else
for arg in "$@"; do
	case "$arg" in
	-h | --help) show_help ;;
	esac
done

# ── header ───────────────────────────────────────────────────

VERSION="$(grep -m1 '^version =' "$PROJECT_ROOT/pyproject.toml" | cut -d'"' -f2)"
echo ""
printf "  ╭──────────────────────────────────────╮\n"
printf "  │  \e[1mcli-llm\e[0m — installer                   │\n"
printf "  │  %-36s │\n" "$VERSION"
printf "  ╰──────────────────────────────────────╯\n"
echo ""

# ── prerequisites ────────────────────────────────────────────

PYTHON=""
for candidate in python3 python; do
	if command_exists "$candidate"; then
		ver=$("$candidate" --version 2>&1 | grep -oP '\d+\.\d+')
		major="${ver%%.*}"
		if [ "$major" -ge 3 ]; then
			PYTHON="$candidate"
			break
		fi
	fi
done

if [ -z "$PYTHON" ]; then
	fail "Python 3 (>=3.9) is required but not found. Install Python first."
fi

info "Using Python: $($PYTHON --version)"

if command_exists uv; then
	INSTALLER="uv"
	INSTALL_CMD="uv pip install"
	info "Using uv for package management (faster)."
elif command_exists pip3; then
	INSTALLER="pip3"
	INSTALL_CMD="pip3 install"
	info "Using pip3 for package management."
elif command_exists pip; then
	INSTALLER="pip"
	INSTALL_CMD="pip install"
	info "Using pip for package management."
else
	fail "Neither uv nor pip found. Install pip (or uv) first."
fi

# ── phase 1: Python package ──────────────────────────────────

info "Installing cli-llm Python package (editable mode) …"
if [ "$INSTALLER" = "uv" ]; then
	uv pip install --editable "$PROJECT_ROOT" 2>&1 | sed "s/^/  $LOG_PREFIX /"
else
	$INSTALL_CMD --editable "$PROJECT_ROOT" 2>&1 | sed "s/^/  $LOG_PREFIX /"
fi
ok "Python package installed."

# ── phase 2: (optional) dev extras ───────────────────────────

info "Installing dev extras …"
if [ "$INSTALLER" = "uv" ]; then
	uv pip install --editable "$PROJECT_ROOT[dev]" 2>&1 | sed "s/^/  $LOG_PREFIX /" || true
else
	$INSTALL_CMD --editable "$PROJECT_ROOT[dev]" 2>&1 | sed "s/^/  $LOG_PREFIX /" || true
fi
ok "Dev extras installed (if available)."

# ── phase 3: create ~/.local/bin/llm entry point ──────────────

LOCAL_BIN="${HOME}/.local/bin"
mkdir -p "$LOCAL_BIN"

# Find the entry-point script that pip/uv created.
# pip-installed scripts live under sys.prefix/bin; uv may use a
# different scheme, so we fall back to a thin wrapper.
ENTRY_POINT="$($PYTHON -c "
import sys, os
p = os.path.join(sys.prefix, 'bin', 'llm')
print(p) if os.path.exists(p) else print('')
" 2>/dev/null || true)"

if [ -n "$ENTRY_POINT" ] && [ -x "$ENTRY_POINT" ]; then
	# Symlink the installed entry point into ~/.local/bin
	ln -sf "$ENTRY_POINT" "$LOCAL_BIN/llm"
	ok "Symlinked llm → $ENTRY_POINT"
else
	# Fallback: thin wrapper that invokes python -m cli_llm
	cat >"$LOCAL_BIN/llm" <<'WRAPPER'
#!/usr/bin/env bash
# cli-llm wrapper — delegates to the installed Python module.
# Created by install.sh; safe to delete / regenerate.
PYTHON="${CLI_LLM_PYTHON:-python3}"
exec "$PYTHON" -m cli_llm "$@"
WRAPPER
	chmod +x "$LOCAL_BIN/llm"
	ok "Created wrapper → $LOCAL_BIN/llm"
fi

# Ensure ~/.local/bin appears in the user's PATH hint
if [[ ":$PATH:" != *":$LOCAL_BIN:"* ]]; then
	warn "$LOCAL_BIN is not on your PATH."
	info "Add this to your shell rc file:"
	info "    export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# ── phase 4: (optional) Go binary ────────────────────────────

if command_exists go && [ -f "$PROJECT_ROOT/src-go/go.mod" ]; then
	echo ""
	echo "  ┌─ Go binary ───────────────────────────────┐"
	echo "  │  Building Go (EINO) binary by default.     │"
	echo "  │  Set CLI_LLM_GO=n to skip.                 │"
	echo "  └────────────────────────────────────────────┘"
	echo ""

	case "${CLI_LLM_GO:-y}" in
	n | N | no | NO | 0)
		info "Skipping Go build (CLI_LLM_GO=n)"
		;;
	*)
		info "Building Go binary …"
		(
			cd "$PROJECT_ROOT/src-go"
			go build -ldflags="-s -w -X github.com/Egg12138/cli-llm/src-go/internal/cli.Version=$(grep -m1 'Version =' internal/cli/root.go | cut -d'"' -f2)" -o "$PROJECT_ROOT/src-go/llm" ./cmd/llm/ 2>&1 | sed "s/^/  $LOG_PREFIX /"
		)
		GO_BIN="$PROJECT_ROOT/src-go/llm"
		if [ -x "$GO_BIN" ]; then
			cp "$GO_BIN" "$LOCAL_BIN/llm"
			ok "Go binary installed → $LOCAL_BIN/llm (replaces Python wrapper)"
		else
			warn "Go build produced no expected binary at $GO_BIN; skipping install."
		fi
		;;
	esac
else
	info "Go toolchain not found or src-go/ missing — skipping Go build."
fi

# ── phase 5: (optional) Rust binary ──────────────────────────

if command_exists cargo && [ -f "$PROJECT_ROOT/src-rs/Cargo.toml" ]; then
	echo ""
	echo "  ┌─ Rust binary ──────────────────────────────┐"
	echo "  │  A Rust implementation is available.        │"
	echo "  │  Set CLI_LLM_RUST=y to build it.            │"
	echo "  └────────────────────────────────────────────┘"
	echo ""

	case "${CLI_LLM_RUST:-}" in
	y | Y | yes | YES | 1)
		info "Building Rust binary …"
		(
			cd "$PROJECT_ROOT/src-rs"
			cargo build --release 2>&1 | sed "s/^/  $LOG_PREFIX /"
		)
		RUST_BIN="$PROJECT_ROOT/src-rs/target/release/cli-llm"
		if [ -x "$RUST_BIN" ]; then
			cp "$RUST_BIN" "$LOCAL_BIN/llm"
			ok "Rust binary installed → $LOCAL_BIN/llm (replaces Python wrapper)"
		else
			warn "Rust build produced no expected binary at $RUST_BIN; skipping install."
		fi
		;;
	*)
		info "Skipping Rust build (set CLI_LLM_RUST=y to enable)"
		;;
	esac
else
	info "Rust toolchain not found or src-rs/ missing — skipping Rust build."
fi

# ── phase 6: plugin discovery and installation ──────────────────

_install_go_plugin() {
	local name="$1"
	local dir="$2"
	info "Building plugin $name (Go) …"
	(
		cd "$PROJECT_ROOT/src-go"
		go build -ldflags="-s -w -X github.com/Egg12138/cli-llm/src-go/internal/session/cli.Version=$(grep -m1 'Version =' internal/session/cli/version.go | cut -d'"' -f2)" -o "$PROJECT_ROOT/src-go/$name" "./cmd/$name/" 2>&1 | sed "s/^/  $LOG_PREFIX /"
	)
	local bin="$PROJECT_ROOT/src-go/$name"
	if [ -x "$bin" ]; then
		cp "$bin" "$LOCAL_BIN/$name"
		ok "Plugin installed → $LOCAL_BIN/$name"
		INSTALLED_PLUGINS+=("$name")
	else
		warn "Plugin $name build failed — skipping"
	fi
}

# Discover available plugins
PLUGIN_NAMES=()
PLUGIN_DIRS=()
PLUGIN_TYPES=()
PLUGIN_DESCS=()
INSTALLED_PLUGINS=()

# Scan Go plugins (src-go/cmd/llm-* directories)
if [ -d "$PROJECT_ROOT/src-go/cmd" ]; then
	for plugin_dir in "$PROJECT_ROOT/src-go/cmd"/llm-*; do
		[ -d "$plugin_dir" ] || continue
		[ -f "$plugin_dir/main.go" ] || continue
		pname="$(basename "$plugin_dir")"
		pdesc=$(head -20 "$plugin_dir/main.go" 2>/dev/null | grep -m1 '//' | sed 's|//[[:space:]]*||')
		[ -z "$pdesc" ] && pdesc="Go plugin"
		PLUGIN_NAMES+=("$pname")
		PLUGIN_DIRS+=("$plugin_dir")
		PLUGIN_TYPES+=("go")
		PLUGIN_DESCS+=("$pdesc")
	done
fi

plugin_count=${#PLUGIN_NAMES[@]}

if [ "$plugin_count" -gt 0 ]; then
	echo ""
	echo "  ┌─ Plugins ──────────────────────────────────┐"
	echo "  │  The following plugins were discovered:     │"
	for ((pi=0; pi<plugin_count; pi++)); do
		display_name="${PLUGIN_NAMES[$pi]#llm-}"
		printf "  │  [%d] %-20s — %s\n" "$((pi+1))" "$display_name" "${PLUGIN_DESCS[$pi]}"
	done
	echo "  └─────────────────────────────────────────────┘"
	echo ""
	echo "  Enter numbers to install (space-separated),"
	echo "  'all' for all, or press Enter to skip:"
	printf "  > "
	read -r selection

	case "${selection:-}" in
		"" | none | skip)
			info "Skipping plugin installation."
			;;
		all | a | ALL)
			for ((pi=0; pi<plugin_count; pi++)); do
				case "${PLUGIN_TYPES[$pi]}" in
					go)
						if command_exists go; then
							_install_go_plugin "${PLUGIN_NAMES[$pi]}" "${PLUGIN_DIRS[$pi]}"
						else
							warn "Go not found — cannot build ${PLUGIN_NAMES[$pi]} (skipping)"
						fi
						;;
					*)
						warn "Unknown plugin type '${PLUGIN_TYPES[$pi]}' for ${PLUGIN_NAMES[$pi]} — skipping"
						;;
				esac
			done
			;;
		*)
			for num in $selection; do
				case "$num" in
					''|*[!0-9]*) continue ;;
				esac
				if [ "$num" -ge 1 ] 2>/dev/null && [ "$num" -le "$plugin_count" ] 2>/dev/null; then
					idx=$((num-1))
					case "${PLUGIN_TYPES[$idx]}" in
						go)
							if command_exists go; then
								_install_go_plugin "${PLUGIN_NAMES[$idx]}" "${PLUGIN_DIRS[$idx]}"
							else
								warn "Go not found — cannot build ${PLUGIN_NAMES[$idx]} (skipping)"
							fi
							;;
						*)
							warn "Unknown plugin type '${PLUGIN_TYPES[$idx]}' for ${PLUGIN_NAMES[$idx]} — skipping"
							;;
					esac
				fi
			done
			;;
	esac
fi

# ── phase 7: config directory ────────────────────────────────

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/cli-llm"
mkdir -p "$CONFIG_DIR"

if [ -f "$PROJECT_ROOT/config.sample.toml" ] && [ ! -f "$CONFIG_DIR/config.toml" ]; then
	cp "$PROJECT_ROOT/config.sample.toml" "$CONFIG_DIR/config.toml"
	ok "Created default config → $CONFIG_DIR/config.toml"
	info "Remember to set your own API keys in $CONFIG_DIR/config.toml"
elif [ -f "$CONFIG_DIR/config.toml" ]; then
	ok "Config already exists → $CONFIG_DIR/config.toml (not overwritten)"
else
	warn "No config.sample.toml found; skipping config setup."
fi

# Ensure the legacy ~/.cli-llm/ path is also present for backward compat
if [ "$CONFIG_DIR" != "$HOME/.cli-llm" ]; then
	LEGACY_DIR="$HOME/.cli-llm"
	if [ ! -d "$LEGACY_DIR" ]; then
		mkdir -p "$LEGACY_DIR"
		# Symlink to the XDG config path so both work
		ln -sfn "$CONFIG_DIR" "$LEGACY_DIR" 2>/dev/null || true
		ok "Created legacy symlink: $LEGACY_DIR → $CONFIG_DIR"
	fi
fi

# ── phase 8: shell helpers (optional) ────────────────────────

if [ -f "$PROJECT_ROOT/llm_bash.sh" ]; then
	SHELL_RC="${HOME}/.bashrc"
	if [ -f "$HOME/.zshrc" ]; then
		SHELL_RC="$HOME/.zshrc"
	fi
	echo ""
	echo "  ┌─ Shell helpers ───────────────────────────┐"
	echo "  │  llm_bash.sh provides API switching        │"
	echo "  │  functions (api-glm, api-qwen, …).         │"
	echo "  └────────────────────────────────────────────┘"
	echo ""
	info "To enable shell helpers, add this to $SHELL_RC:"
	echo ""
	echo "    source \"$PROJECT_ROOT/llm_bash.sh\""
	echo ""
	info "(Or run it directly: source llm_bash.sh)"
fi

	# ── done ─────────────────────────────────────────────────────

	echo ""
	echo "  ┌──────────────────────────────────────┐"
	echo "  │  \e[1mcli-llm installation complete\e[0m      │"
	echo "  └──────────────────────────────────────┘"
	echo ""
	echo "    Run \`llm --help\` to verify the CLI works."
	echo "    Run \`llm chat\` to start a conversation."
	if [ ${#INSTALLED_PLUGINS[@]} -gt 0 ]; then
		for p in "${INSTALLED_PLUGINS[@]}"; do
			local_name="${p#llm-}"
			echo "    Run \`llm ${local_name}\` to use the ${local_name} plugin."
		done
	fi
	echo ""
	echo "    Binary:      $LOCAL_BIN/llm"
	if [ ${#INSTALLED_PLUGINS[@]} -gt 0 ]; then
		for p in "${INSTALLED_PLUGINS[@]}"; do
			echo "    Plugin:      $LOCAL_BIN/$p"
		done
	fi
	echo "    Config file: $CONFIG_DIR/config.toml"
	echo "    Project:     $PROJECT_ROOT"
	echo ""
