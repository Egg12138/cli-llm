package repl

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/runtime"
)

type PlainReporter struct {
	Out io.Writer
}

func (r *PlainReporter) Set(s runtime.Status) {
	fmt.Fprintf(r.Out, "[%s]\n", statusLabel(s))
}

func (r *PlainReporter) Clear() {}

type TTYReporter struct {
	mu     sync.Mutex
	status runtime.Status
	done   chan struct{}
	out    io.Writer
}

func NewTTYReporter(out io.Writer) *TTYReporter {
	r := &TTYReporter{out: out, done: make(chan struct{})}
	go r.run()
	return r
}

func (r *TTYReporter) Set(s runtime.Status) {
	r.mu.Lock()
	r.status = s
	r.mu.Unlock()
}

func (r *TTYReporter) Clear() {
	select {
	case <-r.done:
		return
	default:
		close(r.done)
	}
	fmt.Fprint(r.out, "\r\x1b[K")
}

func (r *TTYReporter) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			label := statusLabel(r.status)
			r.mu.Unlock()
			frame := frames[i%len(frames)]
			i++
			fmt.Fprintf(r.out, "\r%s %s\x1b[K", frame, label)
		case <-r.done:
			return
		}
	}
}

// nopCloser implements io.Closer with a no-op, so TTYReporter can be used
// as a stoppable resource.
func (r *TTYReporter) Close() error {
	r.Clear()
	return nil
}

func statusLabel(s runtime.Status) string {
	switch s {
	case runtime.StatusThinking:
		return "Thinking"
	case runtime.StatusWaitingStream:
		return "Waiting for stream"
	case runtime.StatusStreaming:
		return "Streaming"
	default:
		return ""
	}
}
