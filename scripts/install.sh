#!/usr/bin/env bash
set -euo pipefail

PROGRAM_NAME="cli-llm"
BINARY_NAME="llm"

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(cd -- "${SCRIPT_DIR}/.." && pwd)

UV_BIN=${UV_BIN:-uv}
MODE="user"
MODE_SET=0
ASSUME_YES=0
UNINSTALL=0
VENV_PATH=""
BINDIR=""

USER_DATA_HOME=${XDG_DATA_HOME:-"${HOME}/.local/share"}
USER_VENV_DEFAULT=${CLI_LLM_INSTALL_USER_VENV_DEFAULT:-"${USER_DATA_HOME}/cli-llm/venv"}
SYSTEM_VENV_DEFAULT=${CLI_LLM_INSTALL_SYSTEM_VENV_DEFAULT:-"/opt/cli-llm/venv"}
USER_BINDIR_DEFAULT=${CLI_LLM_INSTALL_USER_BINDIR_DEFAULT:-"${HOME}/.local/bin"}
SYSTEM_BINDIR_DEFAULT=${CLI_LLM_INSTALL_SYSTEM_BINDIR_DEFAULT:-"/usr/local/bin"}

log() {
  printf '%s\n' "$*"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<USAGE
Usage: ./scripts/install.sh [options]

Interactive by default. Supports installation to:
  1. uv venv scope (default path: <PWD>/.venv)
  2. user PATH (default path: ${USER_BINDIR_DEFAULT}) [default mode]
  3. system PATH (default path: ${SYSTEM_BINDIR_DEFAULT})

Options:
  --mode <user|venv|system>   Installation mode (default: user)
  --venv-path <path>          Override venv path for selected mode
  --bindir <path>             Override binary directory for user/system mode
  --yes, -y                   Non-interactive; accept defaults/flags
  --uninstall                 Remove installation for selected mode
  --help, -h                  Show this help

Examples:
  ./scripts/install.sh
  ./scripts/install.sh --mode venv --venv-path .venv --yes
  ./scripts/install.sh --mode user --bindir ~/.local/bin --yes
  ./scripts/install.sh --mode system --yes
  ./scripts/install.sh --mode user --uninstall --yes

Quick LLM configuration:
  1) Env vars (fastest):
       export OPENAI_API_KEY="sk-..."
       export OPENAI_MODEL="gpt-4o-mini"
  2) Config file (recommended for persistent setup):
       ~/.cli-llm/config.toml
       [defaults]
       provider = "openai"
       model = "gpt-4o-mini"
       role = "coder"

Important env vars:
  OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL,
  CLI_LLM_PROVIDER, CLI_LLM_DEFAULT_ROLE
USAGE
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

expand_path() {
  local raw="$1"
  if [[ "$raw" == ~* ]]; then
    printf '%s\n' "${raw/#\~/${HOME}}"
  else
    printf '%s\n' "$raw"
  fi
}

can_write_path() {
  local path="$1"
  local dir
  dir=$(dirname "$path")

  if [[ -e "$path" ]]; then
    [[ -w "$path" ]]
    return
  fi

  while [[ ! -d "$dir" && "$dir" != "/" ]]; do
    dir=$(dirname "$dir")
  done

  [[ -w "$dir" ]]
}

run_as_root_if_needed() {
  local requires_root="$1"
  shift

  if [[ "$requires_root" == "1" && "$(id -u)" -ne 0 ]]; then
    sudo env "PATH=${PATH}" "$@"
  else
    "$@"
  fi
}

prompt_yes_no() {
  local prompt="$1"
  local default_answer="$2"
  local reply

  if [[ "$ASSUME_YES" -eq 1 ]]; then
    return 0
  fi

  while true; do
    if [[ "$default_answer" == "y" ]]; then
      read -r -p "${prompt} [Y/n]: " reply || true
      reply=${reply:-Y}
    else
      read -r -p "${prompt} [y/N]: " reply || true
      reply=${reply:-N}
    fi

    case "$reply" in
      y|Y|yes|YES)
        return 0
        ;;
      n|N|no|NO)
        return 1
        ;;
      *)
        log "Please enter y or n."
        ;;
    esac
  done
}

