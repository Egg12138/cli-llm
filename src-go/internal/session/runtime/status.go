package runtime

type Status int

const (
	StatusThinking Status = iota
	StatusWaitingStream
	StatusStreaming
)

type StatusReporter interface {
	Set(Status)
	Clear()
}

type nopReporter struct{}

func (nopReporter) Set(Status) {}
func (nopReporter) Clear()    {}
