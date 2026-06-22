package chatflow

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type Input struct {
	SystemMessage string
	Prompt        string
	StdinInput    string
	JSONOutput    bool
}

type Workflow struct {
	runnable compose.Runnable[Input, *schema.Message]
}

func New(ctx context.Context, chatModel model.BaseChatModel) (*Workflow, error) {
	chain := compose.NewChain[Input, *schema.Message]()
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input Input) ([]*schema.Message, error) {
		return assembleMessages(input), nil
	}))
	chain.AppendChatModel(chatModel)

	runnable, err := chain.Compile(ctx, compose.WithGraphName("chatflow"))
	if err != nil {
		return nil, err
	}

	return &Workflow{runnable: runnable}, nil
}

func (w *Workflow) Invoke(ctx context.Context, input Input, opts ...compose.Option) (*schema.Message, error) {
	return w.runnable.Invoke(ctx, input, opts...)
}

func (w *Workflow) Stream(ctx context.Context, input Input, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return w.runnable.Stream(ctx, input, opts...)
}

func BuildMessages(input Input) []*schema.Message {
	prompt := strings.Join(filterEmpty(input.Prompt, input.StdinInput), "\n")
	if input.JSONOutput && !strings.Contains(strings.ToLower(prompt), "json") {
		prompt += "\n\nPlease respond in JSON format."
	}

	return []*schema.Message{
		schema.SystemMessage(input.SystemMessage),
		schema.UserMessage(prompt),
	}
}

func assembleMessages(input Input) []*schema.Message {
	return BuildMessages(input)
}

func filterEmpty(values ...string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
