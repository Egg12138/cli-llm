"""Tool-call presets and prompt helpers."""

from .presets import DEFAULT_TOOL_NAMES, ToolDefinition, get_tool_definitions
from .service import (
    ToolCall,
    ToolCallError,
    ToolcallService,
    ToolExecutionResult,
    execute_tool,
    parse_tool_calls,
    parse_streaming_tool_calls,
    safe_stdout,
    to_openai_tools,
    validate_arguments,
)
from .system_prompt import build_toolcall_system_prompt

__all__ = [
    "DEFAULT_TOOL_NAMES",
    "ToolCall",
    "ToolCallError",
    "ToolDefinition",
    "ToolExecutionResult",
    "ToolcallService",
    "build_toolcall_system_prompt",
    "execute_tool",
    "get_tool_definitions",
    "parse_tool_calls",
    "parse_streaming_tool_calls",
    "safe_stdout",
    "to_openai_tools",
    "validate_arguments",
]
