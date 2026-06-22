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
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	if loaded[0].ID != first.ID || loaded[1].ID != second.ID {
		t.Fatalf("entries loaded out of order: %#v", loaded)
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

	_, err = store.Load()
	if err == nil {
		t.Fatalf("expected corrupt JSONL error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected line number in error, got %v", err)
	}
}

func TestStoreLoadMissingFileReturnsEmptyEntries(t *testing.T) {
	t.Parallel()

	store, err := Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	entries, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty entries, got %#v", entries)
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
	if len(loaded) != 2 {
		t.Fatalf("expected both entries in renamed file, got %#v", loaded)
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
