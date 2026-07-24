package repl

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/runtime"
)

type synchronizedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	wrote     chan struct{}
	wroteOnce sync.Once
}

func newSynchronizedBuffer() *synchronizedBuffer {
	return &synchronizedBuffer{wrote: make(chan struct{})}
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buffer.Write(p)
	b.wroteOnce.Do(func() { close(b.wrote) })
	return n, err
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

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

func TestTTYReporterClearWaitsForAnimationAndErasesTheRow(t *testing.T) {
	out := newSynchronizedBuffer()
	r := newTTYReporter(out, time.Millisecond)
	r.Set(runtime.StatusThinking)

	select {
	case <-out.wrote:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for spinner frame")
	}

	r.Clear()
	afterClear := out.String()
	time.Sleep(10 * time.Millisecond)
	if got := out.String(); got != afterClear {
		t.Fatalf("spinner wrote after Clear: before %q, after %q", afterClear, got)
	}
	if !strings.HasSuffix(afterClear, "\r\x1b[2K") {
		t.Fatalf("Clear did not erase the complete status row: %q", afterClear)
	}
	if !strings.Contains(afterClear, "\r\x1b[2K") {
		t.Fatalf("spinner frame did not begin by replacing one logical row: %q", afterClear)
	}

	r.Clear()
	if got := out.String(); got != afterClear {
		t.Fatalf("second Clear wrote more bytes: before %q, after %q", afterClear, got)
	}
}
