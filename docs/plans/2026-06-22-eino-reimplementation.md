# Eino Reimplementation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reimplement the current Python-first `cli-llm` runtime in `src-go/` using Eino while preserving existing user-visible behavior and creating a clean foundation for future agentic features.

**Architecture:** Keep the CLI deterministic by default. Use Eino `components` and `compose` for the core chat/toolcall pipelines, and introduce ADK agents only where autonomy is actually needed. Preserve the current separation between config loading, provider dispatch, prompt assembly, rendering, tool execution, and CLI command wiring.

**Tech Stack:** Go, Eino (`components`, `compose`, `adk`), Cobra or stdlib `flag`, TOML config parser, Go test, OpenAI-compatible transport, markdown terminal renderer.

## Current-State Mapping

The Python implementation already exposes the boundaries we should preserve:

- `src/cli_llm/cli.py`
  Handles subcommands, plugin dispatch, `--agents-context`, and CLI flag parsing.
- `src/cli_llm/config.py`
  Owns layered config resolution: CLI > env > config file > defaults.
- `src/cli_llm/services/session.py`
  Owns prompt assembly, token counting, streaming/non-streaming invocation, and AGENTS context injection.
- `src/cli_llm/providers/`
  Is still intentionally thin: one OpenAI-compatible adapter behind `ProviderRouter`.
- `src/cli_llm/toolcalls/service.py`
  Runs a constrained one-shot toolcall flow, not a general autonomous agent loop.

The Go rewrite should preserve those same seams instead of collapsing everything into one Eino agent.

## Recommended Eino Strategy

### Option A: All-in ADK Agent

Use `adk.ChatModelAgent` as the main runtime for `chat` and `toolcall`.

- Pros: fastest way to demo tool use.
- Cons: too much autonomy for the current product shape, harder to preserve exact Python behavior, and poor fit for `inspect` or provider metadata commands.

### Option B: Compose-First Core, ADK as Optional Layer

Use Eino `components` and `compose` for deterministic runtime flows, then add ADK only for agentic subcommands later.

- Pros: best fit for current CLI, easier parity with Python, safer tests, lower migration risk.
- Cons: slightly more upfront design work than wrapping everything in one agent.

### Option C: Minimal OpenAI Port First, Eino Later

First rebuild the Python architecture in Go with plain OpenAI calls, then replace internals with Eino.

- Pros: simplest short-term port.
- Cons: creates throwaway work and delays actual Eino learning.

**Recommendation:** choose Option B.

Rationale:

- `chat` is mostly deterministic prompt assembly + streaming output, so `compose` is enough.
- `toolcall` requires strict validation and local execution constraints; a small graph is safer than a free-form agent loop.
- Future agentic features can live as new subcommands instead of contaminating the base UX.

## Target Go Architecture

Create the following structure under `src-go/`:

- `src-go/cmd/llm/main.go`
  Go entrypoint.
- `src-go/internal/cli/`
  Command wiring for `chat`, `inspect`, `provider models`, `toolcall`.
- `src-go/internal/config/`
  Layered config loader and app config types.
- `src-go/internal/prompts/`
  Role registry, `AGENTS.md` loading, prompt sanitization.
- `src-go/internal/providers/`
  Eino ChatModel factory for OpenAI-compatible providers.
- `src-go/internal/runtime/`
  Shared request/session orchestration, stream event handling, token accounting.
- `src-go/internal/workflows/chatflow/`
  Deterministic Eino composition for normal chat.
- `src-go/internal/workflows/toolcallflow/`
  Deterministic Eino composition for one-shot tool selection and execution.
- `src-go/internal/tools/`
  Local tool registry, schema definitions, and safe executors.
- `src-go/internal/render/`
  Terminal renderer for markdown/code blocks/JSON output.
- `src-go/internal/plugins/`
  `llm-<subcommand>` discovery and `exec` handoff.
- `src-go/tests/` or package-local `_test.go`
  Behavior-parity tests.

