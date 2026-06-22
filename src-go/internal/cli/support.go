package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
)

var (
	commandStdout          io.Writer = os.Stdout
	commandStderr          io.Writer = os.Stderr
	loadAppConfig                    = defaultLoadAppConfig
	loadAppConfigOverrides           = defaultLoadAppConfigOverrides
)

func defaultLoadAppConfig() (config.AppConfig, error) {
	return defaultLoadAppConfigOverrides(config.Overrides{})
}

func defaultLoadAppConfigOverrides(overrides config.Overrides) (config.AppConfig, error) {
	loader := config.NewLoader(defaultConfigPath(), nil)
	return loader.Load(config.LoadOptions{
		CLI:         overrides,
		Environment: currentEnvironment(),
	})
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cli-llm", "config.toml")
}

func currentEnvironment() map[string]string {
	environment := make(map[string]string)
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		environment[parts[0]] = parts[1]
	}
	return environment
}
