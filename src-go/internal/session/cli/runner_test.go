package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	sessionstore "github.com/Egg12138/cli-llm/src-go/internal/session/store"
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

	err := runChatTurnWithInterrupt(ctx, nil, &stderr, func(turnCtx context.Context) error {
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
	skipped []int
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

func mustRunnerSessionInfo(t *testing.T, modelName string) model.Entry {
	t.Helper()
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	entry, err := model.NewSessionInfo("", model.SessionInfo{Title: "work", Created: now, Model: modelName}, now)
	if err != nil {
		t.Fatalf("NewSessionInfo returned error: %v", err)
	}
	return entry
}

func (s *fakeSessionStore) Load() (sessionstore.LoadResult, error) {
	if s.missing {
		return sessionstore.LoadResult{}, errors.New("missing")
	}
	return sessionstore.LoadResult{Entries: s.entries, SkippedLines: s.skipped}, nil
}

func (s *fakeSessionStore) Exists() (bool, error) {
	return !s.missing, nil
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
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
	if deps.started != nil {
		t.Fatal("missing session must not start the REPL")
	}
}

func TestRunnerResumeNamedReportsSkippedLinesAndBanner(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	deps.store.entries = []model.Entry{mustRunnerMessage(t, "", "user", "hello")}
	deps.store.skipped = []int{2, 5}
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumeNamed, Name: "work"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{"skipped unreadable line(s) [2 5]", "resumed work (1 entries, model gpt-4o-mini)"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr %q missing %q", stderr.String(), want)
		}
	}
}

func TestRunnerResumeRestoresRecordedModel(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.Config.DefaultModel = "configured-model"
	deps.store.entries = []model.Entry{
		mustRunnerSessionInfo(t, "session-model"),
		mustRunnerMessage(t, "", "user", "hello"),
	}
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumeNamed, Name: "work"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.providerModel != "session-model" {
		t.Fatalf("provider model = %q, want the recorded session model", deps.providerModel)
	}
	if deps.started == nil {
		t.Fatal("expected the REPL to start")
	}
	if strings.Contains(stderr.String(), "could not restore") {
		t.Fatalf("unexpected restore warning: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "model session-model") {
		t.Fatalf("resume banner missing the restored model: %q", stderr.String())
	}
}

func TestRunnerResumeSwitchesProviderForRecordedModel(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.Config.Provider = "main"
	deps.Config.APIEndpoint = "https://main.example/v1"
	deps.Config.DefaultModel = "main-model"
	deps.Config.Providers = map[string]config.ProviderConfig{
		"main":  {APIEndpoint: "https://main.example/v1", Models: []string{"main-model"}},
		"other": {APIEndpoint: "https://other.example/v1", Models: []string{"session-model"}},
	}
	deps.store.entries = []model.Entry{
		mustRunnerSessionInfo(t, "session-model"),
		mustRunnerMessage(t, "", "user", "hello"),
	}
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumeNamed, Name: "work"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.providerModel != "session-model" {
		t.Fatalf("provider model = %q, want session-model", deps.providerModel)
	}
	if deps.providerConfig.Provider != "other" || deps.providerConfig.APIEndpoint != "https://other.example/v1" {
		t.Fatalf("provider config = %#v, want the other profile", deps.providerConfig)
	}
}

func TestRunnerResumeFallsBackWhenRecordedModelIsGone(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.Config.Provider = "main"
	deps.Config.DefaultModel = "configured-model"
	deps.Config.Providers = map[string]config.ProviderConfig{
		"main": {Models: []string{"configured-model"}},
	}
	deps.store.entries = []model.Entry{
		mustRunnerSessionInfo(t, "removed-model"),
		mustRunnerMessage(t, "", "user", "hello"),
	}
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeResumeNamed, Name: "work"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.providerModel != "configured-model" {
		t.Fatalf("provider model = %q, want the configured fallback", deps.providerModel)
	}
	if !strings.Contains(stderr.String(), "could not restore model removed-model") {
		t.Fatalf("missing restore warning: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "using configured-model") {
		t.Fatalf("missing fallback notice: %q", stderr.String())
	}
}

func TestRunnerContinueUsesMostRecentSession(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.ListSessionsFunc = func() ([]SessionSummary, error) {
		return []SessionSummary{{Name: "newest"}, {Name: "older"}}, nil
	}
	runner := NewRunner(deps.RunnerDeps)

	if err := runner.Run(Options{Mode: ModeContinue}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.openedName != "newest" {
		t.Fatalf("expected opened newest, got %q", deps.openedName)
	}
}

func TestRunnerContinueWithoutSessionsReportsError(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.ListSessionsFunc = func() ([]SessionSummary, error) { return nil, nil }
	runner := NewRunner(deps.RunnerDeps)

	err := runner.Run(Options{Mode: ModeContinue})
	if err == nil || !strings.Contains(err.Error(), "no sessions found") {
		t.Fatalf("expected no-sessions error, got %v", err)
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

func TestDefaultResumePickerSelectsByNumber(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.PickSessionFunc = nil
	deps.ListSessionsFunc = func() ([]SessionSummary, error) {
		return []SessionSummary{{Name: "alpha"}, {Name: "beta"}}, nil
	}
	deps.Stdin = strings.NewReader("2\n")
	var out bytes.Buffer
	deps.Stdout = &out

	if err := NewRunner(deps.RunnerDeps).Run(Options{Mode: ModeResumePicker}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.openedName != "beta" {
		t.Fatalf("expected opened beta, got %q", deps.openedName)
	}
	for _, want := range []string{"1. alpha\n", "2. beta\n", "Resume which session? [1-2, name, Enter to cancel]: "} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("picker output %q missing %q", out.String(), want)
		}
	}
}

func TestDefaultResumePickerSelectsByNameAndTitle(t *testing.T) {
	t.Parallel()

	summaries := []SessionSummary{{Name: "work-file", Title: "Work title"}, {Name: "plain"}}
	for _, choice := range []string{"work-file\n", "Work title\n", "plain"} {
		deps := fakeRunnerDeps(t)
		deps.PickSessionFunc = nil
		deps.ListSessionsFunc = func() ([]SessionSummary, error) { return summaries, nil }
		deps.Stdin = strings.NewReader(choice)
		var out bytes.Buffer
		deps.Stdout = &out

		want := "work-file"
		if choice == "plain" {
			want = "plain"
		}
		if err := NewRunner(deps.RunnerDeps).Run(Options{Mode: ModeResumePicker}); err != nil {
			t.Fatalf("choice %q returned error: %v", choice, err)
		}
		if deps.openedName != want {
			t.Fatalf("choice %q opened %q, want %q", choice, deps.openedName, want)
		}
		if !strings.Contains(out.String(), "1. Work title (work-file)\n") {
			t.Fatalf("picker output %q did not label title and name", out.String())
		}
		if strings.Contains(out.String(), "2. plain (plain)") {
			t.Fatalf("picker duplicated a session name: %q", out.String())
		}
	}
}

func TestDefaultResumePickerRepromptsOnInvalidSelection(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.PickSessionFunc = nil
	deps.ListSessionsFunc = func() ([]SessionSummary, error) {
		return []SessionSummary{{Name: "alpha"}, {Name: "beta"}}, nil
	}
	deps.Stdin = strings.NewReader("9\nmissing\n2\n")
	var out bytes.Buffer
	deps.Stdout = &out

	if err := NewRunner(deps.RunnerDeps).Run(Options{Mode: ModeResumePicker}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if deps.openedName != "beta" {
		t.Fatalf("expected opened beta, got %q", deps.openedName)
	}
	for _, want := range []string{"No session 9", "No session named \"missing\""} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("picker output %q missing %q", out.String(), want)
		}
	}
}

func TestDefaultResumePickerCancelsOnEmptyInputOrEOF(t *testing.T) {
	t.Parallel()

	for _, stdin := range []string{"\n", ""} {
		deps := fakeRunnerDeps(t)
		deps.PickSessionFunc = nil
		deps.ListSessionsFunc = func() ([]SessionSummary, error) {
			return []SessionSummary{{Name: "alpha"}}, nil
		}
		deps.Stdin = strings.NewReader(stdin)
		deps.Stdout = io.Discard

		err := NewRunner(deps.RunnerDeps).Run(Options{Mode: ModeResumePicker})
		if !errors.Is(err, errResumeCancelled) {
			t.Fatalf("stdin %q returned %v, want errResumeCancelled", stdin, err)
		}
		if deps.openedName != "" {
			t.Fatalf("stdin %q opened %q before cancelling", stdin, deps.openedName)
		}
	}
}

func TestDefaultResumePickerRequiresSessions(t *testing.T) {
	t.Parallel()

	deps := fakeRunnerDeps(t)
	deps.PickSessionFunc = nil
	deps.ListSessionsFunc = func() ([]SessionSummary, error) { return nil, nil }
	deps.Stdout = io.Discard

	err := NewRunner(deps.RunnerDeps).Run(Options{Mode: ModeResumePicker})
	if err == nil || !strings.Contains(err.Error(), "no sessions found") {
		t.Fatalf("expected no-sessions error, got %v", err)
	}
}

func TestDefaultListSessionsReadsTitlesAndSortsByRecency(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".cli-llm", "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create session root: %v", err)
	}
	writeListedSession(t, root, "older", "Older title", time.Unix(1000, 0))
	writeListedSession(t, root, "newer", "Newer title", time.Unix(2000, 0))
	broken := filepath.Join(root, "broken.jsonl")
	if err := os.WriteFile(broken, []byte("not-json\n"), 0o644); err != nil {
		t.Fatalf("write broken session: %v", err)
	}
	if err := os.Chtimes(broken, time.Unix(3000, 0), time.Unix(3000, 0)); err != nil {
		t.Fatalf("set broken mtime: %v", err)
	}

	summaries, err := defaultListSessions()
	if err != nil {
		t.Fatalf("defaultListSessions returned error: %v", err)
	}
	wantNames := []string{"broken", "newer", "older"}
	wantTitles := []string{"", "Newer title", "Older title"}
	if len(summaries) != len(wantNames) {
		t.Fatalf("summaries = %#v, want %d entries", summaries, len(wantNames))
	}
	for i := range wantNames {
		if summaries[i].Name != wantNames[i] || summaries[i].Title != wantTitles[i] {
			t.Fatalf("summary %d = %#v, want name %q title %q", i, summaries[i], wantNames[i], wantTitles[i])
		}
	}
}

func writeListedSession(t *testing.T, root, name, title string, modified time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	info, err := model.NewSessionInfo("", model.SessionInfo{Title: title, Created: now, Model: "mock-model"}, now)
	if err != nil {
		t.Fatalf("create session info: %v", err)
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("encode session info: %v", err)
	}
	path := filepath.Join(root, name+".jsonl")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write session %s: %v", name, err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatalf("set session %s mtime: %v", name, err)
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
	store          *fakeSessionStore
	openedName     string
	started        *graph.State
	providerModel  string
	providerConfig config.AppConfig
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
			probe.providerConfig = cfg
			return &fakeProviderModel{model: cfg.DefaultModel}, nil
		},
		StartREPLFunc: func(req StartREPLRequest) error {
			probe.started = req.State
			return nil
		},
		Stderr:         io.Discard,
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
