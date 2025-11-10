"""Application configuration and shared constants."""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Dict, Mapping, Optional

from logging.handlers import RotatingFileHandler

try:  # pragma: no cover - fallback for Python < 3.11
    import tomllib
except ModuleNotFoundError:  # pragma: no cover - fallback
    import tomli as tomllib  # type: ignore


@dataclass(slots=True)
class ColorCodes:
    """ANSI color codes for terminal output formatting."""

    red: str = "\033[91m"
    bold: str = "\033[1m"
    blue: str = "\033[94m"
    magenta: str = "\033[35m"
    reset: str = "\033[0m"
    bg_yellow: str = "\033[43m"

    def tip(self) -> str:
        return f"{self.blue}{self.bold}"

    def code(self) -> str:
        return f"{self.magenta}{self.bold}"

    def err(self) -> str:
        return f"{self.red}{self.bold}"

    def desc(self) -> str:
        return self.bg_yellow


COLORS = ColorCodes()

DEFAULT_CONFIG_VALUES = {
    "api_endpoint": "https://api.openai.com/v1",
    "default_model": "deepseek-chat",
    "default_role": "coder",
    "provider": "openai",
}

DEFAULT_CONFIG_PATH = Path.home() / ".cli-llm" / "config.toml"


@dataclass(slots=True)
class AppConfig:
    """Runtime configuration resolved from layered sources."""

    api_key: Optional[str] = None
    api_endpoint: str = DEFAULT_CONFIG_VALUES["api_endpoint"]
    default_model: str = DEFAULT_CONFIG_VALUES["default_model"]
    default_role: str = DEFAULT_CONFIG_VALUES["default_role"]
    provider: str = DEFAULT_CONFIG_VALUES["provider"]
    providers: Dict[str, Dict[str, Any]] = field(default_factory=dict)
    app_title: str = "egg-cli-llm"
    app_url: str = "https://github.com/Egg12138/cli-llm"
    extra_headers: Dict[str, str] = field(
        default_factory=lambda: {
            "HTTP-Referer": "https://github.com/Egg12138/cli-llm",
            "X-Title": "egg-cli-llm",
        }
    )

    def resolved_api_key(self) -> str:
        """Ensure an API key string is always available (some providers ignore it)."""
        if not self.api_key:
            logging.getLogger(__name__).warning("OPENAI_API_KEY is empty; using dummy key")
            return "dummy-key"
        return self.api_key


class ConfigLoader:
    """Load configuration with CLI > env > user file > defaults precedence."""

    env_key_map = {
        "OPENAI_API_KEY": "api_key",
        "OPENAI_BASE_URL": "api_endpoint",
        "OPENAI_MODEL": "default_model",
        "CLI_LLM_DEFAULT_ROLE": "default_role",
        "CLI_LLM_PROVIDER": "provider",
    }

    def __init__(
        self,
        user_config_path: Path = DEFAULT_CONFIG_PATH,
        repo_defaults: Optional[Mapping[str, Any]] = None,
    ) -> None:
        self.user_config_path = user_config_path
        self.repo_defaults = dict(DEFAULT_CONFIG_VALUES)
        if repo_defaults:
            self.repo_defaults.update({k: v for k, v in repo_defaults.items() if v is not None})
        self._cached_user_values: Dict[str, Any] = {}
        self._cached_provider_profiles: Dict[str, Dict[str, Any]] = {}
        self._cached_mtime: Optional[float] = None

    def load(
        self,
        cli_overrides: Optional[Mapping[str, Any]] = None,
        environment: Optional[Mapping[str, str]] = None,
    ) -> AppConfig:
        user_values, provider_profiles = self._user_file_values()
        merged: Dict[str, Any] = {}
        merged.update(user_values)
        merged.update(self._env_values(environment))
        if cli_overrides:
            merged.update({k: v for k, v in cli_overrides.items() if v is not None})

        provider_name = merged.get("provider", self.repo_defaults["provider"])
        provider_config = provider_profiles.get(provider_name, {})

        # TODO: ensure api_endpoint ~ model coupled
        resolved_api_endpoint = (
            merged.get("api_endpoint")
            or provider_config.get("api_endpoint")
            or self.repo_defaults["api_endpoint"]
        )
        resolved_model = (
            merged.get("default_model")
            or provider_config.get("default_model")
            or self.repo_defaults["default_model"]
        )
        return AppConfig(
            api_key=merged.get("api_key") or provider_config.get("api_key"),
            api_endpoint=resolved_api_endpoint,
            default_model=resolved_model,
            default_role=merged.get("default_role", self.repo_defaults["default_role"]),
            provider=provider_name,
            providers=provider_profiles,
        )

    def _env_values(self, environment: Optional[Mapping[str, str]]) -> Dict[str, Any]:
        source = environment or os.environ
        resolved: Dict[str, Any] = {}
        for env_key, config_key in self.env_key_map.items():
            value = source.get(env_key)
            if value:
                resolved[config_key] = value
        return resolved

    def _user_file_values(self) -> tuple[Dict[str, Any], Dict[str, Dict[str, Any]]]:
        path = self.user_config_path
        if not path.exists():
            self._cached_user_values = {}
            self._cached_provider_profiles = {}
            self._cached_mtime = None
            return {}, {}

        mtime = path.stat().st_mtime
        if self._cached_mtime == mtime and self._cached_user_values:
            return self._cached_user_values, self._cached_provider_profiles

        try:
            with path.open("rb") as handle:
                data = tomllib.load(handle)
        except (OSError, tomllib.TOMLDecodeError):
            self._cached_user_values = {}
            self._cached_provider_profiles = {}
            self._cached_mtime = mtime
            return {}, {}

        extracted = self._extract_supported_fields(data)
        provider_profiles = self._extract_provider_profiles(data)
        self._cached_user_values = extracted
        self._cached_provider_profiles = provider_profiles
        self._cached_mtime = mtime
        return extracted, provider_profiles

    def _extract_supported_fields(self, data: Mapping[str, Any]) -> Dict[str, Any]:
        extracted: Dict[str, Any] = {}
        provider = data.get("provider", {})
        defaults = data.get("defaults", {})

        def assign(source: Mapping[str, Any], source_key: str, target_key: str) -> None:
            value = source.get(source_key)
            if value is not None:
                extracted[target_key] = value

        assign(provider, "api_key", "api_key")
        assign(provider, "api_endpoint", "api_endpoint")
        assign(defaults, "model", "default_model")
        assign(defaults, "role", "default_role")
        assign(defaults, "provider", "provider")

        for key in ("api_key", "api_endpoint", "default_model", "default_role", "provider"):
            if key not in extracted and key in data and data[key] is not None:
                extracted[key] = data[key]

        return extracted

    def _extract_provider_profiles(self, data: Mapping[str, Any]) -> Dict[str, Dict[str, Any]]:
        providers_section = data.get("providers")
        if not isinstance(providers_section, Mapping):
            return {}

        profiles: Dict[str, Dict[str, Any]] = {}
        for name, raw_config in providers_section.items():
            if not isinstance(raw_config, Mapping):
                continue
            profile: Dict[str, Any] = {}
            for key in ("api_key", "api_endpoint", "default_model"):
                value = raw_config.get(key)
                if value is not None:
                    profile[key] = value
            models = raw_config.get("models")
            if isinstance(models, list):
                profile["models"] = [str(model) for model in models]
            profiles[str(name)] = profile
        return profiles


