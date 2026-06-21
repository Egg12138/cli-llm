"""Renderer-focused tests."""

from __future__ import annotations

from types import SimpleNamespace

from cli_llm.config import AppConfig
from cli_llm.renderers import ResponseRenderer, highlight_code_blocks


def _chunk(content: str) -> SimpleNamespace:
    return SimpleNamespace(
        choices=[SimpleNamespace(delta=SimpleNamespace(content=content))],
    )


def test_highlight_code_blocks_styles_bold_and_code() -> None:
    text = "Explain **bold** example with `code` please."
    result = highlight_code_blocks(text, session_type="Context")
    assert result == text


def test_highlight_code_blocks_skip_code_for_other_session() -> None:
    text = "Plain **bold** with `code` render."
    result = highlight_code_blocks(text, session_type="raw")
    assert result == text


def test_process_streamed_chunk_returns_concatenated_text() -> None:
    renderer = ResponseRenderer(AppConfig())
    response = [
        _chunk("`print('"),
        _chunk("value')`"),
        _chunk(" done"),
    ]

    content = renderer.process_streamed_chunk(response, count_tokens=True)
    assert content == "`print('value')` done"


def test_process_unstreamed_chunk_formats_output(capsys) -> None:
    renderer = ResponseRenderer(AppConfig())
    response = SimpleNamespace(
        model="gpt-4o-mini",
        choices=[
            SimpleNamespace(
                finish_reason="length",
                message=SimpleNamespace(content="**Bold** with `code`."),
            )
        ],
    )

    text = renderer.process_unstreamed_chunk(
        response,
        response_time=1.23,
        count_tokens=False,
        extra_session_type="Reasoning",
    )

    assert text == "**Bold** with `code`."
    stdout = capsys.readouterr().out
    assert "reasoning" in stdout.lower()
    assert "[Length exceeded max_tokens limit]" in stdout
