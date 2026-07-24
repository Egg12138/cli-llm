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
	mu       sync.Mutex
	status   runtime.Status
	done     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
	out      io.Writer
	interval time.Duration
}

func NewTTYReporter(out io.Writer) *TTYReporter {
	return newTTYReporter(out, 100*time.Millisecond)
}

func newTTYReporter(out io.Writer, interval time.Duration) *TTYReporter {
	r := &TTYReporter{
		out:      out,
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
		interval: interval,
	}
	go r.run()
	return r
}

func (r *TTYReporter) Set(s runtime.Status) {
	r.mu.Lock()
	r.status = s
	r.mu.Unlock()
}

func (r *TTYReporter) Clear() {
	r.stopOnce.Do(func() {
		close(r.done)
		<-r.stopped
		fmt.Fprint(r.out, "\r\x1b[2K")
	})
}

func (r *TTYReporter) run() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	defer close(r.stopped)
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-r.done:
			return
		default:
		}
		select {
		case <-r.done:
			return
		case <-ticker.C:
			select {
			case <-r.done:
				return
			default:
			}
			r.mu.Lock()
			label := statusLabel(r.status)
			r.mu.Unlock()
			frame := frames[i%len(frames)]
			i++
			fmt.Fprintf(r.out, "\r\x1b[2K%s %s", frame, label)
		}
	}
}

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
