"""Tests for the layered configuration loader."""

from __future__ import annotations

from cli_llm import config as config_module
from cli_llm.config import ConfigLoader, DEFAULT_CONFIG_VALUES


def _clear_config_env(monkeypatch) -> None:
    for key in (
        "OPENAI_API_KEY",
        "OPENAI_BASE_URL",
        "OPENAI_MODEL",
        "CLI_LLM_DEFAULT_ROLE",
        "CLI_LLM_PROVIDER",
    ):
        monkeypatch.delenv(key, raising=False)


def test_defaults_used_when_no_sources(tmp_path, monkeypatch) -> None:
    _clear_config_env(monkeypatch)
    loader = ConfigLoader(user_config_path=tmp_path / "missing.toml")
    config = loader.load(environment={})

    assert config.default_model == DEFAULT_CONFIG_VALUES["default_model"]
    assert config.default_role == DEFAULT_CONFIG_VALUES["default_role"]


def test_user_file_overrides_defaults(tmp_path, monkeypatch) -> None:
    _clear_config_env(monkeypatch)
    config_path = tmp_path / "config.toml"
    config_path.write_text(
        """
[defaults]
provider = "deepseek"
model = "file-model"
role = "creator"

[providers.deepseek]
api_endpoint = "https://file.example/v1"
api_key = "file-key"
models = ["file-model", "deepseek-chat"]
""",
        encoding="utf-8",
    )

    loader = ConfigLoader(user_config_path=config_path)
    config = loader.load(environment={})

    assert config.provider == "deepseek"
    assert config.api_endpoint == "https://file.example/v1"
    assert config.api_key == "file-key"
    assert config.default_model == "file-model"
    assert config.default_role == "creator"
    assert config.providers["deepseek"]["models"] == ["file-model", "deepseek-chat"]


def test_environment_overrides_file_and_defaults(tmp_path) -> None:
    config_path = tmp_path / "config.toml"
    config_path.write_text(
        """
[defaults]
provider = "deepseek"
model = "file-model"

[providers.deepseek]
api_endpoint = "https://file.example/v1"
""",
        encoding="utf-8",
    )

    loader = ConfigLoader(user_config_path=config_path)
    env = {
        "OPENAI_MODEL": "env-model",
        "OPENAI_BASE_URL": "https://env.example/v1",
        "CLI_LLM_PROVIDER": "deepseek",
    }
    config = loader.load(environment=env)

    assert config.default_model == "env-model"
    assert config.api_endpoint == "https://env.example/v1"
    assert config.provider == "deepseek"


def test_cli_overrides_take_highest_priority(tmp_path) -> None:
    loader = ConfigLoader(user_config_path=tmp_path / "missing.toml")
    env = {"OPENAI_MODEL": "env-model", "CLI_LLM_PROVIDER": "deepseek"}
    config = loader.load(
        environment=env,
        cli_overrides={"default_model": "cli-model", "provider": "openai"},
    )

    assert config.default_model == "cli-model"
    assert config.provider == "openai"


def test_provider_defaults_used_when_section_missing(tmp_path, monkeypatch) -> None:
    _clear_config_env(monkeypatch)
    loader = ConfigLoader(user_config_path=tmp_path / "missing.toml")
    config = loader.load(environment={}, cli_overrides={})

    assert config.provider == DEFAULT_CONFIG_VALUES["provider"]
    assert config.providers == {}


def test_loader_reads_legacy_config_path(tmp_path, monkeypatch) -> None:
    """ConfigLoader with user_config_path pointing to legacy directory reads values."""
    _clear_config_env(monkeypatch)
    legacy_dir = tmp_path / ".cli_llm"
    legacy_dir.mkdir(parents=True, exist_ok=True)
    config_file = legacy_dir / "config.toml"
    config_file.write_text(
        """
[provider]
api_key = "legacy-key"
api_endpoint = "https://legacy.example/v1"

[defaults]
provider = "openai"
model = "legacy-model"
""",
        encoding="utf-8",
    )

    loader = ConfigLoader(user_config_path=config_file)
    config = loader.load(environment={})

    assert config.api_key == "legacy-key"
    assert config.api_endpoint == "https://legacy.example/v1"
    assert config.default_model == "legacy-model"
