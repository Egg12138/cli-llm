package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
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

func TestCompressionAddsCompactionAndKeepsRecentMessagesReachable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	old := mustRuntimeMessage(t, "", "user", "old message with many words", now)
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
		KeepRecentTokens: 1000,
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
	data, err := store.entries[0].CompactionData()
	if err != nil {
		t.Fatalf("CompactionData returned error: %v", err)
	}
	if data.Summary != "compressed summary" || data.FirstKeptEntryID != recent.ID || data.TokensBefore <= 0 {
		t.Fatalf("unexpected compaction data %#v", data)
	}
	history, err := state.ReachableHistory()
	if err != nil {
		t.Fatalf("ReachableHistory returned error: %v", err)
	}
	if len(history) != 2 || history[0].Type != model.EntryTypeCompaction || history[1].ID != recent.ID {
		t.Fatalf("expected compaction -> recent chain, got %#v", history)
	}
}
