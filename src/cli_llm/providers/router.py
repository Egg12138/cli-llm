"""Thin provider dispatch."""

from __future__ import annotations

from dataclasses import dataclass

from ..config import AppConfig
from .openai_provider import OpenAIProvider


@dataclass(slots=True)
class ProviderRouter:
    """Resolve the active provider adapter from runtime config."""

    config: AppConfig

    def resolve(self) -> OpenAIProvider:
        return OpenAIProvider(self.config)
