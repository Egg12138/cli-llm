"""Shared provider request types."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Dict, List, Optional


@dataclass(slots=True)
class ChatRequest:
    """Provider-neutral chat completion request."""

    model: str
    messages: List[Dict[str, Any]]
    temperature: Optional[float] = None
    response_format: Optional[Dict[str, Any]] = None
    stream: bool = False

    def to_openai_params(self, extra_headers: Dict[str, str]) -> Dict[str, Any]:
        params: Dict[str, Any] = {
            "model": self.model,
            "messages": self.messages,
            "temperature": self.temperature,
            "extra_headers": extra_headers,
        }
        if self.response_format is not None:
            params["response_format"] = self.response_format
        if self.stream:
            params["stream"] = True
        return params
