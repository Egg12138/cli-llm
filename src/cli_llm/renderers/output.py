"""Rendering utilities for streamed and non-streamed responses."""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Optional

from ..config import CODEF, RSTF, COLORS, TIPF
from ..config import AppConfig


def highlight_code_blocks(content: str, session_type: str = "Context") -> str:
    """Highlight code blocks and bold text with ANSI escape sequences."""

    content = re.sub(
        r"\*\*(.*?)\*\*",
        lambda m: f"{COLORS.bold}{COLORS.blue}{m.group(1)}{COLORS.reset}",
        content,
        flags=re.DOTALL,
    )

    def replacer(matcher):
        return f"{CODEF}{matcher.group(0)}{RSTF}"

    if session_type in ("Context", "Reasoning"):
        return re.sub(r"`(.*?)`", replacer, content, flags=re.DOTALL)
    return content


@dataclass(slots=True)
class ResponseRenderer:
    """Stateful renderer that tracks token accumulation for streamed responses."""

    app_config: AppConfig

    def process_streamed_chunk(self, response, count_tokens: bool = False) -> str:
        """Process a streamed response, returning the concatenated text."""
        in_code_block = False
        full_content = ""

        for chunk in response:
            content = chunk.choices[0].delta.content if chunk.choices else None
            if not content:
                continue
            if count_tokens:
                full_content += content

            content = re.sub(
                r"\*\*(.*?)\*\*",
                lambda m: f"{COLORS.bold}{COLORS.blue}{m.group(1)}{COLORS.reset}",
                content,
            )

            is_code = "`" in content
            if is_code and not in_code_block:
                in_code_block = True
                print(f"{CODEF}{content}", end="")
            elif is_code and in_code_block:
                in_code_block = False
                print(f"{CODEF}{content}{RSTF}", end="")
            elif not is_code and in_code_block:
                print(f"{CODEF}{content}", end="")
            else:
                print(f"{RSTF}{content}", end="")

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
            print(f"{TIPF}@ {modelname} reasoning ========================================={RSTF}")

        answer = highlight_code_blocks(choice.message.content, extra_session_type or "Context")
        print(answer)
        print(f"{TIPF}@ {modelname} [{status}] Response time: {response_time:.2f}s:{RSTF}")
        return choice.message.content
