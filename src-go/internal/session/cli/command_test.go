package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	mode Mode
	name string
	runs int
	err  error
}

func (r *fakeRunner) Run(options Options) error {
	r.mode = options.Mode
	r.name = options.Name
	r.runs++
	return r.err
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
	var stderr bytes.Buffer
	originalStderr := commandStderr
	commandStderr = &stderr
	t.Cleanup(func() {
		commandStderr = originalStderr
	})
	code := Run([]string{"--unknown"}, runner)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if runner.runs != 0 {
		t.Fatalf("runner should not be called, got %d calls", runner.runs)
	}
	if !strings.Contains(stderr.String(), "unknown") {
		t.Fatalf("expected flag error on stderr, got %q", stderr.String())
	}
}

func TestCommandReportsRunnerErrorAndExitsOne(t *testing.T) {
	runner := &fakeRunner{err: errors.New("resume missing: no such session")}
	var stderr bytes.Buffer
	originalStderr := commandStderr
	commandStderr = &stderr
	t.Cleanup(func() {
		commandStderr = originalStderr
	})

	code := Run([]string{"--resume", "missing"}, runner)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "resume missing") {
		t.Fatalf("expected resume error on stderr, got %q", stderr.String())
	}
}

func TestCommandTreatsCancelledResumeAsCleanExit(t *testing.T) {
	runner := &fakeRunner{err: errResumeCancelled}
	var stderr bytes.Buffer
	originalStderr := commandStderr
	commandStderr = &stderr
	t.Cleanup(func() {
		commandStderr = originalStderr
	})

	code := Run([]string{"--resume"}, runner)

	if code != 0 {
		t.Fatalf("expected exit code 0 for a cancelled resume, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("cancelled resume wrote to stderr: %q", stderr.String())
	}
}

func TestCommandStartsResumePickerWithShorthand(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	if code := Run([]string{"-r"}, runner); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if runner.mode != ModeResumePicker {
		t.Fatalf("expected resume picker mode, got %q", runner.mode)
	}
}

func TestCommandStartsContinueMode(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"--continue"}, {"-c"}} {
		runner := &fakeRunner{}
		if code := Run(args, runner); code != 0 {
			t.Fatalf("args %v: expected exit code 0, got %d", args, code)
		}
		if runner.mode != ModeContinue {
			t.Fatalf("args %v: expected continue mode, got %q", args, runner.mode)
		}
		if runner.name != "" {
			t.Fatalf("args %v: expected empty name, got %q", args, runner.name)
		}
	}
}

func TestCommandRejectsConflictingSessionSelection(t *testing.T) {
	for _, args := range [][]string{{"--resume", "--continue"}, {"--continue", "work"}} {
		runner := &fakeRunner{}
		var stderr bytes.Buffer
		originalStderr := commandStderr
		commandStderr = &stderr
		code := Run(args, runner)
		commandStderr = originalStderr

		if code != 2 {
			t.Fatalf("args %v: expected exit code 2, got %d", args, code)
		}
		if runner.runs != 0 {
			t.Fatalf("args %v: runner should not be called, got %d calls", args, runner.runs)
		}
		if stderr.Len() == 0 {
			t.Fatalf("args %v: expected an error message on stderr", args)
		}
	}
}

func TestParseReplayFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want ReplayMode
	}{
		{name: "default", args: []string{"--resume", "work"}, want: ReplayAuto},
		{name: "pick", args: []string{"--resume"}, want: ReplayAuto},
		{name: "replay", args: []string{"--replay", "--resume", "work"}, want: ReplayOn},
		{name: "no-replay", args: []string{"--no-replay", "--resume", "work"}, want: ReplayOff},
		{name: "continue", args: []string{"--continue", "--no-replay"}, want: ReplayOff},
	}
	for _, tc := range cases {
		options, err := Parse(tc.args)
		if err != nil {
			t.Fatalf("args %v returned error: %v", tc.args, err)
		}
		if options.Replay != tc.want {
			t.Fatalf("args %v parsed replay %q, want %q", tc.args, options.Replay, tc.want)
		}
	}

	if _, err := Parse([]string{"--replay", "--no-replay"}); err == nil {
		t.Fatal("conflicting replay flags should be rejected")
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
