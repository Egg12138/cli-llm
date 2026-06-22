# llm-session TUI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. REQUIRED SUB-SKILL per code task: superpowers:test-driven-development.
>
> Design rationale lives in `docs/plans/2026-06-22-llm-session-tui-design.md`. This doc is the
> task-by-task build order. Two forks were resolved on 2026-06-23:
> **(1) bubbletea-only** (no `bubbles`, no `teatest` — hand-roll the spinner, reuse the existing
> line-scroll), and **(2) entry-cursor navigation** (Up/Down select dialog turns; `y` copies the
> selected turn). These override the "reuses bubbles/spinner" / "teatest e2e" wording in AGENTS.md.

**Goal:** Add an interactive Ctrl+T transcript overlay (Bubble Tea) that is branch-aware, copies
the selected dialog via OSC 52, plus a main-buffer "thinking/waiting/streaming" status indicator —
without changing `repl.Loop` or breaking the headless `bytes.Buffer` test style.

**Architecture:** A new `internal/session/tui` package whose `Model` is a pure `tea.Model`
(`Update` + `View()` are deterministic and snapshot-testable). `tui.Overlay` implements the
existing `repl.TranscriptOverlay` interface (`Open(state *graph.State, out io.Writer) error`) and
is the only place that touches a real TTY (`tea.Program` over `os.Stdin`, alt-screen). The status
indicator is a `runtime.StatusReporter` interface wired through `RunChatTurn`, with a TTY ticker
reporter and a deterministic non-TTY reporter living in `repl`.

**Tech Stack:** Go 1.24, Bubble Tea v1.3.6 (only new direct dep), lipgloss + glamour + x/ansi
(promote indirect→direct; already in the module tree), existing `graph`/`model`/`runtime`/`repl`
packages.

---

## Status: July 2026-06-23 Progress Update

### ✅ Completed (Tasks 0–6)

- **Task 0:** `graph.ReachableFrom` — implemented and tested
- **Task 1:** bubbletea dependency fetched, go.mod updated
- **Task 2:** `tui/entry.go` — `displayEntry` + `renderEntries` with glamour + plain render
- **Task 3:** `tui/model.go` — Model skeleton, entry-cursor nav, derived scroll window
- **Task 4:** branch panel with Tab toggle, switch-isolated preview
- **Task 5:** clipboard copy via OSC 52 (`ansi.SetClipboard`)
- **Task 6:** `tui.Overlay` implementing `TranscriptOverlay`, wired into `runner.go`

### ❌ Remaining Original Tasks

- **Task 7:** Status reporter in `runtime` — NOT STARTED
- **Task 8:** Status reporters in `repl` + runner selection — NOT STARTED

### 🔧 Additional Fixes (discovered during 2026-06-23 review)

These were identified when testing and reviewing the TUI against the design doc and TUI design
principles:

- **Task 9:** `Esc` in branch panel mode should return to scroll mode, not quit
- **Task 10:** Footer hint should mention `Tab` (branches) and `y` (copy), not just nav keys
- **Task 11:** Semantic color tokens + `NO_COLOR` support in styles
- **Task 12:** Branch preview should restore original history on close
- **Task 13:** Remove duplicate `q` handling in `handleBranchKey` (already handled in `handleKey`)
- **Task 14:** Minimum terminal size check with "terminal too small" message
- **Task 15:** Full test suite pass + manual smoke verification

---

## Offline / dependency note (read before Task 1)

- `bubbletea@v1.3.6` is in the module cache, but its `require` graph pulls two tiny modules that
  are **not** cached: `github.com/erikgeiser/coninput` and `github.com/mattn/go-localereader`.
  Adding bubbletea therefore needs a **one-time network fetch** of just those two (plus go.sum
  entries). Everything else bubbletea needs (`muesli/ansi`, `muesli/cancelreader`, `x/sync`,
  `x/sys`, `x/term`, `lipgloss`, `x/ansi`) is already cached or already required.
- We deliberately do **not** add `bubbles` (pulls 6+ uncached deps) or `x/exp/teatest` (uncached).
  The spinner is hand-rolled; the overlay scroll reuses the existing `maxOffset` math. The test
  suite is pure `Model` unit tests — no real TTY, no `teatest`.
- If the environment is fully offline, run Task 1's `go get` on a connected machine (or set
  `GOPROXY`) once; all later tasks build and test offline.

