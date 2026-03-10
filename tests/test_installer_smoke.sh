#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
INSTALLER="${ROOT_DIR}/scripts/install.sh"

if ! command -v uv >/dev/null 2>&1; then
  echo "SKIP: uv is not installed"
  exit 0
fi

TMP_ROOT=$(mktemp -d)
cleanup() {
  rm -rf "${TMP_ROOT}"
}
trap cleanup EXIT

FAKE_HOME="${TMP_ROOT}/home"
mkdir -p "${FAKE_HOME}"

assert_exists() {
  local path="$1"
  if [[ ! -e "$path" && ! -L "$path" ]]; then
    echo "ASSERT FAILED: expected to exist: $path"
    exit 1
  fi
}

assert_not_exists() {
  local path="$1"
  if [[ -e "$path" || -L "$path" ]]; then
    echo "ASSERT FAILED: expected to be absent: $path"
    exit 1
  fi
}

echo "[1/4] install user mode"
HOME="${FAKE_HOME}" XDG_DATA_HOME="${FAKE_HOME}/.local/share" \
  "${INSTALLER}" --mode user --yes >/dev/null

assert_exists "${FAKE_HOME}/.local/bin/llm"
assert_exists "${FAKE_HOME}/.local/share/cli-llm/venv/bin/llm"
"${FAKE_HOME}/.local/bin/llm" --version >/dev/null

echo "[2/4] reinstall user mode (idempotent)"
HOME="${FAKE_HOME}" XDG_DATA_HOME="${FAKE_HOME}/.local/share" \
  "${INSTALLER}" --mode user --yes >/dev/null
assert_exists "${FAKE_HOME}/.local/bin/llm"

echo "[3/4] uninstall user mode"
HOME="${FAKE_HOME}" XDG_DATA_HOME="${FAKE_HOME}/.local/share" \
  "${INSTALLER}" --mode user --uninstall --yes >/dev/null
assert_not_exists "${FAKE_HOME}/.local/bin/llm"
assert_not_exists "${FAKE_HOME}/.local/share/cli-llm/venv"

echo "[4/4] install/uninstall venv mode"
VENV_PATH="${TMP_ROOT}/project-venv"
HOME="${FAKE_HOME}" "${INSTALLER}" --mode venv --venv-path "${VENV_PATH}" --yes >/dev/null
assert_exists "${VENV_PATH}/bin/llm"
"${VENV_PATH}/bin/llm" --version >/dev/null

HOME="${FAKE_HOME}" "${INSTALLER}" --mode venv --venv-path "${VENV_PATH}" --uninstall --yes >/dev/null
assert_not_exists "${VENV_PATH}"

echo "Installer smoke tests passed"
