# Go llm-session Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go-only `llm-session` plugin that provides persistent multi-turn sessions with git-like checkpoints, branches, resume, and a terminal-correct REPL.

**Architecture:** Ship `llm-session` as a separate plugin binary under `src-go/cmd/llm-session`, so the existing `llm session` plugin dispatch can find it without changing Python or the built-in Go command surface. Keep the MVP stable before introducing Eino stateful graphs: implement session storage, branch navigation, prompt assembly, streaming chat, compression, and terminal I/O as explicit packages; later wrap the same state transitions in Eino `compose.Graph` if it proves useful.

**Tech Stack:** Go 1.24, Eino ChatModel via the existing provider factory, JSONL append-only storage, `github.com/chzyer/readline` for normal prompt input, small terminal-control abstraction for alternate screen transcript overlay, Go package tests.

## Scope Decision

Use route A:稳健 MVP 优先.

Do first:

- `llm-session` plugin binary.
- Append-only JSONL session files in `~/.cli-llm/sessions/<name>.jsonl`.
- Entry model: `message`, `custom:checkpoint`, `custom:session_info`, `compaction`, `branch_summary`.
- REPL in the normal terminal buffer by default.
- Ctrl+T transcript overlay using alternate screen and alternate scroll mode.
- Streaming chat using the default configured provider/model.
- Auto-checkpoint after each assistant response.
- `/branches`, `/switch <target>`, `/checkpoint <name>`, `/exit`.
- Auto session title on first user message.
- Context compression when token count crosses the configured threshold.

Do not do in MVP:

- Eino stateful Graph as the primary implementation.
- `@filename`, `#checkpoint` references.
- bracketed paste markers.
- `/export`, `/new`, `/rename`.
- merge semantics.
- Python `src/` changes.

## Current Codebase Anchors

- `src-go/cmd/llm/main.go` is the existing Go entrypoint.
- `src-go/internal/plugins/dispatch.go` already resolves `llm-<subcommand>` executables. A binary named `llm-session` lets `llm session` dispatch without adding `session` to builtins.
- `src-go/internal/config` loads `~/.cli-llm/config.toml` plus env and CLI overrides.
- `src-go/internal/providers/factory.go` builds the OpenAI-compatible Eino model.
- `src-go/internal/render/renderer.go` streams assistant text to a writer.
- `src-go/internal/workflows/chatflow` only supports single-turn messages, so session mode should assemble its own history messages instead of reusing this package directly.

## Terminal Contract

The REPL must treat screen behavior as a first-class contract.

Normal mode:

- Run in the terminal main buffer.
- Write conversation turns directly to stdout.
- Do not enter alternate screen for normal chat.
- Do not intercept mouse wheel scrolling.
- Let terminal-native scrollback handle mouse wheel, touchpad, scrollbar, and PageUp/PageDown where the terminal normally owns them.

Ctrl+T transcript overlay:

- Ctrl+T enters alternate screen.
- On enter, write `\x1b[?1049h` or use an equivalent terminal package call.
- Enable alternate scroll mode with `\x1b[?1007h`.
- Render a read-only transcript viewport from in-memory session turns.
- Wheel events become ArrowUp/ArrowDown in supporting terminals because alternate scroll mode translates them.
- The overlay handles ArrowUp/ArrowDown/PageUp/PageDown/Home/End by moving an internal viewport.
- Ctrl+T or Esc exits the overlay.
- On exit, disable alternate scroll mode with `\x1b[?1007l`, then leave alternate screen with `\x1b[?1049l`.
- Always restore terminal state on panic, interrupt, and normal exit.

This avoids the common bad UX where a CLI app enters alternate screen for the entire session and breaks terminal scrollback. The normal REPL should feel like a shell command; only the transcript viewer is a TUI overlay.

## Proposed Package Layout

- `src-go/cmd/llm-session/main.go`
  Thin entrypoint.
- `src-go/internal/session/cli/`
  Flag parsing and command wiring for `llm-session`.
- `src-go/internal/session/store/`
  JSONL persistence, append, load, atomic directory creation.
- `src-go/internal/session/model/`
  Entry types, hash IDs, branch metadata, session state.
- `src-go/internal/session/graph/`
  Parent-chain walking, branch heads, switch/checkpoint semantics.
- `src-go/internal/session/repl/`
  Normal-buffer interactive loop, slash command dispatch, input reader abstraction.
