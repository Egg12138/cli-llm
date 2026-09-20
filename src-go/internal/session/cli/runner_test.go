package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/creack/pty"
)

func TestRunInteractiveSessionUsesMainBuffer(t *testing.T) {
	store := &fakeSessionStore{}
	var out bytes.Buffer
	d := RunnerDeps{
		Stdin:  bytes.NewBufferString("/exit\r"),
		Stdout: &out,
		Stderr: io.Discard,
	}
	req := StartREPLRequest{
		Name:  "test-session",
		State: graph.NewState(nil),
		Store: store,
		Model: &fakeProviderModel{model: "test-model"},
		Config: config.AppConfig{
			DefaultModel: "test-model",
		},
	}

	if err := runInteractiveSession(req, d); err != nil {
		t.Fatalf("runInteractiveSession returned error: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("\x1b[?1049h")) {
		t.Fatalf("interactive session entered alternate screen: %q", out.Bytes())
	}
}

func TestRunChatTurnWithInterruptReturnsToTheREPLAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr bytes.Buffer

	err := runChatTurnWithInterrupt(ctx, &stderr, func(turnCtx context.Context) error {
		if turnCtx.Err() == nil {
			t.Fatal("turn context was not cancelled")
		}
		return turnCtx.Err()
	})
	if err != nil {
		t.Fatalf("runChatTurnWithInterrupt returned error: %v", err)
	}
	if got := stderr.String(); got != "cancelled\n" {
		t.Fatalf("cancellation output = %q, want %q", got, "cancelled\n")
	}
}

type fakeSessionStore struct {
	entries []model.Entry
	missing bool
	renamed string
}

func mustRunnerMessage(t *testing.T, parentID, role, content string) model.Entry {
	t.Helper()
	entry, err := model.NewMessage(parentID, role, content, time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	return entry
}

func (s *fakeSessionStore) Load() ([]model.Entry, error) {
	if s.missing {
		return nil, errors.New("missing")
	}
	return s.entries, nil
}

func (s *fakeSessionStore) Append(entry model.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *fakeSessionStore) Rename(name string) error {
	s.renamed = name
	return nil
}

type fakeProviderModel struct {
	model string
}

func (m *fakeProviderModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	return schema.AssistantMessage("title", nil), nil
}

func (m *fakeProviderModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("answer", nil)}), nil
}

func TestRunnerFreshPromptsForNameAndStartsREPL(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.PromptNameFunc = func() (string, error) {
		t.Fatalf("fresh session should not prompt for a session name")
		return "", nil
	}
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeFresh}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.HasPrefix(deps.openedName, "session-") {
		t.Fatalf("expected generated temporary session name, got %q", deps.openedName)
	}
	if deps.started == nil || deps.started.CurrentBranch != "main" {
		t.Fatalf("expected repl state, got %#v", deps.started)
	}
}

