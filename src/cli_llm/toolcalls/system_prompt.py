"""System prompt construction for tool-call mode."""

from __future__ import annotations

from datetime import date
from pathlib import Path
from typing import Iterable, Optional

from .presets import ToolDefinition


def build_toolcall_system_prompt(
    tools: Iterable[ToolDefinition],
    *,
    cwd: Path,
    current_date: Optional[str] = None,
) -> str:
    tool_list = list(tools)
    visible_tools = [
        f"- {tool.name}: {tool.prompt_snippet}"
        for tool in tool_list
        if tool.prompt_snippet
    ]
    available_tools = "\n".join(visible_tools) if visible_tools else "(none)"

    guidelines: list[str] = []
    seen: set[str] = set()

    def add_guideline(text: str) -> None:
        normalized = text.strip()
        if normalized and normalized not in seen:
            seen.add(normalized)
            guidelines.append(normalized)

    for tool in tool_list:
        for guideline in tool.prompt_guidelines:
            add_guideline(guideline)

    add_guideline("Be concise.")
    add_guideline("Show file paths clearly when working with files.")

    guideline_text = "\n".join(f"- {line}" for line in guidelines)
    prompt_date = current_date or date.today().isoformat()
    prompt_cwd = str(cwd).replace("\\", "/")

    return f"""You are an expert CLI assistant operating inside cli-llm, a lightweight tool-call harness.
Use the available tools when they are the safest and most direct way to answer the user.

Available tools:
{available_tools}

Tool call rules:
- Call at most one tool.
- Use only the provided tools.
- Do not invent tool names or arguments.
- Prefer read/grep/find/ls over bash for file inspection.
- If no tool is needed, answer normally without a tool call.
- Do not explain the tool call in prose when calling a tool.

Guidelines:
{guideline_text}

Current date: {prompt_date}
Current working directory: {prompt_cwd}"""
