package prompts

import (
	"os"
	"path/filepath"
)

const MaxAgentsBytes = 16384

type BuildOptions struct {
	RoleName             string
	RoleFallback         string
	AgentsContextEnabled bool
	WorkingDir           string
}

type BuildResult struct {
	RoleName      string
	Role          SystemPrompt
	SystemMessage string
	AgentsContext string
	Warnings      []string
}

func BuildSystemPrompt(options BuildOptions, registry RoleRegistry) BuildResult {
	resolvedName, role, warning := registry.Resolve(options.RoleName, options.RoleFallback)
	result := BuildResult{
		RoleName:      resolvedName,
		Role:          role,
		SystemMessage: role.Content,
	}
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}

	if !options.AgentsContextEnabled {
		return result
	}

	agentsContext, warning := LoadAgentsContext(options.WorkingDir)
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	if agentsContext == "" {
		return result
	}

	result.AgentsContext = agentsContext
	result.SystemMessage += "\n\n---\n# Project Context (AGENTS.md)\n---\n" + agentsContext
	return result
}

func LoadAgentsContext(workingDir string) (string, string) {
	raw, err := os.ReadFile(filepath.Join(workingDir, "AGENTS.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", "Warning: ./AGENTS.md not found, skipping agents context."
		}
		return "", "Warning: failed to read ./AGENTS.md, skipping agents context."
	}

	warning := ""
	if len(raw) > MaxAgentsBytes {
		raw = raw[:MaxAgentsBytes]
		warning = "Warning: AGENTS.md exceeds 16 KB, truncating."
	}

	return SanitizeInput(string(raw)), warning
}
