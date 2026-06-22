package repl

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
)

type fakeReader struct {
	lines  []string
	errs   []error
	events []InputEvent
	index  int
}

func (r *fakeReader) ReadLine(prompt string) (string, error) {
	if r.index < len(r.errs) && r.errs[r.index] != nil {
		err := r.errs[r.index]
		r.index++
		return "", err
	}
	if r.index >= len(r.lines) {
		return "", io.EOF
	}
	line := r.lines[r.index]
	r.index++
	return line, nil
}

func (r *fakeReader) ReadEvent(prompt string) InputEvent {
	if r.events == nil {
		line, err := r.ReadLine(prompt)
		return InputEvent{Kind: EventLine, Line: line, Err: err}
	}
	if r.index >= len(r.events) {
		return InputEvent{Kind: EventLine, Err: io.EOF}
	}
	event := r.events[r.index]
	r.index++
	return event
}

type fakeChatRunner struct {
	inputs []string
}

func (r *fakeChatRunner) RunChat(input string) error {
	r.inputs = append(r.inputs, input)
	return nil
}

type fakeOverlay struct {
	opens int
}

func (o *fakeOverlay) Open(state *graph.State, out io.Writer) error {
	o.opens++
	return nil
}

func TestREPLRunsChatLinesAndSlashCommands(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{lines: []string{"hello", "/branches", "/exit"}}
	chat := &fakeChatRunner{}
	overlay := &fakeOverlay{}
	state := graph.NewState(nil)
	var out bytes.Buffer

	code := Loop(LoopOptions{
		Reader:  reader,
		Chat:    chat,
		State:   state,
		Stdout:  &out,
		Overlay: overlay,
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(chat.inputs) != 1 || chat.inputs[0] != "hello" {
		t.Fatalf("expected one chat input hello, got %#v", chat.inputs)
	}
	if overlay.opens != 0 {
		t.Fatalf("normal mode should not open overlay")
	}
	if !bytes.Contains(out.Bytes(), []byte("main")) {
		t.Fatalf("expected branches output in stdout, got %q", out.String())
	}
}

func TestREPLCtrlDExitsCleanly(t *testing.T) {
	t.Parallel()

	code := Loop(LoopOptions{
		Reader: &fakeReader{},
		Chat:   &fakeChatRunner{},
		State:  graph.NewState(nil),
		Stdout: &bytes.Buffer{},
	})
	if code != 0 {
		t.Fatalf("expected clean EOF exit code 0, got %d", code)
	}
}

func TestREPLInterruptReturns130(t *testing.T) {
	t.Parallel()

	code := Loop(LoopOptions{
		Reader: &fakeReader{errs: []error{ErrInterrupted}},
		Chat:   &fakeChatRunner{},
		State:  graph.NewState(nil),
		Stdout: &bytes.Buffer{},
	})
	if code != 130 {
		t.Fatalf("expected interrupt exit code 130, got %d", code)
	}
}

func TestREPLChatErrorReturnsOne(t *testing.T) {
	t.Parallel()

	chat := chatRunnerFunc(func(string) error { return errors.New("boom") })
	code := Loop(LoopOptions{
		Reader: &fakeReader{lines: []string{"hello"}},
		Chat:   chat,
		State:  graph.NewState(nil),
		Stdout: &bytes.Buffer{},
	})
	if code != 1 {
		t.Fatalf("expected chat error exit code 1, got %d", code)
	}
}

func TestREPLTranscriptOverlayReturnsToNormalLoop(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{events: []InputEvent{
		{Kind: EventTranscript},
		{Kind: EventLine, Line: "after overlay"},
		{Kind: EventLine, Line: "/exit"},
	}}
	chat := &fakeChatRunner{}
	overlay := &fakeOverlay{}
	state := graph.NewState(nil)
	var out bytes.Buffer

	code := Loop(LoopOptions{
		Reader:  reader,
		Chat:    chat,
		State:   state,
		Stdout:  &out,
		Overlay: overlay,
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if overlay.opens != 1 {
		t.Fatalf("expected one overlay open, got %d", overlay.opens)
	}
	if len(chat.inputs) != 1 || chat.inputs[0] != "after overlay" {
		t.Fatalf("expected chat after overlay, got %#v", chat.inputs)
	}
}
