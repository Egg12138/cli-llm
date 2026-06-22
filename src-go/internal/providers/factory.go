package providers

import (
	"context"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Factory struct {
	config config.AppConfig
}

func NewFactory(cfg config.AppConfig) Factory {
	return Factory{config: cfg}
}

func (f Factory) BuildConfig(jsonOutput bool) *openaimodel.ChatModelConfig {
	cfg := &openaimodel.ChatModelConfig{
		APIKey:  f.config.ResolvedAPIKey(),
		BaseURL: f.config.APIEndpoint,
		Model:   f.config.DefaultModel,
	}
	if jsonOutput {
		cfg.ResponseFormat = &openaimodel.ChatCompletionResponseFormat{
			Type: openaimodel.ChatCompletionResponseFormatTypeJSONObject,
		}
	}
	return cfg
}

func (f Factory) New(ctx context.Context, jsonOutput bool) (einomodel.ToolCallingChatModel, error) {
	return openaimodel.NewChatModel(ctx, f.BuildConfig(jsonOutput))
}

func (f Factory) PrepareInvocation(ctx context.Context, request ChatRequest) (InvocationPlan, error) {
	chatModel, err := f.New(ctx, request.JSONOutput)
	if err != nil {
		return InvocationPlan{}, err
	}

	if len(request.Tools) > 0 {
		chatModel, err = chatModel.WithTools(request.Tools)
		if err != nil {
			return InvocationPlan{}, err
		}
	}

	requestedModel := request.Model
	if requestedModel == "" {
		requestedModel = f.config.DefaultModel
	}

	options := []einomodel.Option{
		einomodel.WithModel(requestedModel),
	}
	if request.Temperature != nil {
		options = append(options, einomodel.WithTemperature(*request.Temperature))
	}

	return InvocationPlan{
		ChatModel:      chatModel,
		Messages:       append([]*schema.Message(nil), request.Messages...),
		Options:        options,
		BoundTools:     append([]*schema.ToolInfo(nil), request.Tools...),
		ToolChoice:     request.ToolChoice,
		RequestedModel: requestedModel,
		Stream:         request.Stream,
	}, nil
}
