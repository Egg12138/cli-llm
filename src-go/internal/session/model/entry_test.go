package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEntryStableIDUsesCanonicalContent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	first, err := NewMessage("parent-a", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	second, err := NewMessage("parent-a", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected stable IDs, got %q and %q", first.ID, second.ID)
	}
	if len(first.ID) != 12 {
		t.Fatalf("expected 12-char ID, got %q", first.ID)
	}
	for _, r := range first.ID {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			t.Fatalf("expected lowercase hex ID, got %q", first.ID)
		}
	}
}

func TestEntryHashIncludesParentID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	first, err := NewMessage("parent-a", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	second, err := NewMessage("parent-b", "user", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}

	if first.ID == second.ID {
		t.Fatalf("expected different IDs when parent changes, got %q", first.ID)
	}
}

func TestEntryUserMessagePreviewUsesFirstSixteenRunes(t *testing.T) {
	t.Parallel()

	entry, err := NewMessage("", "user", "abcdefghijklmnopq世界", time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}

	preview, err := entry.Preview()
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if preview != "abcdefghijklmnop" {
		t.Fatalf("expected first 16 runes, got %q", preview)
	}
}

func TestEntryCheckpointBranchesAreMetadataPointers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	message, err := NewMessage("", "user", "write a parser", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	checkpoint, err := NewCheckpoint(message.ID, "explore", message.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint returned error: %v", err)
	}

	data, err := checkpoint.CheckpointData()
	if err != nil {
		t.Fatalf("CheckpointData returned error: %v", err)
	}
	if data.Name != "explore" {
		t.Fatalf("expected checkpoint name explore, got %q", data.Name)
	}
	if data.ReturnTo != message.ID {
		t.Fatalf("expected checkpoint returnTo %q, got %q", message.ID, data.ReturnTo)
	}
	preview, err := message.Preview()
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if preview != "write a parser" {
		t.Fatalf("checkpoint should not rename message preview, got %q", preview)
	}
}

func TestEntryJSONRoundTripPreservesTypeAndData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	original, err := NewSessionInfo("", SessionInfo{
		Title:   "Work",
		Created: now,
		Model:   "gpt-4o-mini",
	}, now)
	if err != nil {
		t.Fatalf("NewSessionInfo returned error: %v", err)
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	var decoded Entry
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}

	if decoded.ID != original.ID {
		t.Fatalf("expected ID %q, got %q", original.ID, decoded.ID)
	}
	if decoded.Type != EntryTypeSessionInfo {
		t.Fatalf("expected type %q, got %q", EntryTypeSessionInfo, decoded.Type)
	}
	info, err := decoded.SessionInfoData()
	if err != nil {
		t.Fatalf("SessionInfoData returned error: %v", err)
	}
	if info.Title != "Work" || info.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected session info %#v", info)
	}
}
