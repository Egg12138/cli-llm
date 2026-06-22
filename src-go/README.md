# cli-llm Go/Eino Track

`src-go/` is the Eino-based parity implementation for cli-llm. Python remains
the primary supported runtime; this tree is the candidate implementation for a
future Go install target.

## Architecture

- `cmd/llm/` contains the Go entrypoint.
- `internal/cli/` wires `chat`, `inspect`, `provider models`, and `toolcall`.
- `internal/config/` preserves layered config precedence: CLI > env >
  `~/.cli-llm/config.toml` > defaults.
- `internal/prompts/` owns roles, input sanitization, and optional `AGENTS.md`
  injection with the 16 KB guard.
- `internal/providers/` builds OpenAI-compatible Eino chat models.
- `internal/workflows/chatflow/` uses Eino compose for deterministic chat.
- `internal/workflows/toolcallflow/` uses Eino compose for one-shot tool
  selection and local execution.
- `internal/plugins/` resolves cargo-style `llm-<subcommand>` plugins.
- `cmd/llm-session/` and `internal/session/` implement the Go-only
  persistent session plugin.
- `examples/chatmodel_agent/` is an ADK learning spike and is intentionally not
  part of the core CLI runtime.

## Parity Checklist

- Config precedence works through package tests.
- `chat` supports stream and non-stream paths through the runtime tests.
- JSON output mode injects the expected JSON instruction.
- `toolcall` remains constrained to exactly one enabled tool call.
- Local tool execution validates arguments, working-directory access, and stdout
  truncation.
- `inspect`, `inspect --json`, `inspect --all`, and `provider models` are covered
  by CLI tests.
- Plugin dispatch reserves built-ins, resolves PATH plugins, and falls back to
  chat when no plugin exists.
- `llm-session` covers JSONL sessions, checkpoints, branch switching, resume,
  normal-buffer REPL behavior, and the Ctrl+T transcript overlay through package
  tests.

## Commands

```bash
go test ./...
go run ./cmd/llm --version
go run ./cmd/llm inspect
go build ./cmd/llm-session
go test ./examples/chatmodel_agent -v
go test ./internal/render -run '^$' -bench BenchmarkDefaultRenderMarkdown -benchmem
```

Live provider calls require the same configuration keys as the Python runtime:
`OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`, `CLI_LLM_DEFAULT_ROLE`, and
`CLI_LLM_PROVIDER`, or equivalent entries in `~/.cli-llm/config.toml`.

## llm-session Plugin

Build manually while the Go runtime remains an evaluation track:

```bash
go build ./cmd/llm-session
```

Put the resulting `llm-session` binary on `PATH` to enable plugin dispatch:

```bash
llm-session
llm-session --resume
llm-session --resume work
llm session --resume work   # works when llm-session is on PATH
```

Session files are append-only JSONL under `~/.cli-llm/sessions/<name>.jsonl`.
The MVP commands are `/branches`, `/switch <branch-or-hash>`,
`/checkpoint <name>`, and `/exit`.

Normal chat stays in the terminal main buffer, so native scrollback continues to
work. The transcript viewer is isolated to the alternate-screen overlay path; it
emits alternate scroll mode while open and restores terminal modes on exit. The
current line reader recognizes the Ctrl+T control character when the terminal
input path delivers it.
