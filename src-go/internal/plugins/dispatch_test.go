package plugins

import (
	"errors"
	"testing"
)

func TestPluginDispatchReservesBuiltinCommands(t *testing.T) {
	t.Parallel()

	resolver := NewDispatcher([]string{"chat", "inspect", "provider", "toolcall"}, func(string) (string, error) {
		t.Fatal("plugin lookup should not run for built-in commands")
		return "", nil
	})

	result := resolver.Resolve([]string{"chat", "hello"})
	if result.Kind != Builtin {
		t.Fatalf("expected builtin result, got %q", result.Kind)
	}
	if result.Name != "chat" {
		t.Fatalf("expected built-in name chat, got %q", result.Name)
	}
}

func TestPluginDispatchResolvesPathCommand(t *testing.T) {
	t.Parallel()

	resolver := NewDispatcher([]string{"chat"}, func(name string) (string, error) {
		if name != "llm-review" {
			t.Fatalf("expected llm-review lookup, got %q", name)
		}
		return "/tmp/bin/llm-review", nil
	})

	result := resolver.Resolve([]string{"review", "pr-12"})
	if result.Kind != Plugin {
		t.Fatalf("expected plugin result, got %q", result.Kind)
	}
	if result.Path != "/tmp/bin/llm-review" {
		t.Fatalf("expected plugin path /tmp/bin/llm-review, got %q", result.Path)
	}
	expectedArgs := []string{"/tmp/bin/llm-review", "pr-12"}
	if len(result.ExecArgs) != len(expectedArgs) {
		t.Fatalf("expected args %#v, got %#v", expectedArgs, result.ExecArgs)
	}
	for i := range expectedArgs {
		if result.ExecArgs[i] != expectedArgs[i] {
			t.Fatalf("expected args %#v, got %#v", expectedArgs, result.ExecArgs)
		}
	}
}

func TestPluginDispatchFallsBackToChatWhenMissing(t *testing.T) {
	t.Parallel()

	resolver := NewDispatcher([]string{"chat"}, func(name string) (string, error) {
		if name != "llm-review" {
			t.Fatalf("expected llm-review lookup, got %q", name)
		}
		return "", errors.New("missing")
	})

	result := resolver.Resolve([]string{"review", "pr-12"})
	if result.Kind != Missing {
		t.Fatalf("expected missing result, got %q", result.Kind)
	}
	if result.Name != "review" {
		t.Fatalf("expected missing command review, got %q", result.Name)
	}
}
