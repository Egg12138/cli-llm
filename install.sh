#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
LOG_PREFIX="[install]"
CHECK_PREFIX="[check]"

info() { printf "  %s %s\n" "$LOG_PREFIX" "$*"; }
ok() { printf "  %s \033[32m✓\033[0m %s\n" "$LOG_PREFIX" "$*"; }
warn() { printf "  %s \033[33m!\033[0m %s\n" "$LOG_PREFIX" "$*" >&2; }
check_info() { printf "  %s %s\n" "$CHECK_PREFIX" "$*"; }
check_ok() { printf "  %s \033[32m✓\033[0m %s\n" "$CHECK_PREFIX" "$*"; }
fail() {
	printf "  %s \033[31m✗\033[0m %s\n" "$LOG_PREFIX" "$*" >&2
	exit 1
}

command_exists() { command -v "$1" &>/dev/null; }

show_help() {
	cat <<'EOF'
Usage: ./install.sh [OPTIONS]

Build and install one cli-llm runtime target. With no target variable set,
the Go target is selected.

Options:
  -h, --help    Show this help message and exit

Target selection (choose at most one):
  CLI_LLM_GO=y    Build the Go/Eino target
  CLI_LLM_PY=y    Install the Python target
  CLI_LLM_RUST=y  Build the Rust target

Accepted true values: y, yes, 1, true, on (case-insensitive).
For backward compatibility, CLI_LLM_GO=n selects Python when no other target
is enabled.

Examples:
  ./install.sh
  CLI_LLM_GO=1 ./install.sh
  CLI_LLM_PY=y ./install.sh
  CLI_LLM_RUST=yes ./install.sh
EOF
	exit 0
}

for arg in "$@"; do
	case "$arg" in
	-h | --help) show_help ;;
	*) fail "Unknown option: $arg" ;;
	esac
done

normalize_toggle() {
	local name="$1"
	local value="$2"
	case "${value,,}" in
		y | yes | 1 | true | on) printf '1' ;;
		n | no | 0 | false | off | '') printf '0' ;;
		*) fail "$name must be one of: y, yes, 1, true, on, n, no, 0, false, off" ;;
	esac
}

GO_SET=0
PY_SET=0
RUST_SET=0
[ "${CLI_LLM_GO+x}" = x ] && GO_SET=1
[ "${CLI_LLM_PY+x}" = x ] && PY_SET=1
[ "${CLI_LLM_RUST+x}" = x ] && RUST_SET=1

GO_ENABLED="$(normalize_toggle CLI_LLM_GO "${CLI_LLM_GO:-}")"
PY_ENABLED="$(normalize_toggle CLI_LLM_PY "${CLI_LLM_PY:-}")"
RUST_ENABLED="$(normalize_toggle CLI_LLM_RUST "${CLI_LLM_RUST:-}")"
ENABLED_COUNT=$((GO_ENABLED + PY_ENABLED + RUST_ENABLED))

if [ "$ENABLED_COUNT" -gt 1 ]; then
	fail "Choose exactly one build target; CLI_LLM_GO, CLI_LLM_PY, and CLI_LLM_RUST are mutually exclusive."
fi

if [ "$ENABLED_COUNT" -eq 0 ]; then
	if [ "$GO_SET" -eq 0 ] && [ "$PY_SET" -eq 0 ] && [ "$RUST_SET" -eq 0 ]; then
		GO_ENABLED=1
	elif [ "$GO_SET" -eq 1 ] && [ "$PY_SET" -eq 0 ] && [ "$RUST_SET" -eq 0 ]; then
		PY_ENABLED=1
	else
		fail "No build target is enabled. Set one of CLI_LLM_GO, CLI_LLM_PY, or CLI_LLM_RUST to y."
	fi
fi

read_python_version() {
	sed -n 's/^version = "\([^"]*\)"/\1/p' "$PROJECT_ROOT/pyproject.toml" | head -n 1
}

