"""Tool-call presets and prompt helpers."""

from .presets import DEFAULT_TOOL_NAMES, ToolDefinition, get_tool_definitions
from .system_prompt import build_toolcall_system_prompt

__all__ = [
    "DEFAULT_TOOL_NAMES",
    "ToolDefinition",
    "build_toolcall_system_prompt",
    "get_tool_definitions",
]
