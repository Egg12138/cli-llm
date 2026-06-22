package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
)

var cliCaptureMu sync.Mutex

func TestRunInspectCommandJSON(t *testing.T) {
	t.Parallel()

	cfg := config.AppConfig{
		APIKey:       "secret",
		APIEndpoint:  "https://api.example/v1",
		DefaultModel: "gpt-4o-mini",
		Provider:     "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				APIEndpoint:  "https://api.example/v1",
				DefaultModel: "legacy-model",
				Models:       []string{"legacy-model"},
			},
		},
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runInspectCommand([]string{"inspect", "--json"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected JSON output, got error %v and output %q", err, output)
	}

	record := payload["openai"]
	if record["source"] != "active" {
		t.Fatalf("expected active source, got %#v", record["source"])
	}
	if record["has_api_key"] != true {
		t.Fatalf("expected has_api_key true, got %#v", record["has_api_key"])
	}
}

func TestRunInspectCommandAllFields(t *testing.T) {
	t.Parallel()

	cfg := config.AppConfig{
		Provider: "deepseek",
		Providers: map[string]config.ProviderConfig{
			"deepseek": {
				APIEndpoint:  "https://api.deepseek.com/v1",
				DefaultModel: "deepseek-chat",
				Models:       []string{"deepseek-chat", "deepseek-coder"},
			},
		},
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runInspectCommand([]string{"inspect", "--all"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "Active provider: deepseek") {
		t.Fatalf("expected active provider line, got %q", output)
	}
	if !strings.Contains(output, "models: deepseek-chat, deepseek-coder") {
		t.Fatalf("expected models list, got %q", output)
	}
	if !strings.Contains(output, "api_key_configured: no") {
		t.Fatalf("expected api_key_configured line, got %q", output)
	}
}

func TestRunProviderModelsDefaultsToActiveProvider(t *testing.T) {
	t.Parallel()

	cfg := config.AppConfig{
		Provider: "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				Models: []string{"gpt-4o-mini", "gpt-4o"},
			},
		},
	}

	output, code := captureCLIOutput(t, cfg, func() int {
		return runProviderCommand([]string{"provider", "models"})
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(output, "Models for 'openai':") {
		t.Fatalf("expected provider heading, got %q", output)
	}
	if !strings.Contains(output, "- gpt-4o-mini") {
		t.Fatalf("expected first model, got %q", output)
	}
}

func TestProviderRecordsMarksActiveProfile(t *testing.T) {
	t.Parallel()

	cfg := config.AppConfig{
		APIKey:       "secret",
		APIEndpoint:  "https://api.example/v1",
		DefaultModel: "primary-model",
		Provider:     "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				Models: []string{"legacy-model"},
			},
		},
	}

	records := providerRecords(cfg)
	record := records["openai"]

	if record.Source != "active" {
		t.Fatalf("expected source active, got %q", record.Source)
	}
	if !record.HasAPIKey {
		t.Fatalf("expected api key to be marked present")
	}
	if len(record.Models) != 2 || record.Models[1] != "primary-model" {
		t.Fatalf("expected active model merged into models, got %#v", record.Models)
	}
}

func captureCLIOutput(t *testing.T, cfg config.AppConfig, run func() int) (string, int) {
	t.Helper()
	cliCaptureMu.Lock()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	originalStdout := commandStdout
	originalStderr := commandStderr
	originalLoader := loadAppConfig
	originalOverrideLoader := loadAppConfigOverrides
	defer func() {
		commandStdout = originalStdout
		commandStderr = originalStderr
		loadAppConfig = originalLoader
		loadAppConfigOverrides = originalOverrideLoader
		cliCaptureMu.Unlock()
	}()

	commandStdout = &stdout
	commandStderr = &stderr
	loadAppConfig = func() (config.AppConfig, error) {
		return cfg, nil
	}
	loadAppConfigOverrides = func(overrides config.Overrides) (config.AppConfig, error) {
		applied := cfg
		if overrides.Provider != nil {
			applied.Provider = *overrides.Provider
			if profile, ok := applied.Providers[applied.Provider]; ok {
				if profile.APIEndpoint != "" {
					applied.APIEndpoint = profile.APIEndpoint
				}
				if profile.DefaultModel != "" {
					applied.DefaultModel = profile.DefaultModel
				}
				if profile.APIKey != "" {
					applied.APIKey = profile.APIKey
				}
			}
		}
		if overrides.DefaultModel != nil {
			applied.DefaultModel = *overrides.DefaultModel
		}
		if overrides.APIEndpoint != nil {
			applied.APIEndpoint = *overrides.APIEndpoint
		}
		if overrides.APIKey != nil {
			applied.APIKey = *overrides.APIKey
		}
		if overrides.DefaultRole != nil {
			applied.DefaultRole = *overrides.DefaultRole
		}
		return applied, nil
	}

	code := run()
	return stdout.String() + stderr.String(), code
}