read_go_version() {
	sed -n 's/^const Version = "\([^"]*\)"/\1/p' "$PROJECT_ROOT/src-go/internal/cli/root.go" | head -n 1
}

read_rust_version() {
	sed -n 's/^version = "\([^"]*\)"/\1/p' "$PROJECT_ROOT/src-rs/Cargo.toml" | head -n 1
}

if [ "$GO_ENABLED" -eq 1 ]; then
	TARGET="go"
	TARGET_LABEL="Go (Eino)"
	VERSION="$(read_go_version)"
elif [ "$PY_ENABLED" -eq 1 ]; then
	TARGET="python"
	TARGET_LABEL="Python"
	VERSION="$(read_python_version)"
else
	TARGET="rust"
	TARGET_LABEL="Rust"
	VERSION="$(read_rust_version)"
fi

[ -n "$VERSION" ] || fail "Could not determine the $TARGET_LABEL tool version."

printf '\n'
printf '  ╭──────────────────────────────────────╮\n'
printf '  │  \033[1m%-36s\033[0m │\n' "cli-llm $VERSION"
printf '  │  Target: %-28s │\n' "$TARGET_LABEL"
printf '  ╰──────────────────────────────────────╯\n'
printf '\n'

LOCAL_BIN="${HOME}/.local/bin"
PYTHON_BIN=""
PACKAGE_MANAGER=""

version_at_least() {
	local have="$1"
	local need="$2"
	local have_major have_minor have_patch need_major need_minor need_patch
	IFS=. read -r have_major have_minor have_patch <<<"$have"
	IFS=. read -r need_major need_minor need_patch <<<"$need"
	have_minor=${have_minor:-0}
	have_patch=${have_patch:-0}
	need_minor=${need_minor:-0}
	need_patch=${need_patch:-0}
	if [ "$have_major" -ne "$need_major" ]; then
		[ "$have_major" -gt "$need_major" ]
	elif [ "$have_minor" -ne "$need_minor" ]; then
		[ "$have_minor" -gt "$need_minor" ]
	else
		[ "$have_patch" -ge "$need_patch" ]
	fi
}

