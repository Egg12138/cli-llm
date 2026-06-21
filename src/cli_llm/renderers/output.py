"""Rendering utilities for streamed and non-streamed responses."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from rich.console import Console
from rich.markdown import Markdown

from ..config import AppConfig, TIPF, RSTF

_console = Console()


def highlight_code_blocks(content: str, session_type: str = "Context") -> str:
    """Legacy compat: returns content unchanged (rich handles formatting)."""
    return content


@dataclass(slots=True)
class ResponseRenderer:
    """Stateful renderer that tracks token accumulation for streamed responses."""

    app_config: AppConfig

    def process_streamed_chunk(self, response, count_tokens: bool = False) -> str:
        """Process a streamed response, returning the concatenated text."""
        full_content = ""
        for chunk in response:
            content = chunk.choices[0].delta.content if chunk.choices else None
            if not content:
                continue
            _console.out(content, end="")
            if count_tokens:
                full_content += content
        _console.out("\n")
        if full_content:
            _console.print(Markdown(full_content))
        return full_content

    def process_unstreamed_chunk(
        self,
        response,
        response_time: float,
        count_tokens: bool = False,
        extra_session_type: Optional[str] = None,
    ) -> str:
        """Process a non-streamed response and print timing info."""
        choice = response.choices[0]
        modelname = response.model
        finish_reason = choice.finish_reason
        finish_reason_map = {
            "stop": "Normal",
            "length": "Length exceeded max_tokens limit",
            "content_filter": "Content filter triggered",
            "insufficient_system_resource": "Insufficient system resources",
        }
        status = finish_reason_map.get(finish_reason, "Unknown")

        if extra_session_type == "Reasoning":
            _console.print(f"{TIPF}@ {modelname} reasoning ========================================={RSTF}")

        _console.print(Markdown(choice.message.content))
        _console.print(f"{TIPF}@ {modelname} [{status}] Response time: {response_time:.2f}s:{RSTF}")
        return choice.message.content
