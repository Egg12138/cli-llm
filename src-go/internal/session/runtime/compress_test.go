package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	sessionstore "github.com/Egg12138/cli-llm/src-go/internal/session/store"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeSummarizer struct {
	err   error
	calls int
}

func (s *fakeSummarizer) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return schema.AssistantMessage("compressed summary", nil), nil
}

func (s *fakeSummarizer) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{}), nil
}

func TestCompressionBelowThresholdDoesNothing(t *testing.T) {
	t.Parallel()

	state := graph.NewState([]model.Entry{mustRuntimeMessage(t, "", "user", "short", time.Now())})
	store := &fakeAppendStore{}
	summarizer := &fakeSummarizer{}

	compressed, err := MaybeCompress(context.Background(), CompressionRequest{
		State:            state,
		Store:            store,
		Summarizer:       summarizer,
		ModelName:        "gpt-4o-mini",
		ThresholdTokens:  1000,
		KeepRecentTokens: 100,
		Now:              func() time.Time { return time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("MaybeCompress returned error: %v", err)
	}
	if compressed {
		t.Fatalf("expected no compression")
	}
	if summarizer.calls != 0 || len(store.entries) != 0 {
		t.Fatalf("unexpected summarizer/store activity calls=%d entries=%#v", summarizer.calls, store.entries)
	}
}

func TestCompressionAddsCompactionAtTheLeafAndKeepsRecentMessages(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	old := mustRuntimeMessage(t, "", "user", strings.Repeat("old context ", 40), now)
	recent := mustRuntimeMessage(t, old.ID, "assistant", "recent answer", now)
	state := graph.NewState([]model.Entry{old, recent})
	state.SetHead(recent.ID)
	store := &fakeAppendStore{}
	summarizer := &fakeSummarizer{}

	compressed, err := MaybeCompress(context.Background(), CompressionRequest{
		State:            state,
		Store:            store,
		Summarizer:       summarizer,
		ModelName:        "gpt-4o-mini",
		ThresholdTokens:  1,
		KeepRecentTokens: 1,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("MaybeCompress returned error: %v", err)
	}
	if !compressed {
		t.Fatalf("expected compression")
	}
	if len(store.entries) != 1 || store.entries[0].Type != model.EntryTypeCompaction {
		t.Fatalf("expected compaction entry, got %#v", store.entries)
	}
	compaction := store.entries[0]
	data, err := compaction.CompactionData()
	if err != nil {
		t.Fatalf("CompactionData returned error: %v", err)
	}
	if data.Summary != "compressed summary" || data.FirstKeptEntryID != recent.ID || data.TokensBefore <= 0 {
		t.Fatalf("unexpected compaction data %#v", data)
	}
	if compaction.ParentID != recent.ID {
		t.Fatalf("compaction parent = %q, want the previous leaf %q", compaction.ParentID, recent.ID)
	}
	if state.HeadID != compaction.ID {
		t.Fatalf("state head = %q, want compaction %q", state.HeadID, compaction.ID)
	}
	history, err := state.ReachableHistory()
	if err != nil {
		t.Fatalf("ReachableHistory returned error: %v", err)
	}
	projected := ProjectHistory(history)
	if len(projected) != 2 || projected[0].Type != model.EntryTypeCompaction || projected[1].ID != recent.ID {
		t.Fatalf("expected projected compaction -> recent chain, got %#v", projected)
	}
}

func TestCompressionSkipsWhenTheRecentWindowCoversAllMessages(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	old := mustRuntimeMessage(t, "", "user", "old message", now)
	recent := mustRuntimeMessage(t, old.ID, "assistant", "recent answer", now)
	state := graph.NewState([]model.Entry{old, recent})
	store := &fakeAppendStore{}
	summarizer := &fakeSummarizer{}

	compressed, err := MaybeCompress(context.Background(), CompressionRequest{
		State:            state,
		Store:            store,
		Summarizer:       summarizer,
		ModelName:        "gpt-4o-mini",
		ThresholdTokens:  1,
		KeepRecentTokens: 100000,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("MaybeCompress returned error: %v", err)
	}
	if compressed {
		t.Fatalf("compaction with nothing to summarize should be skipped")
	}
	if summarizer.calls != 0 || len(store.entries) != 0 {
		t.Fatalf("unexpected activity calls=%d entries=%#v", summarizer.calls, store.entries)
	}
}

func TestCompactionProjectionSurvivesReload(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	old := mustRuntimeMessage(t, "", "user", strings.Repeat("old diagnostic context ", 40), now)
	recent := mustRuntimeMessage(t, old.ID, "assistant", "recent answer", now)

	store, err := sessionstore.Open(t.TempDir(), "work")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	for _, entry := range []model.Entry{old, recent} {
		if err := store.Append(entry); err != nil {
			t.Fatalf("append seed entry: %v", err)
		}
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	state := graph.NewState(loaded.Entries)
	summarizer := &fakeSummarizer{}
	if _, err := MaybeCompress(context.Background(), CompressionRequest{
		State:            state,
		Store:            &store,
		Summarizer:       summarizer,
		ModelName:        "gpt-4o-mini",
		ThresholdTokens:  1,
		KeepRecentTokens: 1,
		Now:              func() time.Time { return now },
	}); err != nil {
		t.Fatalf("MaybeCompress returned error: %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload returned error: %v", err)
	}
	reloadedState := graph.NewState(reloaded.Entries)
	messages, err := BuildMessages(*reloadedState, ContextOptions{})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}
	var joined strings.Builder
	for _, message := range messages {
		joined.WriteString(message.Content)
		joined.WriteByte('\n')
	}
	output := joined.String()
	if !strings.Contains(output, "compressed summary") {
		t.Fatalf("reloaded context lost the summary: %s", output)
	}
	if !strings.Contains(output, "recent answer") {
		t.Fatalf("reloaded context lost the kept message: %s", output)
	}
	if strings.Contains(output, "old diagnostic context") {
		t.Fatalf("reloaded context still contains summarized history: %s", output)
	}
}

// BenchmarkMaybeCompressShortHistory covers the per-turn check on an ordinary
// session. It must never build the tokenizer, so it stays in the microsecond
// range; a regression here reintroduces startup latency on every llm-session
// invocation (see docs/benchmarks/).
func BenchmarkMaybeCompressShortHistory(b *testing.B) {
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	entries := make([]model.Entry, 0, 6)
	parent := ""
	for i := 0; i < 6; i++ {
		entry, err := model.NewMessage(parent, "user", strings.Repeat("short context ", 10), now)
		if err != nil {
			b.Fatalf("NewMessage returned error: %v", err)
		}
		entries = append(entries, entry)
		parent = entry.ID
	}
	summarizer := &fakeSummarizer{}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := MaybeCompress(context.Background(), CompressionRequest{
			State:      graph.NewState(entries),
			Store:      &fakeAppendStore{},
			Summarizer: summarizer,
			ModelName:  "bench-model",
		}); err != nil {
			b.Fatalf("MaybeCompress returned error: %v", err)
		}
	}
}
