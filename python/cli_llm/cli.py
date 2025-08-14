#!/usr/bin/env python3
"""CLI entry point for interactive mode."""
import click
import os
import sys
from typing import Optional

# Add the parent directory to sys.path to import from the main module
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from .interactive import InteractiveSession
from ..llm import chat_cli


@click.command()
@click.argument('prompt', required=False, default=None)
@click.option('-n', '--no-stream', is_flag=True, help='Disable streaming of the response.')
@click.option('-r', '--role', default='coder', help='Choose the role to use. Default is coder.')
@click.option('-m', '--model', help='Choose the model to use. Default is $OPENAI_MODEL.')
@click.option('-t', '--temp', type=float, help='Customize temperature value for the model.')
@click.option('-o', '--output-codes', nargs=1, default=None, help='Output the code to the target file.')
@click.option('-d', '--debug', is_flag=True, help='Enable debug mode to show detailed logs.', default=False)
@click.option('--localtest', is_flag=True, help='Do some local tests.', default=False)
@click.option('--count-tokens', is_flag=True, help='Enable token counting and show usage statistics.')
def main(prompt: Optional[str], no_stream: bool, role: str, model: Optional[str], 
         temp: Optional[float], output_codes: Optional[str], debug: bool, 
         localtest: bool, count_tokens: bool) -> None:
    """
    CLI entry point that handles both interactive and non-interactive modes.
    
    If no prompt is provided, enters interactive mode.
    Otherwise, runs in non-interactive mode with the given prompt.
    """
    # If no prompt is provided, enter interactive mode
    if prompt is None and not sys.stdin.isatty():
        # Check if there's input from stdin (piped input)
        stdin_input = sys.stdin.read().strip()
        if stdin_input:
            prompt = stdin_input
    
    if prompt is None:
        # Enter interactive mode
        session = InteractiveSession()
        # Set initial role if specified
        if role and role in session.sys_roles:
            session.current_role = role
        # Set model if specified
        if model:
            session.model = model
        # Set temperature if specified
        if temp is not None:
            session.temperature = temp
        session.run()
    else:
        # Use existing non-interactive mode
        # Reconstruct sys.argv to pass to the existing cli function
        import argparse
        original_argv = sys.argv
        try:
            chat_cli(standalone_mode=False)
        except SystemExit:
            pass


if __name__ == '__main__':
    main()