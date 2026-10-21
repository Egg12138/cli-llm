package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	"github.com/cloudwego/eino/schema"
)

func TestBuildContextIncludesSessionSystemPrompt(t *testing.T) {
	t.Parallel()

	state := graph.NewState(nil)
	messages, err := BuildMessages(*state, ContextOptions{
		CurrentDate: "2026-06-22",
	})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected only system message, got %#v", messages)
	}
	system := messages[0]
	if system.Role != schema.System {
		t.Fatalf("expected system role, got %v", system.Role)
	}
	for _, want := range []string{"2026-06-22"} {
		if !strings.Contains(system.Content, want) {
			t.Fatalf("system prompt missing %q: %s", want, system.Content)
		}
	}
	for _, forbidden := range []string{"Session:", "branch", "checkpoint", "head:", "work", "main", "abc123"} {
		if strings.Contains(system.Content, forbidden) {
			t.Fatalf("system prompt leaked session state %q: %s", forbidden, system.Content)
		}
	}
}

func TestBuildContextConvertsReachableMessagesAndSkipsCheckpoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	user := mustRuntimeMessage(t, "", "user", "hello", now)
	assistant := mustRuntimeMessage(t, user.ID, "assistant", "hi", now)
	checkpoint, err := model.NewCheckpoint(assistant.ID, "", assistant.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint returned error: %v", err)
	}
	side := mustRuntimeMessage(t, user.ID, "assistant", "side", now)
	state := graph.NewState([]model.Entry{user, assistant, checkpoint, side})
	state.HeadID = checkpoint.ID
	state.Branches["main"] = graph.Branch{Name: "main", HeadID: checkpoint.ID}

	messages, err := BuildMessages(*state, ContextOptions{})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}

	if len(messages) != 3 {
		t.Fatalf("expected system + 2 reachable chat messages, got %#v", messages)
	}
	if messages[1].Role != schema.User || messages[1].Content != "hello" {
		t.Fatalf("unexpected user message %#v", messages[1])
	}
	if messages[2].Role != schema.Assistant || messages[2].Content != "hi" {
		t.Fatalf("unexpected assistant message %#v", messages[2])
	}
	for _, message := range messages {
		if message.Content == "side" {
			t.Fatalf("unreachable side branch should not be included")
		}
	}
}

func TestBuildContextIncludesReachableSummaries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	compaction, err := model.NewCompaction("", model.CompactionData{
		Summary:          "old summary",
		FirstKeptEntryID: "kept",
		TokensBefore:     42,
	}, now)
	if err != nil {
		t.Fatalf("NewCompaction returned error: %v", err)
	}
	branchSummary, err := model.NewBranchSummary(compaction.ID, model.BranchSummaryData{
		Summary: "branch summary",
		FromID:  "old-head",
	}, now)
	if err != nil {
		t.Fatalf("NewBranchSummary returned error: %v", err)
	}
	user := mustRuntimeMessage(t, branchSummary.ID, "user", "new question", now)
	unreachableSummary, err := model.NewBranchSummary("", model.BranchSummaryData{Summary: "hidden", FromID: "x"}, now)
	if err != nil {
		t.Fatalf("NewBranchSummary returned error: %v", err)
	}
	state := graph.NewState([]model.Entry{compaction, branchSummary, user, unreachableSummary})
	state.HeadID = user.ID

	messages, err := BuildMessages(*state, ContextOptions{})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}

	var joined strings.Builder
	for _, message := range messages {
		joined.WriteString(message.Content)
		joined.WriteByte('\n')
	}
	output := joined.String()
	for _, want := range []string{"old summary", "branch summary", "new question"} {
		if !strings.Contains(output, want) {
			t.Fatalf("context missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, "hidden") {
		t.Fatalf("unreachable branch summary included: %s", output)
	}
}

func TestBuildContextProjectsCompactionOnReloadedHistory(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	oldUser := mustRuntimeMessage(t, "", "user", "old question", now)
	oldAnswer := mustRuntimeMessage(t, oldUser.ID, "assistant", "old answer", now)
	recentUser := mustRuntimeMessage(t, oldAnswer.ID, "user", "recent question", now)
	recentAnswer := mustRuntimeMessage(t, recentUser.ID, "assistant", "recent answer", now)
	compaction, err := model.NewCompaction(recentAnswer.ID, model.CompactionData{
		Summary:          "old summary",
		FirstKeptEntryID: recentUser.ID,
		TokensBefore:     42,
	}, now)
	if err != nil {
		t.Fatalf("NewCompaction returned error: %v", err)
	}
	newer := mustRuntimeMessage(t, compaction.ID, "user", "newer question", now)
	state := graph.NewState([]model.Entry{oldUser, oldAnswer, recentUser, recentAnswer, compaction, newer})
	state.HeadID = newer.ID

	messages, err := BuildMessages(*state, ContextOptions{})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}
	var joined strings.Builder
	for _, message := range messages {
		joined.WriteString(message.Content)
		joined.WriteByte('\n')
	}
	output := joined.String()
	for _, want := range []string{"old summary", "recent question", "recent answer", "newer question"} {
		if !strings.Contains(output, want) {
			t.Fatalf("context missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, "old question") || strings.Contains(output, "old answer") {
		t.Fatalf("context still contains summarized history: %s", output)
	}
}

func TestBuildContextKeepsNewerHistoryWhenKeptEntryIsMissing(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	old := mustRuntimeMessage(t, "", "user", "old question", now)
	compaction, err := model.NewCompaction(old.ID, model.CompactionData{
		Summary:          "old summary",
		FirstKeptEntryID: "missing",
	}, now)
	if err != nil {
		t.Fatalf("NewCompaction returned error: %v", err)
	}
	newer := mustRuntimeMessage(t, compaction.ID, "user", "newer question", now)
	state := graph.NewState([]model.Entry{old, compaction, newer})
	state.HeadID = newer.ID

	messages, err := BuildMessages(*state, ContextOptions{})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}
	var joined strings.Builder
	for _, message := range messages {
		joined.WriteString(message.Content)
		joined.WriteByte('\n')
	}
	output := joined.String()
	if !strings.Contains(output, "old summary") || !strings.Contains(output, "newer question") {
		t.Fatalf("context did not keep the summary and newer history: %s", output)
	}
	if strings.Contains(output, "old question") {
		t.Fatalf("context still contains summarized history: %s", output)
	}
}

func mustRuntimeMessage(t *testing.T, parentID, role, content string, now time.Time) model.Entry {
	t.Helper()
	entry, err := model.NewMessage(parentID, role, content, now)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	return entry
}
