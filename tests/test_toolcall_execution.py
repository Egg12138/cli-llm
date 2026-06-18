"""Tests for single tool-call execution and safe stdout."""

from __future__ import annotations

from types import SimpleNamespace

import pytest

from cli_llm.config import AppConfig
from cli_llm.toolcalls import ToolCallError, ToolcallService, get_tool_definitions, parse_streaming_tool_calls


class ToolCallProvider:
    def __init__(self, tool_name: str, arguments: str) -> None:
        self.config = AppConfig()
        self.last_request = None
        self._response = SimpleNamespace(
            choices=[
                SimpleNamespace(
                    message=SimpleNamespace(
                        tool_calls=[
                            SimpleNamespace(
                                id="call_1",
                                function=SimpleNamespace(name=tool_name, arguments=arguments),
                            )
                        ]
                    )
                )
            ]
        )

    def create_chat(self, request):
        self.last_request = request
        return self._response


def test_toolcall_service_executes_read_and_sanitizes_stdout(tmp_path) -> None:
    note = tmp_path / "note.txt"
    note.write_text("hello \x1b[31mred\x1b[0m\n", encoding="utf-8")
    provider = ToolCallProvider("read", '{"path": "note.txt"}')
    service = ToolcallService(provider=provider, cwd=tmp_path)

    result = service.run(
        prompt="Read note.txt",
        model="test-model",
        tools=get_tool_definitions(["read"]),
    )

    assert provider.last_request is not None
    assert provider.last_request.stream is False
    assert provider.last_request.tools[0]["function"]["name"] == "read"
    assert result.tool == "read"
    assert result.stdout == "hello red\n"
    assert result.exit_code == 0


def test_toolcall_service_rejects_disabled_bash(tmp_path) -> None:
    provider = ToolCallProvider("bash", '{"command": "echo unsafe"}')
    service = ToolcallService(provider=provider, cwd=tmp_path)

    with pytest.raises(ToolCallError, match="not enabled"):
        service.run(
            prompt="Run a command",
            model="test-model",
            tools=get_tool_definitions(),
        )


def test_toolcall_service_executes_bash_when_explicitly_enabled(tmp_path) -> None:
    provider = ToolCallProvider("bash", '{"command": "printf enabled"}')
    service = ToolcallService(provider=provider, cwd=tmp_path)

    result = service.run(
        prompt="Run a command",
        model="test-model",
        tools=get_tool_definitions(["bash"]),
    )

    assert result.tool == "bash"
    assert result.stdout == "enabled"
    assert result.exit_code == 0


def test_parse_streaming_tool_calls_accumulates_arguments_by_index() -> None:
    chunks = [
        SimpleNamespace(
            choices=[
                SimpleNamespace(
                    delta=SimpleNamespace(
                        tool_calls=[
                            SimpleNamespace(
                                index=0,
                                id="call_1",
                                function=SimpleNamespace(name="read", arguments='{"pa'),
                            )
                        ]
                    )
                )
            ]
        ),
        SimpleNamespace(
            choices=[
                SimpleNamespace(
                    delta=SimpleNamespace(
                        tool_calls=[
                            SimpleNamespace(
                                index=0,
                                id=None,
                                function=SimpleNamespace(name=None, arguments='th":"README.md"}'),
                            )
                        ]
                    )
                )
            ]
        ),
    ]

    calls = parse_streaming_tool_calls(chunks)

    assert len(calls) == 1
    assert calls[0].id == "call_1"
    assert calls[0].name == "read"
    assert calls[0].arguments == {"path": "README.md"}
