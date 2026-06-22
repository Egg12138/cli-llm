package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/prompts"
	"github.com/Egg12138/cli-llm/src-go/internal/render"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/chatflow"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type stubWorkflow struct {
	lastInput  chatflow.Input
	message    *schema.Message
	streamed   *schema.StreamReader[*schema.Message]
	invokeRuns int
	streamRuns int
}

func (s *stubWorkflow) Invoke(ctx context.Context, input chatflow.Input, opts ...compose.Option) (*schema.Message, error) {
	s.lastInput = input
	s.invokeRuns++
	return s.message, nil
}

func (s *stubWorkflow) Stream(ctx context.Context, input chatflow.Input, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	s.lastInput = input
	s.streamRuns++
	return s.streamed, nil
}

func TestChatServiceCountsTokensWhenEnabled(t *testing.T) {
	workflow := &stubWorkflow{
		message: schema.AssistantMessage("final-answer", nil),
	}

	originalCountMessages := countTokensInMessages
	originalCountText := countTokensInText
	t.Cleanup(func() {
		countTokensInMessages = originalCountMessages
		countTokensInText = originalCountText
	})

	countTokensInMessages = func(messages []*schema.Message, model string) (int, error) {
		if model != "gpt-4o-mini" {
			t.Fatalf("expected model gpt-4o-mini, got %q", model)
		}
		if len(messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(messages))
		}
		if got := messages[1].Content; got != "hello\nstdin" {
			t.Fatalf("expected combined prompt, got %q", got)
		}
		return 11, nil
	}
	countTokensInText = func(text string, model string) (int, error) {
		if model != "gpt-4o-mini" {
			t.Fatalf("expected model gpt-4o-mini, got %q", model)
		}
		if text != "final-answer" {
			t.Fatalf("expected final-answer text, got %q", text)
		}
		return 7, nil
	}

	service := NewChatService(workflow, render.Renderer{}, prompts.NewRoleRegistry())
	result, err := service.Run(context.Background(), ChatRequest{
		Prompt:       "hello",
		StdinInput:   "stdin",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		ModelName:    "gpt-4o-mini",
		CountTokens:  true,
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("run chat service: %v", err)
	}

	if result.Usage.InputTokens != 11 {
		t.Fatalf("expected 11 input tokens, got %d", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 7 {
		t.Fatalf("expected 7 output tokens, got %d", result.Usage.OutputTokens)
	}
	if result.Usage.Total() != 18 {
		t.Fatalf("expected total 18 tokens, got %d", result.Usage.Total())
	}
	if workflow.invokeRuns != 1 || workflow.streamRuns != 0 {
		t.Fatalf("expected non-stream path, got invoke=%d stream=%d", workflow.invokeRuns, workflow.streamRuns)
	}
}

func TestChatServiceMarksStreamedResultsAsAlreadyRendered(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](2)
	go func() {
		writer.Send(schema.AssistantMessage("stream-", nil), nil)
		writer.Send(schema.AssistantMessage("answer", nil), nil)
		writer.Close()
	}()

	workflow := &stubWorkflow{streamed: reader}
	var output bytes.Buffer
	service := NewChatService(workflow, render.Renderer{Writer: &output}, prompts.NewRoleRegistry())

	result, err := service.Run(context.Background(), ChatRequest{
		Prompt:       "hello",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("run chat service: %v", err)
	}

	if result.Text != "stream-answer" {
		t.Fatalf("expected streamed text, got %q", result.Text)
	}
	if output.String() != "stream-answer\n" {
		t.Fatalf("expected streamed writer output, got %q", output.String())
	}
	if !result.StreamRendered {
		t.Fatalf("expected stream result to be marked rendered")
	}
	if workflow.invokeRuns != 0 || workflow.streamRuns != 1 {
		t.Fatalf("expected stream path, got invoke=%d stream=%d", workflow.invokeRuns, workflow.streamRuns)
	}
}

func TestChatServicePrefersProviderUsageMetadataWhenAvailable(t *testing.T) {
	workflow := &stubWorkflow{
		message: &schema.Message{
			Role:    schema.Assistant,
			Content: "final-answer",
			ResponseMeta: &schema.ResponseMeta{
				Usage: &schema.TokenUsage{
					PromptTokens:     13,
					CompletionTokens: 8,
					TotalTokens:      21,
				},
			},
		},
	}

	originalCountMessages := countTokensInMessages
	originalCountText := countTokensInText
	t.Cleanup(func() {
		countTokensInMessages = originalCountMessages
		countTokensInText = originalCountText
	})

	countTokensInMessages = func(messages []*schema.Message, model string) (int, error) {
		t.Fatal("countTokensInMessages should not run when provider usage is available")
		return 0, nil
	}
	countTokensInText = func(text string, model string) (int, error) {
		t.Fatal("countTokensInText should not run when provider usage is available")
		return 0, nil
	}

	service := NewChatService(workflow, render.Renderer{}, prompts.NewRoleRegistry())
	result, err := service.Run(context.Background(), ChatRequest{
		Prompt:       "hello",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		ModelName:    "gpt-4o-mini",
		CountTokens:  true,
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("run chat service: %v", err)
	}

	if result.Usage.InputTokens != 13 {
		t.Fatalf("expected provider prompt tokens 13, got %d", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 8 {
		t.Fatalf("expected provider completion tokens 8, got %d", result.Usage.OutputTokens)
	}
	if result.Usage.Total() != 21 {
		t.Fatalf("expected total 21, got %d", result.Usage.Total())
	}
}

func TestChatServiceCountsFallbackTokensFromRawContentNotRenderedOutput(t *testing.T) {
	workflow := &stubWorkflow{
		message: &schema.Message{
			Role:    schema.Assistant,
			Content: "**Bold**",
		},
	}

	originalCountMessages := countTokensInMessages
	originalCountText := countTokensInText
	originalRenderMarkdown := render.RenderMarkdownForTest
	t.Cleanup(func() {
		countTokensInMessages = originalCountMessages
		countTokensInText = originalCountText
		render.RenderMarkdownForTest = originalRenderMarkdown
	})

	render.RenderMarkdownForTest = func(content string) (string, error) {
		return "rendered-bold", nil
	}
	countTokensInMessages = func(messages []*schema.Message, model string) (int, error) {
		return 5, nil
	}
	countTokensInText = func(text string, model string) (int, error) {
		if text != "**Bold**" {
			t.Fatalf("expected raw content for token counting, got %q", text)
		}
		return 4, nil
	}

	service := NewChatService(workflow, render.Renderer{}, prompts.NewRoleRegistry())
	result, err := service.Run(context.Background(), ChatRequest{
		Prompt:       "hello",
		RoleName:     "coder",
		RoleFallback: "coder",
		WorkingDir:   t.TempDir(),
		ModelName:    "gpt-4o-mini",
		CountTokens:  true,
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("run chat service: %v", err)
	}

	if result.Text != "rendered-bold" {
		t.Fatalf("expected rendered display text, got %q", result.Text)
	}
	if result.RawText != "**Bold**" {
		t.Fatalf("expected raw text, got %q", result.RawText)
	}
	if result.Usage.InputTokens != 5 || result.Usage.OutputTokens != 4 {
		t.Fatalf("unexpected usage %#v", result.Usage)
	}
}
