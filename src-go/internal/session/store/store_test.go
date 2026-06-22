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

func TestStoreRejectsUnsafeSessionNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "../work", "nested/work", `nested\work`} {
		_, err := Open(t.TempDir(), name)
		if err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}
