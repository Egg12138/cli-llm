package toolcallflow_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/tools"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/toolcallflow"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type stubToolChatModel struct {
	response      *schema.Message
	generateCalls int
	lastMessages  []*schema.Message
	lastOptions   *model.Options
}

func (s *stubToolChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	s.generateCalls++
	s.lastMessages = cloneMessages(input)
	s.lastOptions = model.GetCommonOptions(nil, opts...)
	return s.response, nil
}

func (s *stubToolChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	panic("stream not used in toolcall workflow tests")
}

func TestToolcallflowExposesOnlyEnabledTools(t *testing.T) {
	ctx := context.Background()
	model := &stubToolChatModel{
		response: assistantToolCall("call-1", "read", `{"path":"README.md"}`),
	}
	workflow, err := toolcallflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new toolcall workflow: %v", err)
	}

	cwd := t.TempDir()
	writeFile(t, cwd, "README.md", "hello")

	definitions, err := tools.GetDefinitions([]string{"read", "ls"})
	if err != nil {
		t.Fatalf("get definitions: %v", err)
	}

	result, err := workflow.Run(ctx, toolcallflow.Request{
		Prompt:     "Read the README",
		Tools:      definitions,
		WorkingDir: cwd,
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	if result.Tool != "read" {
		t.Fatalf("expected read tool, got %q", result.Tool)
	}
	if len(model.lastOptions.Tools) != 2 {
		t.Fatalf("expected 2 exposed tools, got %d", len(model.lastOptions.Tools))
	}
	if model.lastOptions.Tools[0].Name != "read" || model.lastOptions.Tools[1].Name != "ls" {
		t.Fatalf("unexpected exposed tools: %q, %q", model.lastOptions.Tools[0].Name, model.lastOptions.Tools[1].Name)
	}
}

func TestToolcallflowRequiresExactlyOneToolCall(t *testing.T) {
	ctx := context.Background()
	model := &stubToolChatModel{
		response: &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "call-1", Type: "function", Function: schema.FunctionCall{Name: "read", Arguments: `{"path":"README.md"}`}},
				{ID: "call-2", Type: "function", Function: schema.FunctionCall{Name: "ls", Arguments: `{}`}},
			},
		},
	}
	workflow, err := toolcallflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new toolcall workflow: %v", err)
	}

	cwd := t.TempDir()
	writeFile(t, cwd, "README.md", "hello")
	definitions, err := tools.GetDefinitions([]string{"read", "ls"})
	if err != nil {
		t.Fatalf("get definitions: %v", err)
	}

	_, err = workflow.Run(ctx, toolcallflow.Request{
		Prompt:     "Read the README",
		Tools:      definitions,
		WorkingDir: cwd,
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one tool call") {
		t.Fatalf("expected exactly-one-tool-call error, got %v", err)
	}
}

func TestToolcallflowValidatesArgumentsAgainstSchema(t *testing.T) {
	ctx := context.Background()
	model := &stubToolChatModel{
		response: assistantToolCall("call-1", "read", `{"path":"README.md","limit":"oops"}`),
	}
	workflow, err := toolcallflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new toolcall workflow: %v", err)
	}

	cwd := t.TempDir()
	writeFile(t, cwd, "README.md", "hello")
	definitions, err := tools.GetDefinitions([]string{"read"})
	if err != nil {
		t.Fatalf("get definitions: %v", err)
	}

	_, err = workflow.Run(ctx, toolcallflow.Request{
		Prompt:     "Read the README",
		Tools:      definitions,
		WorkingDir: cwd,
	})
	if err == nil || !strings.Contains(err.Error(), "must be an integer") {
		t.Fatalf("expected integer validation error, got %v", err)
	}
}

func TestToolcallflowRejectsPathEscape(t *testing.T) {
	ctx := context.Background()
	model := &stubToolChatModel{
		response: assistantToolCall("call-1", "read", `{"path":"../secret.txt"}`),
	}
	workflow, err := toolcallflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new toolcall workflow: %v", err)
	}

	cwd := t.TempDir()
	definitions, err := tools.GetDefinitions([]string{"read"})
	if err != nil {
		t.Fatalf("get definitions: %v", err)
	}

	_, err = workflow.Run(ctx, toolcallflow.Request{
		Prompt:     "Read the secret file",
		Tools:      definitions,
		WorkingDir: cwd,
	})
	if err == nil || !strings.Contains(err.Error(), "escapes the working directory") {
		t.Fatalf("expected path escape error, got %v", err)
	}
}

func TestToolcallflowSanitizesAndTruncatesStdout(t *testing.T) {
	ctx := context.Background()
	model := &stubToolChatModel{
		response: assistantToolCall("call-1", "bash", `{"command":"printf '\\033[31mRED\\033[0m'; yes A | head -c 70000"}`),
	}
	workflow, err := toolcallflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new toolcall workflow: %v", err)
	}

	definitions, err := tools.GetDefinitions([]string{"bash"})
	if err != nil {
		t.Fatalf("get definitions: %v", err)
	}

	result, err := workflow.Run(ctx, toolcallflow.Request{
		Prompt:     "Run the command",
		Tools:      definitions,
		WorkingDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	if strings.Contains(result.Stdout, "\x1b") {
		t.Fatalf("expected ANSI escapes to be removed, got %q", result.Stdout[:20])
	}
	if !strings.Contains(result.Stdout, "[truncated]") {
		t.Fatalf("expected truncated marker, got length %d", len(result.Stdout))
	}
}

func assistantToolCall(id string, name string, arguments string) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{
				ID:   id,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      name,
					Arguments: arguments,
				},
			},
		},
	}
}

func cloneMessages(messages []*schema.Message) []*schema.Message {
	cloned := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		copy := *message
		cloned = append(cloned, &copy)
	}
	return cloned
}

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
