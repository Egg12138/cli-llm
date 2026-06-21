"""Tests for the --agents-context / -A flag."""

from __future__ import annotations

import os
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from click.testing import CliRunner

from cli_llm.cli import cli, _AGENTS_MAX_BYTES
from cli_llm.services import sanitize_input


def test_agents_context_flag_exists() -> None:
    """The -A / --agents-context flag is registered on the chat command."""
    runner = CliRunner()
    result = runner.invoke(cli, ["chat", "--help"])
    assert result.exit_code == 0
    assert "--agents-context" in result.output
    assert "-A" in result.output


def test_missing_agents_md_warns_and_continues(tmp_path) -> None:
    """When AGENTS.md is absent, a warning is printed to stderr and chat proceeds."""
    runner = CliRunner()
    with patch("cli_llm.cli.os.getcwd", return_value=str(tmp_path)), \
         patch("cli_llm.cli._run_chat") as mock_run:
        result = runner.invoke(cli, ["chat", "-A", "hello"])
    # _run_chat is called with agents_context=True
    mock_run.assert_called_once()
    assert mock_run.call_args.kwargs["agents_context"] is True


def test_missing_file_warns_stderr(tmp_path, capsys) -> None:
    """_run_chat with agents_context=True and no AGENTS.md prints warning to stderr."""
    from cli_llm.cli import _run_chat
    from cli_llm.config import ConfigLoader

    app_config = ConfigLoader().load()

    with patch("cli_llm.cli.os.getcwd", return_value=str(tmp_path)), \
         patch("cli_llm.cli.ChatService") as MockCS, \
         patch("cli_llm.cli.ProviderRouter"), \
         patch("cli_llm.cli.ResponseRenderer"), \
         patch("cli_llm.cli.ensure_url_parser_ok"), \
         patch("cli_llm.cli.select.select", return_value=([], [], [])):
        mock_service = MockCS.return_value
        mock_service.chat.return_value = None
        mock_service.display_tokens_if_any.return_value = None

        _run_chat(
            app_config=app_config,
            prompt="test",
            no_stream=True,
            role=None,
            temp=None,
            json_output=False,
            output_codes=None,
            debug=False,
            localtest=False,
            count_tokens=False,
            input_mode="prompt",
            agents_context=True,
        )

    captured = capsys.readouterr()
    assert "AGENTS.md not found" in captured.err
    mock_service.chat.assert_called_once()
    assert mock_service.chat.call_args.kwargs["agents_context_text"] == ""


def test_large_file_truncation(tmp_path, capsys) -> None:
    """Files larger than 16 KB are truncated and a warning is emitted."""
    agents_file = tmp_path / "AGENTS.md"
    content = "A" * (_AGENTS_MAX_BYTES + 1000)
    agents_file.write_text(content)

    from cli_llm.cli import _run_chat
    from cli_llm.config import ConfigLoader

    app_config = ConfigLoader().load()

    with patch("cli_llm.cli.os.getcwd", return_value=str(tmp_path)), \
         patch("cli_llm.cli.ChatService") as MockCS, \
         patch("cli_llm.cli.ProviderRouter"), \
         patch("cli_llm.cli.ResponseRenderer"), \
         patch("cli_llm.cli.ensure_url_parser_ok"), \
         patch("cli_llm.cli.select.select", return_value=([], [], [])):
        mock_service = MockCS.return_value
        mock_service.chat.return_value = None
        mock_service.display_tokens_if_any.return_value = None

        _run_chat(
            app_config=app_config,
            prompt="test",
            no_stream=True,
            role=None,
            temp=None,
            json_output=False,
            output_codes=None,
            debug=False,
            localtest=False,
            count_tokens=False,
            input_mode="prompt",
            agents_context=True,
        )

    captured = capsys.readouterr()
    assert "truncating" in captured.err.lower()
    passed_text = mock_service.chat.call_args.kwargs["agents_context_text"]
    assert len(passed_text) <= _AGENTS_MAX_BYTES


def test_sanitization_applied(tmp_path, capsys) -> None:
    """Control characters in AGENTS.md are stripped by sanitize_input."""
    agents_file = tmp_path / "AGENTS.md"
    agents_file.write_text("clean\x00dirty\x1btext")

    from cli_llm.cli import _run_chat
    from cli_llm.config import ConfigLoader

    app_config = ConfigLoader().load()

    with patch("cli_llm.cli.os.getcwd", return_value=str(tmp_path)), \
         patch("cli_llm.cli.ChatService") as MockCS, \
         patch("cli_llm.cli.ProviderRouter"), \
         patch("cli_llm.cli.ResponseRenderer"), \
         patch("cli_llm.cli.ensure_url_parser_ok"), \
         patch("cli_llm.cli.select.select", return_value=([], [], [])):
        mock_service = MockCS.return_value
        mock_service.chat.return_value = None
        mock_service.display_tokens_if_any.return_value = None

        _run_chat(
            app_config=app_config,
            prompt="test",
            no_stream=True,
            role=None,
            temp=None,
            json_output=False,
            output_codes=None,
            debug=False,
            localtest=False,
            count_tokens=False,
            input_mode="prompt",
            agents_context=True,
        )

    passed_text = mock_service.chat.call_args.kwargs["agents_context_text"]
    assert "\x00" not in passed_text
    assert "\x1b" not in passed_text
    assert "cleandirtytext" == passed_text


def test_agents_context_appended_to_system_message() -> None:
    """agents_context_text is appended to the system message, not user message."""
    from cli_llm.services.session import ChatService, TokenTracker

    class StubProvider:
        def __init__(self):
            self.config = SimpleNamespace(api_endpoint="http://test", extra_headers={})
            self.last_kwargs = None

        def create_chat(self, request):
            self.last_kwargs = request.to_openai_params({})
            return SimpleNamespace(model="test")

    class StubRenderer:
        def process_streamed_chunk(self, response, count_tokens=False):
            return "ok"

    provider = StubProvider()
    renderer = StubRenderer()
    tracker = TokenTracker()
    service = ChatService(provider, renderer, tracker)

    service.chat(
        prompt="hello",
        no_stream=False,
        model="test-model",
        role_name="coder",
        agents_context_text="# My Project Context",
    )

    messages = provider.last_kwargs["messages"]
    sys_msg = messages[0]["content"]
    user_msg = messages[1]["content"]
    assert "# Project Context (AGENTS.md)" in sys_msg
    assert "# My Project Context" in sys_msg
    assert "AGENTS.md" not in user_msg