func TestRunnerFreshRenamesSessionFromGeneratedTitle(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{}
	out := &strings.Builder{}
	deps := RunnerDeps{
		Config: config.AppConfig{DefaultModel: "gpt-4o-mini"},
		OpenStoreFunc: func(name string) (SessionStore, error) {
			return store, nil
		},
		NewModelFunc: func(cfg config.AppConfig) (einomodel.BaseChatModel, error) {
			return &fakeProviderModel{model: cfg.DefaultModel}, nil
		},
		Stdin:  strings.NewReader("hello world\n"),
		Stdout: out,
		Stderr: io.Discard,
	}
	runner := NewRunner(deps)

	if err := runner.Run(Options{Mode: ModeFresh}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if store.renamed != "title" {
		t.Fatalf("expected generated title to become session name, got %q", store.renamed)
	}
	if strings.Contains(out.String(), "session name:") {
		t.Fatalf("fresh session prompted for a name: %q", out.String())
	}
	const want = "You ›\nhello world\n\nAssistant ›\nanswer\n\n"
	if got := out.String(); got != want {
		t.Fatalf("session output = %q, want %q", got, want)
	}
}

func TestColorsEnabledDefaultsOnForTTYAndHonorsNoColor(t *testing.T) {
	terminal, peer, err := pty.Open()
	if err != nil {
		t.Fatalf("open PTY: %v", err)
	}
	defer terminal.Close()
	defer peer.Close()

	t.Setenv("NO_COLOR", "")
	if !colorsEnabled(peer) {
		t.Fatal("TTY colors should be enabled when NO_COLOR is empty")
	}
	t.Setenv("NO_COLOR", "no")
	if colorsEnabled(peer) {
		t.Fatal("any non-empty NO_COLOR value should disable colors")
	}
	if colorsEnabled(&bytes.Buffer{}) {
		t.Fatal("non-TTY output should not emit colors")
	}
}

func TestRunnerResumeNamedLoadsExistingSession(t *testing.T) {
	t.Parallel()

	existing := mustRunnerMessage(t, "", "user", "hello")
	deps := fakeRunnerDeps(t)
	deps.store.entries = []model.Entry{existing}
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumeNamed, Name: "work"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.openedName != "work" {
		t.Fatalf("expected opened work, got %q", deps.openedName)
	}
	if deps.started.HeadID != existing.ID {
		t.Fatalf("expected resumed head %q, got %q", existing.ID, deps.started.HeadID)
	}
}

func TestRunnerResumeMissingNamedSessionReportsError(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.store.missing = true
	runner := NewRunner(deps.RunnerDeps)

	err := runner.Run(Options{Mode: ModeResumeNamed, Name: "missing"})
	if err == nil || !strings.Contains(err.Error(), "resume missing") {
		t.Fatalf("expected clear resume error, got %v", err)
	}
}

func TestRunnerResumePickerUsesSelectedSession(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.ListSessionsFunc = func() ([]SessionSummary, error) {
		return []SessionSummary{{Name: "work", Title: "Work title"}}, nil
	}
	deps.PickSessionFunc = func(summaries []SessionSummary) (string, error) {
		if summaries[0].Title != "Work title" {
			t.Fatalf("expected title in picker, got %#v", summaries)
		}
		return summaries[0].Name, nil
	}
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumePicker}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.openedName != "work" {
		t.Fatalf("expected opened work, got %q", deps.openedName)
	}
}

func TestRunnerProviderFactoryReceivesDefaultModel(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.Config.DefaultModel = "configured-model"
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumeNamed, Name: "work"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.providerModel != "configured-model" {
		t.Fatalf("expected provider model configured-model, got %q", deps.providerModel)
	}
}

type runnerDepsProbe struct {
	RunnerDeps
	store         *fakeSessionStore
	openedName    string
	started       *graph.State
	providerModel string
}

func fakeRunnerDeps(t *testing.T) *runnerDepsProbe {
	t.Helper()
	probe := &runnerDepsProbe{store: &fakeSessionStore{}}
	probe.RunnerDeps = RunnerDeps{
		Config: config.AppConfig{DefaultModel: "gpt-4o-mini", DefaultRole: "coder"},
		OpenStoreFunc: func(name string) (SessionStore, error) {
			probe.openedName = name
			return probe.store, nil
		},
		NewModelFunc: func(cfg config.AppConfig) (einomodel.BaseChatModel, error) {
			probe.providerModel = cfg.DefaultModel
			return &fakeProviderModel{model: cfg.DefaultModel}, nil
		},
		StartREPLFunc: func(req StartREPLRequest) error {
			probe.started = req.State
			return nil
		},
		PromptNameFunc: func() (string, error) { return "default", nil },
		ListSessionsFunc: func() ([]SessionSummary, error) {
			return []SessionSummary{{Name: "default"}}, nil
		},
		PickSessionFunc: func(summaries []SessionSummary) (string, error) {
			return summaries[0].Name, nil
		},
	}
	return probe
}