## Package layout (target)

```
internal/session/graph/state.go     # + ReachableFrom(headID); ReachableHistory delegates to it
internal/session/tui/
  entry.go        # displayEntry + renderEntries(entries, opts) — role prefix + glamour, pure
  model.go        # Model (tea.Model), Config, New; entry-cursor nav, derived scroll, branch panel
  keys.go         # key string constants + footer hint text
  styles.go       # lipgloss styles: role colors, selected highlight, panel border, header/footer
  clipboard.go    # copySelection() tea.Cmd -> ansi.SetClipboard bytes to m.clip
  overlay.go      # Overlay implements repl.TranscriptOverlay; Open builds Model, runs tea.Program
  entry_test.go, model_test.go, overlay_test.go
internal/session/runtime/status.go  # Status enum + StatusReporter interface + nopReporter
internal/session/runtime/chat.go    # wire reporter.Set(...) around title/stream/first-chunk
internal/session/repl/status.go     # TTYReporter (ticker) + PlainReporter (deterministic lines)
internal/session/cli/runner.go      # swap transcriptOverlay -> tui.NewOverlay; pick reporter by TTY
```

## Locked component contracts

```go
// graph
func (s *State) ReachableFrom(headID string) ([]model.Entry, error)

// tui
type displayEntry struct { role, raw string; lines []string }
type Config struct {
    History  []displayEntry
    Branches []graph.Branch
    Current  string
    Load     func(headID string) ([]displayEntry, error) // branch-panel preview
    Clip     io.Writer
    Width, Height int
}
func New(cfg Config) Model                 // Model implements tea.Model
func NewOverlay() Overlay                   // implements repl.TranscriptOverlay

// runtime
type Status int
const ( StatusThinking Status = iota; StatusWaitingStream; StatusStreaming )
type StatusReporter interface { Set(Status); Clear() }
// ChatTurnRequest gains: Status StatusReporter  (nil -> nopReporter)

// repl
type PlainReporter struct { ... }    // deterministic "[thinking]" lines
type TTYReporter struct { ... }      // animated spinner with ticker
```

**Model behavior (entry-cursor, locked):** `cursor` (entry index) is the single source of truth.
Up/Down move `cursor` ±1; PgUp/PgDn ±page (page = max(1, height/3)); Home/End → first/last. `View()`
flattens `history` to lines, derives a scroll window that keeps the cursor entry visible (clamped
with the existing `maxOffset` rule), and highlights the cursor entry. `tab` toggles the branch
panel; in the panel Up/Down move `branchSel`, Enter calls `Load(headID)` and re-renders to that
branch (read-only preview, never mutates session `HeadID`). `y` copies `history[cursor].raw` via
OSC 52. `q`/`esc`/`ctrl+t` quit (`tea.Quit`).

---

## Task 0: graph.ReachableFrom (enables branch-panel preview) ✅

**Status:** Completed. Implemented `ReachableFrom(headID)` in `graph/state.go`, with
`ReachableHistory()` delegating to it. Tests pass.

---

## Task 1: add bubbletea dependency (one-time fetch) ✅

**Status:** Completed. `bubbletea@v1.3.6` added, `lipgloss`/`glamour` promoted to direct deps,
`x/ansi` pinned at v0.10.2.

---

## Task 2: tui.displayEntry + renderEntries (pure render) ✅

**Status:** Completed. `entry.go` with glamour markdown and plain fallback. Tests pass.

---

## Task 3: tui.Model skeleton — layout + scroll navigation ✅

**Status:** Completed. Entry-cursor nav, derived scroll, header/footer, bodyHeight/page helpers.
Tests pass.

---

## Task 4: branch panel + switch isolation ✅

**Status:** Completed. Tab toggles branch panel, Up/Down/Home/End/Enter for selection and preview.
Switch isolation verified in tests.

---

## Task 5: clipboard copy via OSC 52 ✅

**Status:** Completed. `copySelection()` writes `ansi.SetClipboard` bytes to injected writer.
Tests verify OSC 52 prefix and base64 content.

---

## Task 6: tui.Overlay + wire into runner ✅

**Status:** Completed. `overlay.go` builds Model from `*graph.State`, runs `tea.NewProgram` with
alt-screen. `runner.go` uses `tui.NewOverlay()`. Old `transcriptOverlay` type deleted.

