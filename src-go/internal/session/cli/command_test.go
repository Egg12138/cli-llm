package cli

import (
	"bytes"
	"strings"
	"testing"
)

type fakeRunner struct {
	mode Mode
	name string
	runs int
}

func (r *fakeRunner) Run(options Options) error {
	r.mode = options.Mode
	r.name = options.Name
	r.runs++
	return nil
}

func TestCommandHelpDoesNotRunRunner(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	var out bytes.Buffer
	originalStdout := commandStdout
	commandStdout = &out
	t.Cleanup(func() {
		commandStdout = originalStdout
	})
	code := Run([]string{"--help"}, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runner.runs != 0 {
		t.Fatalf("runner should not be called for help, got %d calls", runner.runs)
	}
	if !strings.Contains(out.String(), "Usage: llm-session") {
		t.Fatalf("expected usage output, got %q", out.String())
	}
}

func TestCommandVersionPrintsVersion(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	var out bytes.Buffer
	originalStdout := commandStdout
	commandStdout = &out
	t.Cleanup(func() {
		commandStdout = originalStdout
	})
	code := Run([]string{"--version"}, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runner.runs != 0 {
		t.Fatalf("runner should not be called for version, got %d calls", runner.runs)
	}
	if !strings.Contains(out.String(), Version) {
		t.Fatalf("expected version %q in output, got %q", Version, out.String())
	}
	if !strings.Contains(out.String(), "llm-session") {
		t.Fatalf("expected llm-session prefix in output, got %q", out.String())
	}
}

func TestCommandStartsFreshMode(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	code := Run([]string{}, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runner.mode != ModeFresh {
		t.Fatalf("expected fresh mode, got %q", runner.mode)
	}
	if runner.name != "" {
		t.Fatalf("expected empty name, got %q", runner.name)
	}
	if runner.runs != 1 {
		t.Fatalf("expected one runner call, got %d", runner.runs)
	}
}

func TestCommandStartsResumePickerMode(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	code := Run([]string{"--resume"}, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runner.mode != ModeResumePicker {
		t.Fatalf("expected resume picker mode, got %q", runner.mode)
	}
	if runner.name != "" {
		t.Fatalf("expected empty name, got %q", runner.name)
	}
}

func TestCommandStartsNamedResumeMode(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	code := Run([]string{"--resume", "work"}, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runner.mode != ModeResumeNamed {
		t.Fatalf("expected named resume mode, got %q", runner.mode)
	}
	if runner.name != "work" {
		t.Fatalf("expected name work, got %q", runner.name)
	}
}

func TestCommandRejectsInvalidFlags(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	code := Run([]string{"--unknown"}, runner)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if runner.runs != 0 {
		t.Fatalf("runner should not be called, got %d calls", runner.runs)
	}
}

func TestDefaultAppConfigReadsEnvironmentAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-session-key")
	t.Setenv("OPENAI_BASE_URL", "https://env-session.example/v1")
	t.Setenv("OPENAI_MODEL", "env-session-model")

	cfg, err := defaultAppConfig()
	if err != nil {
		t.Fatalf("defaultAppConfig returned error: %v", err)
	}

	if cfg.APIKey != "env-session-key" {
		t.Fatalf("expected API key from environment, got %q", cfg.APIKey)
	}
	if cfg.APIEndpoint != "https://env-session.example/v1" {
		t.Fatalf("expected API endpoint from environment, got %q", cfg.APIEndpoint)
	}
	if cfg.DefaultModel != "env-session-model" {
		t.Fatalf("expected default model from environment, got %q", cfg.DefaultModel)
	}
}