TIPF = COLORS.tip()
CODEF = COLORS.code()
ERRF = COLORS.err()
DESCF = COLORS.desc()
RSTF = COLORS.reset


def colored(text: str, prefix: str) -> str:
    """Wrap a string with the provided color prefix and reset sequence."""
    return f"{prefix}{text}{RSTF}"

HELP_TEXTS = {
    "prompt": colored("Input prompt for deepseek.", TIPF),
    "no_stream": " ".join(
        [
            colored("Disable streaming of the response (wait for full result).", TIPF),
            colored("<role>-R does not support stream!", ERRF),
        ]
    ),
    "model": colored("Choose the model to use. Default is $OPENAI_MODEL.", TIPF),
    "provider": colored("Select which provider profile to use. Default comes from config.", TIPF),
    "output_codes": " ".join(
        [
            colored("Output the code to the target file. Support only one code block.", TIPF),
            colored("IN PROGRESS", ERRF),
        ]
    ),
    "debug": colored("Enable debug mode to show detailed logs.", TIPF),
    "test": colored("Do local tests (internal helpers).", TIPF),
    "role": colored(
        "Choose the role to use. Default is coder. Append '-R' to use the reasoner, e.g. 'coder-R'.",
        TIPF,
    ),
    "count_tokens": colored("Enable token counting and show usage statistics.", TIPF),
    "temp": colored("Customize temperature for the model. Defaults to the selected role.", TIPF),
    "json_output": colored("Enable JSON output format (model responds with valid JSON).", TIPF),
}


def setup_logging(
    log_file: str = "/var/log/cli_llm.log",
    max_bytes: int = 1024 * 1024,
    backup_count: int = 5,
) -> logging.Logger:
    """Prepare a rotating file logger plus console handler when interactive."""
    path = Path(log_file)
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.touch(exist_ok=True)
    except PermissionError:
        log_dir = Path.home() / ".cli_llm" / "logs"
        log_dir.mkdir(parents=True, exist_ok=True)
        path = log_dir / "cli_llm.log"

    file_handler = RotatingFileHandler(
        path,
        maxBytes=max_bytes,
        backupCount=backup_count,
    )
    formatter = logging.Formatter("[%(levelname)s] - %(message)s")
    file_handler.setFormatter(formatter)

    logger = logging.getLogger("cli_llm")
    logger.setLevel(logging.INFO)
    logger.handlers.clear()
    logger.addHandler(file_handler)

    if os.isatty(1):
        console_handler = logging.StreamHandler()
        console_handler.setFormatter(formatter)
        logger.addHandler(console_handler)

    return logger
