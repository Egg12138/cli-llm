package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type TitleResult struct {
	Title   string
	Created bool
}

func NormalizeTitle(title, fallback string) string {
	line := firstLine(strings.TrimSpace(title))
	if line == "" {
		line = firstRunes(strings.TrimSpace(fallback), 16)
	}
	return firstRunes(line, 60)
}

func ensureSessionTitle(ctx context.Context, state *graph.State, store AppendStore, titleModel einomodel.BaseChatModel, firstPrompt string, now time.Time, configuredModel string) (TitleResult, error) {
	if titleModel == nil || hasSessionInfo(state) {
		return TitleResult{}, nil
	}
	response, err := titleModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("Generate a short descriptive title for this session.\nReturn one line, at most 60 characters.\n\nUser message:\n" + firstPrompt),
	})
	if err != nil {
		return TitleResult{}, err
	}
	rawTitle := ""
	if response != nil {
		rawTitle = response.Content
	}
	title := NormalizeTitle(rawTitle, firstPrompt)
	infoEntry, err := model.NewSessionInfo("", model.SessionInfo{
		Title:   title,
		Created: now,
		Model:   configuredModel,
	}, now)
	if err != nil {
		return TitleResult{}, err
	}
	if err := store.Append(infoEntry); err != nil {
		return TitleResult{}, err
	}
	if err := state.AddEntry(infoEntry); err != nil {
		return TitleResult{}, err
	}
	return TitleResult{Title: title, Created: true}, nil
}

func hasSessionInfo(state *graph.State) bool {
	for _, id := range state.Order {
		if state.Entries[id].Type == model.EntryTypeSessionInfo {
			return true
		}
	}
	return false
}

func firstLine(value string) string {
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func firstRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
