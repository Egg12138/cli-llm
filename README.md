# TODO

## support custome 3rd party model (OpenAI API)

### directly set by literal

```shell
llm -m coder-R # definded models
llm -m QWen3 --vendor dashscope # using dashscope `QWen3` model 
```

### import models config list

add a pair of subcommands: `llm import` and `llm export`

```
llm import models.toml
llm export -o loaded_modles.toml
```

## Rust Version

* Requesting works (currently it can not send anything)
* Update vscode debug launch.json and task.json
* Quicker render

# CLI-LLM

A simple command-line interface for interacting with large language models.

## Project Structure

```
cli-llm/
├── python/              # Python implementation
│   ├── llm             # Main CLI script
│   ├── prompts.py      # System prompts
│   ├── setup.py        # Package configuration
│   └── requirements.txt # Python dependencies
│
└── rust/               # Rust implementation
    ├── src/            # Source code
    ├── Cargo.toml      # Rust package configuration
    └── Cargo.lock      # Rust dependency lock file
```

## Python Version

### Installation

```bash
# From source
cd python
pip install -e .
```

### Usage

```bash
llm "Your question here"
cat /etc/os-release | llm 'Explain(key=meaning of items, strict=True)'
cat *.c | llm 'InvokingChain(lang=Chinese, strict=True)'
```

### Features

- No feature, just for personal usage.

## Rust Version (In Development)

### Installation

```bash
cd rust
cargo install --path .
```

### Features ()
- Faster parsing
- Faster transfer

## Configuration

Set your API credentials as environment variables:

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_API_ENDPOINT="your-api-endpoint"
```

## Development

### Python Development

```bash
cd python
pip install -r requirements.txt
python -m pytest
```

### Rust Development

```bash
cd rust
cargo build
cargo test
```

## License

MIT License - see LICENSE file for details

```shell
Usage: llm [OPTIONS] [PROMPT]

Options:
  -p, --prompt TEXT               Input prompt for deepseek.
  -n, --no-stream                  the streaming of the
                                  response.(totally faster but need to wait
                                  until all results are generated),
                                  model-R does not support
                                  stream!
  -m, --model [coder|chat|creative|coder-R|chat-R|creative-R]
                                  Choose the model to use.default is
                                  coder. the deepseek-coder is not recommanded
                                  in CLI, Now DeepSeek-R1 Reansoner is
                                  available, append an `-R` to use the
                                  reasoner of current mode, e.g. `coder-R`,
                                  `chat-R`, `creative-R`
  -o, --output-codes TEXT         Output the code to the target
                                  file.Only one code block is
                                  supported(IN PROGESS)
  --help                          Show this message and exit.
```
