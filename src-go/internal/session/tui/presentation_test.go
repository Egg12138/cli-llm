package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPresentationWritesPlainConversationWithRoleSpacing(t *testing.T) {
	var out bytes.Buffer
	presentation := NewPresentation(&out, false)

	if err := presentation.WriteUser("第一行\nsecond line"); err != nil {
		t.Fatalf("WriteUser returned error: %v", err)
	}
	assistant := presentation.NewAssistantWriter()
	if _, err := assistant.Write([]byte("first\n")); err != nil {
		t.Fatalf("first assistant write: %v", err)
	}
	if _, err := assistant.Write([]byte("second")); err != nil {
		t.Fatalf("second assistant write: %v", err)
	}
	if err := assistant.Finish(); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}

	const want = "You ›\n第一行\nsecond line\n\nAssistant ›\nfirst\nsecond\n\n"
	if got := out.String(); got != want {
		t.Fatalf("conversation output = %q, want %q", got, want)
	}
}

func TestAssistantPresentationWaitsForContentAndPreservesExistingBlankLine(t *testing.T) {
	var out bytes.Buffer
	presentation := NewPresentation(&out, false)
	assistant := presentation.NewAssistantWriter()

	if err := assistant.Finish(); err != nil {
		t.Fatalf("empty Finish returned error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("empty response rendered output %q", out.String())
	}
	assistant = presentation.NewAssistantWriter()
	if _, err := assistant.Write([]byte("answer\n\n")); err != nil {
		t.Fatalf("assistant write: %v", err)
	}
	if err := assistant.Finish(); err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}
	if got, want := out.String(), "Assistant ›\nanswer\n\n"; got != want {
		t.Fatalf("assistant output = %q, want %q", got, want)
	}
}

func TestPresentationColorsLabelsAndDimsCommandOutput(t *testing.T) {
	var out bytes.Buffer
	presentation := NewPresentation(&out, true)

	if !strings.Contains(presentation.UserLabel(), "\x1b[") {
		t.Fatalf("colored user label has no ANSI styling: %q", presentation.UserLabel())
	}
	if got := ansi.Strip(presentation.UserLabel()); got != userLabel {
		t.Fatalf("stripped user label = %q, want %q", got, userLabel)
	}
	if _, err := presentation.CommandWriter().Write([]byte("Available commands:\n")); err != nil {
		t.Fatalf("command write: %v", err)
	}
	if !strings.Contains(out.String(), "\x1b[2m") {
		t.Fatalf("command output is not dim: %q", out.String())
	}
	if got := ansi.Strip(out.String()); got != "Available commands:\n" {
		t.Fatalf("stripped command output = %q", got)
	}
}

func TestPresentationWithoutColorEmitsNoANSI(t *testing.T) {
	var out bytes.Buffer
	presentation := NewPresentation(&out, false)
	if err := presentation.WriteUser("question"); err != nil {
		t.Fatalf("WriteUser returned error: %v", err)
	}
	if _, err := presentation.CommandWriter().Write([]byte("command\n")); err != nil {
		t.Fatalf("command write: %v", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("colorless presentation emitted ANSI: %q", out.String())
	}
}
