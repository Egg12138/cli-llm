package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/prompts"
	"github.com/Egg12138/cli-llm/src-go/internal/runtime"
	"github.com/chzyer/readline"
	"github.com/cloudwego/eino/components/model"
)

type stubChatRunner struct {
	requests []runtime.ChatRequest
	result   runtime.ChatResult
	err      error
}

func (s *stubChatRunner) Run(ctx context.Context, request runtime.ChatRequest) (runtime.ChatResult, error) {
	s.requests = append(s.requests, request)
	return s.result, s.err
}

func TestRunChatCommandUsesPromptFlagsAndWarnings(t *testing.T) {
	cfg := config.AppConfig{
		APIEndpoint:  "https://api.example/v1",
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "final-answer",
			Prompt: prompts.BuildResult{
				Warnings: []string{"Warning: AGENTS.md not found, skipping agents context."},
			},
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadChatInput := readChatInput
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readChatInput = originalReadChatInput
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readChatInput = func(string) (string, error) {
		t.Fatal("readChatInput should not run when prompt argument is present")
		return "", nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--no-stream", "--role", "normal", "--json-output", "--agents-context", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "final-answer") {
		t.Fatalf("expected rendered answer, got %q", output)
	}
	if !strings.Contains(output, "AGENTS.md not found") {
		t.Fatalf("expected warning output, got %q", output)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one chat request, got %d", len(runner.requests))
	}

	request := runner.requests[0]
	if request.Prompt != "hello" {
		t.Fatalf("expected prompt hello, got %q", request.Prompt)
	}
	if request.Stream {
		t.Fatalf("expected non-stream request")
	}
	if request.RoleName != "normal" {
		t.Fatalf("expected role normal, got %q", request.RoleName)
	}
	if request.RoleFallback != "coder" {
		t.Fatalf("expected fallback role coder, got %q", request.RoleFallback)
	}
	if !request.JSONOutput {
		t.Fatalf("expected JSON output flag to be forwarded")
	}
	if !request.AgentsContextEnabled {
		t.Fatalf("expected agents context to be enabled")
	}
}

func TestRunChatCommandReadsPromptFromSelectedInputMode(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{result: runtime.ChatResult{Text: "from-input"}}

	originalNewChatRunner := newChatRunner
	originalReadChatInput := readChatInput
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readChatInput = originalReadChatInput
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readChatInput = func(mode string) (string, error) {
		if mode != "editor" {
			t.Fatalf("expected editor input mode, got %q", mode)
		}
		return "prompt from editor", nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--input-mode", "editor"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one chat request, got %d", len(runner.requests))
	}
	if runner.requests[0].Prompt != "prompt from editor" {
		t.Fatalf("expected prompt from editor, got %q", runner.requests[0].Prompt)
	}
}

func TestRunChatCommandReturns130WhenPromptInputIsInterrupted(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}

	originalNewChatRunner := newChatRunner
	originalReadChatInput := readChatInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readChatInput = originalReadChatInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		t.Fatal("chat runner should not be created when input is interrupted")
		return nil, nil
	}
	readChatInput = func(string) (string, error) {
		return "", readline.ErrInterrupt
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat"})
	})

	if code != 130 {
		t.Fatalf("expected exit code 130, got %d", code)
	}
	if !strings.Contains(output, "Interrupted by user") {
		t.Fatalf("expected interruption message, got %q", output)
	}
}

func TestRunChatCommandEditorModePreservesPipedStdin(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{result: runtime.ChatResult{Text: "from-input"}}

	originalNewChatRunner := newChatRunner
	originalReadChatInput := readChatInput
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readChatInput = originalReadChatInput
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readChatInput = func(mode string) (string, error) {
		if mode != "editor" {
			t.Fatalf("expected editor input mode, got %q", mode)
		}
		return "prompt from editor", nil
	}
	readPipedInput = func() (string, error) {
		return "context from pipe", nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--input-mode", "editor"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one chat request, got %d", len(runner.requests))
	}
	if runner.requests[0].Prompt != "prompt from editor" {
		t.Fatalf("expected prompt from editor, got %q", runner.requests[0].Prompt)
	}
	if runner.requests[0].StdinInput != "context from pipe" {
		t.Fatalf("expected piped stdin to be forwarded, got %q", runner.requests[0].StdinInput)
	}
}

