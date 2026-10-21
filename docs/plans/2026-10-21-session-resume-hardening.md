# llm-session resume hardening — plan

Status: PR A, PR C, and PR B's model-restoration half are implemented; active
branch persistence from PR B remains open. Baseline includes the `--resume` picker fix (`cli.pickSession`,
byte-wise `readPickerLine`).
Input: subagent review comparing `src-go/internal/session` with pi's resume
model (JSONL header, leaf→root projection, compaction projection, session
picker, model restore, cwd/fork handling). Claims below were re-checked
against the code; two subagent claims are corrected at the end.

## Already aligned with pi

- Append-only parent-linked JSONL with immutable hash IDs; resume appends after
  the loaded head instead of rewriting (`store/store.go:111`,
  `model/hash.go:9`).
- Branch path reconstruction and message rebuild from the loaded graph, not
  event replay (`graph/state.go:133`, `runtime/context.go:15`).
- Resume continues in place; PTY coverage proves the selected file, not the
  other one, receives the new turn (`acceptance/resume_pty_test.go`).
- Picker accepts number or exact name/title, re-prompts, cancels cleanly on
  empty line/EOF, and does not consume input beyond one line
  (`cli/runner.go:241`, `:283`).
- A failed turn never appends an assistant message or checkpoint, and retries
  never re-append the user message (`runtime/chat.go:100`).

## Highest-value gaps (ranked)

### 1. `--resume NAME` with a typo silently starts an empty session (done)

`store.Open` only validates and creates the directory (`store/store.go:29`);
`Load` returns an empty slice when the file is absent (`store/store.go:128`);
`SessionRunner.Run` wraps errors only when one occurs (`cli/runner.go:70`).
Impact: user talks to an empty context and a new `<typo>.jsonl` appears.
Fix: `ErrSessionNotFound` sentinel from the resume path; keep fresh mode as-is.
Effort S, risk low.

### 2. Loading is all-or-nothing and capped at 64 KiB per line (done)

`Load` uses a default `bufio.Scanner`, rejects the first malformed line, and
surfaces `ErrTooLong` (`store/store.go:139`). `Append` writes an entire
assistant reply as one line (`store/store.go:111`). One long reply or a torn
final line makes a session permanently unresumable.
Fix: raise the scanner buffer, skip-and-count bad lines with line numbers,
report skipped lines on resume. Effort S, risk low.

### 3. Picker has no titles, no recency order, no preview (title/recency done)

`defaultListSessions` only reads directory names and sorts by name
(`cli/runner.go:216`), so `SessionSummary.Title` is always empty and the title
match in `pickSession` (`cli/runner.go:265`) is dead in production. Titles are
already persisted in `custom:session_info` (`runtime/title.go:42`).
Fix: read `session_info` + mtime per file, sort by recency, show entry count.
Effort S, risk low.

### 4. Current branch is never persisted; resume always lands on `main`

`graph.NewState` hardcodes `CurrentBranch: "main"` and finally takes the head
from the `main` pointer (`graph/state.go:27`, `:51`). Turns write per-branch
checkpoints (`runtime/chat.go:167`) but no entry records the active branch, so
after `/switch experiment` + restart new turns append to `main`.
Fix needs a marker entry. Caution: `NewState` treats every non-named entry as
a head candidate, and existing files must keep loading. Effort S–M, risk
medium; strongest candidate for a dedicated PR.

### 5. Model is not restored; `SessionInfo.Model` is write-only (done)

`SessionInfo.Model` is written at title creation (`runtime/title.go:42`) and
never read outside tests. `SessionRunner.Run` always builds the model from
live config (`cli/runner.go:94`). Resuming silently changes models.
Fix: resolve the recorded model through `AppConfig.Providers`, fall back to the
configured model with a one-line notice. Effort S–M, risk low.

### 6. Compaction is unwired and would not survive reload (done)

`MaybeCompress` has no production caller. It appends `compaction` with
`parentId ""` and reparents `firstKeptEntryId` only in memory
(`runtime/compress.go:56`), so a reload discards the cut and the summary is
unreachable. `firstRecentEntry` keeps only the last entry and
`KeepRecentTokens` is unused (`runtime/compress.go:79`).
Fix: persist the projection (`parentId` on the kept entry or projection-time
resolution), implement token-budget retention, add reload tests. Effort M–L,
risk medium-high; separate PR.

## Low-risk improvements

1. `--continue` / `-c` and `-r` alias: newest session by mtime; flag plumbing
   only (`cli/command.go:39`).
2. Resume banner: print `resumed <name> · N entries · model M` once
   (`cli/runner.go:349`).
3. Picker input: `\r` is deliberately skipped so CRLF input cannot leave a
   stray newline for the editor (a lone CR does not occur in canonical TTY
   mode); an explicit length cap is still open (`cli/runner.go:283`).
4. Store hardening: `Append` follows symlinks; `Rename` has a TOCTOU between
   `AvailableName` and `os.Rename` (`store/store.go:50`, `:94`).
5. Report partially readable sessions (`skipped N lines`) with #2.

## Do not adopt now

- Header line, cwd field, per-project session dirs, project/all picker,
  fork-on-cwd-mismatch, `MissingSessionCwdError`: llm-session has one flat
  session root and no cwd-scoped resources (`store/store.go:21`,
  `cli/command.go:171`). Pure cost today.
- In-session `/resume` with runtime rebuild: the loop closes over name, state,
  store, model (`cli/runner.go:349`), and there are no per-session resources to
  reload. Revisit after model restoration.
- `--session-id`, `/import`, `--fork`, extension `beforeSwitch` hooks: no
  import or extension surface; `/import` would turn prompt injection from
  session content into a real attack surface.
- Versioned header + migration: no released format version; add a header only
  together with the schema change in #4/#6, and teach `NewState` to skip it.
- Sanitizing resumed session text: local, self-authored files today; gate it
  when `/import`/sharing lands.
- Generating `branch_summary` on switch: type exists and context accepts it,
  but no path creates it and `/switch` does not produce divergent histories.

## Suggested PR slices

**PR A — truthful resume (gaps 1–3, improvements 1–2, 5) — implemented.**
`ErrSessionNotFound` + `Exists()`, tolerant load with line diagnostics,
picker titles/recency, resume banner, `--continue`/`-c`, `-r` alias. Tests:
store long/torn/corrupt lines; picker metadata and recency; PTY resume with an
unreadable trailing line, selection isolation, and cancellation. No
context-semantics changes.

**PR B — restore where you were (gaps 4–5).** Model restoration implemented:
resume resolves the recorded model through the configured provider profiles,
falls back with a warning, and reports the active model in the resume banner.
Active-branch persistence is still open: it needs a marker entry that
`NewState` must not treat as a head candidate, plus reload tests.

**PR C — compaction projection (gap 6) — implemented.** `MaybeCompress` is
wired into `RunChatTurn`, appends the compaction at the current leaf (so it
stays on the reloaded branch path), selects the kept window by token budget,
and `projectHistory` replaces summarized history at read time. Failure is
best-effort. Tests cover the leaf/parent chain, budget selection, reload
projection, missing kept entries, and turn behavior on summary failure.

## Corrections to the subagent report

- `readPickerLine` does not over-read stdin: it reads one byte at a time
  (`cli/runner.go:283`). Only `\r` handling needs work.
- In memory, compaction does replace older history via reparenting; the defect
  is durability/projection after reload (and unused token budget), not
  "summary appended on top of all messages".
