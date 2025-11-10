"""OpenAI provider abstraction."""

from __future__ import annotations

import openai
from dataclasses import dataclass, field
from typing import Any, Optional

from ..config import AppConfig


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

    def create_chat(self, **kwargs: Any) -> Any:
        """Proxy to chat completion creation."""
        return self.client().chat.completions.create(**kwargs)
