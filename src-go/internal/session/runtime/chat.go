package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	einomodel "github.com/cloudwego/eino/components/model"
)

type AppendStore interface {
	Append(entry model.Entry) error
}

type ChatTurnRequest struct {
	Input       string
	State       *graph.State
	Store       AppendStore
	Writer      io.Writer
	Model       einomodel.BaseChatModel
	TitleModel  einomodel.BaseChatModel
	Options     []einomodel.Option
	SessionName string
	ModelName   string
	Now         func() time.Time
	OnTitle     func(TitleResult) error
	Status      StatusReporter
}

func RunChatTurn(ctx context.Context, req ChatTurnRequest) error {
	if req.State == nil {
		return fmt.Errorf("session state is required")
	}
	if req.Store == nil {
		return fmt.Errorf("session store is required")
	}
	if req.Model == nil {
		return fmt.Errorf("chat model is required")
	}
	reporter := req.Status
	if reporter == nil {
		reporter = nopReporter{}
	}
	statusCleared := false
	clearStatus := func() {
		if statusCleared {
			return
		}
		reporter.Clear()
		statusCleared = true
	}
	defer clearStatus()

	now := time.Now
	if req.Now != nil {
		now = req.Now
	}
	reporter.Set(StatusThinking)
	titleResult, err := ensureSessionTitle(ctx, req.State, req.Store, req.TitleModel, req.Input, now(), req.ModelName)
	if err != nil {
		return err
	}
	if titleResult.Created && req.OnTitle != nil {
		if err := req.OnTitle(titleResult); err != nil {
			return err
		}
	}

	userEntry, err := model.NewMessage(req.State.HeadID, "user", req.Input, now())
	if err != nil {
		return err
	}
	if err := req.Store.Append(userEntry); err != nil {
		return err
	}
	if err := req.State.AddEntry(userEntry); err != nil {
		return err
	}
	req.State.SetHead(userEntry.ID)

	messages, err := BuildMessages(*req.State, ContextOptions{
		CurrentDate: now().Format("2006-01-02"),
	})
	if err != nil {
		return err
	}
	stream, err := req.Model.Stream(ctx, messages, req.Options...)
	if err != nil {
		return err
	}
	defer stream.Close()
	reporter.Set(StatusWaitingStream)

	streamingReported := false
	var content strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if chunk == nil || chunk.Content == "" {
			continue
		}
		if !streamingReported {
			reporter.Set(StatusStreaming)
			clearStatus()
			streamingReported = true
		}
		content.WriteString(chunk.Content)
		if req.Writer != nil {
			if _, err := fmt.Fprint(req.Writer, chunk.Content); err != nil {
				return err
			}
		}
	}
	response := content.String()
	if response != "" && !strings.HasSuffix(response, "\n") && req.Writer != nil {
		if _, err := fmt.Fprintln(req.Writer); err != nil {
			return err
		}
	}

	assistantEntry, err := model.NewMessage(userEntry.ID, "assistant", response, now())
	if err != nil {
		return err
	}
	if err := req.Store.Append(assistantEntry); err != nil {
		return err
	}
	if err := req.State.AddEntry(assistantEntry); err != nil {
		return err
	}

	checkpoint, err := model.NewCheckpoint(assistantEntry.ID, "", assistantEntry.ID, now())
	if err != nil {
		return err
	}
	if err := req.Store.Append(checkpoint); err != nil {
		return err
	}
	if err := req.State.AddEntry(checkpoint); err != nil {
		return err
	}
	req.State.AutoCheckpoint(checkpoint)
	if req.State.CurrentBranch != "" {
		branchCheckpoint, err := model.NewCheckpoint(checkpoint.ID, req.State.CurrentBranch, checkpoint.ID, now())
		if err != nil {
			return err
		}
		if err := req.Store.Append(branchCheckpoint); err != nil {
			return err
		}
		if err := req.State.AddEntry(branchCheckpoint); err != nil {
			return err
		}
	}
	return nil
}
