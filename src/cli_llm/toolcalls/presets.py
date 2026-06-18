"""Preset tool definitions for tool-call mode."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, List, Optional


@dataclass(frozen=True, slots=True)
class ToolDefinition:
    """Model-visible tool schema plus prompt metadata."""

    name: str
    label: str
    description: str
    parameters: Dict[str, Any]
    prompt_snippet: Optional[str] = None
    prompt_guidelines: List[str] = field(default_factory=list)


DEFAULT_TOOL_NAMES = ["read", "grep", "find", "ls"]


def _object_schema(properties: Dict[str, Any], required: List[str]) -> Dict[str, Any]:
    return {
        "type": "object",
        "properties": properties,
        "required": required,
        "additionalProperties": False,
    }


PRESET_TOOLS: Dict[str, ToolDefinition] = {
    "read": ToolDefinition(
        name="read",
        label="read",
        description="Read the contents of a text file. Output is truncated for large files.",
        prompt_snippet="Read file contents",
        prompt_guidelines=["Use read to examine files instead of cat or sed."],
        parameters=_object_schema(
            {
                "path": {"type": "string", "description": "Path to the file to read."},
                "offset": {"type": "integer", "minimum": 0, "description": "Zero-based line offset."},
                "limit": {"type": "integer", "minimum": 1, "description": "Maximum number of lines to return."},
            },
            ["path"],
        ),
    ),
    "grep": ToolDefinition(
        name="grep",
        label="grep",
        description="Search file contents for a pattern. Returns matching paths and lines.",
        prompt_snippet="Search file contents for patterns",
        prompt_guidelines=["Use grep for content search."],
        parameters=_object_schema(
            {
                "pattern": {"type": "string", "description": "Text or regex pattern to search for."},
                "path": {"type": "string", "description": "Directory or file to search."},
                "glob": {"type": "string", "description": "Optional glob filter."},
                "ignore_case": {"type": "boolean", "description": "Search case-insensitively."},
            },
            ["pattern"],
        ),
    ),
    "find": ToolDefinition(
        name="find",
        label="find",
        description="Find files by glob pattern. Returns paths relative to the working directory.",
        prompt_snippet="Find files by glob pattern",
        prompt_guidelines=["Use find for filename or path discovery."],
        parameters=_object_schema(
            {
                "pattern": {"type": "string", "description": "Glob pattern to match."},
                "path": {"type": "string", "description": "Directory to search from."},
                "limit": {"type": "integer", "minimum": 1, "description": "Maximum number of paths to return."},
            },
            ["pattern"],
        ),
    ),
    "ls": ToolDefinition(
        name="ls",
        label="ls",
        description="List directory contents.",
        prompt_snippet="List directory contents",
        prompt_guidelines=[],
        parameters=_object_schema(
            {
                "path": {"type": "string", "description": "Directory to list."},
                "limit": {"type": "integer", "minimum": 1, "description": "Maximum number of entries to return."},
            },
            [],
        ),
    ),
    "bash": ToolDefinition(
        name="bash",
        label="bash",
        description="Execute a bash command in the current working directory. Returns sanitized stdout.",
        prompt_snippet="Execute bash commands",
        prompt_guidelines=["Use bash only when read, grep, find, or ls cannot answer the request."],
        parameters=_object_schema(
            {
                "command": {"type": "string", "description": "Bash command to execute."},
                "timeout": {"type": "integer", "minimum": 1, "description": "Timeout in seconds."},
            },
            ["command"],
        ),
    ),
}


def get_tool_definitions(names: Optional[Iterable[str]] = None) -> List[ToolDefinition]:
    selected = list(names) if names is not None else DEFAULT_TOOL_NAMES
    unknown = [name for name in selected if name not in PRESET_TOOLS]
    if unknown:
        raise ValueError(f"Unknown tool(s): {', '.join(unknown)}")
    return [PRESET_TOOLS[name] for name in selected]
