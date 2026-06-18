"""CLI subcommand tests for provider introspection."""

from __future__ import annotations

import textwrap
from pathlib import Path

import pytest
from click.testing import CliRunner

from cli_llm.cli import CONFIG_LOADER, cli, _provider_records
from cli_llm.config import AppConfig


def _clear_config_env(monkeypatch) -> None:
    for key in (
        "OPENAI_API_KEY",
        "OPENAI_BASE_URL",
        "OPENAI_MODEL",
        "CLI_LLM_DEFAULT_ROLE",
        "CLI_LLM_PROVIDER",
    ):
        monkeypatch.delenv(key, raising=False)


def _providers_command_name() -> str:
    if "providers" in cli.commands:
        return "providers"
    if "inspect" in cli.commands:
        return "inspect"
    raise AssertionError("No providers/inspect command registered")


@pytest.fixture()
def config_writer(tmp_path, monkeypatch):
    _clear_config_env(monkeypatch)
    original_path = CONFIG_LOADER.user_config_path
    original_user_values = CONFIG_LOADER._cached_user_values
    original_provider_profiles = CONFIG_LOADER._cached_provider_profiles
    original_mtime = CONFIG_LOADER._cached_mtime
    original_cached_path = CONFIG_LOADER._cached_path

    def _write(contents: str) -> Path:
        config_path = tmp_path / "config.toml"
        config_path.write_text(textwrap.dedent(contents))
        CONFIG_LOADER.user_config_path = config_path
        CONFIG_LOADER._cached_user_values = {}
        CONFIG_LOADER._cached_provider_profiles = {}
        CONFIG_LOADER._cached_mtime = None
        return config_path

    yield _write

    CONFIG_LOADER.user_config_path = original_path
    CONFIG_LOADER._cached_user_values = original_user_values
    CONFIG_LOADER._cached_provider_profiles = original_provider_profiles
    CONFIG_LOADER._cached_mtime = original_mtime
    CONFIG_LOADER._cached_path = original_cached_path


def test_providers_command_lists_profiles(config_writer) -> None:
    config_writer(
        """
        [defaults]
        provider = "deepseek"
        model = "deepseek-chat"

        [providers.deepseek]
        api_endpoint = "https://api.deepseek.com/v1"
        models = ["deepseek-chat", "deepseek-coder"]
        """
    )

    runner = CliRunner()
    result = runner.invoke(cli, [_providers_command_name()])

    assert result.exit_code == 0
    assert "deepseek" in result.output
    assert ("api_endpoint" in result.output) or ("endpoint" in result.output)


def test_provider_models_defaults_to_active_provider(config_writer) -> None:
    config_writer(
        """
        [defaults]
        provider = "openai"
        model = "gpt-4o-mini"

        [providers.openai]
        models = ["gpt-4o-mini", "gpt-4o"]
        """
    )

    runner = CliRunner()
    result = runner.invoke(cli, ["provider", "models"])

    assert result.exit_code == 0
    assert "Models for 'openai'" in result.output
    assert "gpt-4o-mini" in result.output


def test_provider_records_marks_active_profile() -> None:
    config = AppConfig(
        api_key="secret",
        api_endpoint="https://api.example/v1",
        default_model="primary-model",
        provider="openai",
        providers={"openai": {"models": ["legacy-model"]}},
    )

    records = _provider_records(config)

    assert records["openai"]["source"] == "active"
    assert "primary-model" in records["openai"]["models"]
    assert records["openai"]["has_api_key"] is True