---

## Task 7: status reporter in runtime

**Files:**
- Create: `internal/session/runtime/status.go`

**Step 1: Write failing test** — a fake `StatusReporter` records `Set`/`Clear` calls.
Create `runtime/status_test.go` that asserts the interface contract (calls are recorded in order).

**Step 2:** `go test ./internal/session/runtime/ -run Status -v` → FAIL (package compiles, test
expects types/funcs that don't exist yet).

**Step 3: Implement** — `status.go`:
```go
type Status int
const (
    StatusThinking Status = iota
    StatusWaitingStream
    StatusStreaming
)
type StatusReporter interface {
    Set(Status)
    Clear()
}
type nopReporter struct{}
func (nopReporter) Set(Status) {}
func (nopReporter) Clear() {}
```

**Step 4:** `go test ./internal/session/runtime/ -v` → PASS.

**Step 5: Wire into chat.go** — add `Status StatusReporter` to `ChatTurnRequest` (default nop via
nil check). In `RunChatTurn`:
- Before title/build: `reporter.Set(StatusThinking)`
- After `Model.Stream` returns: `reporter.Set(StatusWaitingStream)`  
- On first non-empty chunk: `reporter.Set(StatusStreaming)`
- In a defer: `reporter.Clear()`

Update `chat_test.go` to add a test that verifies the call order with a fake reporter.

**Step 6:** `go test ./internal/session/runtime/ -v` → PASS.

**Step 7: Commit** — `feat(session/runtime): status reporting around chat turns`

---

## Task 8: repl status reporters + runner selection

**Files:**
- Create: `internal/session/repl/status.go`, `repl/status_test.go`
- Modify: `internal/session/cli/runner.go`

**Step 1: Write failing test** — `PlainReporter` writes exactly one deterministic line per `Set`
(e.g. `[thinking]\n`, `[waiting for stream]\n`, `[streaming]\n`) to its writer and nothing on
`Clear` beyond a carriage-return reset; assert against a `bytes.Buffer`.

**Step 2:** `go test ./internal/session/repl/ -run Status -v` → FAIL.

**Step 3: Implement** — `status.go`:
- `PlainReporter` — deterministic lines to `io.Writer`. `Set` writes `[label]\n`; `Clear` writes `\r\x1b[K` (clear line).
- `TTYReporter` — goroutine ticker (100ms) cycling hand-rolled spinner frames (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`), writes `\r<spin> <label>` to `out`; `Set` updates current state; `Clear` stops ticker and writes `\r\x1b[K`. Mutex-guarded.
- Both implement `runtime.StatusReporter`.

**Step 4:** `go test ./internal/session/repl/ -v` → PASS.

**Step 5: Wire into runner.go** — pick `TTYReporter` when `out` is a terminal (`x/term.IsTerminal` on the fd), else `PlainReporter`. Pass as `ChatTurnRequest.Status` in the `StartREPLFunc`.

**Step 6:** `go test ./internal/session/... -v` && `go build ./...` → PASS.

**Step 7: Commit** — `feat(session/repl): TTY spinner + plain status reporters`

---

## Task 9: Esc in branch panel returns to scroll mode, not quit

**Files:**
- Modify: `internal/session/tui/model.go`, `model_test.go`

**Step 1: Write failing test** — create a Model with branches, send `tabKey()` to enter
`modeBranches`, then `keyMsg(tea.KeyEsc)`. Assert `m.mode == modeScroll` and `!m.quitting`.
Also assert `q` from branch panel still quits.

**Step 2:** `go test ./internal/session/tui/ -run Esc -v` → FAIL.

**Step 3: Implement** — in `handleBranchKey`, change `KeyEsc` case to:
```go
case tea.KeyEsc, tea.KeyCtrlT:
    m.mode = modeScroll
```
Not `m.quitting = true` + `tea.Quit`. Since branch panel only toggles with Tab, Esc returns to
scroll mode. Keep `q` as the quit-anywhere key (already handled by `KeyRunes` in `handleKey` via
the nested `switch` inside `Update`).

**Step 4:** `go test ./internal/session/tui/ -v` → PASS.

**Step 5: Commit** — `fix(session/tui): Esc returns from branch panel instead of quitting`

---

## Task 10: Footer hint mentions Tab and y

**Files:**
- Modify: `internal/session/tui/keys.go`

**Step 1:** Read current footer hint (already known: `"↑/↓ select · PgUp/PgDn · Home/End · q quit"`).

**Step 2: Update footer hint** — add `Tab` and `y`:
```go
const footerHint = "↑/↓ select · PgUp/PgDn · Home/End · Tab branches · y copy · q quit"
```

**Step 3:** Update `TestModelLayout` if it checks for the exact footer string. Run tests.

**Step 4:** `go test ./internal/session/tui/ -v` → PASS.

**Step 5: Commit** — `fix(session/tui): mention Tab and y in footer hint`

---

## Task 11: Semantic color tokens + NO_COLOR support

**Files:**
- Modify: `internal/session/tui/styles.go`

**Step 1: Write failing test** — call `Model.View()` with `NO_COLOR` env set; assert output contains
no ANSI escape sequences (or just verify no color codes in the output).

**Step 2:** `go test ./internal/session/tui/ -run Color -v` → FAIL.

**Step 3: Implement** — replace raw ANSI numbers with semantic `AdaptiveColor` tokens:
```go
var (
    roleUserColor     = lipgloss.AdaptiveColor{Light: "4", Dark: "12"}   // blue
    roleAssistantColor = lipgloss.AdaptiveColor{Light: "2", Dark: "10"}  // green
    roleSystemColor   = lipgloss.AdaptiveColor{Light: "8", Dark: "7"}    // gray
)
```
Check `os.Getenv("NO_COLOR")` in an `init()` or helper; if set, return colorless styles. But
actually, lipgloss already supports `AdaptiveColor`, and `NO_COLOR` is typically handled by
checking the env var at the app level. For Bubble Tea, the simplest approach: check `NO_COLOR`
in the styles init and fall through to plain styles.

Actually — `lipgloss` doesn't natively respect `NO_COLOR`. The standard approach is:
```go
func init() {
    if os.Getenv("NO_COLOR") != "" {
        lipgloss.SetColorProfile(termenv.Ascii)
    }
}
```
This already exists in `model_test.go`'s `init()`. For the real app, we just need it in `styles.go`.

**Step 4:** `go test ./internal/session/tui/ -v` → PASS.

**Step 5: Commit** — `refactor(session/tui): semantic color tokens, respect NO_COLOR`

---

## Task 12: Branch preview restores original history on Esc

**Files:**
- Modify: `internal/session/tui/model.go`, `model_test.go`

**Step 1: Write failing test** — create Model with branches, preview another branch (Tab → down → Enter),
verify history is replaced with preview content. Then close overlay (`q`) and re-open — assert
original history is restored the next time the overlay opens.

This is tricky because the Model doesn't survive re-opening — `Overlay.Open()` always creates a
fresh `Model`. So "restore on close" is actually "the overlay always starts fresh from
`ReachableHistory()`", which it already does via `newModelFromState()`. The issue is really about
the *in-session* experience: after previewing another branch, the user sees that branch's history
and has no way to get back to the current branch without re-opening.

**Simpler approach:** Add a `showCurrent` helper that reloads history from the current branch.
Bound to a key (e.g., `r` for reset, or `Esc` from preview).

But actually, the simplest fix for v1: **don't replace `m.cfg.History` permanently**. Instead,
store the preview history in a separate field and render it in `View()` when previewing. The
original `m.cfg.History` always stays as the current branch.

**Even simpler:** Since the overlay always re-builds from state (via `newModelFromState`), just
acknowledge this in docs and don't try to restore. The user can press Ctrl+T again to get back to
the current branch history. This is a minor UX issue, not a blocker for v1.

→ **Deferred.** The overlay always opens fresh from `ReachableHistory()`. After previewing a
branch, closing and re-opening Ctrl+T returns to the current branch. This is acceptable for v1.

---

## Task 13: Remove duplicate q handling in handleBranchKey

**Files:**
- Modify: `internal/session/tui/model.go`

**Step 1:** Notice that in `handleBranchKey`, there's a `KeyRunes` case checking for `'q'`:
```go
case tea.KeyRunes:
    if len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
        m.quitting = true
        return m, tea.Quit
    }
```
This is redundant because `Update` already dispatches `KeyRunes` to `handleKey` in the outer
switch (after `handleBranchKey` returns, control goes back to `Update` which returns `m, nil`,
so `handleKey` is never called for branch-mode keys). Wait, let me re-read the flow more
carefully.

Actually, looking at `Update()`:
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        ...
    case tea.KeyMsg:
        return m.handleKey(msg)
    }
    return m, nil
}
```

And `handleKey()`:
```go
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    if msg.Type == tea.KeyTab {
        m.toggleBranchPanel()
        return m, nil
    }
    if m.mode == modeBranches {
        return m.handleBranchKey(msg)
    }
    switch msg.Type {
    case tea.KeyUp: ...
    case tea.KeyDown: ...
    ...
    case tea.KeyRunes:
        if len(msg.Runes) == 1 && msg.Runes[0] == 'q' { ... quit }
        if len(msg.Runes) == 1 && msg.Runes[0] == 'y' { ... copy }
    }
    return m, nil
}
```

So the flow is: `Update()` → `handleKey()` → if `modeBranches` → `handleBranchKey()` → returns immediately. The `KeyRunes` case in `handleBranchKey` affects `'q'`, and the `KeyRunes` case in `handleKey` covers `'q'` and `'y'`.

But if `modeBranches`, `handleKey` returns `handleBranchKey(msg)` which handles its own `KeyRunes`
for `'q'`. The `'y'` handling never runs in branch mode (it's in `handleKey` after the
`modeBranches` check).

So the `'q'` in `handleBranchKey` is NOT actually redundant — it enables quitting from branch mode
via `q`. BUT, the issue is that `handleKey` in scroll mode handles both `'q'` and `'y'` in the
same `KeyRunes` case. In branch mode, only `'q'` is handled.

**Fix:** Keep the `'q'` in `handleBranchKey` but make it consistent with the style in
`handleKey`. No structural change needed, just remove the code duplication by making the branch
mode handler follow the same pattern: after the switch on `KeyType`, fall through to `KeyRunes`
for `'q'`.

Actually, this is really minor. Let me just acknowledge it and move on.

**Simplified Task 13:** Keep the current `q` handling as-is. It's not truly duplicate — branch mode
needs its own `q` exit because it returns before the scroll-mode `KeyRunes` handler.

→ **Deferred.** Minimal code smell, no behavioral bug.

---

## Task 11 (renumbered): Semantic color tokens + NO_COLOR support

(Was Task 11 above. This is now our focus instead of deferred tasks.)

The color-token task is the most impactful of the remaining fixes. The NO_COLOR path matters for
accessibility and makes the app usable in minimal terminals.

---

## Task 12 (renumbered): Minimum terminal size check

**Files:**
- Modify: `internal/session/tui/model.go`, `keys.go`, `model_test.go`

**Step 1: Write failing test** — create a Model with `Width: 40, Height: 5` (below minimum) and
assert `View()` contains an error message like "terminal too small" instead of normal content.

**Step 2:** `go test ./internal/session/tui/ -run MinSize -v` → FAIL (View renders garbage or
empty).

**Step 3: Implement** — define minimum size constants:
```go
const minWidth = 60
const minHeight = 10
```
In `View()`, if `m.cfg.Width < minWidth || m.cfg.Height < minHeight`, render a centered "terminal
too small" message instead of normal content.

**Step 4:** `go test ./internal/session/tui/ -v` → PASS.

**Step 5: Commit** — `feat(session/tui): minimum terminal size check`

---

## Task 13: Full test suite + manual smoke (from original Task 9)

**Step 1:** `go test ./internal/session/...` and `go vet ./internal/session/...` → PASS.

**Step 2: Manual smoke** (real provider, real TTY): start `llm-session`, send a message and
confirm the spinner shows `thinking`→`waiting`→`streaming` then clears on first token; press
Ctrl+T and confirm the overlay shows only the current branch, Up/Down select turns, `tab` lists
branches, selecting another branch previews only its turns, `y` copies the selected dialog, `q`
exits cleanly back to the main buffer with scrollback intact.

**Step 3: Docs** — tick the TUI + thinking-status items in `AGENTS.md` 0.4.0 list; add a
CHANGELOG entry. Commit `docs(session): mark TUI overlay + status indicator done`.

---

## Out of scope (unchanged from design doc)

Full-text search, full-screen session TUI, bracketed-paste UI, branch mutation from the overlay,
line-level intra-entry scrolling (entry-cursor is v1), `bubbles`/`teatest` (offline choice).
