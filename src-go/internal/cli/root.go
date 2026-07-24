package cli

import (
	"fmt"
	"os"
	"syscall"

	"github.com/Egg12138/cli-llm/src-go/internal/plugins"
)

type ExecutionMode string

const (
	ModeBuiltin ExecutionMode = "builtin"
	ModePlugin  ExecutionMode = "plugin"
)

type ExecutionPlan struct {
	Mode       ExecutionMode
	Args       []string
	PluginPath string
}

// Version is the current cli-llm release, kept in sync with pyproject.toml.
const Version = "0.4.3"

var (
	builtinSubcommands = []string{"chat", "inspect", "provider", "toolcall"}
	passthroughFlags   = map[string]struct{}{
		"-h":        {},
		"--help":    {},
		"-V":        {},
		"--version": {},
	}
)

func PrepareExecution(args []string, lookPath func(string) (string, error)) (ExecutionPlan, error) {
	if len(args) == 0 {
		return ExecutionPlan{
			Mode: ModeBuiltin,
			Args: []string{"chat"},
		}, nil
	}

	if _, ok := passthroughFlags[args[0]]; ok {
		return ExecutionPlan{
			Mode: ModeBuiltin,
			Args: append([]string(nil), args...),
		}, nil
	}

	if replacement, ok := commandAliases[args[0]]; ok {
		resolved := make([]string, len(args))
		resolved[0] = replacement
		copy(resolved[1:], args[1:])
		args = resolved
	}

	dispatchResult := plugins.NewDispatcher(builtinSubcommands, lookPath).Resolve(args)
	switch dispatchResult.Kind {
	case plugins.Builtin:
		return ExecutionPlan{
			Mode: ModeBuiltin,
			Args: append([]string(nil), args...),
		}, nil
	case plugins.Plugin:
		return ExecutionPlan{
			Mode:       ModePlugin,
			Args:       dispatchResult.ExecArgs,
			PluginPath: dispatchResult.Path,
		}, nil
	}

	forwarded := make([]string, 0, len(args)+1)
	forwarded = append(forwarded, "chat")
	forwarded = append(forwarded, args...)
	return ExecutionPlan{
		Mode: ModeBuiltin,
		Args: forwarded,
	}, nil
}

func Execute(args []string, lookPath func(string) (string, error)) (int, error) {
	plan, err := PrepareExecution(args, lookPath)
	if err != nil {
		return 1, err
	}

	if plan.Mode == ModePlugin {
		if err := syscall.Exec(plan.PluginPath, plan.Args, os.Environ()); err != nil {
			return 1, fmt.Errorf("exec plugin %s: %w", plan.PluginPath, err)
		}
		return 0, nil
	}

	return runBuiltin(plan.Args)
}

func runBuiltin(args []string) (int, error) {
	if len(args) == 0 {
		return runChatCommand([]string{"chat"}), nil
	}

	switch args[0] {
	case "-h", "--help":
		return runRootHelp(), nil
	case "-V", "--version":
		return runRootVersion(), nil
	case "chat":
		return runChatCommand(args), nil
	case "inspect":
		return runInspectCommand(args), nil
	case "provider":
		return runProviderCommand(args), nil
	case "toolcall":
		return runToolcallCommand(args), nil
	default:
		return runChatCommand(args), nil
	}
}

func runRootHelp() int {
	fmt.Fprintf(commandStdout, "cli-llm %s\n\n", Version)
	fmt.Fprintln(commandStdout, "Usage: llm [OPTIONS] COMMAND [ARGS]...")
	fmt.Fprintln(commandStdout)
	fmt.Fprintln(commandStdout, "Commands:")

	aliasOf := make(map[string]string)
	for alias, cmd := range commandAliases {
		aliasOf[cmd] = alias
	}

	entries := []struct {
		name, desc string
	}{
		{"chat", "Run a chat request"},
		{"inspect", "Inspect provider profiles"},
		{"provider", "Inspect provider metadata"},
		{"session", "Run the llm-session plugin"},
		{"toolcall", "Run a single tool-call request"},
	}
	for _, e := range entries {
		line := "  " + e.name
		if alias, ok := aliasOf[e.name]; ok {
			line += " (" + alias + ")"
		}
		fmt.Fprintf(commandStdout, "%-28s %s\n", line, e.desc)
	}
	return 0
}

func runRootVersion() int {
	fmt.Fprintln(commandStdout, Version)
	return 0
}