## Eino Component Mapping

Map current capabilities to Eino like this:

- Current `OpenAIProvider.create_chat`
  -> Eino `ChatModel`.
- Current `ChatService.chat`
  -> `chatflow` graph:
  sanitize input -> build messages -> invoke chat model -> stream/render output.
- Current `ToolcallService.run`
  -> `toolcallflow` graph:
  build system prompt -> invoke model with tool schemas -> parse one tool call -> validate -> execute local tool -> format result.
- Current future “agentic mode”
  -> separate ADK runner:
  `ChatModelAgent` first, `DeepAgent` only after a real multi-step product requirement exists.

## Learning Path For Eino

Do not start with `DeepAgent`.

Learn in this order:

1. `components`:
   Build a single OpenAI-compatible `ChatModel` call and understand message/event types.
2. `tool` integration:
   Bind one local tool and inspect how Eino represents tool schema and tool results.
3. `compose`:
   Build a small deterministic graph for `chat` and then for `toolcall`.
4. callbacks / streaming:
   Learn how to stream model output and attach logging or telemetry hooks.
5. `adk.ChatModelAgent`:
   Add a small demo command proving controlled tool-using agent behavior.
6. advanced ADK:
   Only then evaluate sub-agents, interrupt/resume, middleware, or `DeepAgent`.

## Scope Boundaries

Do in the first migration:

- `chat`
- `inspect`
- `provider models`
- `toolcall`
- plugin dispatch
- layered config parity
- `AGENTS.md` context injection

Do not do in the first migration:

- multi-agent orchestration
- long-running checkpoints
- human interrupt/resume
- new provider families beyond OpenAI-compatible endpoints
- behavior changes to config precedence

## Task 1: Bootstrap the Go module and command shell

**Files:**
- Create: `src-go/go.mod`
- Create: `src-go/cmd/llm/main.go`
- Create: `src-go/internal/cli/root.go`
- Create: `src-go/internal/cli/chat.go`
- Create: `src-go/internal/cli/inspect.go`
- Create: `src-go/internal/cli/provider.go`
- Create: `src-go/internal/cli/toolcall.go`
- Test: `src-go/internal/cli/root_test.go`

**Step 1: Write the failing test**

Write `src-go/internal/cli/root_test.go` asserting:

- no subcommand defaults to `chat`
- unknown subcommands attempt plugin dispatch before chat fallback
- built-in subcommands are reserved

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/cli -run TestRootCommand -v`
Expected: FAIL because the Go command layer does not exist yet.

**Step 3: Write minimal implementation**

- Initialize the Go module.
- Add a root CLI with the same subcommand names as Python.
- Implement argument forwarding and plugin dispatch seams without full behavior yet.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/cli -run TestRootCommand -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/go.mod src-go/cmd/llm/main.go src-go/internal/cli src-go/internal/cli/root_test.go
git commit -m "feat(go): bootstrap cli shell for eino rewrite"
```

## Task 2: Port layered configuration

**Files:**
- Create: `src-go/internal/config/config.go`
- Create: `src-go/internal/config/loader.go`
- Create: `src-go/internal/config/loader_test.go`
- Reference: `src/cli_llm/config.py`

**Step 1: Write the failing test**

Cover:

- CLI overrides beat env
- env beats TOML file
- TOML file beats defaults
- provider profile values merge correctly

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/config -run TestConfigLoader -v`
Expected: FAIL because no loader exists.

**Step 3: Write minimal implementation**

- Port `AppConfig`.
- Preserve env keys: `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`, `CLI_LLM_DEFAULT_ROLE`, `CLI_LLM_PROVIDER`.
- Preserve the current default values unless we intentionally revise them later.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/config -run TestConfigLoader -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/config
git commit -m "feat(go): port layered configuration loader"
```

## Task 3: Build the Eino ChatModel factory

