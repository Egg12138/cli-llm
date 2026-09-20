package tui

import (
	"bytes"
	"testing"

	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
)

func TestEditorReaderReturnsSubmittedEventWithoutAltScreen(t *testing.T) {
	in := bytes.NewBufferString("hello 世界\r")
	var out bytes.Buffer
	reader := NewEditorReader(in, &out)

	event := reader.ReadEvent("> ")
	if event.Kind != sessionrepl.EventLine || event.Line != "hello 世界" || event.Err != nil {
		t.Fatalf("ReadEvent returned %#v", event)
	}
	if bytes.Contains(out.Bytes(), []byte("\x1b[?1049h")) {
		t.Fatalf("inline editor entered alternate screen: %q", out.Bytes())
	}
}

func TestEditorReaderDoesNotCommitSlashCommandsAsChat(t *testing.T) {
	in := bytes.NewBufferString("/help\r")
	var out bytes.Buffer
	reader := NewEditorReader(in, &out)

	event := reader.ReadEvent("> ")
	if event.Line != "/help" || event.Err != nil {
		t.Fatalf("ReadEvent returned %#v", event)
	}
	if bytes.HasSuffix(out.Bytes(), []byte("> /help\n")) {
		t.Fatalf("slash command was committed as a chat prompt: %q", out.Bytes())
	}
}
