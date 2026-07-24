package cli

import (
	"strings"
	"testing"
)

func TestPrepareExecutionAliasesBuiltin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		alias   string
		want    string
		extra   []string
	}{
		{alias: "c", want: "chat"},
		{alias: "i", want: "inspect"},
		{alias: "p", want: "provider"},
		{alias: "t", want: "toolcall"},
	}

	for _, tt := range tests {
		args := append([]string{tt.alias}, tt.extra...)
		plan, err := PrepareExecution(args, func(name string) (string, error) {
			t.Fatalf("plugin lookup should not run for builtin alias %q", tt.alias)
			return "", nil
		})
		if err != nil {
			t.Fatalf("PrepareExecution(%q): %v", tt.alias, err)
		}
		if plan.Mode != ModeBuiltin {
			t.Fatalf("PrepareExecution(%q): expected builtin, got %q", tt.alias, plan.Mode)
		}
		if plan.Args[0] != tt.want {
			t.Fatalf("PrepareExecution(%q): args[0] = %q, want %q", tt.alias, plan.Args[0], tt.want)
		}
	}
}

func TestPrepareExecutionAliasPlugin(t *testing.T) {
	t.Parallel()

	plan, err := PrepareExecution([]string{"s", "--resume", "work"}, func(name string) (string, error) {
		if name != "llm-session" {
			t.Fatalf("expected llm-session lookup, got %q", name)
		}
		return "/tmp/llm-session", nil
	})
	if err != nil {
		t.Fatalf("PrepareExecution: %v", err)
	}
	if plan.Mode != ModePlugin {
		t.Fatalf("expected plugin mode, got %q", plan.Mode)
	}
	if plan.PluginPath != "/tmp/llm-session" {
		t.Fatalf("expected plugin path /tmp/llm-session, got %q", plan.PluginPath)
	}
	expected := []string{"/tmp/llm-session", "--resume", "work"}
	if len(plan.Args) != len(expected) {
		t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
	}
	for i := range expected {
		if plan.Args[i] != expected[i] {
			t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
		}
	}
}

func TestPrepareExecutionAliasPreservesExtraArgs(t *testing.T) {
	t.Parallel()

	plan, err := PrepareExecution([]string{"c", "--model", "gpt-4", "--temp", "0.5"}, func(name string) (string, error) {
		t.Fatal("plugin lookup should not run")
		return "", nil
	})
	if err != nil {
		t.Fatalf("PrepareExecution: %v", err)
	}
	if plan.Mode != ModeBuiltin {
		t.Fatalf("expected builtin mode, got %q", plan.Mode)
	}
	expected := []string{"chat", "--model", "gpt-4", "--temp", "0.5"}
	if len(plan.Args) != len(expected) {
		t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
	}
	for i := range expected {
		if plan.Args[i] != expected[i] {
			t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
		}
	}
}

func TestPrepareExecutionNonAliasPassesThrough(t *testing.T) {
	t.Parallel()

	plan, err := PrepareExecution([]string{"session", "--resume"}, func(name string) (string, error) {
		if name != "llm-session" {
			t.Fatalf("expected llm-session lookup, got %q", name)
		}
		return "/tmp/llm-session", nil
	})
	if err != nil {
		t.Fatalf("PrepareExecution: %v", err)
	}
	// With full name "session", should go to plugin directly — no alias rewrite
	if plan.Mode != ModePlugin {
		t.Fatalf("expected plugin mode, got %q", plan.Mode)
	}
}

func TestPrepareExecutionPassthroughFlagsNotAliased(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"-h", "--help", "-V", "--version"} {
		plan, err := PrepareExecution([]string{flag}, func(name string) (string, error) {
			t.Fatalf("plugin lookup should not run for flag %q", flag)
			return "", nil
		})
		if err != nil {
			t.Fatalf("PrepareExecution(%q): %v", flag, err)
		}
		if plan.Args[0] != flag {
			t.Fatalf("PrepareExecution(%q): args[0] = %q, want %q", flag, plan.Args[0], flag)
		}
	}
}

func TestRootHelpShowsAliases(t *testing.T) {
	var out strings.Builder
	originalStdout := commandStdout
	commandStdout = &out
	t.Cleanup(func() { commandStdout = originalStdout })

	code := runRootHelp()
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	for _, pair := range []string{"chat (c)", "inspect (i)", "provider (p)", "session (s)", "toolcall (t)"} {
		if !strings.Contains(out.String(), pair) {
			t.Fatalf("expected help to contain %q, got:\n%s", pair, out.String())
		}
	}
}
