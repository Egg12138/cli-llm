package runtime

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeAppendStore struct {
	entries []model.Entry
}

func (s *fakeAppendStore) Append(entry model.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

type fakeChatModel struct {
	streamErr error
	chunks    []string
	inputs    [][]*schema.Message
}

func (m *fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	return schema.AssistantMessage("generated", nil), nil
}

func (m *fakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	m.inputs = append(m.inputs, input)
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	messages := make([]*schema.Message, 0, len(m.chunks))
	for _, chunk := range m.chunks {
		messages = append(messages, schema.AssistantMessage(chunk, nil))
	}
	return schema.StreamReaderFromArray(messages), nil
}

func TestChatTurnAppendsUserAssistantAndCheckpoint(t *testing.T) {
	t.Parallel()

	state := graph.NewState(nil)
	store := &fakeAppendStore{}
	chatModel := &fakeChatModel{chunks: []string{"hel", "lo"}}
	var out bytes.Buffer
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	err := RunChatTurn(context.Background(), ChatTurnRequest{
		Input:       "say hello",
		State:       state,
		Store:       store,
		Writer:      &out,
		Model:       chatModel,
		SessionName: "work",
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RunChatTurn returned error: %v", err)
	}

	if out.String() != "hello\n" {
		t.Fatalf("expected streamed writer output hello newline, got %q", out.String())
	}
	if len(store.entries) != 4 {
		t.Fatalf("expected user assistant checkpoint and branch pointer entries, got %#v", store.entries)
	}
	if store.entries[0].Type != model.EntryTypeMessage || mustMessageData(t, store.entries[0]).Role != "user" {
		t.Fatalf("expected user entry first, got %#v", store.entries[0])
	}
	if store.entries[1].Type != model.EntryTypeMessage || mustMessageData(t, store.entries[1]).Role != "assistant" {
		t.Fatalf("expected assistant entry second, got %#v", store.entries[1])
	}
	if store.entries[2].Type != model.EntryTypeCheckpoint {
		t.Fatalf("expected checkpoint third, got %#v", store.entries[2])
	}
	if store.entries[3].Type != model.EntryTypeCheckpoint {
		t.Fatalf("expected branch pointer checkpoint fourth, got %#v", store.entries[3])
	}
	checkpoint, err := store.entries[2].CheckpointData()
	if err != nil {
		t.Fatalf("CheckpointData returned error: %v", err)
	}
	if checkpoint.ReturnTo != store.entries[1].ID {
		t.Fatalf("expected checkpoint returnTo assistant %q, got %q", store.entries[1].ID, checkpoint.ReturnTo)
	}
	pointer, err := store.entries[3].CheckpointData()
	if err != nil {
		t.Fatalf("branch pointer CheckpointData returned error: %v", err)
	}
	if pointer.Name != "main" || pointer.ReturnTo != store.entries[2].ID {
		t.Fatalf("unexpected branch pointer %#v", pointer)
	}
	if state.HeadID != store.entries[2].ID || state.Branches["main"].HeadID != store.entries[2].ID {
		t.Fatalf("state head not advanced to checkpoint: %#v", state)
	}
	if got := chatModel.inputs[0][len(chatModel.inputs[0])-1].Content; got != "say hello" {
		t.Fatalf("expected model context to include user input, got %q", got)
	}
}

func TestChatTurnPersistsCurrentBranchHead(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	root, err := model.NewMessage("", "user", "root", now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	state := graph.NewState([]model.Entry{root})
	if err := state.Switch("experiment"); err != nil {
		t.Fatalf("Switch returned error: %v", err)
	}
	store := &fakeAppendStore{}
	chatModel := &fakeChatModel{chunks: []string{"branch answer"}}

	err = RunChatTurn(context.Background(), ChatTurnRequest{
		Input:      "continue branch",
		State:      state,
		Store:      store,
		Writer:     &bytes.Buffer{},
		Model:      chatModel,
		TitleModel: nil,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RunChatTurn returned error: %v", err)
	}

	reloaded := graph.NewState(append([]model.Entry{root}, store.entries...))
	if reloaded.Branches["experiment"].HeadID != state.HeadID {
		t.Fatalf("expected experiment to reload at %q, got %#v", state.HeadID, reloaded.Branches["experiment"])
	}
}

func TestChatTurnReportsStatusInOrder(t *testing.T) {
	t.Parallel()

	state := graph.NewState(nil)
	store := &fakeAppendStore{}
	chatModel := &fakeChatModel{chunks: []string{"hel", "lo"}}
	var out bytes.Buffer
	fake := &fakeStatusReporter{}

	err := RunChatTurn(context.Background(), ChatTurnRequest{
		Input:  "hi",
		State:  state,
		Store:  store,
		Writer: &out,
		Model:  chatModel,
		Status: fake,
		Now:    func() time.Time { return time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("RunChatTurn returned error: %v", err)
	}

	if len(fake.calls) < 3 {
		t.Fatalf("expected at least 3 status calls, got %d: %v", len(fake.calls), fake.calls)
	}
	if fake.calls[0] != StatusThinking {
		t.Fatalf("first status should be StatusThinking, got %v", fake.calls[0])
	}
	if fake.calls[1] != StatusWaitingStream {
		t.Fatalf("second status should be StatusWaitingStream, got %v", fake.calls[1])
	}
	if fake.calls[2] != StatusStreaming {
		t.Fatalf("third status should be StatusStreaming, got %v", fake.calls[2])
	}
	if !fake.clear {
		t.Fatal("expected Clear() to be called")
	}
}

func TestChatTurnFailedModelLeavesNoAssistantOrCheckpoint(t *testing.T) {
	t.Parallel()

	state := graph.NewState(nil)
	store := &fakeAppendStore{}
	chatModel := &fakeChatModel{streamErr: errors.New("boom")}

	err := RunChatTurn(context.Background(), ChatTurnRequest{
		Input:       "say hello",
		State:       state,
		Store:       store,
		Writer:      &bytes.Buffer{},
		Model:       chatModel,
		SessionName: "work",
		Now:         func() time.Time { return time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC) },
	})
	if err == nil {
		t.Fatalf("expected model error")
	}
	if len(store.entries) != 1 {
		t.Fatalf("expected only user entry after failed model, got %#v", store.entries)
	}
	if store.entries[0].Type != model.EntryTypeMessage || mustMessageData(t, store.entries[0]).Role != "user" {
		t.Fatalf("expected user entry, got %#v", store.entries[0])
	}
}

func mustMessageData(t *testing.T, entry model.Entry) model.MessageData {
	t.Helper()
	data, err := entry.MessageData()
	if err != nil {
		t.Fatalf("MessageData returned error: %v", err)
	}
	return data
}
