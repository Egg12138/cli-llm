"""CLI entry-point wiring for cli-llm."""

from __future__ import annotations

import json
import select
import sys
from typing import Any, Dict, Optional

import click # type: ignore

from .utils import colored, RSTF, NOTF, TIPF
from ._version import __version__
from .config import AppConfig, ConfigLoader, HELP_TEXTS, setup_logging
from .providers import ProviderRouter
from .renderers import ResponseRenderer
from .services import ChatService, TokenTracker, ensure_url_parser_ok, sanitize_input
from .toolcalls import get_tool_definitions

CONFIG_LOADER = ConfigLoader()


@click.group()
@click.version_option(version=__version__, prog_name="cli-llm")
def cli() -> None:
    """cli-llm multi-command entrypoint."""


@cli.command(name="chat")
@click.argument("prompt", required=False, default=None)
@click.option("-n", "--no-stream", is_flag=True, help=HELP_TEXTS["no_stream"])
@click.option("-p", "--provider", help=HELP_TEXTS.get("provider", "Select the provider profile."))
@click.option("-r", "--role", help=HELP_TEXTS["role"])
@click.option("-m", "--model", help=HELP_TEXTS["model"])
@click.option("-t", "--temp", type=float, help=HELP_TEXTS["temp"])
@click.option("-j", "--json-output", is_flag=True, help=HELP_TEXTS["json_output"])
@click.option("-o", "--output-codes", nargs=1, default=None, help=HELP_TEXTS["output_codes"])
@click.option("-d", "--debug", is_flag=True, help=HELP_TEXTS["debug"], default=False)
@click.option("--localtest", is_flag=True, help=HELP_TEXTS["test"], default=False)
@click.option("--count-tokens", is_flag=True, help=HELP_TEXTS["count_tokens"])
def chat_command(
    prompt: Optional[str],
    no_stream: bool,
    provider: Optional[str],
    model: Optional[str],
    role: Optional[str],
    temp: Optional[float],
    json_output: bool,
    output_codes: Optional[str],
    debug: bool,
    localtest: bool,
    count_tokens: bool,
) -> None:
    app_config = CONFIG_LOADER.load(cli_overrides={"default_model": model, "provider": provider})
    _run_chat(
        app_config=app_config,
        prompt=prompt,
        no_stream=no_stream,
        role=role,
        temp=temp,
        json_output=json_output,
        output_codes=output_codes,
        debug=debug,
        localtest=localtest,
        count_tokens=count_tokens,
    )


@cli.command("inspect")
@click.option("--json", "json_mode", is_flag=True, help="Print provider data as JSON.")
@click.option("--all", "all_fields", is_flag=True, default=False, help="Print all fields of provder(incuding keys).")
def providers_cmd(json_mode: bool, all_fields: bool) -> None:
    """List provider profiles discovered from config/env defaults."""

    app_config = CONFIG_LOADER.load()
    records = _provider_records(app_config)

    if json_mode:
        print(json.dumps(records, indent=2, sort_keys=True))
        return
    
    if all_fields:
        print("Displaying all fields for each provider.")

    active = app_config.provider
    lines = [f"Active provider: {active}"]
    for name in sorted(records.keys()):
        record = records[name]
        src = record.get('source')
        models = ", ".join(record.get("models", [])) or "-"
        has_key = "yes" if record.get("has_api_key") else "no"
        lines.extend(
            [
                colored(f"- {name} ", TIPF)
                + colored(f"({src})", NOTF if src == 'active' else TIPF),
                f"  api_endpoint: {record.get('api_endpoint', '-')}",
                f"  default_model: {record.get('default_model', '-')}",
            ]
        )

        lines.extend([f"  models: {models}",] if all_fields else [])
        lines.extend([f"  api_key_configured: {has_key}",] if all_fields else [])
        
        
    print("\n".join(lines))


@cli.group()
def provider() -> None:
    """Inspect metadata for current provider."""


@provider.command("models")
@click.argument("provider_name", required=False)
@click.option("--json", "json_mode", is_flag=True, help="Print only the model list as JSON.")
def provider_models(provider_name: Optional[str], json_mode: bool) -> None:
    """Show the models declared for a provider profile."""

    app_config = CONFIG_LOADER.load()
    target = provider_name or app_config.provider
    records = _provider_records(app_config)
    record = records.get(target)
    if record is None:
        raise click.UsageError(f"Provider '{target}' is not available.")

    models = record.get("models") or ([record["default_model"]] if record.get("default_model") else [])

    if json_mode:
        print(json.dumps({"provider": target, "models": models}, indent=2))
        return

    if not models:
        print(f"No models declared for provider '{target}'.")
        return

    print(f"Models for '{target}':")
    for name in models:
        print(f"- {name}")


