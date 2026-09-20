package cli

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestTurnKeyMonitorCancelsEscapeAndCtrlCAndRestoresTerminal(t *testing.T) {
	for _, test := range []struct {
		name string
		key  byte
	}{
		{name: "escape", key: turnCancelEscape},
		{name: "ctrl-c", key: turnCancelCtrlC},
	} {
		t.Run(test.name, func(t *testing.T) {
			terminal, peer, err := pty.Open()
			if err != nil {
				t.Fatalf("open PTY: %v", err)
			}
			defer terminal.Close()
			defer peer.Close()

			before, err := term.GetState(int(peer.Fd()))
			if err != nil {
				t.Fatalf("get initial terminal state: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			monitor, err := startTurnKeyMonitor(peer, cancel)
			if err != nil {
				t.Fatalf("startTurnKeyMonitor: %v", err)
			}
			if _, err := terminal.Write([]byte{test.key}); err != nil {
				t.Fatalf("write cancel key: %v", err)
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("cancel key did not cancel the turn context")
			}
			if err := monitor.stop(); err != nil {
				t.Fatalf("stop monitor: %v", err)
			}
			after, err := term.GetState(int(peer.Fd()))
			if err != nil {
				t.Fatalf("get restored terminal state: %v", err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatal("terminal state was not restored after cancellation")
			}
		})
	}
}

func TestTurnKeyMonitorIgnoresOrdinaryInput(t *testing.T) {
	terminal, peer, err := pty.Open()
	if err != nil {
		t.Fatalf("open PTY: %v", err)
	}
	defer terminal.Close()
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	monitor, err := startTurnKeyMonitor(peer, cancel)
	if err != nil {
		t.Fatalf("startTurnKeyMonitor: %v", err)
	}
	if _, err := terminal.Write([]byte("typing ahead")); err != nil {
		t.Fatalf("write ordinary input: %v", err)
	}
	select {
	case <-ctx.Done():
		t.Fatal("ordinary input cancelled the active turn")
	case <-time.After(50 * time.Millisecond):
	}
	if err := monitor.stop(); err != nil {
		t.Fatalf("stop monitor: %v", err)
	}
}

func TestTurnKeyMonitorSkipsNonTerminalInput(t *testing.T) {
	monitor, err := startTurnKeyMonitor(&bytes.Buffer{}, func() {})
	if err != nil {
		t.Fatalf("startTurnKeyMonitor: %v", err)
	}
	if monitor != nil {
		t.Fatal("non-terminal input unexpectedly started a key monitor")
	}
}
