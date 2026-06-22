package providers

import (
	"context"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/cloudwego/eino/schema"
)

func TestChatModelFactoryBuildConfigUsesAppConfig(t *testing.T) {
	t.Parallel()

	factory := NewFactory(config.AppConfig{
		APIKey:       "test-key",
		APIEndpoint:  "https://example.com/v1",
		DefaultModel: "gpt-4o-mini",
	})

	cfg := factory.BuildConfig(false)
	if cfg.APIKey != "test-key" {
		t.Fatalf("expected API key test-key, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://example.com/v1" {
		t.Fatalf("expected base URL https://example.com/v1, got %q", cfg.BaseURL)
	}
	if cfg.Model != "gpt-4o-mini" {
		t.Fatalf("expected model gpt-4o-mini, got %q", cfg.Model)
	}
}

func TestChatModelFactoryBuildConfigEnablesJSONObjectResponseFormat(t *testing.T) {
	t.Parallel()

	factory := NewFactory(config.AppConfig{
		APIKey:       "test-key",
		APIEndpoint:  "https://example.com/v1",
		DefaultModel: "gpt-4o-mini",
	})

	cfg := factory.BuildConfig(true)
	if cfg.ResponseFormat == nil {
		t.Fatal("expected response format to be configured")
	}
	if cfg.ResponseFormat.Type != "json_object" {
		t.Fatalf("expected json_object response format, got %#v", cfg.ResponseFormat.Type)
	}
}

func TestChatModelFactoryPrepareInvocationPreservesRequestSettings(t *testing.T) {
	t.Parallel()

	factory := NewFactory(config.AppConfig{
		APIKey:       "test-key",
		APIEndpoint:  "https://example.com/v1",
		DefaultModel: "gpt-4o-mini",
	})

	temp := float32(0.2)
	tool := &schema.ToolInfo{
		Name: "read",
		Desc: "Read a local file.",
	}
	request := ChatRequest{
		Model: "gpt-4.1-mini",
		Messages: []*schema.Message{
			schema.SystemMessage("You are a coder."),
			schema.UserMessage("inspect the repo"),
		},
		Temperature: &temp,
		Tools:       []*schema.ToolInfo{tool},
		ToolChoice: &ToolChoiceConfig{
			Choice: schema.ToolChoiceAllowed,
		},
		Stream: true,
	}

	plan, err := factory.PrepareInvocation(context.Background(), request)
	if err != nil {
		t.Fatalf("PrepareInvocation returned error: %v", err)
	}

	if plan.RequestedModel != "gpt-4.1-mini" {
		t.Fatalf("expected requested model gpt-4.1-mini, got %q", plan.RequestedModel)
	}
	if !plan.Stream {
		t.Fatal("expected stream mode to be preserved")
	}
	if len(plan.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(plan.Messages))
	}
	if got := plan.Messages[0].Role; got != schema.System {
		t.Fatalf("expected first role system, got %q", got)
	}
	if len(plan.BoundTools) != 1 {
		t.Fatalf("expected 1 bound tool, got %d", len(plan.BoundTools))
	}
	if plan.BoundTools[0].Name != "read" {
		t.Fatalf("expected bound tool read, got %q", plan.BoundTools[0].Name)
	}
	if plan.ToolChoice == nil || plan.ToolChoice.Choice != schema.ToolChoiceAllowed {
		t.Fatalf("expected allowed tool choice, got %#v", plan.ToolChoice)
	}
	if len(plan.Options) != 2 {
		t.Fatalf("expected model+temperature options, got %d", len(plan.Options))
	}
	if plan.ChatModel == nil {
		t.Fatal("expected chat model to be initialized")
	}
}

func TestChatModelFactoryNewReturnsToolCallingModel(t *testing.T) {
	t.Parallel()

	factory := NewFactory(config.AppConfig{
		APIKey:       "test-key",
		APIEndpoint:  "https://example.com/v1",
		DefaultModel: "gpt-4o-mini",
	})

	model, err := factory.New(context.Background(), false)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if model == nil {
		t.Fatal("expected non-nil model")
	}
}
