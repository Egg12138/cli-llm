#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
TMP_ROOT=$(mktemp -d)
trap 'rm -rf "$TMP_ROOT"' EXIT

PROJECT="$TMP_ROOT/project"
FAKE_BIN="$TMP_ROOT/bin"
FAKE_HOME="$TMP_ROOT/home"
mkdir -p "$PROJECT/src/cli_llm" "$PROJECT/src-go/internal/cli" \
	"$PROJECT/src-go/cmd/llm" "$PROJECT/src-rs/src" "$FAKE_BIN" "$FAKE_HOME"
cp "$ROOT_DIR/install.sh" "$PROJECT/install.sh"
cp "$ROOT_DIR/pyproject.toml" "$PROJECT/pyproject.toml"
cp "$ROOT_DIR/config.sample.toml" "$PROJECT/config.sample.toml"
cp "$ROOT_DIR/src-go/go.mod" "$PROJECT/src-go/go.mod"
cp "$ROOT_DIR/src-go/go.sum" "$PROJECT/src-go/go.sum"
cp "$ROOT_DIR/src-go/internal/cli/root.go" "$PROJECT/src-go/internal/cli/root.go"
cp "$ROOT_DIR/src-rs/Cargo.toml" "$PROJECT/src-rs/Cargo.toml"
cp "$ROOT_DIR/src-rs/Cargo.lock" "$PROJECT/src-rs/Cargo.lock"
cp "$ROOT_DIR/src-rs/src/main.rs" "$PROJECT/src-rs/src/main.rs"

cat >"$FAKE_BIN/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = version ]; then
	printf '%s\n' 'go version go1.25.1 linux/amd64'
	exit 0
fi
if [ "${1:-}" = mod ]; then
	[ "${2:-}" != verify ] || printf '%s\n' 'all modules verified'
	exit 0
fi
if [ "${1:-}" = list ]; then
	exit 0
fi
output=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = -o ]; then
		output="$2"
		break
	fi
	shift
done
[ -n "$output" ]
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$output"
chmod +x "$output"
EOF

cat >"$FAKE_BIN/uv" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = --version ]; then
	printf '%s\n' 'uv 0.8.17'
fi
EOF

cat >"$FAKE_BIN/rustc" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' 'rustc 1.89.0'
EOF

cat >"$FAKE_BIN/cargo" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = --version ]; then
	printf '%s\n' 'cargo 1.89.0'
	exit 0
fi
if [ "${1:-}" = fetch ]; then
	exit 0
