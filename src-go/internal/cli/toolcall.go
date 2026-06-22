package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/prompts"
	"github.com/Egg12138/cli-llm/src-go/internal/providers"
	"github.com/Egg12138/cli-llm/src-go/internal/tools"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/toolcallflow"
	"github.com/cloudwego/eino/components/model"
)

type toolcallRunner interface {
	Run(ctx context.Context, request toolcallflow.Request) (tools.ExecutionResult, error)
}

var newToolcallRunner = defaultNewToolcallRunner

func defaultNewToolcallRunner(appConfig config.AppConfig) (toolcallRunner, error) {
	ctx := context.Background()
	chatModel, err := providers.NewFactory(appConfig).New(ctx, false)
	if err != nil {
		return nil, err
	}

	return toolcallflow.New(ctx, chatModel)
}

func runToolcallCommand(args []string) int {
	flags := flag.NewFlagSet("toolcall", flag.ContinueOnError)
	flags.SetOutput(commandStderr)
	flags.Usage = func() {
		fmt.Fprintln(commandStderr, "Usage: llm toolcall [OPTIONS] PROMPT")
		flags.PrintDefaults()
	}

	var toolsCSV string
	var listTools bool
	var provider string
	var modelName string
	var jsonMode bool

	flags.StringVar(&toolsCSV, "tools", "", "Comma-separated preset tools to enable.")
	flags.StringVar(&toolsCSV, "t", "", "Comma-separated preset tools to enable.")
	flags.BoolVar(&listTools, "list-tools", false, "List enabled preset tools and exit.")
	flags.BoolVar(&listTools, "l", false, "List enabled preset tools and exit.")
	flags.StringVar(&provider, "provider", "", "Select the provider profile.")
	flags.StringVar(&provider, "p", "", "Select the provider profile.")
	flags.StringVar(&modelName, "model", "", "Override the configured model.")
	flags.StringVar(&modelName, "m", "", "Override the configured model.")
	flags.BoolVar(&jsonMode, "json", false, "Print tool execution result as JSON.")
	flags.BoolVar(&jsonMode, "j", false, "Print tool execution result as JSON.")

	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	var selectedTools []string
	if toolsCSV != "" {
		for _, name := range strings.Split(toolsCSV, ",") {
			trimmed := strings.TrimSpace(name)
			if trimmed != "" {
				selectedTools = append(selectedTools, trimmed)
			}
		}
	}

	definitions, err := tools.GetDefinitions(selectedTools)
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 2
	}

	if listTools {
		for _, definition := range definitions {
			summary := definition.PromptSnippet
			if summary == "" {
				summary = definition.Description
			}
			fmt.Fprintf(commandStdout, "%s\t%s\n", definition.Name, summary)
		}
		return 0
	}

	promptText := prompts.SanitizeInput(strings.TrimSpace(strings.Join(flags.Args(), " ")))
	if promptText == "" {
		fmt.Fprintln(commandStderr, "Missing prompt.")
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

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	runner, err := newToolcallRunner(appConfig)
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	result, err := runner.Run(context.Background(), toolcallflow.Request{
		Prompt:      promptText,
		Tools:       definitions,
		WorkingDir:  cwd,
		CurrentDate: time.Now().Format("2006-01-02"),
		ModelOptions: []model.Option{
			model.WithModel(appConfig.DefaultModel),
		},
	})
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	if jsonMode {
		payload := map[string]any{
			"tool":      result.Tool,
			"arguments": result.Arguments,
			"stdout":    result.Stdout,
			"exit_code": result.ExitCode,
		}
		encoder := json.NewEncoder(commandStdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(payload); err != nil {
			fmt.Fprintln(commandStderr, err)
			return 1
		}
		return 0
	}

	fmt.Fprint(commandStdout, result.Stdout)
	if result.Stdout != "" && !strings.HasSuffix(result.Stdout, "\n") {
		fmt.Fprintln(commandStdout)
	}
	return 0
}