- `src-go/internal/session/terminal/`
  Terminal mode abstraction, Ctrl+T overlay, viewport scrolling, ANSI sequence writer.
- `src-go/internal/session/runtime/`
  Chat model invocation, title generation, context assembly, compression.
- `src-go/internal/session/token/`
  Token counting helpers and compression threshold policy.

## Task 1: Create plugin command shell

**Files:**
- Create: `src-go/cmd/llm-session/main.go`
- Create: `src-go/internal/session/cli/command.go`
- Test: `src-go/internal/session/cli/command_test.go`

**Step 1: Write the failing test**

Test:

- `Run([]string{})` starts fresh mode.
- `Run([]string{"--resume"})` enters interactive resume selection mode.
- `Run([]string{"--resume", "work"})` resumes named session.
- invalid flags return exit code `2`.

Use injected dependencies:

```go
type fakeRunner struct {
	mode string
	name string
}
```

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/cli -run TestCommand -v`

Expected: FAIL because the package does not exist.

**Step 3: Write minimal implementation**

Implement:

- `type Mode string` with `ModeFresh`, `ModeResumePicker`, `ModeResumeNamed`.
- `type Options struct { Mode Mode; Name string }`.
- `func Parse(args []string) (Options, error)`.
- `func Run(args []string, runner Runner) int`.
- `cmd/llm-session/main.go` calls `sessioncli.Main(os.Args[1:])`.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/cli -run TestCommand -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/cmd/llm-session src-go/internal/session/cli
git commit -m "feat(go): add llm-session command shell"
```

## Task 2: Define session entry model and stable IDs

**Files:**
- Create: `src-go/internal/session/model/entry.go`
- Create: `src-go/internal/session/model/hash.go`
- Test: `src-go/internal/session/model/entry_test.go`

**Step 1: Write the failing test**

Cover:

- same entry content produces same 12-char lowercase hex ID.
- changing parent ID changes the hash.
- user message preview is the first 16 display runes.
- branch entries are metadata pointers and do not rename message nodes.
- JSON round trip preserves entry type and data.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/model -run TestEntry -v`

Expected: FAIL because the package does not exist.

**Step 3: Write minimal implementation**

Define:

```go
type Entry struct {
    ID        string          `json:"id"`
    ParentID  string          `json:"parentId,omitempty"`
    Type      string          `json:"type"`
    CreatedAt time.Time       `json:"createdAt"`
    Data      json.RawMessage `json:"data"`
}
```

Add typed constructors:

- `NewMessage(parentID, role, content string, now time.Time) (Entry, error)`
- `NewCheckpoint(parentID, name, returnTo string, now time.Time) (Entry, error)`
- `NewSessionInfo(parentID string, info SessionInfo, now time.Time) (Entry, error)`
- `NewCompaction(parentID string, data CompactionData, now time.Time) (Entry, error)`
- `NewBranchSummary(parentID string, data BranchSummaryData, now time.Time) (Entry, error)`

Hash canonical JSON excluding `ID`.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/model -run TestEntry -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/model
git commit -m "feat(go): add session entry model"
```

## Task 3: Implement append-only JSONL store

**Files:**
- Create: `src-go/internal/session/store/store.go`
- Test: `src-go/internal/session/store/store_test.go`

**Step 1: Write the failing test**

Use `t.TempDir()` and cover:

- store creates the sessions directory with `0755`.
- append writes one JSON object per line.
- load returns entries in file order.
- corrupt JSONL returns an error with line number.
- missing file loads as empty state when creating a new session.
- session name rejects path separators and empty strings.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/store -run TestStore -v`

Expected: FAIL because store does not exist.

**Step 3: Write minimal implementation**

Implement:

- `DefaultRoot() string` -> `~/.cli-llm/sessions`.
- `Open(root, name string) (Store, error)`.
- `Store.Path() string`.
- `Store.Append(entry model.Entry) error`.
- `Store.Load() ([]model.Entry, error)`.

Use `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)`.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/store -run TestStore -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/store
git commit -m "feat(go): add session jsonl store"
```

## Task 4: Build session state and branch navigation

**Files:**
- Create: `src-go/internal/session/graph/state.go`
- Create: `src-go/internal/session/graph/navigation.go`
- Test: `src-go/internal/session/graph/state_test.go`

**Step 1: Write the failing test**

Cover:

