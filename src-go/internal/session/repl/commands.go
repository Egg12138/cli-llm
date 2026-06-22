package repl

import (
	"fmt"
	"io"
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
)

type CommandKind string

const (
	CommandChat        CommandKind = "chat"
	CommandExit        CommandKind = "exit"
	CommandBranches    CommandKind = "branches"
	CommandSwitch      CommandKind = "switch"
	CommandCheckpoint  CommandKind = "checkpoint"
	CommandTranscript  CommandKind = "transcript"
	CommandUnknown     CommandKind = "unknown"
)

type Action string

const (
	ActionContinue   Action = "continue"
	ActionChat       Action = "chat"
	ActionExit       Action = "exit"
	ActionTranscript Action = "transcript"
)

type Command struct {
	Kind CommandKind
	Arg  string
	Line string
}

func ParseLine(line string) Command {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "/") {
		return Command{Kind: CommandChat, Line: line}
	}
	name, arg, _ := strings.Cut(trimmed, " ")
	switch name {
	case "/exit":
		return Command{Kind: CommandExit, Line: line}
	case "/branches":
		return Command{Kind: CommandBranches, Line: line}
	case "/switch":
		return Command{Kind: CommandSwitch, Arg: strings.TrimSpace(arg), Line: line}
	case "/checkpoint":
		return Command{Kind: CommandCheckpoint, Arg: strings.TrimSpace(arg), Line: line}
	case "/t", "/transcript":
		return Command{Kind: CommandTranscript, Line: line}
	default:
		return Command{Kind: CommandUnknown, Arg: strings.TrimPrefix(name, "/"), Line: line}
	}
}

func ExecuteCommand(state *graph.State, command Command, out io.Writer) (Action, error) {
	return ExecuteCommandWithStore(state, command, out, nil)
}

func ExecuteCommandWithStore(state *graph.State, command Command, out io.Writer, store CommandStore) (Action, error) {
	switch command.Kind {
	case CommandChat:
		return ActionChat, nil
	case CommandExit:
		return ActionExit, nil
	case CommandBranches:
		for _, branch := range state.ListBranches() {
			if branch.Parent != "" {
				fmt.Fprintf(out, "%s (from %s)\n", branch.Name, branch.Parent)
			} else {
				fmt.Fprintf(out, "%s\n", branch.Name)
			}
		}
		return ActionContinue, nil
	case CommandSwitch:
		_, existed := state.Branches[command.Arg]
		_, isHash := state.Entries[command.Arg]
		if err := state.Switch(command.Arg); err != nil {
			return ActionContinue, err
		}
		if !existed && !isHash {
			if err := persistNamedCheckpoint(state, store, command.Arg); err != nil {
				return ActionContinue, err
			}
		}
		fmt.Fprintf(out, "switched to %s\n", command.Arg)
		return ActionContinue, nil
	case CommandCheckpoint:
		if err := state.LabelCheckpoint(command.Arg); err != nil {
			return ActionContinue, err
		}
		if err := persistNamedCheckpoint(state, store, command.Arg); err != nil {
			return ActionContinue, err
		}
		fmt.Fprintf(out, "checkpoint %s -> %s\n", command.Arg, state.HeadID)
		return ActionContinue, nil
	case CommandTranscript:
		return ActionTranscript, nil
	case CommandUnknown:
		fmt.Fprintf(out, "unknown command: /%s\n", command.Arg)
		return ActionContinue, nil
	default:
		return ActionContinue, fmt.Errorf("unsupported command kind %q", command.Kind)
	}
}
