package repl

import (
	"errors"
	"fmt"
	"io"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
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

type LoopOptions struct {
	Reader  InputReader
	Chat    ChatRunner
	State   *graph.State
	Stdout  io.Writer
	Stderr  io.Writer
	Overlay TranscriptOverlay
}

func Loop(opts LoopOptions) int {
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}
	for {
		event := readInputEvent(opts.Reader, "> ")
		if event.Kind == EventTranscript {
			if opts.Overlay != nil {
				if err := opts.Overlay.Open(opts.State, out); err != nil {
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
		action, err := ExecuteCommand(opts.State, command, out)
		if err != nil {
			fmt.Fprintln(out, err)
			continue
		}
		switch action {
		case ActionExit:
			return 0
		case ActionChat:
			if opts.Chat == nil {
				fmt.Fprintln(out, "chat runner is not configured")
				return 1
			}
			if err := opts.Chat.RunChat(line); err != nil {
				fmt.Fprintln(out, err)
				return 1
			}
		}
	}
}

func readInputEvent(reader InputReader, prompt string) InputEvent {
	if eventReader, ok := reader.(InputEventReader); ok {
		return eventReader.ReadEvent(prompt)
	}
	line, err := reader.ReadLine(prompt)
	return InputEvent{Kind: EventLine, Line: line, Err: err}
}
