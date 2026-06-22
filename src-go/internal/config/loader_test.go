package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoaderPrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[defaults]
provider = "deepseek"
model = "file-model"
role = "normal"

[provider]
api_key = "file-key"
api_endpoint = "https://file.example/v1"

[providers.deepseek]
api_key = "deepseek-key"
api_endpoint = "https://deepseek.example/v1"
default_model = "deepseek-chat"
models = ["deepseek-chat", "deepseek-coder"]

[providers.openai]
api_key = "openai-key"
api_endpoint = "https://openai.example/v1"
default_model = "gpt-4o-mini"
models = ["gpt-4o", "gpt-4o-mini"]
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loader := NewLoader(configPath, nil)
	cfg, err := loader.Load(LoadOptions{
		Environment: map[string]string{
			"OPENAI_API_KEY":       "env-key",
			"OPENAI_BASE_URL":      "https://env.example/v1",
			"OPENAI_MODEL":         "env-model",
			"CLI_LLM_DEFAULT_ROLE": "coder",
			"CLI_LLM_PROVIDER":     "openai",
		},
		CLI: Overrides{
			DefaultModel: stringPtr("cli-model"),
			Provider:     stringPtr("deepseek"),
		},
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider != "deepseek" {
		t.Fatalf("expected provider deepseek, got %q", cfg.Provider)
	}
	if cfg.DefaultModel != "cli-model" {
		t.Fatalf("expected default model cli-model, got %q", cfg.DefaultModel)
	}
	if cfg.DefaultRole != "coder" {
		t.Fatalf("expected default role coder, got %q", cfg.DefaultRole)
	}
	if cfg.APIEndpoint != "https://env.example/v1" {
		t.Fatalf("expected env API endpoint, got %q", cfg.APIEndpoint)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("expected env API key, got %q", cfg.APIKey)
	}
}

func TestConfigLoaderUsesProviderProfileForActiveProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[defaults]
provider = "openai"

[providers.openai]
api_key = "profile-key"
api_endpoint = "https://profile.example/v1"
default_model = "gpt-4o-mini"
models = ["gpt-4o", "gpt-4o-mini"]
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loader := NewLoader(configPath, nil)
	cfg, err := loader.Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", cfg.Provider)
	}
	if cfg.APIKey != "profile-key" {
		t.Fatalf("expected profile API key, got %q", cfg.APIKey)
	}
	if cfg.APIEndpoint != "https://profile.example/v1" {
		t.Fatalf("expected profile API endpoint, got %q", cfg.APIEndpoint)
	}
	if cfg.DefaultModel != "gpt-4o-mini" {
		t.Fatalf("expected profile default model, got %q", cfg.DefaultModel)
	}
	if len(cfg.Providers["openai"].Models) != 2 {
		t.Fatalf("expected 2 models, got %#v", cfg.Providers["openai"].Models)
	}
}

func TestConfigLoaderFallsBackToDefaultsWithoutFile(t *testing.T) {
	t.Parallel()

	loader := NewLoader(filepath.Join(t.TempDir(), "missing.toml"), nil)
	cfg, err := loader.Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider != DefaultConfigValues.Provider {
		t.Fatalf("expected default provider %q, got %q", DefaultConfigValues.Provider, cfg.Provider)
	}
	if cfg.APIEndpoint != DefaultConfigValues.APIEndpoint {
		t.Fatalf("expected default endpoint %q, got %q", DefaultConfigValues.APIEndpoint, cfg.APIEndpoint)
	}
	if cfg.DefaultModel != DefaultConfigValues.DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultConfigValues.DefaultModel, cfg.DefaultModel)
	}
}

func stringPtr(value string) *string {
	return &value
}
