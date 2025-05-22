# CLI-LLM (Rust Version)

A high-performance command-line interface for interacting with large language models, implemented in Rust.

## Features (Planned)

- High performance implementation
- Memory efficient
- Native binary distribution
- Cross-platform support
- Streaming and non-streaming response modes
- Code highlighting and formatting
- Detailed error handling and logging

## Installation

```bash
cargo install --path .
```

## Usage

```bash
llm "Your question here"
```

## Configuration

Set your API credentials as environment variables:

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_API_ENDPOINT="your-api-endpoint"
```

## Development

```bash
# Build
cargo build

# Run tests
cargo test

# Run with debug output
RUST_LOG=debug cargo run -- "Your question here"
```

## Project Structure

```
rust/
├── src/            # Source code
├── Cargo.toml      # Package configuration
└── Cargo.lock      # Dependency lock file
```

## License

MIT License - see LICENSE file for details 