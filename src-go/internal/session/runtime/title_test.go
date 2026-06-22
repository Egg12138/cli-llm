package runtime

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeTitleModel struct {
	response string
	calls    int
}

func (m *fakeTitleModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	m.calls++
	return schema.AssistantMessage(m.response, nil), nil
}

func (m *fakeTitleModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("answer", nil)}), nil
}

func TestTitleTrimsOneLineAndSixtyRunes(t *testing.T) {
	t.Parallel()

	title := NormalizeTitle("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 extra\nsecond line", "fallback")
	if title != "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567" {
		t.Fatalf("unexpected normalized title %q", title)
	}
}

func TestTitleFallsBackToPromptPreview(t *testing.T) {
	t.Parallel()

	title := NormalizeTitle(" \n", "abcdefghijklmnopq")
	if title != "abcdefghijklmnop" {
		t.Fatalf("expected prompt preview fallback, got %q", title)
	}
}

func TestChatTurnTitlesFirstPromptAndDoesNotRetitleLater(t *testing.T) {
	t.Parallel()

	state := graph.NewState(nil)
	store := &fakeAppendStore{}
	chatModel := &fakeTitleModel{response: "Useful title\nignored"}
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	err := RunChatTurn(context.Background(), ChatTurnRequest{
		Input:       "explain checkpoint storage",
		State:       state,
		Store:       store,
		Writer:      &bytes.Buffer{},
		Model:       chatModel,
		TitleModel:  chatModel,
		SessionName: "work",
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RunChatTurn first returned error: %v", err)
	}
	if chatModel.calls != 1 {
		t.Fatalf("expected one title call, got %d", chatModel.calls)
	}
	if len(store.entries) == 0 || store.entries[0].Type != model.EntryTypeSessionInfo {
		t.Fatalf("expected first appended entry to be session info, got %#v", store.entries)
	}
	info, err := store.entries[0].SessionInfoData()
	if err != nil {
		t.Fatalf("SessionInfoData returned error: %v", err)
	}
	if info.Title != "Useful title" {
		t.Fatalf("expected useful title, got %q", info.Title)
	}

	err = RunChatTurn(context.Background(), ChatTurnRequest{
		Input:       "second turn",
		State:       state,
		Store:       store,
		Writer:      &bytes.Buffer{},
		Model:       chatModel,
		TitleModel:  chatModel,
		SessionName: "work",
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RunChatTurn second returned error: %v", err)
	}
	if chatModel.calls != 1 {
		t.Fatalf("expected no retitle call, got %d", chatModel.calls)
	}
}
