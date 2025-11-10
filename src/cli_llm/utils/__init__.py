"""Miscellaneous utility helpers for cli-llm."""

from __future__ import annotations

from dataclasses import dataclass

RESET_SEQUENCE = "\033[0m"


@dataclass(slots=True)
class ColorCodes:
    """ANSI color codes for terminal output formatting."""

    red: str = "\033[91m"
    bold: str = "\033[1m"
    blue: str = "\033[94m"
    magenta: str = "\033[35m"
    reset: str = RESET_SEQUENCE
    bg_yellow: str = "\033[43m"

    def tip(self) -> str:
        return f"{self.blue}{self.bold}"

    def code(self) -> str:
        return f"{self.magenta}{self.bold}"

    def err(self) -> str:
        return f"{self.red}{self.bold}"

    def desc(self) -> str:
        return self.bg_yellow


CLRS = ColorCodes()
TIPF = CLRS.tip()
CODEF = CLRS.code()
ERRF = CLRS.err()
NOTF = CLRS.err()
DESCF = CLRS.desc()
RSTF = CLRS.reset


def colored(text: str, prefix: str, reset: str = RESET_SEQUENCE) -> str:
    """Return ``text`` wrapped in ANSI ``prefix`` and ``reset`` sequences."""

    return f"{prefix}{text}{reset}"


__all__ = [
    "ColorCodes",
    "CLRS",
    "TIPF",
    "CODEF",
    "ERRF",
    "DESCF",
    "RSTF",
    "NOTF",
    "colored",
    "RESET_SEQUENCE",
]