resolve_defaults() {
  case "$MODE" in
    venv)
      if [[ -z "$VENV_PATH" ]]; then
        VENV_PATH="${PROJECT_ROOT}/.venv"
      fi
      ;;
    user)
      if [[ -z "$VENV_PATH" ]]; then
        VENV_PATH="$USER_VENV_DEFAULT"
      fi
      if [[ -z "$BINDIR" ]]; then
        BINDIR="$USER_BINDIR_DEFAULT"
      fi
      ;;
    system)
      if [[ -z "$VENV_PATH" ]]; then
        VENV_PATH="$SYSTEM_VENV_DEFAULT"
      fi
      if [[ -z "$BINDIR" ]]; then
        BINDIR="$SYSTEM_BINDIR_DEFAULT"
      fi
      ;;
    *)
      die "Unsupported mode: $MODE"
      ;;
  esac

  VENV_PATH=$(expand_path "$VENV_PATH")
  if [[ -n "$BINDIR" ]]; then
    BINDIR=$(expand_path "$BINDIR")
  fi
}

interactive_collect() {
  if [[ "$ASSUME_YES" -eq 1 || ! -t 0 ]]; then
    return
  fi

  if [[ "$MODE_SET" -eq 0 ]]; then
    log "Select installation mode:"
    log "  1) user   (default, install for current user)"
    log "  2) venv   (install into project/local venv)"
    log "  3) system (install for all users)"
    read -r -p "Mode [1/2/3, default 1]: " selection || true
    selection=${selection:-1}
    case "$selection" in
      1) MODE="user" ;;
      2) MODE="venv" ;;
      3) MODE="system" ;;
      *) die "Invalid mode selection: $selection" ;;
    esac
  fi

  resolve_defaults

  case "$MODE" in
    venv)
      read -r -p "Venv path [${VENV_PATH}]: " reply || true
      if [[ -n "${reply}" ]]; then
        VENV_PATH=$(expand_path "$reply")
      fi
      ;;
    user|system)
      read -r -p "Venv path [${VENV_PATH}]: " venv_reply || true
      if [[ -n "${venv_reply}" ]]; then
        VENV_PATH=$(expand_path "$venv_reply")
      fi
      read -r -p "Binary directory [${BINDIR}]: " bindir_reply || true
      if [[ -n "${bindir_reply}" ]]; then
        BINDIR=$(expand_path "$bindir_reply")
      fi
      ;;
  esac
}

ensure_dependencies() {
  command_exists "$UV_BIN" || die "uv is required. Install it first: https://docs.astral.sh/uv/getting-started/installation/"
}

create_venv_and_install() {
  local requires_root="$1"
  local venv_path="$2"

  run_as_root_if_needed "$requires_root" mkdir -p "$venv_path"
  run_as_root_if_needed "$requires_root" "$UV_BIN" venv "$venv_path"
  run_as_root_if_needed "$requires_root" "$UV_BIN" pip install --python "${venv_path}/bin/python" "$PROJECT_ROOT"
}

ensure_symlink() {
  local requires_root="$1"
  local source_bin="$2"
  local link_path="$3"

  run_as_root_if_needed "$requires_root" mkdir -p "$(dirname "$link_path")"

  if [[ -e "$link_path" || -L "$link_path" ]]; then
    if [[ "$ASSUME_YES" -eq 0 ]]; then
      if ! prompt_yes_no "Replace existing ${link_path}?" "y"; then
        log "Skipped replacing ${link_path}."
        return
      fi
    fi
    run_as_root_if_needed "$requires_root" rm -f "$link_path"
  fi

  run_as_root_if_needed "$requires_root" ln -s "$source_bin" "$link_path"
}

verify_binary() {
  local requires_root="$1"
  local bin_path="$2"

  if [[ "$requires_root" == "1" && "$(id -u)" -ne 0 ]]; then
    sudo "$bin_path" --version || warn "Installed, but could not run version check via sudo for ${bin_path}"
  else
    "$bin_path" --version || warn "Installed, but version check failed for ${bin_path}"
  fi
}

print_path_hint_if_needed() {
  local bindir="$1"
  case ":${PATH}:" in
    *":${bindir}:"*)
      ;;
    *)
      warn "${bindir} is not in PATH for this shell."
      log "Add this to your shell profile:"
      log "  export PATH=\"${bindir}:\$PATH\""
      ;;
  esac
}

print_config_quick_guide() {
  cat <<'GUIDE'

LLM configuration quick guide:
1) Fast start (env vars):
   export OPENAI_API_KEY="sk-..."
   export OPENAI_MODEL="gpt-4o-mini"

