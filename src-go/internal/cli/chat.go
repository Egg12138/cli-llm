package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/prompts"
	"github.com/Egg12138/cli-llm/src-go/internal/providers"
	"github.com/Egg12138/cli-llm/src-go/internal/render"
	"github.com/Egg12138/cli-llm/src-go/internal/runtime"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/chatflow"
	"github.com/chzyer/readline"
	"github.com/cloudwego/eino/components/model"
)

type chatRunner interface {
	Run(ctx context.Context, request runtime.ChatRequest) (runtime.ChatResult, error)
}

var (
	newChatRunner              = defaultNewChatRunner
	readChatInput              = defaultReadChatInput
	readPipedInput             = defaultReadPipedInput
	stdinIsTerminalFunc        = stdinIsTerminal
	readAllStdin               = defaultReadAllStdin
	readPromptInput            = defaultReadPromptInput
	getenvFunc                 = os.Getenv
	setenvFunc                 = os.Setenv
	commandExistsFunc          = defaultCommandExists
	resolveChatHistoryPathFunc = resolveChatHistoryPath
	readPromptWithHistory      = defaultReadPromptWithHistory
)

var errUserInterrupted = errors.New("interrupted by user")

func defaultNewChatRunner(appConfig config.AppConfig, request runtime.ChatRequest) (chatRunner, error) {
	ctx := context.Background()
	chatModel, err := providers.NewFactory(appConfig).New(ctx, request.JSONOutput)
	if err != nil {
		return nil, err
	}

	workflow, err := chatflow.New(ctx, chatModel)
	if err != nil {
		return nil, err
	}

	return runtime.NewChatService(workflow, render.Renderer{Writer: commandStdout}, prompts.NewRoleRegistry()), nil
}

