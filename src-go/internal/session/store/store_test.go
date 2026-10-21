package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func TestStoreCreatesSessionsDirectory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "sessions")
	store, err := Open(root, "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if store.Path() != filepath.Join(root, "work.jsonl") {
		t.Fatalf("unexpected path %q", store.Path())
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected root to be directory")
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected directory mode 0755, got %o", info.Mode().Perm())
	}
}

func TestStoreAppendWritesJSONLinesAndLoadReturnsFileOrder(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	first, err := model.NewMessage("", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	second, err := model.NewMessage(first.ID, "assistant", "hi", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}

	if err := store.Append(first); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := store.Append(second); err != nil {
		t.Fatalf("append second: %v", err)
	}

	content, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d: %q", len(lines), content)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].ID != first.ID || loaded.Entries[1].ID != second.ID {
		t.Fatalf("entries loaded out of order: %#v", loaded.Entries)
	}
	if len(loaded.SkippedLines) != 0 {
		t.Fatalf("expected no skipped lines, got %v", loaded.SkippedLines)
	}
}

func TestStoreLoadSkipsCorruptLinesAndReportsNumbers(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	valid, err := model.NewMessage("", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if err := store.Append(valid); err != nil {
		t.Fatalf("append valid: %v", err)
	}
	file, err := os.OpenFile(store.Path(), os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open session file: %v", err)
	}
	if _, err := file.WriteString("{}\nnot-json\n"); err != nil {
		_ = file.Close()
		t.Fatalf("write corrupt lines: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close session file: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load should tolerate corrupt lines, got %v", err)
	}
	if len(loaded.Entries) != 1 || loaded.Entries[0].ID != valid.ID {
		t.Fatalf("expected the valid entry to survive, got %#v", loaded.Entries)
	}
	if got, want := loaded.SkippedLines, []int{2, 3}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("skipped lines = %v, want %v", got, want)
	}
}

func TestStoreLoadHandlesLinesLargerThanScannerDefaults(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	content := strings.Repeat("x", 200*1024)
	entry, err := model.NewMessage("", "assistant", content, now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if err := store.Append(entry); err != nil {
		t.Fatalf("append long entry: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected the long entry, got %#v", loaded)
	}
	data, err := loaded.Entries[0].MessageData()
	if err != nil {
		t.Fatalf("MessageData returned error: %v", err)
	}
	if data.Content != content {
		t.Fatalf("long content was truncated: got %d bytes, want %d", len(data.Content), len(content))
	}
}

func TestStoreLoadSkipsTornTrailingLine(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte("{\"id\":\"abc\"}\n{\"id\":\"torn\""), 0o644); err != nil {
		t.Fatalf("write torn session: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load should tolerate a torn trailing line, got %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected the complete first line, got %#v", loaded.Entries)
	}
	if got, want := loaded.SkippedLines, []int{2}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("skipped lines = %v, want %v", got, want)
	}
}

func TestStoreExistsReportsMissingFile(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected a missing session file to report exists=false")
	}
	if err := os.WriteFile(store.Path(), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	exists, err = store.Exists()
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected an existing session file to report exists=true")
	}
}

func TestStoreLoadReportsCorruptJSONLineNumber(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte("{}\nnot-json\n"), 0o644); err != nil {
		t.Fatalf("write corrupt jsonl: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load should tolerate corrupt lines, got %v", err)
	}
	if len(loaded.Entries) != 0 {
		t.Fatalf("expected no usable entries, got %#v", loaded.Entries)
	}
	if got, want := loaded.SkippedLines, []int{1, 2}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("skipped lines = %v, want %v", got, want)
	}
}

func TestStoreLoadMissingFileReturnsEmptyEntries(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Entries) != 0 {
		t.Fatalf("expected empty entries, got %#v", loaded.Entries)
	}
}

func TestStoreRenameMovesSessionFileAndKeepsAppending(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := Open(root, "session-temp")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	first, err := model.NewMessage("", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if err := store.Append(first); err != nil {
		t.Fatalf("append first: %v", err)
	}

	if err := store.Rename("useful-title"); err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "session-temp.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("expected old session file to be moved, stat err=%v", err)
	}
	if store.Path() != filepath.Join(root, "useful-title.jsonl") {
		t.Fatalf("unexpected renamed path %q", store.Path())
	}

	second, err := model.NewMessage(first.ID, "assistant", "hi", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if err := store.Append(second); err != nil {
		t.Fatalf("append second: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("expected both entries in renamed file, got %#v", loaded.Entries)
	}
}

func TestNameFromTitleCreatesSafeSessionName(t *testing.T) {
	t.Parallel()

	if got := NameFromTitle("Useful Session: Checkpoints / Branches"); got != "useful-session-checkpoints-branches" {
		t.Fatalf("unexpected name %q", got)
	}
	if got := NameFromTitle(" \n "); got != "session" {
		t.Fatalf("expected fallback name, got %q", got)
	}
}

func TestStoreAvailableNameAvoidsExistingSessionFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := Open(root, "session-temp")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "useful-title.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write existing session: %v", err)
	}

	if got := store.AvailableName("Useful Title"); got != "useful-title-2" {
		t.Fatalf("expected conflict suffix, got %q", got)
	}
}

func TestStoreRejectsUnsafeSessionNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "../work", "nested/work", `nested\work`} {
		_, err := Open(t.TempDir(), name)
		if err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}
