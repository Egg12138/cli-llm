from __future__ import annotations

import sys

from cli_llm.cli import SUBCOMMAND_NAMES, main


def test_known_subcommand_in_names():
    assert "chat" in SUBCOMMAND_NAMES
    assert "inspect" in SUBCOMMAND_NAMES
    assert "provider" in SUBCOMMAND_NAMES
    assert "toolcall" in SUBCOMMAND_NAMES
    assert "session" in SUBCOMMAND_NAMES


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


def test_session_plugin_dispatches_when_binary_exists(monkeypatch):
    monkeypatch.setattr(sys, "argv", ["llm", "session", "--resume", "work"])
    monkeypatch.setattr("shutil.which", lambda name: "/tmp/llm-session" if name == "llm-session" else None)

    exec_calls: list[tuple[str, list[str]]] = []

    def fake_execvp(path: str, args: list[str]) -> None:
        exec_calls.append((path, args))
        raise SystemExit(0)

    monkeypatch.setattr("os.execvp", fake_execvp)

    try:
        main()
    except SystemExit as exc:
        assert exc.code == 0

    assert exec_calls == [("/tmp/llm-session", ["/tmp/llm-session", "--resume", "work"])]


def test_session_subcommand_is_visible_in_help():
    from click.testing import CliRunner

    import cli_llm.cli as cli_mod

    result = CliRunner().invoke(cli_mod.cli, ["--help"])

    assert result.exit_code == 0
    assert "session" in result.output


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