**Files:**
- Create: `src-go/internal/providers/factory.go`
- Create: `src-go/internal/providers/types.go`
- Create: `src-go/internal/providers/factory_test.go`
- Reference: `src/cli_llm/providers/openai_provider.py`
- Reference: `src/cli_llm/providers/types.py`

**Step 1: Write the failing test**

Cover:

- OpenAI-compatible model factory uses config endpoint and key
- provider selection resolves the active profile
- request translation preserves model, messages, temperature, tool metadata, and streaming mode

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/providers -run TestChatModelFactory -v`
Expected: FAIL because provider factory does not exist.

**Step 3: Write minimal implementation**

- Create a provider factory that returns an Eino `ChatModel`.
- Keep the provider abstraction thin.
- Avoid introducing provider-specific branching that the current Python app does not need.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/providers -run TestChatModelFactory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/providers
git commit -m "feat(go): add eino chat model factory"
```

## Task 4: Port prompts, roles, and AGENTS context injection

**Files:**
- Create: `src-go/internal/prompts/roles.go`
- Create: `src-go/internal/prompts/agents_context.go`
- Create: `src-go/internal/prompts/sanitize.go`
- Create: `src-go/internal/prompts/prompts_test.go`
- Reference: `src/cli_llm/prompts.py`
- Reference: `src/cli_llm/services/session.py`
- Reference: `src/cli_llm/cli.py`

**Step 1: Write the failing test**

Cover:

- predefined roles resolve correctly
- unknown role falls back to configured default
- `AGENTS.md` is appended only when enabled
- oversized `AGENTS.md` is truncated at 16 KB
- missing `AGENTS.md` is a warning, not a failure

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/prompts -run TestPromptAssembly -v`
Expected: FAIL because prompt helpers do not exist.

**Step 3: Write minimal implementation**

- Port role lookup.
- Port sanitization.
- Port `AGENTS.md` loading semantics exactly.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/prompts -run TestPromptAssembly -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/prompts
git commit -m "feat(go): port prompts and agents context handling"
```

## Task 5: Implement deterministic chatflow with Eino compose

**Files:**
- Create: `src-go/internal/workflows/chatflow/graph.go`
- Create: `src-go/internal/runtime/chat_service.go`
- Create: `src-go/internal/render/renderer.go`
- Create: `src-go/internal/workflows/chatflow/graph_test.go`
- Reference: `src/cli_llm/services/session.py`
- Reference: `src/cli_llm/renderers/output.py`

**Step 1: Write the failing test**

Cover:

- prompt and optional stdin input are combined correctly
- JSON output mode injects JSON instructions
- stream and non-stream mode both route through the workflow correctly

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/workflows/chatflow -run TestChatflow -v`
Expected: FAIL because chatflow does not exist.

**Step 3: Write minimal implementation**

- Build a `compose` graph that assembles messages and invokes the chat model.
- Keep rendering outside the graph where terminal concerns dominate.
- Add callback hooks for later token/accounting and debug logging.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/workflows/chatflow -run TestChatflow -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/workflows/chatflow src-go/internal/runtime src-go/internal/render
git commit -m "feat(go): implement eino compose chatflow"
```

## Task 6: Implement deterministic toolcallflow with local safe executors

**Files:**
- Create: `src-go/internal/tools/definitions.go`
- Create: `src-go/internal/tools/executor.go`
- Create: `src-go/internal/workflows/toolcallflow/graph.go`
- Create: `src-go/internal/workflows/toolcallflow/graph_test.go`
- Reference: `src/cli_llm/toolcalls/service.py`
- Reference: `src/cli_llm/toolcalls/presets.py`
- Reference: `src/cli_llm/toolcalls/system_prompt.py`

**Step 1: Write the failing test**

Cover:

