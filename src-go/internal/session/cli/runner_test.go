package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeSessionStore struct {
	entries []model.Entry
	missing bool
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
	deps.PromptNameFunc = func() (string, error) { return "work", nil }
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeFresh}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.openedName != "work" {
		t.Fatalf("expected store name work, got %q", deps.openedName)
	}
	if deps.started == nil || deps.started.CurrentBranch != "main" {
		t.Fatalf("expected repl state, got %#v", deps.started)
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
