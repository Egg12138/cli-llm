package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
)

type providerRecord struct {
	APIEndpoint  string   `json:"api_endpoint"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	HasAPIKey    bool     `json:"has_api_key"`
	Source       string   `json:"source"`
}

func providerRecords(appConfig config.AppConfig) map[string]providerRecord {
	records := make(map[string]providerRecord, len(appConfig.Providers))
	for name, profile := range appConfig.Providers {
		records[name] = providerRecord{
			APIEndpoint:  profile.APIEndpoint,
			DefaultModel: profile.DefaultModel,
			Models:       append([]string(nil), profile.Models...),
			HasAPIKey:    profile.APIKey != "",
			Source:       "user-config",
		}
	}

	active, ok := records[appConfig.Provider]
	if !ok {
		return records
	}

	active.HasAPIKey = active.HasAPIKey || appConfig.APIKey != ""
	active.Source = "active"
	if appConfig.DefaultModel != "" && !containsString(active.Models, appConfig.DefaultModel) {
		active.Models = append(active.Models, appConfig.DefaultModel)
	}
	if active.APIEndpoint == "" {
		active.APIEndpoint = appConfig.APIEndpoint
	}
	if active.DefaultModel == "" {
		active.DefaultModel = appConfig.DefaultModel
	}
	records[appConfig.Provider] = active
	return records
}

func runInspectCommand(args []string) int {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(commandStderr)

	var jsonMode bool
	var allFields bool
	flags.BoolVar(&jsonMode, "json", false, "Print provider data as JSON.")
	flags.BoolVar(&jsonMode, "j", false, "Print provider data as JSON.")
	flags.BoolVar(&allFields, "all", false, "Print all fields for each provider.")
	flags.BoolVar(&allFields, "a", false, "Print all fields for each provider.")

	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}

	appConfig, err := loadAppConfig()
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 1
	}

	records := providerRecords(appConfig)
	if jsonMode {
		encoder := json.NewEncoder(commandStdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(records); err != nil {
			fmt.Fprintln(commandStderr, err)
			return 1
		}
		return 0
	}

	if allFields {
		fmt.Fprintln(commandStdout, "Displaying all fields for each provider.")
	}

	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintf(commandStdout, "Active provider: %s\n", appConfig.Provider)
	for _, name := range names {
		record := records[name]
		fmt.Fprintf(commandStdout, "- %s (%s)\n", name, record.Source)
		fmt.Fprintf(commandStdout, "  api_endpoint: %s\n", dashIfEmpty(record.APIEndpoint))
		fmt.Fprintf(commandStdout, "  default_model: %s\n", dashIfEmpty(record.DefaultModel))
		if allFields {
			models := strings.Join(record.Models, ", ")
			fmt.Fprintf(commandStdout, "  models: %s\n", dashIfEmpty(models))
			fmt.Fprintf(commandStdout, "  api_key_configured: %s\n", yesNo(record.HasAPIKey))
		}
	}

	return 0
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func dashIfEmpty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