- only enabled tools are exposed
- exactly one tool call is required
- arguments are validated by schema
- local path access cannot escape the working directory
- stdout is sanitized and truncated

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/workflows/toolcallflow -run TestToolcallflow -v`
Expected: FAIL because toolcallflow and tool executors do not exist.

**Step 3: Write minimal implementation**

- Model the current one-shot toolcall behavior exactly.
- Use Eino tool/schema primitives where helpful, but keep the execution gate deterministic.
- Do not replace this with a general autonomous agent loop.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/workflows/toolcallflow -run TestToolcallflow -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/tools src-go/internal/workflows/toolcallflow
git commit -m "feat(go): port constrained toolcall workflow"
```

## Task 7: Port inspect/provider metadata commands

**Files:**
- Modify: `src-go/internal/cli/inspect.go`
- Modify: `src-go/internal/cli/provider.go`
- Create: `src-go/internal/cli/provider_test.go`
- Reference: `src/cli_llm/cli.py`

**Step 1: Write the failing test**

Cover:

- `inspect --json`
- `inspect --all`
- `provider models`
- active provider record merging

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/cli -run TestProviderCommands -v`
Expected: FAIL because command behavior is incomplete.

**Step 3: Write minimal implementation**

- Port `_provider_records` semantics.
- Keep text and JSON output behavior aligned with Python unless deliberately changed.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/cli -run TestProviderCommands -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/cli
git commit -m "feat(go): port provider inspection commands"
```

## Task 8: Port plugin dispatch and end-to-end behavior checks

**Files:**
- Create: `src-go/internal/plugins/dispatch.go`
- Create: `src-go/internal/plugins/dispatch_test.go`
- Modify: `src-go/internal/cli/root.go`
- Reference: `src/cli_llm/cli.py`

**Step 1: Write the failing test**

Cover:

- built-in commands are never replaced by plugins
- `llm-<subcommand>` is resolved from `PATH`
- missing plugin falls back to `chat`

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/plugins -run TestPluginDispatch -v`
Expected: FAIL because plugin dispatch is incomplete.

**Step 3: Write minimal implementation**

- Port current plugin behavior with `exec`.
- Keep the resolution rules simple and explicit.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/plugins -run TestPluginDispatch -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/internal/plugins src-go/internal/cli/root.go
git commit -m "feat(go): port plugin dispatch behavior"
```

## Task 9: Add one ADK learning spike, not product coupling

**Files:**
- Create: `src-go/examples/chatmodel_agent/main.go`
- Create: `src-go/examples/chatmodel_agent/README.md`
- Create: `src-go/examples/chatmodel_agent/main_test.go`

**Step 1: Write the failing test**

Cover:

- agent can bind at least one local tool
- event iteration returns model output and tool activity in expected order

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./examples/chatmodel_agent -run TestChatModelAgentExample -v`
Expected: FAIL because the example does not exist.

**Step 3: Write minimal implementation**

- Add one isolated ADK example.
- Keep it out of the core `chat` command until a real product requirement justifies that jump.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./examples/chatmodel_agent -run TestChatModelAgentExample -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src-go/examples/chatmodel_agent
git commit -m "feat(go): add eino adk learning example"
```

## Task 10: Final parity verification and docs alignment

**Files:**
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `CHANGELOG.md`
- Create: `src-go/README.md`

**Step 1: Write the failing test**

Add or update a parity checklist document or test harness proving:

- config precedence still works
- `chat` still supports stream/non-stream
- `toolcall` remains constrained
- plugin dispatch still works

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./...`
Expected: FAIL until parity gaps are closed.

**Step 3: Write minimal implementation**

- Align docs with the new Go track.
- State clearly whether Python remains primary or whether the Go/Eino implementation becomes the active runtime.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add README.md AGENTS.md CHANGELOG.md src-go/README.md src-go
git commit -m "docs: document eino rewrite architecture and parity status"
```

## Execution Notes

- Keep Python behavior as the oracle until the Go implementation reaches parity.
- Treat `toolcall` as a controlled workflow, not a general-purpose agent.
- Use Eino to strengthen architecture, not to justify avoidable complexity.
- If the user later wants a full agent mode, add a new command such as `llm agent` instead of mutating `llm chat`.
