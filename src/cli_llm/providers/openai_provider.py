"""OpenAI provider abstraction."""

from __future__ import annotations

import openai
from dataclasses import dataclass, field
from typing import Optional

from ..config import AppConfig
from .types import ChatRequest


class ProviderError(RuntimeError):
    """Raised when provider initialisation fails."""


@dataclass(slots=True)
class OpenAIProvider:
    """Lazy OpenAI client wrapper."""

    config: AppConfig
    _client: Optional[openai.OpenAI] = field(default=None, init=False, repr=False)

    def client(self) -> openai.OpenAI:
        if self._client is None:
            api_key = self.config.resolved_api_key()
            try:
                self._client = openai.OpenAI(api_key=api_key, base_url=self.config.api_endpoint)
            except Exception as exc:  # pragma: no cover - defensive
                raise ProviderError(str(exc)) from exc
        return self._client

    def create_chat(self, request: ChatRequest) -> Any:
        """Create a chat completion through an OpenAI-compatible endpoint."""
        return self.client().chat.completions.create(
            **request.to_openai_params(self.config.extra_headers)
        )
