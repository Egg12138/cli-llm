"""Single tool-call execution service."""

from __future__ import annotations

import fnmatch
import json
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Iterable, List, Protocol

from ..providers import ChatRequest
from .presets import ToolDefinition
from .system_prompt import build_toolcall_system_prompt

MAX_STDOUT_CHARS = 64 * 1024


class ToolCallError(RuntimeError):
    """Raised when a model tool call cannot be executed safely."""


@dataclass(frozen=True, slots=True)
class ToolCall:
    id: str
    name: str
    arguments: Dict[str, Any]


@dataclass(frozen=True, slots=True)
class ToolExecutionResult:
    tool: str
    arguments: Dict[str, Any]
    stdout: str
    exit_code: int


class ToolcallProvider(Protocol):
    def create_chat(self, request: ChatRequest) -> Any:
        ...


def to_openai_tools(tools: Iterable[ToolDefinition]) -> List[Dict[str, Any]]:
    return [
        {
            "type": "function",
            "function": {
                "name": tool.name,
                "description": tool.description,
                "parameters": tool.parameters,
                "strict": False,
            },
        }
        for tool in tools
    ]


def parse_tool_calls(response: Any) -> List[ToolCall]:
    try:
        raw_calls = response.choices[0].message.tool_calls
    except (AttributeError, IndexError, TypeError) as exc:
        raise ToolCallError("Provider response did not include tool calls.") from exc

    calls: List[ToolCall] = []
    for raw_call in raw_calls or []:
        function = raw_call.function
        try:
            arguments = json.loads(function.arguments or "{}")
        except json.JSONDecodeError as exc:
            raise ToolCallError(f"Invalid tool arguments for {function.name}: {exc}") from exc
        if not isinstance(arguments, dict):
            raise ToolCallError(f"Tool arguments for {function.name} must be a JSON object.")
        calls.append(ToolCall(id=str(raw_call.id), name=str(function.name), arguments=arguments))
    return calls


def validate_arguments(tool: ToolDefinition, arguments: Dict[str, Any]) -> Dict[str, Any]:
    schema = tool.parameters
    required = schema.get("required", [])
    properties = schema.get("properties", {})
    for key in required:
        if key not in arguments:
            raise ToolCallError(f"Missing required argument '{key}' for tool {tool.name}.")
    if schema.get("additionalProperties") is False:
        unknown = sorted(set(arguments) - set(properties))
        if unknown:
            raise ToolCallError(f"Unknown argument(s) for tool {tool.name}: {', '.join(unknown)}")
    for key, value in arguments.items():
        expected = properties.get(key, {}).get("type")
        if expected == "string" and not isinstance(value, str):
            raise ToolCallError(f"Argument '{key}' for tool {tool.name} must be a string.")
        if expected == "integer" and (not isinstance(value, int) or isinstance(value, bool)):
            raise ToolCallError(f"Argument '{key}' for tool {tool.name} must be an integer.")
        if expected == "boolean" and not isinstance(value, bool):
            raise ToolCallError(f"Argument '{key}' for tool {tool.name} must be a boolean.")
    return arguments


def safe_stdout(text: str, max_chars: int = MAX_STDOUT_CHARS) -> str:
    without_ansi = re.sub(r"\x1b\[[0-9;?]*[ -/]*[@-~]", "", text)
    cleaned = re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]", "", without_ansi)
    if len(cleaned) > max_chars:
        return cleaned[:max_chars] + "\n[truncated]\n"
    return cleaned


def _resolve_path(cwd: Path, value: str | None) -> Path:
    path = cwd if not value else cwd / value
    resolved = path.resolve()
    root = cwd.resolve()
    if root != resolved and root not in resolved.parents:
        raise ToolCallError("Tool path escapes the working directory.")
    return resolved


def execute_tool(tool: ToolDefinition, arguments: Dict[str, Any], cwd: Path) -> ToolExecutionResult:
    validated = validate_arguments(tool, arguments)
    if tool.name == "read":
        return _execute_read(tool, validated, cwd)
    if tool.name == "ls":
        return _execute_ls(tool, validated, cwd)
    if tool.name == "find":
        return _execute_find(tool, validated, cwd)
    if tool.name == "grep":
        return _execute_grep(tool, validated, cwd)
    if tool.name == "bash":
        return _execute_bash(tool, validated, cwd)
    raise ToolCallError(f"No executor for tool {tool.name}.")


