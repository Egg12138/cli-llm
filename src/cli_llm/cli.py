"""CLI entry-point wiring for cli-llm."""

from __future__ import annotations

import select
import sys
from typing import Optional

import click

from ._version import __version__
from .config import ConfigLoader, HELP_TEXTS, TIPF, RSTF, setup_logging
from .providers import OpenAIProvider
from .renderers import ResponseRenderer
from .services import ChatService, TokenTracker, ensure_url_parser_ok, sanitize_input

CONFIG_LOADER = ConfigLoader()


@click.command()
@click.version_option(version=__version__, prog_name="cli-llm")
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
def chat_cli(
    prompt: Optional[str],
    no_stream: bool,
    provider: Optional[str],
    model: Optional[str],
    role: str,
    temp: Optional[float],
    json_output: bool,
    output_codes: Optional[str],
    debug: bool,
    localtest: bool,
    count_tokens: bool,
) -> None:
    app_config = CONFIG_LOADER.load(cli_overrides={"default_model": model, "provider": provider})
    logger = setup_logging()

    if prompt is None:
        prompt = input(f"{TIPF}[Ask]:{RSTF}")
    prompt = sanitize_input(prompt)

    if select.select([sys.stdin], [], [], 0.0)[0]:
        stdin_input = sys.stdin.read().strip()
    else:
        stdin_input = ""

    ensure_url_parser_ok()

    if app_config.provider != "openai":
        raise click.UsageError(
            f"Provider '{app_config.provider}' is not supported yet. Only 'openai' is available."
        )

    active_model = app_config.default_model
    active_role = role or app_config.default_role
    full_prompt = "\n".join(filter(None, [prompt, stdin_input]))

    provider_client = OpenAIProvider(app_config)
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


def main() -> None:
    try:
        chat_cli()
    except KeyboardInterrupt:
        print(f"\n{TIPF}Interrupted by user{RSTF}")
        sys.exit(130)


if __name__ == "__main__":
    main()
