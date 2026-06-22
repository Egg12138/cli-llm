package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptAssemblyResolvesPredefinedRole(t *testing.T) {
	registry := NewRoleRegistry()

	result := BuildSystemPrompt(BuildOptions{
		RoleName:     "coder",
		RoleFallback: "chat",
	}, registry)

	if result.RoleName != "coder" {
		t.Fatalf("expected coder role, got %q", result.RoleName)
	}
	if !strings.Contains(result.Role.Content, "helpful programmer assistant") {
		t.Fatalf("expected coder system prompt content, got %q", result.Role.Content)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", result.Warnings)
	}
}

func TestPromptAssemblyFallsBackToConfiguredDefaultRole(t *testing.T) {
	registry := NewRoleRegistry()

	result := BuildSystemPrompt(BuildOptions{
		RoleName:     "does-not-exist",
		RoleFallback: "chat",
	}, registry)

	if result.RoleName != "chat" {
		t.Fatalf("expected fallback chat role, got %q", result.RoleName)
	}
	if result.Role.Temperature != 1.3 {
		t.Fatalf("expected chat temperature, got %v", result.Role.Temperature)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected one warning, got %v", result.Warnings)
	}
}

func TestRoleRegistryUsesCurrentPromptDefinitions(t *testing.T) {
	registry := NewRoleRegistry()

	_, coderRole, _ := registry.Resolve("coder", "coder")
	if !strings.Contains(coderRole.Content, "function_format") {
		t.Fatalf("expected coder role to include current command prompt content, got %q", coderRole.Content)
	}

	_, metaRole, _ := registry.Resolve("meta", "coder")
	if !strings.Contains(metaRole.Content, "Best-of-N 采样") {
		t.Fatalf("expected meta role to include current methodology prompt content, got %q", metaRole.Content)
	}
}

func TestPromptAssemblyAppendsAgentsContextOnlyWhenEnabled(t *testing.T) {
	registry := NewRoleRegistry()
	tempDir := t.TempDir()
	agentsPath := filepath.Join(tempDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("project\x00-context"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	disabled := BuildSystemPrompt(BuildOptions{
		RoleName:             "coder",
		RoleFallback:         "coder",
		AgentsContextEnabled: false,
		WorkingDir:           tempDir,
	}, registry)
	if strings.Contains(disabled.SystemMessage, "# Project Context (AGENTS.md)") {
		t.Fatalf("agents context should not be appended when disabled")
	}

	enabled := BuildSystemPrompt(BuildOptions{
		RoleName:             "coder",
		RoleFallback:         "coder",
		AgentsContextEnabled: true,
		WorkingDir:           tempDir,
	}, registry)
	if !strings.Contains(enabled.SystemMessage, "# Project Context (AGENTS.md)") {
		t.Fatalf("expected agents context header in system message")
	}
	if strings.Contains(enabled.AgentsContext, "\x00") {
		t.Fatalf("expected agents context to be sanitized, got %q", enabled.AgentsContext)
	}
	if enabled.AgentsContext != "project-context" {
		t.Fatalf("unexpected sanitized agents context %q", enabled.AgentsContext)
	}
}

func TestPromptAssemblyTruncatesOversizedAgentsContext(t *testing.T) {
	registry := NewRoleRegistry()
	tempDir := t.TempDir()
	agentsPath := filepath.Join(tempDir, "AGENTS.md")
	content := strings.Repeat("A", MaxAgentsBytes+512)
	if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	result := BuildSystemPrompt(BuildOptions{
		RoleName:             "coder",
		RoleFallback:         "coder",
		AgentsContextEnabled: true,
		WorkingDir:           tempDir,
	}, registry)

	if len(result.AgentsContext) != MaxAgentsBytes {
		t.Fatalf("expected agents context length %d, got %d", MaxAgentsBytes, len(result.AgentsContext))
	}
	if len(result.Warnings) != 1 || !strings.Contains(strings.ToLower(result.Warnings[0]), "truncating") {
		t.Fatalf("expected truncation warning, got %v", result.Warnings)
	}
}

func TestPromptAssemblyMissingAgentsFileIsWarningOnly(t *testing.T) {
	registry := NewRoleRegistry()

	result := BuildSystemPrompt(BuildOptions{
		RoleName:             "coder",
		RoleFallback:         "coder",
		AgentsContextEnabled: true,
		WorkingDir:           t.TempDir(),
	}, registry)

	if result.AgentsContext != "" {
		t.Fatalf("expected empty agents context, got %q", result.AgentsContext)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "AGENTS.md not found") {
		t.Fatalf("expected missing file warning, got %v", result.Warnings)
	}
}
