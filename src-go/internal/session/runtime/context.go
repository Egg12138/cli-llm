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
	for _, entry := range projectHistory(history) {
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

// projectHistory applies the newest compaction on the path. The summary is
// placed first, followed by the entries after firstKeptEntryId; everything
// older (including earlier compactions) is dropped from the model context but
// remains in the append-only file.
func projectHistory(history []model.Entry) []model.Entry {
	last := -1
	for i, entry := range history {
		if entry.Type == model.EntryTypeCompaction {
			last = i
		}
	}
	if last < 0 {
		return history
	}
	data, err := history[last].CompactionData()
	if err != nil {
		return history[last:]
	}
	start := -1
	for i := 0; i < len(history); i++ {
		if i == last {
			continue
		}
		if history[i].ID == data.FirstKeptEntryID {
			start = i
			break
		}
	}
	projected := make([]model.Entry, 0, len(history))
	projected = append(projected, history[last])
	startIndex := start
	if startIndex < 0 {
		// A missing kept entry degrades to "summary + newer history" instead of
		// dropping the conversation that followed the compaction.
		startIndex = last + 1
	}
	for i := startIndex; i < len(history); i++ {
		if history[i].Type == model.EntryTypeCompaction {
			continue
		}
		projected = append(projected, history[i])
	}
	return projected
}

func systemPrompt(opts ContextOptions) string {
	return fmt.Sprintf(`You are an expert conversation assistant operating inside llm-session, a multi-turn session harness.

Guidelines:
- Be concise in your responses.
- If the user asks about session internals, answer from first principles.

Current date: %s`, opts.CurrentDate)
}
