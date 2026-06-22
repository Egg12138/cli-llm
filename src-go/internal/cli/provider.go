package cli

import (
	"encoding/json"
	"flag"
	"fmt"
)

func runProviderCommand(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(commandStderr, "provider requires a subcommand")
		return 2
	}

	switch args[1] {
	case "models":
		return runProviderModelsCommand(args[2:])
	default:
		fmt.Fprintf(commandStderr, "unknown provider subcommand: %s\n", args[1])
		return 2
	}
}

func runProviderModelsCommand(args []string) int {
	flags := flag.NewFlagSet("provider models", flag.ContinueOnError)
	flags.SetOutput(commandStderr)

	var jsonMode bool
	flags.BoolVar(&jsonMode, "json", false, "Print only the model list as JSON.")
	flags.BoolVar(&jsonMode, "j", false, "Print only the model list as JSON.")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	remaining := flags.Args()
	if len(remaining) > 1 {
		fmt.Fprintln(commandStderr, "provider models accepts at most one provider name")
		return 2
	}

	appConfig, err := loadAppConfig()
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	target := appConfig.Provider
	if len(remaining) == 1 {
		target = remaining[0]
	}

	records := providerRecords(appConfig)
	record, ok := records[target]
	if !ok {
		fmt.Fprintf(commandStderr, "provider %q is not available\n", target)
		return 1
	}

	models := append([]string(nil), record.Models...)
	if len(models) == 0 && record.DefaultModel != "" {
		models = []string{record.DefaultModel}
	}

	if jsonMode {
		payload := map[string]any{
			"provider": target,
			"models":   models,
		}
		encoder := json.NewEncoder(commandStdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(payload); err != nil {
			fmt.Fprintln(commandStderr, err)
			return 1
		}
		return 0
	}

	if len(models) == 0 {
		fmt.Fprintf(commandStdout, "No models declared for provider %q.\n", target)
		return 0
	}

	fmt.Fprintf(commandStdout, "Models for '%s':\n", target)
	for _, model := range models {
		fmt.Fprintf(commandStdout, "- %s\n", model)
	}
	return 0
}
