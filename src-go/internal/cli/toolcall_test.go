package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/tools"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/toolcallflow"
)

type stubToolcallRunner struct {
	requests []toolcallflow.Request
	result   tools.ExecutionResult
	err      error
}

func (s *stubToolcallRunner) Run(ctx context.Context, request toolcallflow.Request) (tools.ExecutionResult, error) {
	s.requests = append(s.requests, request)
	return s.result, s.err
}

func TestRunToolcallCommandListsDefaultTools(t *testing.T) {
	output, code := captureCLIOutput(t, config.AppConfig{}, func() int {
		return runToolcallCommand([]string{"toolcall", "--list-tools"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "read\tRead file contents") {
		t.Fatalf("expected read tool listing, got %q", output)
	}
	if !strings.Contains(output, "ls\tList directory contents") {
		t.Fatalf("expected ls tool listing, got %q", output)
	}
}

func TestRunToolcallCommandRequiresPrompt(t *testing.T) {
	output, code := captureCLIOutput(t, config.AppConfig{}, func() int {
		return runToolcallCommand([]string{"toolcall", "--tools", "read"})
	})

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(strings.ToLower(output), "missing prompt") {
		t.Fatalf("expected missing prompt error, got %q", output)
	}
}

func TestRunToolcallCommandOutputsJSONAndSelectedTools(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		Provider:     "openai",
	}
	runner := &stubToolcallRunner{
		result: tools.ExecutionResult{
			Tool:      "read",
			Arguments: map[string]any{"path": "README.md"},
			Stdout:    "hello\n",
			ExitCode:  0,
		},
	}

	originalNewToolcallRunner := newToolcallRunner
	t.Cleanup(func() {
		newToolcallRunner = originalNewToolcallRunner
	})

	newToolcallRunner = func(config.AppConfig) (toolcallRunner, error) {
		return runner, nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runToolcallCommand([]string{"toolcall", "--json", "--tools", "read", "Open README"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one toolcall request, got %d", len(runner.requests))
	}

	request := runner.requests[0]
	if request.Prompt != "Open README" {
		t.Fatalf("expected prompt Open README, got %q", request.Prompt)
	}
	if len(request.Tools) != 1 || request.Tools[0].Name != "read" {
		t.Fatalf("expected selected read tool, got %#v", request.Tools)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected JSON output, got %v and %q", err, output)
	}
	if payload["tool"] != "read" {
		t.Fatalf("expected tool read in JSON payload, got %#v", payload["tool"])
	}
}

func TestRunToolcallCommandProviderOverrideUsesSelectedProfileSettings(t *testing.T) {
	cfg := config.AppConfig{
		APIEndpoint:  "https://api.openai.example/v1",
		DefaultModel: "openai-default",
		Provider:     "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				APIEndpoint:  "https://api.openai.example/v1",
				DefaultModel: "openai-default",
			},
			"deepseek": {
				APIEndpoint:  "https://api.deepseek.example/v1",
				DefaultModel: "deepseek-chat",
			},
		},
	}
	runner := &stubToolcallRunner{
		result: tools.ExecutionResult{
			Tool:      "read",
			Arguments: map[string]any{"path": "README.md"},
			Stdout:    "hello\n",
			ExitCode:  0,
		},
	}

	originalNewToolcallRunner := newToolcallRunner
	t.Cleanup(func() {
		newToolcallRunner = originalNewToolcallRunner
	})

	var receivedConfig config.AppConfig
	newToolcallRunner = func(appConfig config.AppConfig) (toolcallRunner, error) {
		receivedConfig = appConfig
		return runner, nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runToolcallCommand([]string{"toolcall", "--provider", "deepseek", "Open README"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if receivedConfig.Provider != "deepseek" {
		t.Fatalf("expected provider deepseek, got %q", receivedConfig.Provider)
	}
	if receivedConfig.APIEndpoint != "https://api.deepseek.example/v1" {
		t.Fatalf("expected deepseek endpoint, got %q", receivedConfig.APIEndpoint)
	}
	if receivedConfig.DefaultModel != "deepseek-chat" {
		t.Fatalf("expected deepseek model, got %q", receivedConfig.DefaultModel)
	}
}
