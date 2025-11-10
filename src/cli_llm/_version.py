"""Version helper that stays in sync with pyproject metadata."""

from __future__ import annotations

from importlib import metadata
from pathlib import Path
from typing import Optional

try:  # pragma: no cover - stdlib first
    import tomllib
except ModuleNotFoundError:  # pragma: no cover - py39/py310
    import tomli as tomllib  # type: ignore

PACKAGE_NAME = "cli-llm"


def _find_pyproject() -> Optional[Path]:
    current = Path(__file__).resolve()
    for parent in [current.parent] + list(current.parents):
        candidate = parent / "pyproject.toml"
        if candidate.exists():
            return candidate
    return None


def _read_pyproject_version(pyproject: Path) -> Optional[str]:
    try:
        with pyproject.open("rb") as handle:
            data = tomllib.load(handle)
        version = data.get("project", {}).get("version")
        return str(version) if version is not None else None
    except (OSError, tomllib.TOMLDecodeError):  # pragma: no cover - defensive
        return None


def _resolve_version() -> str:
    pyproject = _find_pyproject()
    if pyproject:
        version = _read_pyproject_version(pyproject)
        if version:
            return version

    try:
        return metadata.version(PACKAGE_NAME)
    except metadata.PackageNotFoundError:
        return "0.0.0"


__version__ = _resolve_version()
