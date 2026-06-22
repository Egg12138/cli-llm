package render

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/cloudwego/eino/schema"
)

type Renderer struct {
	Writer io.Writer
}

var renderMarkdown = defaultRenderMarkdown
var RenderMarkdownForTest = defaultRenderMarkdown

func (Renderer) RenderMessage(message *schema.Message) (string, error) {
	if message == nil {
		return "", nil
	}
	return RenderMarkdownForTest(message.Content)
}

func (r Renderer) RenderStream(stream *schema.StreamReader[*schema.Message]) (string, error) {
	if stream == nil {
		return "", nil
	}

	writer := r.Writer
	if writer == nil {
		writer = os.Stdout
	}

	fullText := ""
	for {
		message, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if message == nil {
			break
		}
		if message.Content == "" {
			continue
		}
		fullText += message.Content
		if _, err := fmt.Fprint(writer, message.Content); err != nil {
			return "", err
		}
	}
	if fullText != "" {
		if _, err := fmt.Fprintln(writer); err != nil {
			return "", err
		}
	}
	return fullText, nil
}

func defaultRenderMarkdown(content string) (string, error) {
	if content == "" {
		return "", nil
	}
	if !shouldRenderMarkdown(content) {
		return content, nil
	}
	rendered, err := glamour.Render(content, "dark")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(rendered, "\n"), nil
}

func shouldRenderMarkdown(content string) bool {
	if strings.Contains(content, "```") {
		return false
	}

	markers := []string{
		"**",
		"__",
		"`",
		"# ",
		"\n# ",
		"- ",
		"\n- ",
		"* ",
		"\n* ",
	}
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}
