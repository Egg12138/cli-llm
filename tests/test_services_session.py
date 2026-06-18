"""Service-layer tests covering chat orchestration and helpers."""

from __future__ import annotations

from types import SimpleNamespace

from cli_llm.config import AppConfig
from cli_llm.prompts import SYS_ROLES, SystemPrompt
from cli_llm.services import ChatService, TokenTracker, ensure_url_parser_ok, sanitize_input
from cli_llm.services import session as session_module


class DummyProvider:
    """Minimal provider stub capturing create() invocations."""

    def __init__(self, response: SimpleNamespace | None = None) -> None:
        self.config = AppConfig()
        self._response = response or SimpleNamespace(model="dummy-model")
        self.last_request = None
        self.last_kwargs = None

    def client(self):
        provider = self

        class _Client:
            def __init__(self) -> None:
                self.chat = SimpleNamespace(
                    completions=SimpleNamespace(create=self._create),
                )

            def _create(self, **kwargs):
                provider.last_kwargs = kwargs
                return provider._response

        return _Client()

    def create_chat(self, request):
        self.last_request = request
        self.last_kwargs = request.to_openai_params(self.config.extra_headers)
        return self._response


class DummyRenderer:
    """Renderer stub that records streamed/unstreamed invocations."""

    def __init__(self) -> None:
        self.stream_calls = []
        self.unstream_calls = []

    def process_streamed_chunk(self, response, count_tokens: bool = False) -> str:
        self.stream_calls.append((response, count_tokens))
        return "streamed-output"

    def process_unstreamed_chunk(
        self,
        response,
        response_time: float,
        count_tokens: bool = False,
        extra_session_type: str | None = None,
    ) -> str:
        self.unstream_calls.append((response, response_time, count_tokens, extra_session_type))
        return "final-answer"


def test_token_tracker_display_zero_tokens_message(capsys) -> None:
    tracker = TokenTracker()
    tracker.display()

    captured = capsys.readouterr().out
    assert "No tokens counted yet" in captured


def test_token_tracker_display_totals(capsys) -> None:
    tracker = TokenTracker()
    tracker.add_input(10)
    tracker.add_output(5)
    tracker.display()

    captured = capsys.readouterr().out
    assert "Input tokens: 10" in captured
    assert "Output tokens: 5" in captured
    assert "Total tokens: 15" in captured


def test_sanitize_input_drops_control_characters() -> None:
    dirty = "safe\x00text\x1bwith\u007fcontrols\ud800"
    assert sanitize_input(dirty) == "safetextwithcontrols"


def test_ensure_url_parser_ok_sets_signal_and_env(monkeypatch) -> None:
    recorded: dict[str, object] = {}

    def fake_signal(sig, handler):
        recorded["sig"] = sig
        recorded["handler"] = handler

    env: dict[str, str] = {}
    monkeypatch.setattr(session_module.signal, "signal", fake_signal)
    monkeypatch.setattr(session_module.os, "environ", env)

    ensure_url_parser_ok()

    assert env["NO_PROXY"] == "localhost"
    assert recorded["sig"] == session_module.signal.SIGINT
    assert recorded["handler"] == session_module.sigint_handler


def test_chat_service_get_sys_role_falls_back_to_default() -> None:
    provider = DummyProvider()
    renderer = DummyRenderer()
    tracker = TokenTracker()
    service = ChatService(provider, renderer, tracker)

    default_name = SystemPrompt.default_role()
    role = service.get_sys_role("does-not-exist")
    assert role is SYS_ROLES[default_name]


def test_chat_service_stream_counts_tokens_and_sets_json(monkeypatch) -> None:
    response = SimpleNamespace(model="gpt-4o-mini")
    provider = DummyProvider(response=response)
    renderer = DummyRenderer()
    tracker = TokenTracker()
    service = ChatService(provider, renderer, tracker)

    def fake_count_messages(self, messages, model):
        return 11

    def fake_count_text(self, text, model):
        return 4

    monkeypatch.setattr(ChatService, "count_tokens_in_messages", fake_count_messages)
    monkeypatch.setattr(ChatService, "count_tokens_in_text", fake_count_text)

    service.chat(
        prompt="Tell me something helpful.",
        no_stream=False,
        model="gpt-4o-mini",
        role_name="coder",
        count_tokens=True,
        custom_temp=None,
        json_output=True,
    )

    assert tracker.input_tokens == 11
    assert tracker.output_tokens == 4
    assert renderer.stream_calls and renderer.stream_calls[0][1] is True
    assert provider.last_kwargs["stream"] is True
    assert provider.last_kwargs["response_format"] == {"type": "json_object"}
    appended_prompt = provider.last_kwargs["messages"][1]["content"]
    assert "Please respond in JSON format." in appended_prompt


def test_chat_service_non_stream_uses_usage_counts(monkeypatch) -> None:
    response = SimpleNamespace(
        model="gpt-4o-mini",
        usage=SimpleNamespace(completion_tokens=12),
        choices=[
            SimpleNamespace(
                finish_reason="stop",
                message=SimpleNamespace(content="answer"),
            )
        ],
    )
    provider = DummyProvider(response=response)
    renderer = DummyRenderer()
    tracker = TokenTracker()
    service = ChatService(provider, renderer, tracker)

    def fake_count_messages(self, messages, model):
        return 5

    monkeypatch.setattr(ChatService, "count_tokens_in_messages", fake_count_messages)

    service.chat(
        prompt="Already JSON aware prompt mentioning json output.",
        no_stream=True,
        model="gpt-4o-mini",
        role_name="coder",
        count_tokens=True,
        custom_temp=0.5,
        json_output=False,
    )

    assert tracker.input_tokens == 5
    assert tracker.output_tokens == 12
    assert renderer.unstream_calls
    assert "response_format" not in provider.last_kwargs
    assert "Please respond in JSON format." not in provider.last_kwargs["messages"][1]["content"]
    assert "stream" not in provider.last_kwargs


def test_count_token_helpers_use_tiktoken(monkeypatch) -> None:
    token_map = {
        "sys": 3,
        "usr": 5,
        "final": 7,
    }

    class DummyEncoding:
        def encode(self, text: str):
            return [0] * token_map[text]

    monkeypatch.setattr(session_module.tiktoken, "get_encoding", lambda name: DummyEncoding())

    service = ChatService(DummyProvider(), DummyRenderer(), TokenTracker())
    messages = [
        {"role": "system", "content": "sys"},
        {"role": "user", "content": "usr"},
    ]
    assert service.count_tokens_in_messages(messages, "gpt-4o-mini") == (3 + 4) + (5 + 4) + 2
    assert service.count_tokens_in_text("final", "gpt-4o-mini") == 7