fi
mkdir -p target/release
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >target/release/cli-llm
chmod +x target/release/cli-llm
EOF
chmod +x "$FAKE_BIN"/*

run_installer() {
	local output="$1"
	shift
	printf '\n' | env -u CLI_LLM_GO -u CLI_LLM_PY -u CLI_LLM_RUST \
		HOME="$FAKE_HOME" XDG_CONFIG_HOME="$FAKE_HOME/config" \
		PATH="$FAKE_BIN:$PATH" "$@" "$PROJECT/install.sh" >"$output" 2>&1
}

assert_contains() {
	local file="$1"
	local expected="$2"
	if ! grep -Fq "$expected" "$file"; then
		printf 'ASSERT FAILED: expected %q in %s\n' "$expected" "$file" >&2
		cat "$file" >&2
		exit 1
	fi
}

assert_not_contains() {
	local file="$1"
	local unexpected="$2"
	if grep -Fq "$unexpected" "$file"; then
		printf 'ASSERT FAILED: did not expect %q in %s\n' "$unexpected" "$file" >&2
		cat "$file" >&2
		exit 1
	fi
}

assert_before() {
	local file="$1"
	local first="$2"
	local second="$3"
	local first_line second_line
	first_line=$(grep -Fn "$first" "$file" | head -n 1 | cut -d: -f1)
	second_line=$(grep -Fn "$second" "$file" | head -n 1 | cut -d: -f1)
	if [ -z "$first_line" ] || [ -z "$second_line" ] || [ "$first_line" -ge "$second_line" ]; then
		printf 'ASSERT FAILED: expected %q before %q in %s\n' "$first" "$second" "$file" >&2
		cat "$file" >&2
		exit 1
	fi
}

GO_OUTPUT="$TMP_ROOT/go.out"
run_installer "$GO_OUTPUT" env CLI_LLM_GO=1
assert_contains "$GO_OUTPUT" "cli-llm 0.4.4"
assert_contains "$GO_OUTPUT" "Target: Go (Eino)"
assert_contains "$GO_OUTPUT" "Toolchain: go version go1.25.1 linux/amd64"
assert_contains "$GO_OUTPUT" "$FAKE_BIN/go"
assert_contains "$GO_OUTPUT" "Go prerequisite check complete"
assert_contains "$GO_OUTPUT" "Step 1/2: Building ./cmd/llm"
assert_before "$GO_OUTPUT" "Checking prerequisites for Go" "Step 1/2: Building"
assert_not_contains "$GO_OUTPUT" "go mod download"
assert_not_contains "$GO_OUTPUT" "Python"
assert_not_contains "$GO_OUTPUT" "Rust"
assert_not_contains "$GO_OUTPUT" '\e['
assert_not_contains "$GO_OUTPUT" "— installer"

PY_OUTPUT="$TMP_ROOT/python.out"
run_installer "$PY_OUTPUT" env CLI_LLM_PY=yes
assert_contains "$PY_OUTPUT" "cli-llm 0.3.0"
assert_contains "$PY_OUTPUT" "Target: Python"
assert_contains "$PY_OUTPUT" "Toolchain: Python"
assert_contains "$PY_OUTPUT" "Package manager: uv 0.8.17"
assert_contains "$PY_OUTPUT" "Python prerequisite check complete"
assert_contains "$PY_OUTPUT" "Step 1/3: Installing the Python package"
assert_before "$PY_OUTPUT" "Checking prerequisites for Python" "Step 1/3: Installing"
assert_not_contains "$PY_OUTPUT" "dry run"
assert_not_contains "$PY_OUTPUT" "Target: Go"
assert_not_contains "$PY_OUTPUT" "Target: Rust"

RUST_OUTPUT="$TMP_ROOT/rust.out"
run_installer "$RUST_OUTPUT" env CLI_LLM_RUST=on
assert_contains "$RUST_OUTPUT" "cli-llm 0.1.0"
assert_contains "$RUST_OUTPUT" "Target: Rust"
assert_contains "$RUST_OUTPUT" "Toolchain: rustc 1.89.0"
assert_contains "$RUST_OUTPUT" "$FAKE_BIN/rustc"
assert_contains "$RUST_OUTPUT" "Build tool: cargo 1.89.0"
assert_contains "$RUST_OUTPUT" "$FAKE_BIN/cargo"
assert_contains "$RUST_OUTPUT" "Rust prerequisite check complete"
assert_contains "$RUST_OUTPUT" "Step 1/2: Building release binary"
assert_before "$RUST_OUTPUT" "Checking prerequisites for Rust" "Step 1/2: Building"
assert_not_contains "$RUST_OUTPUT" "cargo fetch"
assert_not_contains "$RUST_OUTPUT" "Target: Python"
assert_not_contains "$RUST_OUTPUT" "Target: Go"

DEFAULT_OUTPUT="$TMP_ROOT/default.out"
run_installer "$DEFAULT_OUTPUT" env
assert_contains "$DEFAULT_OUTPUT" "Target: Go (Eino)"

GO_DISABLED_OUTPUT="$TMP_ROOT/go-disabled.out"
run_installer "$GO_DISABLED_OUTPUT" env CLI_LLM_GO=n
assert_contains "$GO_DISABLED_OUTPUT" "Target: Python"

if printf '\n' | env HOME="$FAKE_HOME" PATH="$FAKE_BIN:$PATH" \
	CLI_LLM_GO=1 CLI_LLM_PY=1 "$PROJECT/install.sh" >"$TMP_ROOT/conflict.out" 2>&1; then
	printf '%s\n' 'ASSERT FAILED: conflicting targets should fail' >&2
	exit 1
fi
assert_contains "$TMP_ROOT/conflict.out" "mutually exclusive"

PRECHECK_HOME="$TMP_ROOT/precheck-home"
mv "$PROJECT/src-go/go.sum" "$PROJECT/src-go/go.sum.saved"
if printf '\n' | env -u CLI_LLM_PY -u CLI_LLM_RUST HOME="$PRECHECK_HOME" \
	PATH="$FAKE_BIN:$PATH" CLI_LLM_GO=1 "$PROJECT/install.sh" \
	>"$TMP_ROOT/precheck-failure.out" 2>&1; then
	printf '%s\n' 'ASSERT FAILED: missing prerequisites should fail' >&2
	exit 1
fi
assert_contains "$TMP_ROOT/precheck-failure.out" "Go module checksums are missing"
if [ -e "$PRECHECK_HOME/.local/bin/llm" ]; then
	printf '%s\n' 'ASSERT FAILED: prerequisite failure must not install a binary' >&2
	exit 1
fi
mv "$PROJECT/src-go/go.sum.saved" "$PROJECT/src-go/go.sum"

printf '%s\n' 'Root installer target tests passed'