2) Persistent setup (~/.cli-llm/config.toml):
   [defaults]
   provider = "openai"
   model = "gpt-4o-mini"
   role = "coder"

Optional env vars:
  OPENAI_BASE_URL, CLI_LLM_PROVIDER, CLI_LLM_DEFAULT_ROLE

Verify:
  llm --help
  llm providers
GUIDE
}

install_mode_venv() {
  local requires_root="0"

  create_venv_and_install "$requires_root" "$VENV_PATH"

  log "Installed ${PROGRAM_NAME} in venv: ${VENV_PATH}"
  log "Use it with: ${VENV_PATH}/bin/${BINARY_NAME} --help"
  log "Or activate: source ${VENV_PATH}/bin/activate"
  print_config_quick_guide
}

install_mode_user_or_system() {
  local requires_root="$1"
  local link_path="${BINDIR}/${BINARY_NAME}"

  create_venv_and_install "$requires_root" "$VENV_PATH"
  ensure_symlink "$requires_root" "${VENV_PATH}/bin/${BINARY_NAME}" "$link_path"
  verify_binary "$requires_root" "$link_path"

  log "Installed ${PROGRAM_NAME} executable: ${link_path}"
  if [[ "$requires_root" == "0" ]]; then
    print_path_hint_if_needed "$BINDIR"
  fi
  print_config_quick_guide
}

remove_path_if_exists() {
  local requires_root="$1"
  local path="$2"

  if [[ -e "$path" || -L "$path" ]]; then
    run_as_root_if_needed "$requires_root" rm -rf "$path"
    log "Removed: ${path}"
  else
    log "Not present: ${path}"
  fi
}

uninstall_mode() {
  case "$MODE" in
    venv)
      remove_path_if_exists "0" "$VENV_PATH"
      ;;
    user)
      remove_path_if_exists "0" "${BINDIR}/${BINARY_NAME}"
      remove_path_if_exists "0" "$VENV_PATH"
      ;;
    system)
      remove_path_if_exists "1" "${BINDIR}/${BINARY_NAME}"
      remove_path_if_exists "1" "$VENV_PATH"
      ;;
    *)
      die "Unsupported mode for uninstall: $MODE"
      ;;
  esac
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --mode)
        [[ $# -ge 2 ]] || die "Missing value for --mode"
        MODE="$2"
        MODE_SET=1
        shift 2
        ;;
      --venv-path)
        [[ $# -ge 2 ]] || die "Missing value for --venv-path"
        VENV_PATH="$2"
        shift 2
        ;;
      --bindir)
        [[ $# -ge 2 ]] || die "Missing value for --bindir"
        BINDIR="$2"
        shift 2
        ;;
      --yes|-y)
        ASSUME_YES=1
        shift
        ;;
      --uninstall)
        UNINSTALL=1
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
  done
}

summarize_plan() {
  log ""
  if [[ "$UNINSTALL" -eq 1 ]]; then
    log "Planned action: uninstall"
  else
    log "Planned action: install"
  fi
  log "Mode: ${MODE}"
  log "Project root: ${PROJECT_ROOT}"
  log "Venv path: ${VENV_PATH}"
  if [[ "$MODE" != "venv" ]]; then
    log "Binary directory: ${BINDIR}"
  fi
  log ""
}

main() {
  parse_args "$@"

  ensure_dependencies

  if [[ "$MODE_SET" -eq 1 ]]; then
    case "$MODE" in
      user|venv|system) ;;
      *) die "--mode must be one of: user, venv, system" ;;
    esac
  fi

  resolve_defaults
  interactive_collect

  summarize_plan

  if [[ "$ASSUME_YES" -eq 0 ]]; then
    if [[ "$UNINSTALL" -eq 1 ]]; then
      prompt_yes_no "Proceed with uninstall?" "n" || exit 0
    else
      prompt_yes_no "Proceed with install?" "y" || exit 0
    fi
  fi

  if [[ "$UNINSTALL" -eq 1 ]]; then
    uninstall_mode
    log "Uninstall complete."
    return
  fi

  case "$MODE" in
    venv)
      install_mode_venv
      ;;
    user)
      install_mode_user_or_system "0"
      ;;
    system)
      install_mode_user_or_system "1"
      ;;
    *)
      die "Unsupported mode: $MODE"
      ;;
  esac

  log "Install complete."
}

main "$@"
