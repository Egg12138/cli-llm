"""Tests for ChatService provider dispatch."""

from __future__ import annotations

from types import SimpleNamespace

from cli_llm.config import AppConfig
from cli_llm.services import ChatService, TokenTracker


class RoutedProvider:
    def __init__(self) -> None:
        self.config = AppConfig()
        self.last_request = None

    def client(self):
        raise AssertionError("ChatService must use create_chat instead of SDK client access")

    def create_chat(self, request):
        self.last_request = request
        return SimpleNamespace(
            model="routed-model",
            choices=[
                SimpleNamespace(
                    finish_reason="stop",
                    message=SimpleNamespace(content="answer"),
                )
            ],
        )


class RecordingRenderer:
    def __init__(self) -> None:
        self.unstream_calls = []

    def process_unstreamed_chunk(
        self,
        response,
        response_time: float,
        count_tokens: bool = False,
        extra_session_type: str | None = None,
    ) -> str:
        self.unstream_calls.append((response, response_time, count_tokens, extra_session_type))
        return "answer"

    def process_streamed_chunk(self, response, count_tokens: bool = False) -> str:
        raise AssertionError("not used by this test")


def test_chat_service_uses_provider_create_chat_interface() -> None:
    provider = RoutedProvider()
    renderer = RecordingRenderer()
    service = ChatService(provider, renderer, TokenTracker())

    service.chat(
        prompt="Use routed provider.",
        no_stream=True,
        model="routed-model",
        role_name="coder",
        count_tokens=False,
        custom_temp=0.2,
        json_output=False,
    )

    assert provider.last_request is not None
    assert provider.last_request.model == "routed-model"
    assert provider.last_request.stream is False
    assert provider.last_request.temperature == 0.2
    assert renderer.unstream_calls
