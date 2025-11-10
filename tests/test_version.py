"""Ensure runtime version matches pyproject metadata when available."""

from __future__ import annotations

import tomllib

from cli_llm import __version__


def test_version_matches_pyproject() -> None:
    with open("pyproject.toml", "rb") as handle:
        data = tomllib.load(handle)
    expected = data["project"]["version"]
    assert __version__ == expected
