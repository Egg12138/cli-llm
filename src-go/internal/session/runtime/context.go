package runtime

import (
	"fmt"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	"github.com/cloudwego/eino/schema"
)

type ContextOptions struct {
	CurrentDate string
}

func BuildMessages(state graph.State, opts ContextOptions) ([]*schema.Message, error) {
	messages := []*schema.Message{schema.SystemMessage(systemPrompt(opts))}
	history, err := state.ReachableHistory()
	if err != nil {
		return nil, err
	}
	for _, entry := range history {
		switch entry.Type {
		case model.EntryTypeMessage:
			data, err := entry.MessageData()
			if err != nil {
				return nil, err
			}
			switch data.Role {
			case "user":
				messages = append(messages, schema.UserMessage(data.Content))
			case "assistant":
				messages = append(messages, schema.AssistantMessage(data.Content, nil))
			default:
				messages = append(messages, schema.SystemMessage(data.Content))
			}
		case model.EntryTypeCompaction:
			data, err := entry.CompactionData()
			if err != nil {
				return nil, err
			}
			messages = append(messages, schema.SystemMessage("Previous conversation summary:\n"+data.Summary))
		case model.EntryTypeBranchSummary:
			data, err := entry.BranchSummaryData()
			if err != nil {
				return nil, err
			}
			messages = append(messages, schema.SystemMessage("Branch summary:\n"+data.Summary))
		}
	}
	return messages, nil
}

func systemPrompt(opts ContextOptions) string {
	return fmt.Sprintf(`You are an expert conversation assistant operating inside llm-session, a multi-turn session harness. Session checkpoints and branches are managed locally outside the model context.

Guidelines:
- Be concise in your responses.
- If the user asks about session internals, answer from first principles.

Current date: %s`, opts.CurrentDate)
}