func TestRunChatCommandUsesSelectedRoleTemperatureByDefault(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{result: runtime.ChatResult{Text: "final-answer"}}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--role", "normal", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one chat request, got %d", len(runner.requests))
	}

	options := model.GetCommonOptions(nil, runner.requests[0].ModelOptions...)
	if options.Temperature == nil {
		t.Fatal("expected role temperature to be forwarded")
	}
	if *options.Temperature != 1.3 {
		t.Fatalf("expected normal role temperature 1.3, got %v", *options.Temperature)
	}
}

func TestRunChatCommandSetsNoProxyForLocalhost(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{result: runtime.ChatResult{Text: "final-answer"}}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	originalSetenv := setenvFunc
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
		setenvFunc = originalSetenv
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	seen := map[string]string{}
	setenvFunc = func(key string, value string) error {
		seen[key] = value
		return nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if seen["NO_PROXY"] != "localhost" {
		t.Fatalf("expected NO_PROXY localhost, got %#v", seen)
	}
}

func TestRunChatCommandLocaltestSkipsProviderCall(t *testing.T) {
	cfg := config.AppConfig{
		APIEndpoint: "https://api.example/v1",
		Provider:    "openai",
	}

	originalNewChatRunner := newChatRunner
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		t.Fatal("chat runner should not be created for localtest")
		return nil, nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--localtest", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "Provider base URL: https://api.example/v1") {
		t.Fatalf("expected localtest output, got %q", output)
	}
}

func TestRunChatCommandDisplaysTokenUsageWhenEnabled(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "final-answer",
			Usage: runtime.TokenUsage{
				InputTokens:  11,
				OutputTokens: 7,
			},
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--count-tokens", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "Input tokens: 11") {
		t.Fatalf("expected input token output, got %q", output)
	}
	if !strings.Contains(output, "Output tokens: 7") {
		t.Fatalf("expected output token output, got %q", output)
	}
	if !strings.Contains(output, "Total tokens: 18") {
		t.Fatalf("expected total token output, got %q", output)
	}
	if len(runner.requests) != 1 || !runner.requests[0].CountTokens {
		t.Fatalf("expected count-tokens request to be forwarded, got %#v", runner.requests)
	}
}

func TestRunChatCommandWritesSingleCodeBlockToTargetFile(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "Here is code:\n```python\nprint('hello')\n```\n",
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "snippet.py")
	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--output-codes", targetPath, "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	if string(data) != "print('hello')\n" {
		t.Fatalf("expected extracted code block, got %q", string(data))
	}
	if strings.Contains(output, "Here is code:") {
		t.Fatalf("expected chat prose to be suppressed, got %q", output)
	}
	if !strings.Contains(output, "Wrote code block to") {
		t.Fatalf("expected success message, got %q", output)
	}
	if len(runner.requests) != 1 || runner.requests[0].Stream {
		t.Fatalf("expected output-codes to force non-stream request, got %#v", runner.requests)
	}
}

func TestRunChatCommandFailsWhenNoCodeBlockIsPresent(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "No code here.",
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--output-codes", filepath.Join(t.TempDir(), "snippet.py"), "hello"})
	})

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output, "expected exactly one fenced code block") {
		t.Fatalf("expected code block error, got %q", output)
	}
}

func TestRunChatCommandFailsWhenMultipleCodeBlocksArePresent(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "```go\nfmt.Println(1)\n```\n\n```go\nfmt.Println(2)\n```",
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--output-codes", filepath.Join(t.TempDir(), "snippet.go"), "hello"})
	})

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(output, "expected exactly one fenced code block") {
		t.Fatalf("expected multiple code block error, got %q", output)
	}
}

func TestRunChatCommandDebugPrintsExecutionSummary(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "final-answer",
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--debug", "--count-tokens", "--role", "normal", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output)
	}
	if !strings.Contains(output, "DEBUG chat provider=openai model=deepseek-chat role=normal stream=true") {
		t.Fatalf("expected debug summary line, got %q", output)
	}
	if !strings.Contains(output, "DEBUG chat prompt_chars=5 stdin_chars=0 agents_context=false json_output=false count_tokens=true") {
		t.Fatalf("expected debug prompt details, got %q", output)
	}
}

