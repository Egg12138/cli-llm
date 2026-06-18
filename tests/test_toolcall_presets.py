"""Tests for preset tool definitions and toolcall prompt construction."""

from __future__ import annotations

from click.testing import CliRunner

from cli_llm.cli import cli
from cli_llm.toolcalls import build_toolcall_system_prompt, get_tool_definitions


def test_default_tool_definitions_exclude_bash() -> None:
    tools = get_tool_definitions()

    assert [tool.name for tool in tools] == ["read", "grep", "find", "ls"]


def test_tool_definitions_include_bash_only_when_requested() -> None:
    tools = get_tool_definitions(["read", "bash"])

    assert [tool.name for tool in tools] == ["read", "bash"]
    bash = tools[1]
    assert bash.parameters["properties"]["command"]["type"] == "string"


def test_toolcall_system_prompt_injects_tool_rules_without_full_schema(tmp_path) -> None:
    prompt = build_toolcall_system_prompt(get_tool_definitions(), cwd=tmp_path, current_date="2026-06-18")

    assert "Available tools:" in prompt
    assert "- read: Read file contents" in prompt
    assert "Tool call rules:" in prompt
    assert "- Call at most one tool." in prompt
    assert "Use read to examine files instead of cat or sed." in prompt
    assert '"properties"' not in prompt
    assert f"Current working directory: {tmp_path}" in prompt


def test_toolcall_list_tools_defaults_to_safe_read_only_set() -> None:
    result = CliRunner().invoke(cli, ["toolcall", "--list-tools"])

    assert result.exit_code == 0
    assert "read" in result.output
    assert "grep" in result.output
    assert "find" in result.output
    assert "ls" in result.output
    assert "bash" not in result.output