check_python_prerequisites() {
	check_info "Checking prerequisites for $TARGET_LABEL …"
	[ -f "$PROJECT_ROOT/pyproject.toml" ] || fail "Python project metadata is missing: pyproject.toml"
	[ -d "$PROJECT_ROOT/src/cli_llm" ] || fail "Python package source is missing: src/cli_llm"
	check_ok "Project metadata and Python sources found."

	local candidate
	for candidate in python3 python; do
		if command_exists "$candidate"; then
			local version
			version=$("$candidate" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")' 2>/dev/null || true)
			if [ -n "$version" ]; then
				local major="${version%%.*}"
				local minor="${version#*.}"
				if [ "$major" -gt 3 ] || { [ "$major" -eq 3 ] && [ "$minor" -ge 9 ]; }; then
					PYTHON_BIN="$candidate"
					break
				fi
			fi
		fi
	done
	[ -n "$PYTHON_BIN" ] || fail "Python >=3.9 is required for target Python but was not found."
	check_ok "Toolchain: $($PYTHON_BIN --version 2>&1) ($(command -v "$PYTHON_BIN"))"

	if command_exists uv; then
		PACKAGE_MANAGER="uv"
	elif command_exists pip3; then
		PACKAGE_MANAGER="pip3"
	elif command_exists pip; then
		PACKAGE_MANAGER="pip"
	else
		fail "uv or pip is required for target Python but was not found."
	fi

	if [ "$PACKAGE_MANAGER" = "uv" ]; then
		check_ok "Package manager: $(uv --version 2>&1 | head -n 1) ($(command -v uv))"
	else
		check_ok "Package manager: $($PACKAGE_MANAGER --version 2>&1 | head -n 1) ($(command -v "$PACKAGE_MANAGER"))"
	fi

	check_ok "Python prerequisite check complete."
}

check_go_prerequisites() {
	check_info "Checking prerequisites for $TARGET_LABEL …"
	[ -f "$PROJECT_ROOT/src-go/go.mod" ] || fail "Go module manifest is missing: src-go/go.mod"
	[ -f "$PROJECT_ROOT/src-go/go.sum" ] || fail "Go module checksums are missing: src-go/go.sum"
	[ -d "$PROJECT_ROOT/src-go/cmd/llm" ] || fail "Go command source is missing: src-go/cmd/llm"
	check_ok "go.mod, go.sum, and command sources found."
	command_exists go || fail "Go is required for target Go but was not found."

	local toolchain required installed
	toolchain=$(go version 2>&1)
	required=$(sed -n 's/^go[[:space:]]\{1,\}\([0-9.]*\)$/\1/p' "$PROJECT_ROOT/src-go/go.mod" | head -n 1)
	installed=$(printf '%s\n' "$toolchain" | sed -n 's/.* go\([0-9][0-9.]*\).*/\1/p')
	[ -n "$required" ] && [ -n "$installed" ] || fail "Could not determine the required and installed Go versions."
	version_at_least "$installed" "$required" || fail "Go >=$required is required; found $installed."
	check_ok "Toolchain: $toolchain ($(command -v go)); requires >=$required."

	check_ok "Go prerequisite check complete."
}

check_rust_prerequisites() {
	check_info "Checking prerequisites for $TARGET_LABEL …"
	[ -f "$PROJECT_ROOT/src-rs/Cargo.toml" ] || fail "Rust package manifest is missing: src-rs/Cargo.toml"
	[ -f "$PROJECT_ROOT/src-rs/Cargo.lock" ] || fail "Rust dependency lockfile is missing: src-rs/Cargo.lock"
	[ -f "$PROJECT_ROOT/src-rs/src/main.rs" ] || fail "Rust command source is missing: src-rs/src/main.rs"
	check_ok "Cargo.toml, Cargo.lock, and command sources found."
	command_exists cargo || fail "Cargo is required for target Rust but was not found."
	command_exists rustc || fail "rustc is required for target Rust but was not found."
	check_ok "Toolchain: $(rustc --version 2>&1) ($(command -v rustc))"
	check_ok "Build tool: $(cargo --version 2>&1) ($(command -v cargo))"

	check_ok "Rust prerequisite check complete."
}

install_python() {

	info "Step 1/3: Installing the Python package (editable mode) …"
	if [ "$PACKAGE_MANAGER" = "uv" ]; then
		uv pip install --editable "$PROJECT_ROOT" 2>&1 | sed "s/^/  $LOG_PREFIX /"
	else
		"$PACKAGE_MANAGER" install --editable "$PROJECT_ROOT" 2>&1 | sed "s/^/  $LOG_PREFIX /"
	fi
	ok "Step 1/3 complete: Python package installed."

	info "Step 2/3: Installing development extras …"
	if [ "$PACKAGE_MANAGER" = "uv" ]; then
		uv pip install --editable "$PROJECT_ROOT[dev]" 2>&1 | sed "s/^/  $LOG_PREFIX /" || warn "Development extras were not installed."
	else
		"$PACKAGE_MANAGER" install --editable "$PROJECT_ROOT[dev]" 2>&1 | sed "s/^/  $LOG_PREFIX /" || warn "Development extras were not installed."
	fi
	ok "Step 2/3 complete: Development extras processed."

	info "Step 3/3: Installing executable → $LOCAL_BIN/llm"
	local entry_point
	entry_point=$("$PYTHON_BIN" -c 'import os, sys; p = os.path.join(sys.prefix, "bin", "llm"); print(p if os.path.exists(p) else "")' 2>/dev/null || true)
	if [ -n "$entry_point" ] && [ -x "$entry_point" ]; then
		ln -sf "$entry_point" "$LOCAL_BIN/llm"
	else
		{
			printf '%s\n' '#!/usr/bin/env bash'
			printf '%s\n' 'PYTHON="${CLI_LLM_PYTHON:-python3}"'
			printf '%s\n' 'exec "$PYTHON" -m cli_llm "$@"'
		} >"$LOCAL_BIN/llm"
		chmod +x "$LOCAL_BIN/llm"
	fi
	ok "Step 3/3 complete: Python executable installed → $LOCAL_BIN/llm"
}

install_go() {
	info "Step 1/2: Building ./cmd/llm …"
	(
		cd "$PROJECT_ROOT/src-go"
		go build -ldflags="-s -w -X github.com/Egg12138/cli-llm/src-go/internal/cli.Version=$VERSION" -o "$PROJECT_ROOT/src-go/llm" ./cmd/llm/ 2>&1 | sed "s/^/  $LOG_PREFIX /"
	)
	ok "Step 1/2 complete: Go binary built."

	info "Step 2/2: Installing executable → $LOCAL_BIN/llm"
	cp "$PROJECT_ROOT/src-go/llm" "$LOCAL_BIN/llm"
	chmod +x "$LOCAL_BIN/llm"
	ok "Step 2/2 complete: Go executable installed → $LOCAL_BIN/llm"
}

install_rust() {
	info "Step 1/2: Building release binary …"
	(
		cd "$PROJECT_ROOT/src-rs"
		cargo build --release 2>&1 | sed "s/^/  $LOG_PREFIX /"
	)
	ok "Step 1/2 complete: Rust release binary built."

	local binary="$PROJECT_ROOT/src-rs/target/release/cli-llm"
	[ -x "$binary" ] || fail "Rust build produced no executable at $binary"
	info "Step 2/2: Installing executable → $LOCAL_BIN/llm"
	cp "$binary" "$LOCAL_BIN/llm"
	chmod +x "$LOCAL_BIN/llm"
	ok "Step 2/2 complete: Rust executable installed → $LOCAL_BIN/llm"
}

case "$TARGET" in
	python) check_python_prerequisites ;;
	go) check_go_prerequisites ;;
	rust) check_rust_prerequisites ;;