- initial session creates `main` branch.
- auto-checkpoint advances the current branch head.
- `/checkpoint explore` labels current head by creating or updating a branch.
- `/switch main` moves to branch head.
- `/switch <hash>` detaches to that entry.
- `/switch unknown` creates a new branch from current head.
- branch list returns name, head hash, and parent branch when known.
- parent-chain history returns only entries reachable from current head.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/graph -run TestState -v`

Expected: FAIL because graph package does not exist.

**Step 3: Write minimal implementation**

Implement a pure in-memory state:

```go
type State struct {
    Entries map[string]model.Entry
    Order []string
    Branches map[string]Branch
    CurrentBranch string
    HeadID string
    Detached bool
}
```

Keep topology precomputed when commands mutate branch heads. Do not perform dynamic tree rendering for MVP.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/graph -run TestState -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/graph
git commit -m "feat(go): add session branch navigation"
```

## Task 5: Add context assembly for chat history

**Files:**
- Create: `src-go/internal/session/runtime/context.go`
- Test: `src-go/internal/session/runtime/context_test.go`

**Step 1: Write the failing test**

Cover:

- system prompt includes current date, session name, branch, and head.
- reachable user/assistant messages are converted to Eino `schema.Message`.
- compaction entry becomes a synthetic system or assistant context summary.
- branch summaries are included only when reachable.
- checkpoint entries are not sent as normal chat messages.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/runtime -run TestBuildContext -v`

Expected: FAIL because runtime package does not exist.

**Step 3: Write minimal implementation**

Implement:

- `type ContextOptions struct { SessionName, BranchName, HeadID, CurrentDate string }`.
- `func BuildMessages(state graph.State, opts ContextOptions) ([]*schema.Message, error)`.

Do not call the model here.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/runtime -run TestBuildContext -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/runtime/context.go src-go/internal/session/runtime/context_test.go
git commit -m "feat(go): assemble session chat context"
```

## Task 6: Implement chat invocation and auto-checkpoint

**Files:**
- Create: `src-go/internal/session/runtime/chat.go`
- Test: `src-go/internal/session/runtime/chat_test.go`

**Step 1: Write the failing test**

Use a fake chat model/stream. Cover:

- user entry is appended before model invocation.
- assistant entry is appended after successful stream.
- checkpoint entry is appended after assistant entry.
- failed model call leaves no assistant/checkpoint entry.
- stream chunks are forwarded to the normal-mode writer.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/runtime -run TestChatTurn -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

- `type ChatTurnRequest struct { Input string; State *graph.State; Store store.Store; Writer io.Writer; Model model.BaseChatModel; Options []model.Option }`.
- `func RunChatTurn(ctx context.Context, req ChatTurnRequest) error`.

Prefer the existing provider factory in command wiring, but keep this runtime package model-interface driven for tests.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/runtime -run TestChatTurn -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/runtime/chat.go src-go/internal/session/runtime/chat_test.go
git commit -m "feat(go): run session chat turns"
```

## Task 7: Add auto session titling

**Files:**
- Modify: `src-go/internal/session/runtime/chat.go`
- Create: `src-go/internal/session/runtime/title.go`
- Test: `src-go/internal/session/runtime/title_test.go`

**Step 1: Write the failing test**

Cover:

- first user message triggers title request.
- title is trimmed to one line and 60 display runes.
- empty model title falls back to first prompt preview.
- title is stored in `custom:session_info`.
- later turns do not retitle.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/runtime -run 'TestTitle|TestChatTurn' -v`

Expected: FAIL on new title behavior.

**Step 3: Write minimal implementation**

Prompt:

```text
Generate a short descriptive title for this session.
Return one line, at most 60 characters.

User message:
{first_message}
```