func runChatCommand(args []string) int {
	flags := flag.NewFlagSet("chat", flag.ContinueOnError)
	flags.SetOutput(commandStderr)
	flags.Usage = func() {
		fmt.Fprintln(commandStderr, "Usage: llm chat [OPTIONS] [PROMPT]")
		flags.PrintDefaults()
	}

	var noStream bool
	var provider string
	var role string
	var modelName string
	var temperature float64
	var temperatureSet bool
	var jsonOutput bool
	var outputCodes string
	var debug bool
	var localtest bool
	var countTokens bool
	var inputMode string
	var agentsContext bool

	flags.BoolVar(&noStream, "no-stream", false, "Disable streaming responses.")
	flags.BoolVar(&noStream, "n", false, "Disable streaming responses.")
	flags.StringVar(&provider, "provider", "", "Select the provider profile.")
	flags.StringVar(&provider, "p", "", "Select the provider profile.")
	flags.StringVar(&role, "role", "", "System role to use.")
	flags.StringVar(&role, "r", "", "System role to use.")
	flags.StringVar(&modelName, "model", "", "Override the configured model.")
	flags.StringVar(&modelName, "m", "", "Override the configured model.")
	flags.Func("temp", "Override the configured temperature.", func(value string) error {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		temperature = parsed
		temperatureSet = true
		return nil
	})
	flags.Func("t", "Override the configured temperature.", func(value string) error {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		temperature = parsed
		temperatureSet = true
		return nil
	})
	flags.BoolVar(&jsonOutput, "json-output", false, "Request JSON output.")
	flags.BoolVar(&jsonOutput, "j", false, "Request JSON output.")
	flags.StringVar(&outputCodes, "output-codes", "", "Extract exactly one fenced code block and write it to the target file.")
	flags.StringVar(&outputCodes, "o", "", "Extract exactly one fenced code block and write it to the target file.")
	flags.BoolVar(&debug, "debug", false, "Enable debug mode.")
	flags.BoolVar(&debug, "d", false, "Enable debug mode.")
	flags.BoolVar(&localtest, "localtest", false, "Print provider base URL and exit.")
	flags.BoolVar(&localtest, "L", false, "Print provider base URL and exit.")
	flags.BoolVar(&countTokens, "count-tokens", false, "Count input and output tokens and show usage totals.")
	flags.BoolVar(&countTokens, "c", false, "Count input and output tokens and show usage totals.")
	flags.StringVar(&inputMode, "input-mode", "prompt", "Input mode: prompt, editor, or stdin.")
	flags.StringVar(&inputMode, "i", "prompt", "Input mode: prompt, editor, or stdin.")
	flags.BoolVar(&agentsContext, "agents-context", false, "Read ./AGENTS.md from cwd and append it to the system prompt.")
	flags.BoolVar(&agentsContext, "A", false, "Read ./AGENTS.md from cwd and append it to the system prompt.")

	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if inputMode != "prompt" && inputMode != "editor" && inputMode != "stdin" {
		fmt.Fprintf(commandStderr, "unsupported input mode %q\n", inputMode)
		return 2
	}

	overrides := config.Overrides{}
	if provider != "" {
		overrides.Provider = &provider
	}
	if modelName != "" {
		overrides.DefaultModel = &modelName
	}
	appConfig, err := loadAppConfigOverrides(overrides)
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	promptText := prompts.SanitizeInput(strings.TrimSpace(strings.Join(flags.Args(), " ")))
	if promptText == "" {
		promptText, err = readChatInput(inputMode)
		if err != nil {
			if errors.Is(err, errUserInterrupted) || errors.Is(err, readline.ErrInterrupt) {
				fmt.Fprintln(commandStdout, "Interrupted by user")
				return 130
			}
			fmt.Fprintln(commandStderr, err)
			return 1
		}
		promptText = prompts.SanitizeInput(strings.TrimSpace(promptText))
	}
	if promptText == "" {
		return 0
	}

	stdinInput := ""
	if len(flags.Args()) > 0 || inputMode == "editor" {
		stdinInput, err = readPipedInput()
		if err != nil {
			fmt.Fprintln(commandStderr, err)
			return 1
		}
	}

	if localtest {
		fmt.Fprintf(commandStdout, "Provider base URL: %s\n", appConfig.APIEndpoint)
		return 0
	}

	if err := prepareChatEnvironment(); err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	activeRole := appConfig.DefaultRole
	if role != "" {
		activeRole = role
	}
	_, resolvedRole, _ := prompts.NewRoleRegistry().Resolve(activeRole, appConfig.DefaultRole)

	streamOutput := !noStream && outputCodes == ""
	effectiveTemperature := resolvedRole.Temperature
	if temperatureSet {
		effectiveTemperature = float32(temperature)
	}
	modelOptions := []model.Option{
		model.WithModel(appConfig.DefaultModel),
		model.WithTemperature(effectiveTemperature),
	}

	if debug {
		fmt.Fprintf(commandStderr, "DEBUG chat provider=%s model=%s role=%s stream=%t input_mode=%s\n",
			appConfig.Provider, appConfig.DefaultModel, activeRole, streamOutput, inputMode)
		fmt.Fprintf(commandStderr, "DEBUG chat prompt_chars=%d stdin_chars=%d agents_context=%t json_output=%t count_tokens=%t\n",
			len(promptText), len(stdinInput), agentsContext, jsonOutput, countTokens)
	}

	request := runtime.ChatRequest{
		Prompt:               promptText,
		StdinInput:           stdinInput,
		RoleName:             activeRole,
		RoleFallback:         appConfig.DefaultRole,
		WorkingDir:           cwd,
		ModelName:            appConfig.DefaultModel,
		AgentsContextEnabled: agentsContext,
		CountTokens:          countTokens,
		JSONOutput:           jsonOutput,
		Stream:               streamOutput,
		ModelOptions:         modelOptions,
	}

	runner, err := newChatRunner(appConfig, request)
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	result, err := runner.Run(context.Background(), request)
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	for _, warning := range result.Prompt.Warnings {
		fmt.Fprintln(commandStderr, warning)
	}
	if outputCodes != "" {
		rawContent := result.RawText
		if rawContent == "" {
			rawContent = result.Text
		}
		codeBlock, err := extractSingleCodeBlock(rawContent)
		if err != nil {
			fmt.Fprintln(commandStderr, err)
			return 1
		}
		if err := writeCodeBlock(outputCodes, codeBlock); err != nil {
			fmt.Fprintln(commandStderr, err)
			return 1
		}
		fmt.Fprintf(commandStdout, "Wrote code block to %s\n", outputCodes)
	} else if result.Text != "" && !result.StreamRendered {
		fmt.Fprint(commandStdout, result.Text)
		if !strings.HasSuffix(result.Text, "\n") {
			fmt.Fprintln(commandStdout)
		}
	}
	if countTokens {
		fmt.Fprintf(commandStdout, "\nInput tokens: %d\n", result.Usage.InputTokens)
		fmt.Fprintf(commandStdout, "Output tokens: %d\n", result.Usage.OutputTokens)
		fmt.Fprintf(commandStdout, "Total tokens: %d\n", result.Usage.Total())
	}
	if result.Duration > 0 {
		if !streamOutput {
			fmt.Fprintf(commandStdout, "@ %s [%s] Response time: %.2fs:\n",
				appConfig.DefaultModel, finishReasonLabel(result.FinishReason), result.Duration.Seconds())
		} else {
			fmt.Fprintf(commandStdout, "Response time: %.2fs\n", result.Duration.Seconds())
		}
	}

	return 0
}

