package repl

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/runtime"
)

func TestPlainReporterWritesDeterministicLines(t *testing.T) {
	var buf bytes.Buffer
	r := &PlainReporter{Out: &buf}

	r.Set(runtime.StatusThinking)
	if !strings.Contains(buf.String(), "Thinking") {
		t.Fatalf("expected thinking in output, got %q", buf.String())
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("expected trailing newline, got %q", buf.String())
	}

	r.Set(runtime.StatusWaitingStream)
	if !strings.Contains(buf.String(), "Waiting") {
		t.Fatalf("expected waiting in output after second set, got %q", buf.String())
	}

	r.Set(runtime.StatusStreaming)
	if !strings.Contains(buf.String(), "Streaming") {
		t.Fatalf("expected streaming in output, got %q", buf.String())
	}

	prev := buf.String()
	r.Clear()
	if buf.String() != prev {
		t.Fatalf("PlainReporter Clear should be a no-op, but output changed from %q to %q", prev, buf.String())
	}
}

func TestTTYReporterAnimatesAndClears(t *testing.T) {
	var buf bytes.Buffer
	r := NewTTYReporter(&buf)

	r.Set(runtime.StatusThinking)
	time.Sleep(250 * time.Millisecond) // let 2-3 ticks happen

	r.Clear()
	output := buf.String()

	if !strings.Contains(output, "Thinking") {
		t.Fatalf("expected Thinking label in output, got %q", output)
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 1 && strings.Contains(output, "\r") {
		// TTYReporter uses \r to overwrite, so all output is on one displayed line
		// Just verify the content
	}
}
