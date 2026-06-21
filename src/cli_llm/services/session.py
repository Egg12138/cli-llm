"""Chat session orchestration and utilities."""

from __future__ import annotations

import logging
import os
import re
import signal
import sys
import time
from dataclasses import dataclass
from typing import Any, Optional

import tiktoken

from ..config import TIPF, RSTF, ERRF
from ..providers import ChatRequest, OpenAIProvider
from ..renderers import ResponseRenderer
from .. import prompts
from ..prompts import SYS_ROLES

LOGGER = logging.getLogger("cli_llm")


@dataclass(slots=True)
class TokenTracker:
    """Tracks input/output token usage for a chat session."""

    input_tokens: int = 0
    output_tokens: int = 0

    def add_input(self, count: int) -> None:
        self.input_tokens += count

    def add_output(self, count: int) -> None:
        self.output_tokens += count

    def display(self) -> None:
        total_tokens = self.input_tokens + self.output_tokens
        if total_tokens == 0:
            print(f"\n{TIPF}📊 No tokens counted yet. Use --count-tokens to enable token counting.{RSTF}")
            return
        print(f"\n{TIPF}📊 Token Usage:{RSTF}")
        print(f"  Input tokens: {self.input_tokens:,}")
        print(f"  Output tokens: {self.output_tokens:,}")
        print(f"  Total tokens: {total_tokens:,}")
        estimated_cost = (self.input_tokens * 0.00001) + (self.output_tokens * 0.00003)
        print(f"  Estimated cost: ~${estimated_cost:.4f}")


def sanitize_input(input_str: str) -> str:
    """Clean special characters from the input string."""
    sanitized_str = re.sub(r"[\x00-\x1F\x7F-\x9F\uD800-\uDFFF]", "", input_str)
    LOGGER.debug("Input cleaned: %s...", input_str[:50])
    return sanitized_str


def sigint_handler(sig: int, frame: Any) -> None:
    """Handle SIGINT signal gracefully."""
    print(f"\n{TIPF}SIGINT received. Exiting...{RSTF}")
    sys.exit(0)


def ensure_url_parser_ok() -> None:
    """Set up URL parsing related environment variables and signal handling."""
    signal.signal(signal.SIGINT, sigint_handler)
    os.environ["NO_PROXY"] = "localhost"
    LOGGER.info("URL parser configured successfully")


class ChatService:
    """Orchestrates prompts, provider calls, and rendering."""

    def __init__(
        self,
        provider: OpenAIProvider,
        renderer: ResponseRenderer,
        token_tracker: TokenTracker,
    ) -> None:
        self.provider = provider
        self.renderer = renderer
        self.token_tracker = token_tracker

    def get_sys_role(self, role: str, fallback: str = "coder") -> prompts.SystemPrompt:
        if role not in SYS_ROLES:
            LOGGER.warning("role %s is not predefined, using %s", role, fallback)
            return SYS_ROLES[fallback]
        return SYS_ROLES[role]

    def count_tokens_in_messages(self, messages: list, model: str) -> int:
        encoding = self._encoding_for_model(model)
        total_tokens = 0

        for message in messages:
            if message.get("content"):
                total_tokens += len(encoding.encode(message["content"]))
            total_tokens += 4
            if message.get("name"):
                total_tokens += 1
        total_tokens += 2
        return total_tokens

    def count_tokens_in_text(self, text: str, model: str) -> int:
        encoding = self._encoding_for_model(model)
        return len(encoding.encode(text))

    def _encoding_for_model(self, model: str) -> tiktoken.Encoding:
        model_encoding_map = {
            "deepseek-coder": "cl100k_base",
            "deepseek-chat": "cl100k_base",
            "deepseek-reasoner": "cl100k_base",
            "gpt-4": "cl100k_base",
            "gpt-3.5-turbo": "cl100k_base",
            "gpt-4o": "cl100k_base",
            "gpt-4o-mini": "cl100k_base",
        }
        encoding_name = model_encoding_map.get(model, "cl100k_base")
        try:
            return tiktoken.get_encoding(encoding_name)
        except KeyError:  # pragma: no cover - defensive fallback
            return tiktoken.get_encoding("cl100k_base")

    def chat(
        self,
        prompt: str,
        no_stream: bool,
        model: str,
        role_name: str,
        count_tokens: bool = False,
        custom_temp: Optional[float] = None,
        json_output: bool = False,
        role_fallback: str = "coder",
        agents_context_text: str = "",
    ) -> None:
        role = self.get_sys_role(role_name, fallback=role_fallback)
        temperature = custom_temp if custom_temp is not None else role.temperature

        if json_output and "json" not in prompt.lower():
            prompt = f"{prompt}\n\nPlease respond in JSON format."

        messages = [
            {"role": "system", "content": role.content},
            {"role": "user", "content": prompt},
        ]

        if agents_context_text:
            messages[0]["content"] += (
                f"\n\n---\n# Project Context (AGENTS.md)\n---\n{agents_context_text}"
            )

        if count_tokens:
            token_count = self.count_tokens_in_messages(messages, model)
            self.token_tracker.add_input(token_count)
            LOGGER.info("📊 Input tokens: %s", token_count)

        LOGGER.info("🚀 Request to %s (%s)", model, "non-stream" if no_stream else "stream")
        response_format = None
        if json_output:
            response_format = {"type": "json_object"}

        start_time = time.time()
        try:
            if not no_stream:
                request = ChatRequest(
                    model=model,
                    messages=messages,
                    temperature=temperature,
                    response_format=response_format,
                    stream=True,
                )
                response = self.provider.create_chat(request)
                print(f"{TIPF} 💭Generating...{RSTF}")
                full_content = self.renderer.process_streamed_chunk(
                    response,
                    count_tokens=count_tokens,
                )
                if count_tokens and full_content:
                    model_name = getattr(response, "model", model)
                    output_token_count = self.count_tokens_in_text(full_content, model_name)
                    self.token_tracker.add_output(output_token_count)
                    LOGGER.info("📊 Streamed output tokens: %s", output_token_count)
            else:
                request = ChatRequest(
                    model=model,
                    messages=messages,
                    temperature=temperature,
                    response_format=response_format,
                    stream=False,
                )
                response = self.provider.create_chat(request)
                answer = self.renderer.process_unstreamed_chunk(
                    response,
                    time.time() - start_time,
                    count_tokens=count_tokens,
                    extra_session_type="Reasoning",
                )
                if count_tokens:
                    if hasattr(response, "usage") and response.usage:
                        output_tokens = response.usage.completion_tokens
                        self.token_tracker.add_output(output_tokens)
                        LOGGER.info("📊 Output tokens: %s", output_tokens)
                    else:
                        output_tokens = self.count_tokens_in_text(answer, response.model)
                        self.token_tracker.add_output(output_tokens)
                        LOGGER.info("📊 Estimated output tokens: %s", output_tokens)

            response_time = time.time() - start_time
            LOGGER.info("✅ Response completed in %.2fs", response_time)
            print(f"\n{TIPF}⏱️ Response time: {response_time:.2f}s{RSTF}")

        except Exception as exc:
            LOGGER.error("⚠️ Provider error: %s", exc, exc_info=True)
            print(f"{ERRF}❌ Error: {exc}{RSTF}")

    def display_tokens_if_any(self) -> None:
        if self.token_tracker.input_tokens or self.token_tracker.output_tokens:
            self.token_tracker.display()
