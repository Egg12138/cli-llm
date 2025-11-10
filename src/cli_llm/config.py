"""Application configuration and shared constants."""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, Optional

from logging.handlers import RotatingFileHandler


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


@dataclass(slots=True)
class AppConfig:
    """Runtime configuration resolved from environment or user input."""

    api_key: Optional[str]
    api_endpoint: Optional[str]
    default_model: Optional[str]
    app_title: str = "egg-cli-llm"
    app_url: str = "https://github.com/Egg12138/cli-llm"
    extra_headers: Dict[str, str] = field(
        default_factory=lambda: {
            "HTTP-Referer": "https://github.com/Egg12138/cli-llm",
            "X-Title": "egg-cli-llm",
        }
    )

    @classmethod
    def from_env(cls) -> "AppConfig":
        """Build configuration from environment variables."""
        api_key = os.getenv("OPENAI_API_KEY")
        api_endpoint = os.getenv("OPENAI_BASE_URL")
        default_model = os.getenv("OPENAI_MODEL")
        return cls(
            api_key=api_key,
            api_endpoint=api_endpoint,
            default_model=default_model,
        )

    def resolved_api_key(self) -> str:
        """Ensure an API key string is always available (some providers ignore it)."""
        if not self.api_key:
            logging.getLogger(__name__).warning("OPENAI_API_KEY is empty; using dummy key")
            return "dummy-key"
        return self.api_key


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
    "output_codes": " ".join(
        [
            colored("Output the code to the target file. Only one code block is supported.", TIPF),
            colored("IN PROGRESS", DESCF),
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