func TestChatHelpMentionsImplementedOutputCodesAndTokenFlags(t *testing.T) {
	output, code := captureCLIOutput(t, config.AppConfig{}, func() int {
		return runChatCommand([]string{"chat", "--help"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "Extract exactly one fenced code block and write it to the target file.") {
		t.Fatalf("expected updated output-codes help text, got %q", output)
	}
	if !strings.Contains(output, "Count input and output tokens and show usage totals.") {
		t.Fatalf("expected updated count-tokens help text, got %q", output)
	}
}

func TestRunChatCommandProviderOverrideUsesSelectedProfileSettings(t *testing.T) {
	cfg := config.AppConfig{
		APIEndpoint:  "https://api.openai.example/v1",
		DefaultModel: "openai-default",
		DefaultRole:  "coder",
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
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "final-answer",
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	var receivedConfig config.AppConfig
	newChatRunner = func(appConfig config.AppConfig, request runtime.ChatRequest) (chatRunner, error) {
		receivedConfig = appConfig
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--provider", "deepseek", "hello"})
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

func TestRunChatCommandBuildsRunnerWithJSONOutputRequest(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text: "{\"ok\":true}",
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	var buildRequest runtime.ChatRequest
	newChatRunner = func(appConfig config.AppConfig, request runtime.ChatRequest) (chatRunner, error) {
		buildRequest = request
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	_, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--json-output", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !buildRequest.JSONOutput {
		t.Fatalf("expected runner build request to enable JSON output, got %#v", buildRequest)
	}
}

func TestRunChatCommandPrintsResponseMetadataForNonStream(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text:         "final-answer",
			FinishReason: "length",
			Duration:     1230 * time.Millisecond,
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "--no-stream", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output)
	}
	if !strings.Contains(output, "final-answer") {
		t.Fatalf("expected answer output, got %q", output)
	}
	if !strings.Contains(output, "@ deepseek-chat [Length exceeded max_tokens limit] Response time: 1.23s:") {
		t.Fatalf("expected response metadata line, got %q", output)
	}
}

func TestRunChatCommandPrintsResponseTimeForStream(t *testing.T) {
	cfg := config.AppConfig{
		DefaultModel: "deepseek-chat",
		DefaultRole:  "coder",
		Provider:     "openai",
	}
	runner := &stubChatRunner{
		result: runtime.ChatResult{
			Text:           "stream-answer",
			Duration:       2 * time.Second,
			StreamRendered: true,
		},
	}

	originalNewChatRunner := newChatRunner
	originalReadPipedInput := readPipedInput
	t.Cleanup(func() {
		newChatRunner = originalNewChatRunner
		readPipedInput = originalReadPipedInput
	})

	newChatRunner = func(config.AppConfig, runtime.ChatRequest) (chatRunner, error) {
		return runner, nil
	}
	readPipedInput = func() (string, error) {
		return "", nil
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runChatCommand([]string{"chat", "hello"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with output %q", code, output)
	}
	if strings.Contains(output, "stream-answer") {
		t.Fatalf("expected no duplicate streamed body output, got %q", output)
	}
	if !strings.Contains(output, "Response time: 2.00s") {
		t.Fatalf("expected stream response-time line, got %q", output)
	}
}

func TestDefaultReadChatInputStdinModeReadsTerminalInput(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminalFunc
	originalReadAllStdin := readAllStdin
	t.Cleanup(func() {
		stdinIsTerminalFunc = originalStdinIsTerminal
		readAllStdin = originalReadAllStdin
	})

	stdinIsTerminalFunc = func() bool {
		return true
	}
	readAllStdin = func() (string, error) {
		return "line1\nline2", nil
	}

	text, err := defaultReadChatInput("stdin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if text != "line1\nline2" {
		t.Fatalf("expected explicit stdin content, got %q", text)
	}
}

func TestResolveEditorCommandPrefersEditorEnvThenVisualThenCandidates(t *testing.T) {
	originalGetenv := getenvFunc
	originalCommandExists := commandExistsFunc
	t.Cleanup(func() {
		getenvFunc = originalGetenv
		commandExistsFunc = originalCommandExists
	})

	getenvFunc = func(key string) string {
		switch key {
		case "EDITOR":
			return ""
		case "VISUAL":
			return "hx"
		default:
			return ""
		}
	}
	commandExistsFunc = func(name string) bool {
		return false
	}

	editor, ok := resolveEditorCommand()
	if !ok {
		t.Fatal("expected editor resolution to succeed")
	}
	if editor != "hx" {
		t.Fatalf("expected VISUAL fallback, got %q", editor)
	}

	getenvFunc = func(key string) string { return "" }
	commandExistsFunc = func(name string) bool {
		return name == "nano"
	}

	editor, ok = resolveEditorCommand()
	if !ok {
		t.Fatal("expected candidate resolution to succeed")
	}
	if editor != "nano" {
		t.Fatalf("expected nano candidate, got %q", editor)
	}
}

func TestDefaultReadChatInputEditorFallsBackToPromptWhenNoEditorExists(t *testing.T) {
	originalGetenv := getenvFunc
	originalCommandExists := commandExistsFunc
	originalReadPromptInput := readPromptInput
	t.Cleanup(func() {
		getenvFunc = originalGetenv
		commandExistsFunc = originalCommandExists
		readPromptInput = originalReadPromptInput
	})

	getenvFunc = func(key string) string { return "" }
	commandExistsFunc = func(name string) bool { return false }
	readPromptInput = func() (string, error) {
		return "fallback prompt", nil
	}

	output, _ := captureCLIOutput(t, config.AppConfig{}, func() int {
		text, err := defaultReadChatInput("editor")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if text != "fallback prompt" {
			t.Fatalf("expected prompt fallback, got %q", text)
		}
		return 0
	})

	if !strings.Contains(output, "No EDITOR found") {
		t.Fatalf("expected fallback warning, got %q", output)
	}
}

func TestResolveChatHistoryPathPrefersXDGCacheHome(t *testing.T) {
	originalGetenv := getenvFunc
	t.Cleanup(func() {
		getenvFunc = originalGetenv
	})

	getenvFunc = func(key string) string {
		switch key {
		case "XDG_CACHE_HOME":
			return "/tmp/cache-root"
		case "HOME":
			return "/tmp/home-root"
		default:
			return ""
		}
	}

	path, err := resolveChatHistoryPath()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := filepath.Join("/tmp/cache-root", "cli-llm", "chat_history")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestResolveChatHistoryPathFallsBackToHomeCache(t *testing.T) {
	originalGetenv := getenvFunc
	t.Cleanup(func() {
		getenvFunc = originalGetenv
	})

	getenvFunc = func(key string) string {
		switch key {
		case "XDG_CACHE_HOME":
			return ""
		case "HOME":
			return "/tmp/home-root"
		default:
			return ""
		}
	}

	path, err := resolveChatHistoryPath()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := filepath.Join("/tmp/home-root", ".cache", "cli-llm", "chat_history")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestDefaultReadPromptInputUsesHistoryReader(t *testing.T) {
	originalReadPromptWithHistory := readPromptWithHistory
	t.Cleanup(func() {
		readPromptWithHistory = originalReadPromptWithHistory
	})

	readPromptWithHistory = func(prompt string, historyPath string) (string, error) {
		if prompt != "[Ask]: " {
			t.Fatalf("expected prompt label, got %q", prompt)
		}
		if historyPath != "/tmp/chat_history" {
			t.Fatalf("expected history path /tmp/chat_history, got %q", historyPath)
		}
		return "answer from history reader", nil
	}

	originalResolveChatHistoryPath := resolveChatHistoryPathFunc
	t.Cleanup(func() {
		resolveChatHistoryPathFunc = originalResolveChatHistoryPath
	})
	resolveChatHistoryPathFunc = func() (string, error) {
		return "/tmp/chat_history", nil
	}

	text, err := defaultReadPromptInput()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if text != "answer from history reader" {
		t.Fatalf("expected prompt text, got %q", text)
	}
}

func TestDefaultReadPromptInputPropagatesInterrupt(t *testing.T) {
	originalReadPromptWithHistory := readPromptWithHistory
	t.Cleanup(func() {
		readPromptWithHistory = originalReadPromptWithHistory
	})

	readPromptWithHistory = func(prompt string, historyPath string) (string, error) {
		return "", readline.ErrInterrupt
	}

	originalResolveChatHistoryPath := resolveChatHistoryPathFunc
	t.Cleanup(func() {
		resolveChatHistoryPathFunc = originalResolveChatHistoryPath
	})
	resolveChatHistoryPathFunc = func() (string, error) {
		return "/tmp/chat_history", nil
	}

	_, err := defaultReadPromptInput()
	if !errors.Is(err, readline.ErrInterrupt) {
		t.Fatalf("expected readline interrupt, got %v", err)
	}
}
