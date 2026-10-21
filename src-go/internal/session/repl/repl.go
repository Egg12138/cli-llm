package repl

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

var ErrInterrupted = errors.New("interrupted")

type InputReader interface {
	ReadLine(prompt string) (string, error)
}

type InputEventReader interface {
	ReadEvent(prompt string) InputEvent
}

type EventKind string

const (
	EventLine       EventKind = "line"
	EventTranscript EventKind = "transcript"
)

type InputEvent struct {
	Kind EventKind
	Line string
	Err  error
}

type ChatRunner interface {
	RunChat(input string) error
}

type chatRunnerFunc func(string) error

func (fn chatRunnerFunc) RunChat(input string) error {
	return fn(input)
}

type TranscriptOverlay interface {
	Open(state *graph.State, out io.Writer) error
}

type CommandStore interface {
	Append(entry model.Entry) error
}

// ResumeResult reports the session a /resume command switched to. The loop
// adopts the returned state and store so later commands and the transcript
// overlay act on the resumed session.
type ResumeResult struct {
	State *graph.State
	Store CommandStore
}

type ResumeFunc func(arg string) (ResumeResult, error)

type LoopOptions struct {
	Reader        InputReader
	Chat          ChatRunner
	State         *graph.State
	Prompt        string
	Stdout        io.Writer
	CommandOutput io.Writer
	Stderr        io.Writer
	Overlay       TranscriptOverlay
	Store         CommandStore
	Resume        ResumeFunc
	ReportError   func(error)
}

func Loop(opts LoopOptions) int {
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}
	commandOut := opts.CommandOutput
	if commandOut == nil {
		commandOut = out
	}
	prompt := opts.Prompt
	if prompt == "" {
		prompt = "> "
	}
	state := opts.State
	store := opts.Store
	for {
		event := readInputEvent(opts.Reader, prompt)
		if event.Kind == EventTranscript {
			if opts.Overlay != nil {
				if err := opts.Overlay.Open(state, out); err != nil {
					fmt.Fprintln(out, err)
					return 1
				}
			}
			continue
		}
		line := event.Line
		err := event.Err
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0
			}
			if errors.Is(err, ErrInterrupted) {
				return 130
			}
			return 1
		}
		command := ParseLine(line)
		output := commandOut
		if command.Kind == CommandUnknown {
			output = out
		}
		action, err := ExecuteCommandWithStore(state, command, output, store)
		if err != nil {
			fmt.Fprintln(out, err)
			continue
		}
		switch action {
		case ActionExit:
			return 0
		case ActionTranscript:
			if opts.Overlay != nil {
				if err := opts.Overlay.Open(state, out); err != nil {
					fmt.Fprintln(out, err)
					return 1
				}
			}
		case ActionResume:
			if opts.Resume == nil {
				fmt.Fprintln(out, "resume is not available in this session")
				continue
			}
			result, err := opts.Resume(command.Arg)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			if result.State != nil {
				state = result.State
			}
			if result.Store != nil {
				store = result.Store
			}
		case ActionChat:
			if opts.Chat == nil {
				fmt.Fprintln(out, "chat runner is not configured")
				return 1
			}
			if err := opts.Chat.RunChat(line); err != nil {
				if opts.ReportError != nil {
					opts.ReportError(err)
				} else {
					fmt.Fprintln(out, err)
				}
				continue
			}
		}
	}
}

func persistNamedCheckpoint(state *graph.State, store CommandStore, name string) error {
	if store == nil || name == "" {
		return nil
	}
	entry, err := model.NewCheckpoint(state.HeadID, name, state.HeadID, time.Now())
	if err != nil {
		return err
	}
	if err := store.Append(entry); err != nil {
		return err
	}
	return state.AddEntry(entry)
}

func readInputEvent(reader InputReader, prompt string) InputEvent {
	if eventReader, ok := reader.(InputEventReader); ok {
		return eventReader.ReadEvent(prompt)
	}
	line, err := reader.ReadLine(prompt)
	return InputEvent{Kind: EventLine, Line: line, Err: err}
}
