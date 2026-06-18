"""Tests for provider dispatch wiring."""

from __future__ import annotations

from cli_llm.config import AppConfig
from cli_llm.providers import OpenAIProvider, ProviderRouter


def test_provider_router_returns_openai_compatible_provider_for_profiles() -> None:
    config = AppConfig(provider="deepseek", api_endpoint="https://api.deepseek.com/v1")

    provider = ProviderRouter(config).resolve()

    assert isinstance(provider, OpenAIProvider)
    assert provider.config is config
