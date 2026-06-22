package config

type Defaults struct {
	APIEndpoint  string
	DefaultModel string
	DefaultRole  string
	Provider     string
}

var DefaultConfigValues = Defaults{
	APIEndpoint:  "https://api.openai.com/v1",
	DefaultModel: "deepseek-chat",
	DefaultRole:  "coder",
	Provider:     "openai",
}

type ProviderConfig struct {
	APIKey       string
	APIEndpoint  string
	DefaultModel string
	Models       []string
}

type AppConfig struct {
	APIKey       string
	APIEndpoint  string
	DefaultModel string
	DefaultRole  string
	Provider     string
	Providers    map[string]ProviderConfig
	AppTitle     string
	AppURL       string
	ExtraHeaders map[string]string
}

func (c AppConfig) ResolvedAPIKey() string {
	if c.APIKey == "" {
		return "dummy-key"
	}
	return c.APIKey
}