Use the same configured model. Save title before the main assistant response.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/runtime -run 'TestTitle|TestChatTurn' -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/runtime/title.go src-go/internal/session/runtime/chat.go src-go/internal/session/runtime/*_test.go
git commit -m "feat(go): title sessions from first prompt"
```

## Task 8: Implement slash commands

**Files:**
- Create: `src-go/internal/session/repl/commands.go`
- Test: `src-go/internal/session/repl/commands_test.go`

**Step 1: Write the failing test**

Cover:

- `/exit` returns an exit action.
- `/branches` formats all branch heads.
- `/switch main` switches branch.
- `/switch <hash>` detaches.
- `/switch newname` creates branch.
- `/checkpoint name` creates branch label.
- unknown slash command returns a user-visible error and continues.
- non-slash input is classified as chat input.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/repl -run TestSlashCommands -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement parser and action executor separate from terminal input:

- `ParseLine(line string) Command`.
- `ExecuteCommand(state *graph.State, command Command, out io.Writer) (Action, error)`.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/repl -run TestSlashCommands -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/repl/commands.go src-go/internal/session/repl/commands_test.go
git commit -m "feat(go): add session slash commands"
```

## Task 9: Implement normal-buffer REPL

**Files:**
- Create: `src-go/internal/session/repl/repl.go`
- Test: `src-go/internal/session/repl/repl_test.go`

**Step 1: Write the failing test**

Use fake input reader and fake chat runner. Cover:

- normal chat lines call chat runner.
- slash commands do not call chat runner.
- Ctrl+D exits cleanly.
- interrupt exits with code `130`.
- normal mode writes to stdout without entering alternate screen.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/repl -run TestREPL -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
type InputReader interface {
    ReadLine(prompt string) (string, error)
}
```

Use `readline` in the real adapter. Keep transcript overlay trigger as an injected event or special key path for the next task; do not mix raw terminal parsing into business tests.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/repl -run TestREPL -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/repl/repl.go src-go/internal/session/repl/repl_test.go
git commit -m "feat(go): add normal-buffer session repl"
```

## Task 10: Add terminal transcript overlay

**Files:**
- Create: `src-go/internal/session/terminal/control.go`
- Create: `src-go/internal/session/terminal/transcript.go`
- Test: `src-go/internal/session/terminal/transcript_test.go`

**Step 1: Write the failing test**

Use a bytes buffer as terminal output and fake key events. Cover:

- entering overlay writes enter alternate screen sequence.
- entering overlay enables alternate scroll mode `\x1b[?1007h`.
- exiting overlay disables alternate scroll mode before leaving alternate screen.
- ArrowUp/ArrowDown updates viewport.
- PageUp/PageDown updates viewport by viewport height.
- Home/End jump to top/bottom.
- panic-safe cleanup path restores modes.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/terminal -run TestTranscript -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement terminal control as tiny explicit methods:

- `EnterAltScreen()`
- `LeaveAltScreen()`
- `EnableAltScroll()`
- `DisableAltScroll()`
- `Clear()`
- `MoveCursor(row, col int)`

Use raw ANSI sequences in one package only. Do not let session runtime write these sequences.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/terminal -run TestTranscript -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/terminal
git commit -m "feat(go): add transcript alternate-screen overlay"
```

## Task 11: Wire Ctrl+T into the REPL

**Files:**
- Modify: `src-go/internal/session/repl/repl.go`
- Modify: `src-go/internal/session/repl/repl_test.go`

**Step 1: Write the failing test**

Cover:

- Ctrl+T opens transcript overlay.
- transcript overlay gets the current in-memory reachable turns.
- exiting overlay returns to the same REPL loop.
- chat after overlay still writes to normal buffer.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/repl -run TestREPLTranscript -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Add a readline adapter path that can detect Ctrl+T. If `chzyer/readline` cannot reliably bind Ctrl+T without raw-mode complications, introduce a small `InputEventReader` abstraction now and implement the real raw-mode reader in a follow-up task before live release. Do not compromise the normal-buffer contract.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/repl -run TestREPLTranscript -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/repl
git commit -m "feat(go): wire transcript overlay into session repl"
```

## Task 12: Add context compression

**Files:**
- Create: `src-go/internal/session/token/count.go`
- Create: `src-go/internal/session/runtime/compress.go`
- Test: `src-go/internal/session/runtime/compress_test.go`
- Test: `src-go/internal/session/token/count_test.go`

**Step 1: Write the failing test**

Cover:

- below threshold does nothing.
- above threshold calls summarizer.
- compaction entry records `summary`, `firstKeptEntryId`, and `tokensBefore`.
- recent messages remain reachable after compaction.
- failed summarizer leaves state unchanged.

Use a low threshold in tests.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/token ./internal/session/runtime -run 'TestToken|TestCompression' -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Use current defaults:

- threshold: `200000`.
- keep recent tokens: `20000`.

Reuse `tiktoken-go` already present in the module. The compressor uses the same configured model in v1.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/token ./internal/session/runtime -run 'TestToken|TestCompression' -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/token src-go/internal/session/runtime/compress.go src-go/internal/session/runtime/compress_test.go
git commit -m "feat(go): add session context compression"
```

## Task 13: Wire command runner end-to-end

**Files:**
- Modify: `src-go/internal/session/cli/command.go`
- Create: `src-go/internal/session/cli/runner.go`
- Test: `src-go/internal/session/cli/runner_test.go`

**Step 1: Write the failing test**

Cover:

- fresh session prompts for a safe session name.
- fresh session creates a store and initializes state.
- resume named loads existing JSONL.
- resume missing named session reports clear error.
- resume picker lists sessions by title when available and filename otherwise.
- provider factory receives the configured default model.

**Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/session/cli -run TestRunner -v`

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement real runner wiring:

- load config with `config.NewLoader`.
- create provider model with `providers.NewFactory`.
- open store.
- load graph state.
- start REPL.

Keep dependencies injectable for tests.

**Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/session/cli -run TestRunner -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add src-go/internal/session/cli
git commit -m "feat(go): wire llm-session runner"
```

## Task 14: Add docs and install/build notes

**Files:**
- Modify: `src-go/README.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Optional Modify: `install.sh`

**Step 1: Write the failing doc check**

If there is no doc linter, use a grep-based smoke test in a shell command:

Run: `rg -n "llm-session|llm session|Ctrl\\+T|sessions" README.md CHANGELOG.md src-go/README.md`

Expected before docs: missing one or more required mentions.

**Step 2: Update docs**

Document:

- `go build ./cmd/llm-session`.
- `llm-session`, `llm-session --resume`, `llm-session --resume <name>`.
- `llm session` works when `llm-session` is on PATH.
- normal buffer scrollback behavior.
- Ctrl+T transcript overlay behavior.
- session files path.
- MVP slash commands.

Only update `install.sh` if this milestone is promoted to install target. Otherwise state manual build clearly.

**Step 3: Run doc check**

Run: `rg -n "llm-session|llm session|Ctrl\\+T|sessions" README.md CHANGELOG.md src-go/README.md`

Expected: finds all required docs.

**Step 4: Commit**

```bash
git add README.md CHANGELOG.md src-go/README.md install.sh
git commit -m "docs: document go llm-session"
```

## Task 15: Final verification

**Files:**
- No source edits expected.

**Step 1: Run focused tests**

Run:

```bash
cd src-go
go test ./internal/session/...
```

Expected: PASS.

**Step 2: Run full Go tests**

Run:

```bash
cd src-go
go test ./...
```

Expected: PASS.

**Step 3: Build binaries**

Run:

```bash
cd src-go
go build ./cmd/llm
go build ./cmd/llm-session
```

Expected: both builds pass.

**Step 4: Manual smoke test**

With a test provider config:

```bash
cd src-go
go run ./cmd/llm-session
go run ./cmd/llm-session --resume
go run ./cmd/llm-session --resume test-session
```

Verify:

- normal output stays in main buffer.
- terminal scrollback works normally in normal mode.
- Ctrl+T enters transcript overlay.
- mouse wheel in overlay moves viewport where terminal supports alternate scroll mode.
- exiting overlay restores normal screen.
- `/branches`, `/checkpoint`, `/switch`, `/exit` behave as documented.

**Step 5: Commit if verification required changes**

```bash
git add <changed-files>
git commit -m "test(go): verify llm-session integration"
```

## Risk Notes

- `chzyer/readline` may not expose Ctrl+T cleanly. If that blocks a correct transcript overlay, introduce a narrow raw input adapter for session mode instead of forcing readline into undefined behavior.
- Alternate scroll mode is terminal-emulator dependent. Tests should verify emitted sequences and viewport logic; manual smoke testing must verify behavior in the target terminal.
- Streaming output and prompt input can fight over cursor state if the model streams while readline owns the prompt. The REPL should only start streaming after input submission and should redraw the next prompt after stream completion.
- JSONL append is crash-friendly but not multi-process safe. MVP can document single-writer behavior; file locking can be added later if needed.
- Compression changes history shape. Keep it behind deterministic tests with low thresholds before using the 200k default.

## Execution Handoff

Plan complete and saved to `docs/plans/2026-06-22-go-llm-session.md`.

Recommended execution: use `superpowers:executing-plans` in a dedicated worktree, complete tasks in order, and run the listed tests before each commit.
