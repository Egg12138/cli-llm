from __future__ import annotations

import sys

from cli_llm.cli import SUBCOMMAND_NAMES, main


def test_known_subcommand_in_names():
    assert "chat" in SUBCOMMAND_NAMES
    assert "inspect" in SUBCOMMAND_NAMES
    assert "provider" in SUBCOMMAND_NAMES
    assert "toolcall" in SUBCOMMAND_NAMES


def test_plugin_not_found_falls_back_to_chat(monkeypatch):
    """Unknown subcommand without a matching plugin defaults to chat."""
    monkeypatch.setattr(sys, "argv", ["llm", "nonexistent-plugin"])
    monkeypatch.setattr("shutil.which", lambda _: None)

    forwarded_args: list[list[str]] = []

    class FakeCli:
        @staticmethod
        def main(args: list[str] | None = None, **_: object) -> None:
            forwarded_args.append(list(args or []))

    import cli_llm.cli as cli_mod
    monkeypatch.setattr(cli_mod, "cli", FakeCli())

    main()
    assert forwarded_args == [["chat", "nonexistent-plugin"]]


def test_default_routing_to_chat(monkeypatch):
    monkeypatch.setattr(sys, "argv", ["llm"])

    forwarded_args: list[list[str]] = []

    class FakeCli:
        @staticmethod
        def main(args: list[str] | None = None, **_: object) -> None:
            forwarded_args.append(list(args or []))

    import cli_llm.cli as cli_mod
    monkeypatch.setattr(cli_mod, "cli", FakeCli())

    main()
    assert forwarded_args == [["chat"]]
