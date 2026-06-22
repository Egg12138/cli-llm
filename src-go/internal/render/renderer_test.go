package render

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestRenderMessageUsesMarkdownRenderer(t *testing.T) {
	originalRenderMarkdown := RenderMarkdownForTest
	t.Cleanup(func() {
		RenderMarkdownForTest = originalRenderMarkdown
	})

	RenderMarkdownForTest = func(content string) (string, error) {
		if content != "**Bold** with `code`." {
			t.Fatalf("expected markdown content, got %q", content)
		}
		return "rendered-output", nil
	}

	renderer := Renderer{}
	text, err := renderer.RenderMessage(&schema.Message{
		Role:    schema.Assistant,
		Content: "**Bold** with `code`.",
	})
	if err != nil {
		t.Fatalf("render message: %v", err)
	}
	if text != "rendered-output" {
		t.Fatalf("expected rendered output, got %q", text)
	}
}

func TestRenderMessagePropagatesMarkdownErrors(t *testing.T) {
	originalRenderMarkdown := RenderMarkdownForTest
	t.Cleanup(func() {
		RenderMarkdownForTest = originalRenderMarkdown
	})

	RenderMarkdownForTest = func(content string) (string, error) {
		return "", io.ErrUnexpectedEOF
	}

	renderer := Renderer{}
	_, err := renderer.RenderMessage(&schema.Message{
		Role:    schema.Assistant,
		Content: "**Bold**",
	})
	if err == nil || err != io.ErrUnexpectedEOF {
		t.Fatalf("expected markdown error, got %v", err)
	}
}

func TestRenderStreamWritesChunksAndReturnsConcatenatedText(t *testing.T) {
	var output bytes.Buffer
	reader, writer := schema.Pipe[*schema.Message](2)
	go func() {
		writer.Send(schema.AssistantMessage("stream-", nil), nil)
		writer.Send(schema.AssistantMessage("answer", nil), nil)
		writer.Close()
	}()

	renderer := Renderer{Writer: &output}
	text, err := renderer.RenderStream(reader)
	if err != nil {
		t.Fatalf("render stream: %v", err)
	}

	if text != "stream-answer" {
		t.Fatalf("expected concatenated text, got %q", text)
	}
	if got := output.String(); !strings.Contains(got, "stream-answer") {
		t.Fatalf("expected streamed writer output, got %q", got)
	}
}
