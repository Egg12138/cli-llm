package cli

import (
	"errors"
	"testing"
)

func TestPrepareExecutionDefaultsToChatWhenNoArgs(t *testing.T) {
	t.Parallel()

	plan, err := PrepareExecution(nil, func(string) (string, error) {
		t.Fatal("plugin lookup should not run for empty args")
		return "", nil
	})
	if err != nil {
		t.Fatalf("PrepareExecution returned error: %v", err)
	}

	if plan.Mode != ModeBuiltin {
		t.Fatalf("expected builtin mode, got %q", plan.Mode)
	}
	if len(plan.Args) != 1 || plan.Args[0] != "chat" {
		t.Fatalf("expected forwarded args [chat], got %#v", plan.Args)
	}
}

func TestPrepareExecutionFallsBackToChatWhenPluginMissing(t *testing.T) {
	t.Parallel()

	lookups := 0
	plan, err := PrepareExecution([]string{"deploy", "--dry-run"}, func(name string) (string, error) {
		lookups++
		if name != "llm-deploy" {
			t.Fatalf("expected llm-deploy lookup, got %q", name)
		}
		return "", errors.New("missing")
	})
	if err != nil {
		t.Fatalf("PrepareExecution returned error: %v", err)
	}

	if lookups != 1 {
		t.Fatalf("expected one plugin lookup, got %d", lookups)
	}
	if plan.Mode != ModeBuiltin {
		t.Fatalf("expected builtin mode, got %q", plan.Mode)
	}
	expected := []string{"chat", "deploy", "--dry-run"}
	if len(plan.Args) != len(expected) {
		t.Fatalf("expected %d args, got %#v", len(expected), plan.Args)
	}
	for i := range expected {
		if plan.Args[i] != expected[i] {
			t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
		}
	}
}

func TestPrepareExecutionUsesPluginWhenAvailable(t *testing.T) {
	t.Parallel()

	plan, err := PrepareExecution([]string{"review", "pr-12"}, func(name string) (string, error) {
		if name != "llm-review" {
			t.Fatalf("expected llm-review lookup, got %q", name)
		}
		return "/tmp/llm-review", nil
	})
	if err != nil {
		t.Fatalf("PrepareExecution returned error: %v", err)
	}

	if plan.Mode != ModePlugin {
		t.Fatalf("expected plugin mode, got %q", plan.Mode)
	}
	if plan.PluginPath != "/tmp/llm-review" {
		t.Fatalf("expected plugin path /tmp/llm-review, got %q", plan.PluginPath)
	}
	expected := []string{"/tmp/llm-review", "pr-12"}
	if len(plan.Args) != len(expected) {
		t.Fatalf("expected %d args, got %#v", len(expected), plan.Args)
	}
	for i := range expected {
		if plan.Args[i] != expected[i] {
			t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
		}
	}
}

func TestPrepareExecutionReservesBuiltinSubcommands(t *testing.T) {
	t.Parallel()

	plan, err := PrepareExecution([]string{"toolcall", "ls"}, func(string) (string, error) {
		t.Fatal("plugin lookup should not run for builtin subcommands")
		return "", nil
	})
	if err != nil {
		t.Fatalf("PrepareExecution returned error: %v", err)
	}

	if plan.Mode != ModeBuiltin {
		t.Fatalf("expected builtin mode, got %q", plan.Mode)
	}
	expected := []string{"toolcall", "ls"}
	if len(plan.Args) != len(expected) {
		t.Fatalf("expected %d args, got %#v", len(expected), plan.Args)
	}
	for i := range expected {
		if plan.Args[i] != expected[i] {
			t.Fatalf("expected args %#v, got %#v", expected, plan.Args)
		}
	}
}