@cli.command("toolcall")
@click.argument("prompt", required=False)
@click.option("--tools", "tools_csv", help="Comma-separated preset tools to enable.")
@click.option("--list-tools", is_flag=True, help="List enabled preset tools and exit.")
def toolcall_command(prompt: Optional[str], tools_csv: Optional[str], list_tools: bool) -> None:
    """Run a single tool-call-oriented request."""

    tool_names = None
    if tools_csv:
        tool_names = [name.strip() for name in tools_csv.split(",") if name.strip()]
    try:
        tools = get_tool_definitions(tool_names)
    except ValueError as exc:
        raise click.UsageError(str(exc)) from exc

    if list_tools:
        for tool in tools:
            print(f"{tool.name}\t{tool.prompt_snippet or tool.description}")
        return

    if not prompt:
        raise click.UsageError("Missing prompt.")
    raise click.ClickException("toolcall execution is not implemented yet.")


def _run_chat(
    *,
    app_config: AppConfig,
    prompt: Optional[str],
    no_stream: bool,
    role: Optional[str],
    temp: Optional[float],
    json_output: bool,
    output_codes: Optional[str],
    debug: bool,
    localtest: bool,
    count_tokens: bool,
) -> None:
    logger = setup_logging()

    if prompt is None:
        prompt = input(f"{TIPF}[Ask]:{RSTF}")
    prompt = sanitize_input(prompt)

    if select.select([sys.stdin], [], [], 0.0)[0]:
        stdin_input = sys.stdin.read().strip()
    else:
        stdin_input = ""

    ensure_url_parser_ok()

    active_model = app_config.default_model
    active_role = role or app_config.default_role
    full_prompt = "\n".join(filter(None, [prompt, stdin_input]))

    provider_client = ProviderRouter(app_config).resolve()
    renderer = ResponseRenderer(app_config)
    token_tracker = TokenTracker()
    chat_service = ChatService(provider_client, renderer, token_tracker)

    if debug:
        logger.setLevel("DEBUG")
        for handler in logger.handlers:
            handler.setLevel("DEBUG")

    if localtest:
        print(f"Provider base URL: {provider_client.config.api_endpoint}")
        return

    chat_service.chat(
        full_prompt,
        no_stream=no_stream,
        model=active_model,
        role_name=active_role,
        count_tokens=count_tokens,
        custom_temp=temp,
        json_output=json_output,
    )

    chat_service.display_tokens_if_any()


def _provider_records(app_config: AppConfig) -> Dict[str, Dict[str, Any]]:
    records: Dict[str, Dict[str, Any]] = {}
    for name, profile in app_config.providers.items():
        record = {
            "api_endpoint": profile.get("api_endpoint"),
            "default_model": profile.get("default_model"),
            "models": profile.get("models") or [],
            "has_api_key": bool(profile.get("api_key")),
            "source": "user-config",
        }
        records[name] = record

    active = records.setdefault(app_config.provider, {})
    active["api_endpoint"] = app_config.api_endpoint
    active["default_model"] = app_config.default_model
    active.setdefault("models", [])
    active["has_api_key"] = bool(app_config.api_key or active.get("has_api_key"))
    active["source"] = "active"
    if app_config.default_model and app_config.default_model not in active["models"]:
        active["models"].append(app_config.default_model)

    return records


SUBCOMMAND_NAMES = {"chat", "inspect", "provider", "toolcall"}
PASSTHROUGH_FLAGS = {"-h", "--help", "-V", "--version"}


def main() -> None:
    args = sys.argv[1:]
    if args and args[0] in PASSTHROUGH_FLAGS:
        forwarded = args
    elif not args or args[0] not in SUBCOMMAND_NAMES:
        forwarded = ["chat", *args]
    else:
        forwarded = args

    try:
        cli.main(args=forwarded)
    except KeyboardInterrupt:
        print(f"\n{TIPF}Interrupted by user{RSTF}")
        sys.exit(130)


if __name__ == "__main__":
    main()
