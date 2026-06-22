package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type DemoEvent struct {
	Kind     string
	ToolName string
	Content  string
}

type greetInput struct {
	Name string `json:"name" jsonschema:"description=Name to greet"`
}

func main() {
	events, err := RunDemo(context.Background(), "Ada")
	if err != nil {
		panic(err)
	}
	for _, event := range events {
		if event.ToolName == "" {
			fmt.Printf("%s: %s\n", event.Kind, event.Content)
			continue
		}
		fmt.Printf("%s:%s: %s\n", event.Kind, event.ToolName, event.Content)
	}
}

func RunDemo(ctx context.Context, name string) ([]DemoEvent, error) {
	greetTool, err := utils.InferTool[greetInput, string](
		"greet",
		"Return a greeting for a name.",
		func(_ context.Context, input greetInput) (string, error) {
			return "hello " + input.Name, nil
		},
	)
	if err != nil {
		return nil, err
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "chatmodel-agent-demo",
		Description: "Demonstrates a ChatModelAgent with one local tool.",
		Model:       &scriptedToolModel{name: name},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{greetTool},
			},
		},
		MaxIterations: 3,
	})
	if err != nil {
		return nil, err
	}

	iter := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent}).Query(ctx, "greet "+name)
	return collectEvents(iter)
}

func collectEvents(iter *adk.AsyncIterator[*adk.AgentEvent]) ([]DemoEvent, error) {
	var events []DemoEvent
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		message, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return nil, err
		}
		if message == nil {
			continue
		}

		switch event.Output.MessageOutput.Role {
		case schema.Assistant:
			events = append(events, DemoEvent{Kind: "model", Content: message.Content})
		case schema.Tool:
			events = append(events, DemoEvent{
				Kind:     "tool",
				ToolName: event.Output.MessageOutput.ToolName,
				Content:  message.Content,
			})
		}
	}
	return events, nil
}

type scriptedToolModel struct {
	name string
}

func (m *scriptedToolModel) Generate(_ context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if !hasTools(opts) {
		return nil, errors.New("expected tool definitions to be bound")
	}
	for i := len(input) - 1; i >= 0; i-- {
		if input[i].Role == schema.Tool {
			toolContent := strings.Trim(input[i].Content, `"`)
			return schema.AssistantMessage("tool said "+toolContent, nil), nil
		}
	}
	return schema.AssistantMessage("calling greet", []schema.ToolCall{
		{
			ID: "call-greet",
			Function: schema.FunctionCall{
				Name:      "greet",
				Arguments: fmt.Sprintf(`{"name":%q}`, m.name),
			},
		},
	}), nil
}

func (m *scriptedToolModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	message, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func hasTools(opts []model.Option) bool {
	options := model.GetCommonOptions(&model.Options{}, opts...)
	return len(options.Tools) > 0
}
