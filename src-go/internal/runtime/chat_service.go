package runtime

import (
	"context"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/prompts"
	"github.com/Egg12138/cli-llm/src-go/internal/render"
	"github.com/Egg12138/cli-llm/src-go/internal/workflows/chatflow"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

type chatWorkflow interface {
	Invoke(ctx context.Context, input chatflow.Input, opts ...compose.Option) (*schema.Message, error)
	Stream(ctx context.Context, input chatflow.Input, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error)
}

type ChatRequest struct {
	Prompt               string
	StdinInput           string
	RoleName             string
	RoleFallback         string
	WorkingDir           string
	ModelName            string
	AgentsContextEnabled bool
	CountTokens          bool
	JSONOutput           bool
	Stream               bool
	ModelOptions         []model.Option
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

func (u TokenUsage) Total() int {
	return u.InputTokens + u.OutputTokens
}

func (u TokenUsage) EstimatedCostUSD() float64 {
	return float64(u.InputTokens)*0.00001 + float64(u.OutputTokens)*0.00003
}

type ChatResult struct {
	Text           string
	RawText        string
	Prompt         prompts.BuildResult
	Usage          TokenUsage
	StreamRendered bool
	FinishReason   string
	Duration       time.Duration
}

type ChatService struct {
	workflow chatWorkflow
	renderer render.Renderer
	registry prompts.RoleRegistry
}

func NewChatService(workflow chatWorkflow, renderer render.Renderer, registry prompts.RoleRegistry) ChatService {
	return ChatService{
		workflow: workflow,
		renderer: renderer,
		registry: registry,
	}
}

var (
	countTokensInMessages = defaultCountTokensInMessages
	countTokensInText     = defaultCountTokensInText
)

func (s ChatService) Run(ctx context.Context, request ChatRequest) (ChatResult, error) {
	start := time.Now()
	promptResult := prompts.BuildSystemPrompt(prompts.BuildOptions{
		RoleName:             request.RoleName,
		RoleFallback:         request.RoleFallback,
		AgentsContextEnabled: request.AgentsContextEnabled,
		WorkingDir:           request.WorkingDir,
	}, s.registry)

	flowInput := chatflow.Input{
		SystemMessage: promptResult.SystemMessage,
		Prompt:        request.Prompt,
		StdinInput:    request.StdinInput,
		JSONOutput:    request.JSONOutput,
	}

	usage := TokenUsage{}
	if request.CountTokens && request.Stream {
		inputTokens, err := countTokensInMessages(chatflow.BuildMessages(flowInput), request.ModelName)
		if err != nil {
			return ChatResult{}, err
		}
		usage.InputTokens = inputTokens
	}

	var (
		text     string
		rawText  string
		err      error
		response *schema.Message
	)
	if request.Stream {
		stream, streamErr := s.workflow.Stream(ctx, flowInput, compose.WithChatModelOption(request.ModelOptions...))
		if streamErr != nil {
			return ChatResult{}, streamErr
		}
		text, err = s.renderer.RenderStream(stream)
		rawText = text
	} else {
		message, invokeErr := s.workflow.Invoke(ctx, flowInput, compose.WithChatModelOption(request.ModelOptions...))
		if invokeErr != nil {
			return ChatResult{}, invokeErr
		}
		response = message
		if message != nil {
			rawText = message.Content
		}
		text, err = s.renderer.RenderMessage(message)
	}
	if err != nil {
		return ChatResult{}, err
	}
	if request.CountTokens {
		if response != nil && response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
			usage.InputTokens = response.ResponseMeta.Usage.PromptTokens
			usage.OutputTokens = response.ResponseMeta.Usage.CompletionTokens
		} else {
			if !request.Stream {
				inputTokens, countErr := countTokensInMessages(chatflow.BuildMessages(flowInput), request.ModelName)
				if countErr != nil {
					return ChatResult{}, countErr
				}
				usage.InputTokens = inputTokens
			}
			outputTokens, countErr := countTokensInText(rawText, request.ModelName)
			if countErr != nil {
				return ChatResult{}, countErr
			}
			usage.OutputTokens = outputTokens
		}
	}

	return ChatResult{
		Text:           text,
		RawText:        rawText,
		Prompt:         promptResult,
		Usage:          usage,
		StreamRendered: request.Stream,
		FinishReason:   finishReason(response),
		Duration:       time.Since(start),
	}, nil
}

func finishReason(message *schema.Message) string {
	if message == nil || message.ResponseMeta == nil {
		return ""
	}
	return message.ResponseMeta.FinishReason
}

func defaultCountTokensInMessages(messages []*schema.Message, modelName string) (int, error) {
	encoding, err := encodingForModel(modelName)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, message := range messages {
		if message == nil {
			continue
		}
		if message.Content != "" {
			total += len(encoding.Encode(message.Content, nil, nil))
		}
		total += 4
		if message.Name != "" {
			total++
		}
	}
	total += 2
	return total, nil
}

func defaultCountTokensInText(text string, modelName string) (int, error) {
	encoding, err := encodingForModel(modelName)
	if err != nil {
		return 0, err
	}
	return len(encoding.Encode(text, nil, nil)), nil
}

func encodingForModel(modelName string) (*tiktoken.Tiktoken, error) {
	modelEncodingMap := map[string]string{
		"deepseek-coder":    "cl100k_base",
		"deepseek-chat":     "cl100k_base",
		"deepseek-reasoner": "cl100k_base",
		"gpt-4":             "cl100k_base",
		"gpt-3.5-turbo":     "cl100k_base",
		"gpt-4o":            "cl100k_base",
		"gpt-4o-mini":       "cl100k_base",
	}
	encodingName := modelEncodingMap[modelName]
	if encodingName == "" {
		encodingName = "cl100k_base"
	}
	return tiktoken.GetEncoding(encodingName)
}