def _execute_read(tool: ToolDefinition, arguments: Dict[str, Any], cwd: Path) -> ToolExecutionResult:
    path = _resolve_path(cwd, arguments["path"])
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines(keepends=True)
    offset = int(arguments.get("offset", 0))
    limit = arguments.get("limit")
    selected = lines[offset : offset + int(limit)] if limit else lines[offset:]
    return ToolExecutionResult(tool=tool.name, arguments=arguments, stdout=safe_stdout("".join(selected)), exit_code=0)


def _execute_ls(tool: ToolDefinition, arguments: Dict[str, Any], cwd: Path) -> ToolExecutionResult:
    path = _resolve_path(cwd, arguments.get("path"))
    limit = int(arguments.get("limit", 200))
    entries = []
    for child in sorted(path.iterdir(), key=lambda item: item.name):
        entries.append(child.name + ("/" if child.is_dir() else ""))
        if len(entries) >= limit:
            break
    return ToolExecutionResult(tool=tool.name, arguments=arguments, stdout=safe_stdout("\n".join(entries) + "\n"), exit_code=0)


def _execute_find(tool: ToolDefinition, arguments: Dict[str, Any], cwd: Path) -> ToolExecutionResult:
    root = _resolve_path(cwd, arguments.get("path"))
    pattern = arguments["pattern"]
    limit = int(arguments.get("limit", 200))
    matches = []
    for path in sorted(root.rglob("*")):
        rel = path.relative_to(cwd.resolve()).as_posix()
        if fnmatch.fnmatch(path.name, pattern) or fnmatch.fnmatch(rel, pattern):
            matches.append(rel + ("/" if path.is_dir() else ""))
            if len(matches) >= limit:
                break
    return ToolExecutionResult(tool=tool.name, arguments=arguments, stdout=safe_stdout("\n".join(matches) + "\n"), exit_code=0)


def _execute_grep(tool: ToolDefinition, arguments: Dict[str, Any], cwd: Path) -> ToolExecutionResult:
    root = _resolve_path(cwd, arguments.get("path"))
    pattern = arguments["pattern"]
    glob = arguments.get("glob")
    flags = re.IGNORECASE if arguments.get("ignore_case") else 0
    matcher = re.compile(pattern, flags)
    files = [root] if root.is_file() else [path for path in root.rglob("*") if path.is_file()]
    lines = []
    for path in sorted(files):
        rel = path.relative_to(cwd.resolve()).as_posix()
        if glob and not fnmatch.fnmatch(rel, glob):
            continue
        for index, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), start=1):
            if matcher.search(line):
                lines.append(f"{rel}:{index}:{line}")
    return ToolExecutionResult(tool=tool.name, arguments=arguments, stdout=safe_stdout("\n".join(lines) + "\n"), exit_code=0)


def _execute_bash(tool: ToolDefinition, arguments: Dict[str, Any], cwd: Path) -> ToolExecutionResult:
    timeout = int(arguments.get("timeout", 30))
    completed = subprocess.run(
        ["bash", "-lc", arguments["command"]],
        cwd=cwd,
        timeout=timeout,
        capture_output=True,
        text=True,
        check=False,
    )
    return ToolExecutionResult(
        tool=tool.name,
        arguments=arguments,
        stdout=safe_stdout(completed.stdout),
        exit_code=completed.returncode,
    )


@dataclass(slots=True)
class ToolcallService:
    provider: ToolcallProvider
    cwd: Path

    def run(self, *, prompt: str, model: str, tools: List[ToolDefinition]) -> ToolExecutionResult:
        system_prompt = build_toolcall_system_prompt(tools, cwd=self.cwd)
        request = ChatRequest(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": prompt},
            ],
            tools=to_openai_tools(tools),
            tool_choice="auto",
            stream=False,
        )
        response = self.provider.create_chat(request)
        calls = parse_tool_calls(response)
        if len(calls) != 1:
            raise ToolCallError(f"Expected exactly one tool call, got {len(calls)}.")
        call = calls[0]
        definitions = {tool.name: tool for tool in tools}
        tool = definitions.get(call.name)
        if tool is None:
            raise ToolCallError(f"Tool '{call.name}' is not enabled.")
        return execute_tool(tool, call.arguments, self.cwd)