esac

mkdir -p "$LOCAL_BIN"

case "$TARGET" in
	python) install_python ;;
	go) install_go ;;
	rust) install_rust ;;
esac

if [[ ":$PATH:" != *":$LOCAL_BIN:"* ]]; then
	warn "$LOCAL_BIN is not on your PATH."
	info "Add this to your shell rc file: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

_install_go_plugin() {
	local name="$1"
	local version
	version=$(sed -n 's/^var Version = "\([^"]*\)"/\1/p' "$PROJECT_ROOT/src-go/internal/session/cli/version.go" | head -n 1)
	info "Plugin target: $name (Go, ${version:-unknown version})"
	info "Plugin toolchain: $(go version 2>&1)"
	info "Building plugin $name …"
	(
		cd "$PROJECT_ROOT/src-go"
		go build -ldflags="-s -w -X github.com/Egg12138/cli-llm/src-go/internal/session/cli.Version=$version" -o "$PROJECT_ROOT/src-go/$name" "./cmd/$name/" 2>&1 | sed "s/^/  $LOG_PREFIX /"
	)
	cp "$PROJECT_ROOT/src-go/$name" "$LOCAL_BIN/$name"
	chmod +x "$LOCAL_BIN/$name"
	ok "Plugin installed → $LOCAL_BIN/$name"
	INSTALLED_PLUGINS+=("$name")
}

PLUGIN_NAMES=()
PLUGIN_TYPES=()
PLUGIN_DESCS=()
INSTALLED_PLUGINS=()

