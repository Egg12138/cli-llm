package chatflow_test

import (
	"context"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/prompts"
	"github.com/Egg12138/cli-llm/src-go/internal/render"
	"github.com/Egg12138/cli-llm/src-go/internal/runtime"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/chatflow"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type stubChatModel struct {
	generateCalls int
	streamCalls   int
	lastMessages  []*schema.Message
	lastOptions   []model.Option
}

func (s *stubChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	s.generateCalls++
	s.lastMessages = cloneMessages(input)
	s.lastOptions = append([]model.Option(nil), opts...)
	return schema.AssistantMessage("final-answer", nil), nil
}

func (s *stubChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	s.streamCalls++
	s.lastMessages = cloneMessages(input)
	s.lastOptions = append([]model.Option(nil), opts...)

	reader, writer := schema.Pipe[*schema.Message](2)
	go func() {
		writer.Send(schema.AssistantMessage("stream-", nil), nil)
		writer.Send(schema.AssistantMessage("answer", nil), nil)
		writer.Close()
	}()

	return reader, nil
}

func TestChatflowCombinesPromptAndOptionalStdin(t *testing.T) {
	ctx := context.Background()
	model := &stubChatModel{}
	workflow, err := chatflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new workflow: %v", err)
	}

	service := runtime.NewChatService(workflow, render.Renderer{}, prompts.NewRoleRegistry())
	result, err := service.Run(ctx, runtime.ChatRequest{
		Prompt:       "first line",
		StdinInput:   "second line",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("run chat service: %v", err)
	}

	if result.Text != "final-answer" {
		t.Fatalf("expected final answer, got %q", result.Text)
	}
	if model.generateCalls != 1 || model.streamCalls != 0 {
		t.Fatalf("expected generate path only, got generate=%d stream=%d", model.generateCalls, model.streamCalls)
	}
	if got := model.lastMessages[1].Content; got != "first line\nsecond line" {
		t.Fatalf("expected combined prompt, got %q", got)
	}
}

func TestChatflowInjectsJSONInstruction(t *testing.T) {
	ctx := context.Background()
	model := &stubChatModel{}
	workflow, err := chatflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new workflow: %v", err)
	}

	service := runtime.NewChatService(workflow, render.Renderer{}, prompts.NewRoleRegistry())
	_, err = service.Run(ctx, runtime.ChatRequest{
		Prompt:       "Explain the output",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		JSONOutput:   true,
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("run chat service: %v", err)
	}

	if got := model.lastMessages[1].Content; got != "Explain the output\n\nPlease respond in JSON format." {
		t.Fatalf("unexpected JSON-injected prompt %q", got)
	}
}

func TestChatflowRoutesStreamAndNonStreamModes(t *testing.T) {
	ctx := context.Background()
	model := &stubChatModel{}
	workflow, err := chatflow.New(ctx, model)
	if err != nil {
		t.Fatalf("new workflow: %v", err)
	}

	service := runtime.NewChatService(workflow, render.Renderer{}, prompts.NewRoleRegistry())
	nonStreamResult, err := service.Run(ctx, runtime.ChatRequest{
		Prompt:       "non-stream request",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("run non-stream chat service: %v", err)
	}
	if nonStreamResult.Text != "final-answer" {
		t.Fatalf("expected final-answer, got %q", nonStreamResult.Text)
	}

	streamResult, err := service.Run(ctx, runtime.ChatRequest{
		Prompt:       "stream request",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("run stream chat service: %v", err)
	}
	if streamResult.Text != "stream-answer" {
		t.Fatalf("expected stream-answer, got %q", streamResult.Text)
	}
	if model.generateCalls != 1 || model.streamCalls != 1 {
		t.Fatalf("expected one generate and one stream call, got generate=%d stream=%d", model.generateCalls, model.streamCalls)
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
