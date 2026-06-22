package runtime

import "testing"

type fakeStatusReporter struct {
	calls []Status
	clear bool
}

func (f *fakeStatusReporter) Set(s Status) {
	f.calls = append(f.calls, s)
}

func (f *fakeStatusReporter) Clear() {
	f.clear = true
}

func TestStatusReporterInterface(t *testing.T) {
	f := &fakeStatusReporter{}
	f.Set(StatusThinking)
	if len(f.calls) != 1 || f.calls[0] != StatusThinking {
		t.Fatalf("expected StatusThinking call, got %v", f.calls)
	}
	f.Set(StatusWaitingStream)
	f.Set(StatusStreaming)
	if len(f.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(f.calls))
	}
	f.Clear()
	if !f.clear {
		t.Fatal("expected Clear() to set clear flag")
	}
}

func TestNopReporterIsNoop(t *testing.T) {
	var n nopReporter
	n.Set(StatusThinking)
	n.Set(StatusWaitingStream)
	n.Clear()
}

func TestStatusValues(t *testing.T) {
	if StatusThinking != 0 {
		t.Fatalf("StatusThinking = %d, want 0", StatusThinking)
	}
	if StatusWaitingStream != 1 {
		t.Fatalf("StatusWaitingStream = %d, want 1", StatusWaitingStream)
	}
	if StatusStreaming != 2 {
		t.Fatalf("StatusStreaming = %d, want 2", StatusStreaming)
	}
}
