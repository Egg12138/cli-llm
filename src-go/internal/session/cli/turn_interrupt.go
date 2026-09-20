package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"

	"github.com/chzyer/readline"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

const (
	turnCancelEscape = byte(0x1b)
	turnCancelCtrlC  = byte(0x03)
)

type turnKeyMonitor struct {
	file    *os.File
	reader  cancelreader.CancelReader
	state   *readline.State
	done    chan struct{}
	readErr error
}

func startTurnKeyMonitor(in io.Reader, cancel context.CancelFunc) (*turnKeyMonitor, error) {
	file, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return nil, nil
	}

	// readline.MakeRaw intentionally leaves OPOST enabled, so streamed '\n'
	// bytes keep normal terminal line positioning while input becomes immediate.
	state, err := readline.MakeRaw(int(file.Fd()))
	if err != nil {
		return nil, err
	}
	reader, err := cancelreader.NewReader(file)
	if err != nil {
		_ = readline.Restore(int(file.Fd()), state)
		return nil, err
	}
	monitor := &turnKeyMonitor{
		file:   file,
		reader: reader,
		state:  state,
		done:   make(chan struct{}),
	}
	go monitor.read(cancel)
	return monitor, nil
}

func (m *turnKeyMonitor) read(cancel context.CancelFunc) {
	defer close(m.done)
	buffer := make([]byte, 32)
	for {
		n, err := m.reader.Read(buffer)
		if err != nil {
			if !errors.Is(err, cancelreader.ErrCanceled) {
				m.readErr = err
				cancel()
			}
			return
		}
		input := buffer[:n]
		if bytes.IndexByte(input, turnCancelEscape) >= 0 || bytes.IndexByte(input, turnCancelCtrlC) >= 0 {
			cancel()
			return
		}
	}
}

func (m *turnKeyMonitor) stop() error {
	if m == nil {
		return nil
	}
	m.reader.Cancel()
	<-m.done
	closeErr := m.reader.Close()
	restoreErr := readline.Restore(int(m.file.Fd()), m.state)
	return errors.Join(m.readErr, closeErr, restoreErr)
}
