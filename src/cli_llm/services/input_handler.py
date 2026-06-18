"""Input mode handlers — dispatches between three input strategies.

Modes
-----
prompt   : prompt_toolkit with raw‑mode, history, multi‑line (default)
editor   : opens $EDITOR (vi/nano/emacs …) for composing longer messages
stdin    : reads until Ctrl+D (EOF) — multi‑line by nature
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
from pathlib import Path

from prompt_toolkit import PromptSession
from prompt_toolkit.formatted_text import ANSI as ANSIFormattedText
from prompt_toolkit.history import FileHistory
from prompt_toolkit.key_binding import KeyBindings, KeyPressEvent
from prompt_toolkit.keys import Keys

CACHE_DIR = Path(os.environ.get("XDG_CACHE_HOME", Path.home() / ".cache")) / "cli-llm"
HISTORY_PATH = CACHE_DIR / "chat_history"

# ── key bindings (shared by prompt mode) ─────────────────────

_bindings = KeyBindings()


@_bindings.add(Keys.Enter)
def _submit(event: KeyPressEvent) -> None:
    """Submit the current buffer."""
    event.current_buffer.validate_and_handle()


@_bindings.add(Keys.Escape, Keys.Enter)
def _newline_meta(event: KeyPressEvent) -> None:
    """Insert newline (Meta+Enter / Esc+Enter)."""
    event.current_buffer.insert_text("\n")


# ── mode 1: prompt_toolkit (default) ─────────────────────────


def _read_promptkit(prompt_text: str) -> str:
    """Rich terminal input via prompt_toolkit (raw mode, history, line‑editing)."""
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    history = FileHistory(str(HISTORY_PATH))

    session: PromptSession = PromptSession(
        history=history,
        key_bindings=_bindings,
        multiline=False,  # Enter submits; Alt+Enter → newline
        enable_history_search=True,
    )

    return session.prompt(ANSIFormattedText(prompt_text))


# ── mode 2: external editor ──────────────────────────────────


def _read_editor(prompt_text: str) -> str:
    """Open $EDITOR and return the composed text."""
    editor = os.environ.get("EDITOR")
    if not editor:
        editor = os.environ.get("VISUAL")
    if not editor:
        # Reasonable fallback order
        for candidate in ("vim", "nvim", "nano", "emacs", "vi"):
            if _cmd_exists(candidate):
                editor = candidate
                break
    if not editor:
        print(
            "No EDITOR found — falling back to prompt mode. "
            "Set $EDITOR or $VISUAL to use editor mode.",
            file=sys.stderr,
        )
        return _read_promptkit(prompt_text)

    print(f"Opening $EDITOR ({editor}) …", file=sys.stderr)
    with tempfile.NamedTemporaryFile(
        mode="w+", suffix=".md", prefix="clillm_", delete=False
    ) as f:
        fname = f.name

    try:
        subprocess.check_call([editor, fname])
        with open(fname, encoding="utf-8") as f:
            content = f.read().strip()
    except subprocess.CalledProcessError:
        print("Editor exited abnormally — discarding input.", file=sys.stderr)
        content = ""
    finally:
        os.unlink(fname)

    return content


# ── mode 3: stdin (raw, EOF-terminated) ──────────────────────


def _read_stdin(prompt_text: str) -> str:
    """Read multi‑line input until EOF (Ctrl+D on most terminals).

    Shows the prompt on stderr so it doesn't get captured.
    """
    print(prompt_text, end="", flush=True, file=sys.stderr)
    lines: list[str] = []
    try:
        for line in sys.stdin:
            lines.append(line.rstrip("\n"))
    except KeyboardInterrupt:
        print(file=sys.stderr)
        raise
    print(file=sys.stderr)
    return "\n".join(lines)


# ── helpers ──────────────────────────────────────────────────


def _cmd_exists(name: str) -> bool:
    return (
        subprocess.run(["which", name], capture_output=True, check=False).returncode
        == 0
    )


# ── public dispatcher ────────────────────────────────────────


def read_input(
    prompt_text: str = "[Ask]:",
    mode: str = "prompt",
) -> str:
    """Read user input using the selected *mode*.

    Parameters
    ----------
    prompt_text :
        Label shown to the user (may contain ANSI escape codes).
    mode :
        One of ``"prompt"`` (default), ``"editor"``, or ``"stdin"``.

    Returns
    -------
    The input string, never ``None`` (returns ``""`` on empty input).
    """
    try:
        if mode == "editor":
            return _read_editor(prompt_text)
        if mode == "stdin":
            return _read_stdin(prompt_text)
        # default: prompt_toolkit
        return _read_promptkit(prompt_text)
    except KeyboardInterrupt:
        # Let the caller handle it uniformly
        raise
