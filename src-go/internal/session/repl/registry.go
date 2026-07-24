package repl

import (
	"fmt"
	"io"
	"strings"
)

type CommandSpec struct {
	Name        string
	Aliases     []string
	Usage       string
	Description string
	Kind        CommandKind
}

var enabledCommands = []CommandSpec{
	{Name: "help", Usage: "/help", Description: "List enabled commands", Kind: CommandHelp},
	{Name: "exit", Usage: "/exit", Description: "Save the session and exit", Kind: CommandExit},
	{Name: "transcript", Aliases: []string{"t"}, Usage: "/transcript", Description: "Open the transcript overlay", Kind: CommandTranscript},
	{Name: "branches", Usage: "/branches", Description: "List session branches", Kind: CommandBranches},
	{Name: "switch", Usage: "/switch <target>", Description: "Switch to a branch or checkpoint", Kind: CommandSwitch},
	{Name: "checkpoint", Usage: "/checkpoint <name>", Description: "Label the current position", Kind: CommandCheckpoint},
}

func EnabledCommands() []CommandSpec {
	commands := make([]CommandSpec, len(enabledCommands))
	for i, command := range enabledCommands {
		commands[i] = command
		commands[i].Aliases = append([]string(nil), command.Aliases...)
	}
	return commands
}

func LookupCommand(name string) (CommandSpec, bool) {
	name = strings.TrimPrefix(name, "/")
	for _, command := range enabledCommands {
		if command.Name == name {
			return command, true
		}
		for _, alias := range command.Aliases {
			if alias == name {
				return command, true
			}
		}
	}
	return CommandSpec{}, false
}

func WriteHelp(out io.Writer) {
	fmt.Fprintln(out, "Available commands:")
	for _, command := range enabledCommands {
		label := command.Usage
		if len(command.Aliases) > 0 {
			aliases := make([]string, len(command.Aliases))
			for i, alias := range command.Aliases {
				aliases[i] = "/" + alias
			}
			label += " (" + strings.Join(aliases, ", ") + ")"
		}
		fmt.Fprintf(out, "  %-24s %s\n", label, command.Description)
	}
}
