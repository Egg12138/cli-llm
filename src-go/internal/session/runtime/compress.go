package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	sessiontoken "github.com/Egg12138/cli-llm/src-go/internal/session/token"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type CompressionRequest struct {
	State            *graph.State
	Store            AppendStore
	Summarizer       einomodel.BaseChatModel
	ModelName        string
	ThresholdTokens  int
	KeepRecentTokens int
	Now              func() time.Time
}

func MaybeCompress(ctx context.Context, req CompressionRequest) (bool, error) {
	history, err := req.State.ReachableHistory()
	if err != nil {
		return false, err
	}
	text := historyText(ProjectHistory(history))
	tokensBefore, needsCompression, err := sessiontoken.NeedsCompression(text, req.ModelName, req.ThresholdTokens)
	if err != nil {
		return false, err
	}
	if !needsCompression {
		return false, nil
	}
	if req.Summarizer == nil {
		return false, nil
	}
	firstKept := firstKeptEntry(history, req.KeepRecentTokens, req.ModelName)
	if firstKept.ID == "" || !hasSummarizableMessages(history, firstKept.ID) {
		return false, nil
	}
	response, err := req.Summarizer.Generate(ctx, []*schema.Message{
		schema.UserMessage("Summarize this conversation so it can be used as compact context:\n\n" + text),
	})
	if err != nil {
		return false, err
	}
	summary := ""
	if response != nil {
		summary = strings.TrimSpace(response.Content)
	}
	now := time.Now
	if req.Now != nil {
		now = req.Now
	}
	// The compaction is appended at the current leaf and becomes the new head,
	// so the branch path after a reload still contains it. The summarized
	// entries stay in the file; projection trims them at read time.
	compaction, err := model.NewCompaction(req.State.HeadID, model.CompactionData{
		Summary:          summary,
		FirstKeptEntryID: firstKept.ID,
		TokensBefore:     tokensBefore,
	}, now())
	if err != nil {
		return false, err
	}
	if err := req.Store.Append(compaction); err != nil {
		return false, err
	}
	if err := req.State.AddEntry(compaction); err != nil {
		return false, err
	}
	req.State.SetHead(compaction.ID)
	return true, nil
}

// firstKeptEntry returns the first message of the trailing token budget. The
// last message is always kept, even when it exceeds the budget.
func firstKeptEntry(history []model.Entry, keepRecentTokens int, modelName string) model.Entry {
	if keepRecentTokens <= 0 {
		keepRecentTokens = sessiontoken.DefaultKeepRecentTokens
	}
	total := 0
	var kept model.Entry
	for i := len(history) - 1; i >= 0; i-- {
		entry := history[i]
		if entry.Type != model.EntryTypeMessage {
			continue
		}
		data, err := entry.MessageData()
		if err != nil {
			continue
		}
		tokens, err := sessiontoken.CountText(data.Content, modelName)
		if err != nil {
			tokens = len(data.Content)/4 + 1
		}
		if kept.ID != "" && total+tokens > keepRecentTokens {
			break
		}
		total += tokens
		kept = entry
	}
	return kept
}

func hasSummarizableMessages(history []model.Entry, firstKeptID string) bool {
	for _, entry := range history {
		if entry.ID == firstKeptID {
			return false
		}
		if entry.Type == model.EntryTypeMessage {
			return true
		}
	}
	return false
}

func historyText(history []model.Entry) string {
	var builder strings.Builder
	for _, entry := range history {
		if entry.Type != model.EntryTypeMessage {
			continue
		}
		data, err := entry.MessageData()
		if err != nil {
			continue
		}
		builder.WriteString(data.Role)
		builder.WriteString(": ")
		builder.WriteString(data.Content)
		builder.WriteByte('\n')
	}
	return builder.String()
}
