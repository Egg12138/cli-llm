package config

import (
	"errors"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type Overrides struct {
	APIKey       *string
	APIEndpoint  *string
	DefaultModel *string
	DefaultRole  *string
	Provider     *string
}

type LoadOptions struct {
	CLI         Overrides
	Environment map[string]string
}

type Loader struct {
	userConfigPath string
	repoDefaults   Defaults
}

type mergedValues struct {
	APIKey       string
	APIEndpoint  string
	DefaultModel string
	DefaultRole  string
	Provider     string
}

type rawConfig struct {
	ProviderSection  rawProviderSection          `toml:"provider"`
	DefaultsSection  rawDefaultsSection          `toml:"defaults"`
	ProvidersSection map[string]rawProviderEntry `toml:"providers"`
	APIKey           string                      `toml:"api_key"`
	APIEndpoint      string                      `toml:"api_endpoint"`
	DefaultModel     string                      `toml:"default_model"`
	DefaultRole      string                      `toml:"default_role"`
	Provider         string                      `toml:"provider"`
}

type rawProviderSection struct {
	APIKey      string `toml:"api_key"`
	APIEndpoint string `toml:"api_endpoint"`
}

type rawDefaultsSection struct {
	Model    string `toml:"model"`
	Role     string `toml:"role"`
	Provider string `toml:"provider"`
}

type rawProviderEntry struct {
	APIKey       string   `toml:"api_key"`
	APIEndpoint  string   `toml:"api_endpoint"`
	DefaultModel string   `toml:"default_model"`
	Models       []string `toml:"models"`
}

func NewLoader(userConfigPath string, repoDefaults *Defaults) Loader {
	defaults := DefaultConfigValues
	if repoDefaults != nil {
		if repoDefaults.APIEndpoint != "" {
			defaults.APIEndpoint = repoDefaults.APIEndpoint
		}
		if repoDefaults.DefaultModel != "" {
			defaults.DefaultModel = repoDefaults.DefaultModel
		}
		if repoDefaults.DefaultRole != "" {
			defaults.DefaultRole = repoDefaults.DefaultRole
		}
		if repoDefaults.Provider != "" {
			defaults.Provider = repoDefaults.Provider
		}
	}

	return Loader{
		userConfigPath: userConfigPath,
		repoDefaults:   defaults,
	}
}

func (l Loader) Load(options LoadOptions) (AppConfig, error) {
	userValues, providerProfiles, err := l.userFileValues()
	if err != nil {
		return AppConfig{}, err
	}

	merged := userValues
	applyMerged(&merged, envValues(options.Environment))
	applyOverrides(&merged, options.CLI)

	providerName := merged.Provider
	if providerName == "" {
		providerName = l.repoDefaults.Provider
	}
	providerConfig := providerProfiles[providerName]

	apiEndpoint := firstNonEmpty(merged.APIEndpoint, providerConfig.APIEndpoint, l.repoDefaults.APIEndpoint)
	defaultModel := firstNonEmpty(merged.DefaultModel, providerConfig.DefaultModel, l.repoDefaults.DefaultModel)

	return AppConfig{
		APIKey:       firstNonEmpty(merged.APIKey, providerConfig.APIKey),
		APIEndpoint:  apiEndpoint,
		DefaultModel: defaultModel,
		DefaultRole:  firstNonEmpty(merged.DefaultRole, l.repoDefaults.DefaultRole),
		Provider:     providerName,
		Providers:    providerProfiles,
		AppTitle:     "egg-cli-llm",
		AppURL:       "https://github.com/Egg12138/cli-llm",
		ExtraHeaders: map[string]string{
			"HTTP-Referer": "https://github.com/Egg12138/cli-llm",
			"X-Title":      "egg-cli-llm",
		},
	}, nil
}

func (l Loader) userFileValues() (mergedValues, map[string]ProviderConfig, error) {
	if l.userConfigPath == "" {
		return mergedValues{}, map[string]ProviderConfig{}, nil
	}

	data, err := os.ReadFile(l.userConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mergedValues{}, map[string]ProviderConfig{}, nil
		}
		return mergedValues{}, nil, err
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return mergedValues{}, map[string]ProviderConfig{}, nil
	}

	merged := mergedValues{
		APIKey:       firstNonEmpty(raw.ProviderSection.APIKey, raw.APIKey),
		APIEndpoint:  firstNonEmpty(raw.ProviderSection.APIEndpoint, raw.APIEndpoint),
		DefaultModel: firstNonEmpty(raw.DefaultsSection.Model, raw.DefaultModel),
		DefaultRole:  firstNonEmpty(raw.DefaultsSection.Role, raw.DefaultRole),
		Provider:     firstNonEmpty(raw.DefaultsSection.Provider, raw.Provider),
	}

	profiles := make(map[string]ProviderConfig, len(raw.ProvidersSection))
	for name, entry := range raw.ProvidersSection {
		profiles[name] = ProviderConfig{
			APIKey:       entry.APIKey,
			APIEndpoint:  entry.APIEndpoint,
			DefaultModel: entry.DefaultModel,
			Models:       append([]string(nil), entry.Models...),
		}
	}

	return merged, profiles, nil
}

func envValues(environment map[string]string) mergedValues {
	if environment == nil {
		environment = map[string]string{}
	}
	return mergedValues{
		APIKey:       environment["OPENAI_API_KEY"],
		APIEndpoint:  environment["OPENAI_BASE_URL"],
		DefaultModel: environment["OPENAI_MODEL"],
		DefaultRole:  environment["CLI_LLM_DEFAULT_ROLE"],
		Provider:     environment["CLI_LLM_PROVIDER"],
	}
}

func applyOverrides(target *mergedValues, overrides Overrides) {
	if overrides.APIKey != nil {
		target.APIKey = *overrides.APIKey
	}
	if overrides.APIEndpoint != nil {
		target.APIEndpoint = *overrides.APIEndpoint
	}
	if overrides.DefaultModel != nil {
		target.DefaultModel = *overrides.DefaultModel
	}
	if overrides.DefaultRole != nil {
		target.DefaultRole = *overrides.DefaultRole
	}
	if overrides.Provider != nil {
		target.Provider = *overrides.Provider
	}
}

func applyMerged(target *mergedValues, other mergedValues) {
	target.APIKey = firstNonEmpty(other.APIKey, target.APIKey)
	target.APIEndpoint = firstNonEmpty(other.APIEndpoint, target.APIEndpoint)
	target.DefaultModel = firstNonEmpty(other.DefaultModel, target.DefaultModel)
	target.DefaultRole = firstNonEmpty(other.DefaultRole, target.DefaultRole)
	target.Provider = firstNonEmpty(other.Provider, target.Provider)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