func finishReasonLabel(reason string) string {
	switch reason {
	case "stop", "":
		return "Normal"
	case "length":
		return "Length exceeded max_tokens limit"
	case "content_filter":
		return "Content filter triggered"
	case "insufficient_system_resource":
		return "Insufficient system resources"
	default:
		return "Unknown"
	}
}

func defaultReadChatInput(mode string) (string, error) {
	switch mode {
	case "prompt":
		if !stdinIsTerminalFunc() {
			return defaultReadPipedInput()
		}
		return readPromptInput()
	case "editor":
		editor, ok := resolveEditorCommand()
		if !ok {
			fmt.Fprintln(commandStderr, "No EDITOR found - falling back to prompt mode. Set $EDITOR or $VISUAL to use editor mode.")
			return readPromptInput()
		}

		file, err := os.CreateTemp("", "cli-llm-*.md")
		if err != nil {
			return "", err
		}
		path := file.Name()
		if err := file.Close(); err != nil {
			return "", err
		}
		defer os.Remove(path)

		parts := strings.Fields(editor)
		cmd := exec.Command(parts[0], append(parts[1:], path)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = commandStdout
		cmd.Stderr = commandStderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(commandStderr, "Editor exited abnormally - discarding input.")
			return "", nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	case "stdin":
		return readAllStdin()
	default:
		return "", fmt.Errorf("unsupported input mode %q", mode)
	}
}

func defaultReadPromptInput() (string, error) {
	historyPath, err := resolveChatHistoryPathFunc()
	if err == nil && historyPath != "" {
		return readPromptWithHistory("[Ask]: ", historyPath)
	}

	fmt.Fprint(commandStdout, "[Ask]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func defaultReadPipedInput() (string, error) {
	if stdinIsTerminalFunc() {
		return "", nil
	}

	return defaultReadAllStdin()
}

func defaultReadAllStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func resolveEditorCommand() (string, bool) {
	if editor := strings.TrimSpace(getenvFunc("EDITOR")); editor != "" {
		return editor, true
	}
	if editor := strings.TrimSpace(getenvFunc("VISUAL")); editor != "" {
		return editor, true
	}
	for _, candidate := range []string{"vim", "nvim", "nano", "emacs", "vi"} {
		if commandExistsFunc(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func prepareChatEnvironment() error {
	return setenvFunc("NO_PROXY", "localhost")
}

func defaultCommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func resolveChatHistoryPath() (string, error) {
	cacheRoot := strings.TrimSpace(getenvFunc("XDG_CACHE_HOME"))
	if cacheRoot == "" {
		home := strings.TrimSpace(getenvFunc("HOME"))
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		cacheRoot = filepath.Join(home, ".cache")
	}

	historyDir := filepath.Join(cacheRoot, "cli-llm")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(historyDir, "chat_history"), nil
}

func defaultReadPromptWithHistory(prompt string, historyPath string) (string, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      prompt,
		HistoryFile: historyPath,
		Stdin:       os.Stdin,
		Stdout:      commandStdout,
		Stderr:      commandStderr,
	})
	if err != nil {
		return "", err
	}
	defer rl.Close()

	line, err := rl.Readline()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		if err == readline.ErrInterrupt {
			return "", errUserInterrupted
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}
