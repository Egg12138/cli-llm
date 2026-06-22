package toolcallflow

import (
	"context"
	"fmt"

	"github.com/Egg12138/cli-llm/src-go/internal/tools"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type Request struct {
	Prompt       string
	Tools        []tools.Definition
	WorkingDir   string
	CurrentDate  string
	ModelOptions []model.Option
}

type flowInput struct {
	SystemPrompt string
	Prompt       string
}

type Workflow struct {
	runnable compose.Runnable[flowInput, *schema.Message]
}

func New(ctx context.Context, chatModel model.BaseChatModel) (*Workflow, error) {
	chain := compose.NewChain[flowInput, *schema.Message]()
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input flowInput) ([]*schema.Message, error) {
		return []*schema.Message{
			schema.SystemMessage(input.SystemPrompt),
			schema.UserMessage(input.Prompt),
		}, nil
	}))
	chain.AppendChatModel(chatModel)

	runnable, err := chain.Compile(ctx, compose.WithGraphName("toolcallflow"))
	if err != nil {
		return nil, err
	}

	return &Workflow{runnable: runnable}, nil
}

func (w *Workflow) Run(ctx context.Context, request Request) (tools.ExecutionResult, error) {
	toolInfos, err := tools.ToToolInfos(request.Tools)
	if err != nil {
		return tools.ExecutionResult{}, err
	}

	allowedToolNames := make([]string, 0, len(request.Tools))
	definitionsByName := make(map[string]tools.Definition, len(request.Tools))
	for _, definition := range request.Tools {
		allowedToolNames = append(allowedToolNames, definition.Name)
		definitionsByName[definition.Name] = definition
	}

	callOptions := append([]model.Option{
		model.WithTools(toolInfos),
		model.WithToolChoice(schema.ToolChoiceAllowed, allowedToolNames...),
	}, request.ModelOptions...)

	response, err := w.runnable.Invoke(ctx, flowInput{
		SystemPrompt: tools.BuildSystemPrompt(request.Tools, request.WorkingDir, request.CurrentDate),
		Prompt:       request.Prompt,
	}, compose.WithChatModelOption(callOptions...))
	if err != nil {
		return tools.ExecutionResult{}, err
	}

	calls, err := tools.ParseToolCalls(response)
	if err != nil {
		return tools.ExecutionResult{}, err
	}
	if len(calls) != 1 {
		return tools.ExecutionResult{}, fmt.Errorf("expected exactly one tool call, got %d", len(calls))
	}

	call := calls[0]
	definition, ok := definitionsByName[call.Name]
	if !ok {
		return tools.ExecutionResult{}, fmt.Errorf("tool '%s' is not enabled", call.Name)
	}

	executor := tools.Executor{WorkingDir: request.WorkingDir}
	return executor.Execute(definition, call.Arguments)
}