if [ -d "$PROJECT_ROOT/src-go/cmd" ]; then
	for plugin_dir in "$PROJECT_ROOT/src-go/cmd"/llm-*; do
		[ -d "$plugin_dir" ] || continue
		[ -f "$plugin_dir/main.go" ] || continue
		pname="$(basename "$plugin_dir")"
		pdesc=$(sed -n 's|^//[[:space:]]*||p' "$plugin_dir/main.go" | head -n 1)
		[ -z "$pdesc" ] && pdesc="Go plugin"
		PLUGIN_NAMES+=("$pname")
		PLUGIN_TYPES+=("go")
		PLUGIN_DESCS+=("$pdesc")
	done
fi

plugin_count=${#PLUGIN_NAMES[@]}
if [ "$plugin_count" -gt 0 ]; then
	printf '\n'
	printf '  Available plugins:\n'
	for ((pi = 0; pi < plugin_count; pi++)); do
		printf '    [%d] %-20s — %s\n' "$((pi + 1))" "${PLUGIN_NAMES[$pi]#llm-}" "${PLUGIN_DESCS[$pi]}"
	done
	printf '\n  Enter numbers to install (space-separated), all, or Enter to skip:\n  > '
	selection=""
	read -r selection || true

	if [ "$selection" = "all" ] || [ "$selection" = "a" ] || [ "$selection" = "ALL" ]; then
		selection=""
		for ((pi = 1; pi <= plugin_count; pi++)); do
			selection+=" $pi"
		done
	fi

	if [ -z "$selection" ] || [ "$selection" = "none" ] || [ "$selection" = "skip" ]; then
		info "Skipping plugin installation."
	else
		for num in $selection; do
			case "$num" in
			'' | *[!0-9]*) continue ;;
			esac
			if [ "$num" -ge 1 ] && [ "$num" -le "$plugin_count" ]; then
				idx=$((num - 1))
				if [ "${PLUGIN_TYPES[$idx]}" = "go" ] && command_exists go; then
					_install_go_plugin "${PLUGIN_NAMES[$idx]}"
				else
					warn "Go not found — cannot build ${PLUGIN_NAMES[$idx]} (skipping)"
				fi
			fi
		done
	fi
fi

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

if [ "$CONFIG_DIR" != "$HOME/.cli-llm" ] && [ ! -e "$HOME/.cli-llm" ]; then
	ln -s "$CONFIG_DIR" "$HOME/.cli-llm" 2>/dev/null || true
	ok "Created legacy symlink: $HOME/.cli-llm → $CONFIG_DIR"
fi

if [ -f "$PROJECT_ROOT/llm_bash.sh" ]; then
	SHELL_RC="$HOME/.bashrc"
	[ -f "$HOME/.zshrc" ] && SHELL_RC="$HOME/.zshrc"
	info "Optional shell helpers: add 'source \"$PROJECT_ROOT/llm_bash.sh\"' to $SHELL_RC"
fi

printf '\n'
printf '  ╭──────────────────────────────────────╮\n'
printf '  │  \033[1m%-36s\033[0m │\n' 'cli-llm installation complete'
printf '  ╰──────────────────────────────────────╯\n'
printf '\n'
printf '    Run `llm --help` to verify the CLI works.\n'
printf '    Run `llm chat` to start a conversation.\n'
for plugin in "${INSTALLED_PLUGINS[@]}"; do
	printf '    Run `llm %s` to use the %s plugin.\n' "${plugin#llm-}" "${plugin#llm-}"
done
printf '\n'
printf '    Installed:   cli-llm %s (%s)\n' "$VERSION" "$TARGET_LABEL"
printf '    Binary:      %s/llm\n' "$LOCAL_BIN"
for plugin in "${INSTALLED_PLUGINS[@]}"; do
	printf '    Plugin:      %s/%s\n' "$LOCAL_BIN" "$plugin"
done
printf '    Config file: %s/config.toml\n' "$CONFIG_DIR"
printf '    Project:     %s\n\n' "$PROJECT_ROOT"
