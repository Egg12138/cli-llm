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
	text := historyText(history)
	tokensBefore, err := sessiontoken.CountText(text, req.ModelName)
	if err != nil {
		return false, err
	}
	if !sessiontoken.ShouldCompress(tokensBefore, req.ThresholdTokens) {
		return false, nil
	}
	if req.Summarizer == nil {
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
	firstKept := firstRecentEntry(history)
	compaction, err := model.NewCompaction("", model.CompactionData{
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
	if firstKept.ID != "" {
		req.State.Reparent(firstKept.ID, compaction.ID)
	}
	return true, nil
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

func firstRecentEntry(history []model.Entry) model.Entry {
	if len(history) == 0 {
		return model.Entry{}
	}
	return history[len(history)-1]
}
