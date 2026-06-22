package providers

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ToolChoiceConfig struct {
	Choice           schema.ToolChoice
	AllowedToolNames []string
}

type ChatRequest struct {
	Model       string
	Messages    []*schema.Message
	Temperature *float32
	JSONOutput  bool
	Tools       []*schema.ToolInfo
	ToolChoice  *ToolChoiceConfig
	Stream      bool
}

type InvocationPlan struct {
	ChatModel      model.ToolCallingChatModel
	Messages       []*schema.Message
	Options        []model.Option
	BoundTools     []*schema.ToolInfo
	ToolChoice     *ToolChoiceConfig
	RequestedModel string
	Stream         bool
}
