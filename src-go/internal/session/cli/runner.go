package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
	"github.com/Egg12138/cli-llm/src-go/internal/providers"
	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	sessionruntime "github.com/Egg12138/cli-llm/src-go/internal/session/runtime"
	sessionstore "github.com/Egg12138/cli-llm/src-go/internal/session/store"
	sessiontui "github.com/Egg12138/cli-llm/src-go/internal/session/tui"
	einomodel "github.com/cloudwego/eino/components/model"
	"golang.org/x/term"
	tea "github.com/charmbracelet/bubbletea"
)

type SessionStore interface {
	Load() ([]model.Entry, error)
	Rename(name string) error
	sessionruntime.AppendStore
}

type SessionSummary struct {
	Name  string
	Title string
}

type StartREPLRequest struct {
	Name   string
	State  *graph.State
	Store  SessionStore
	Model  einomodel.BaseChatModel
	Config config.AppConfig
	NoTUI  bool
}

type RunnerDeps struct {
	Config           config.AppConfig
	OpenStoreFunc    func(name string) (SessionStore, error)
	NewModelFunc     func(config.AppConfig) (einomodel.BaseChatModel, error)
	StartREPLFunc    func(StartREPLRequest) error
	PromptNameFunc   func() (string, error)
	ListSessionsFunc func() ([]SessionSummary, error)
	PickSessionFunc  func([]SessionSummary) (string, error)
	Stdout           io.Writer
	Stderr           io.Writer
	Stdin            io.Reader
}

type SessionRunner struct {
	deps RunnerDeps
}

func NewRunner(deps RunnerDeps) SessionRunner {
	return SessionRunner{deps: deps.withDefaults()}
}

func (r SessionRunner) Run(options Options) error {
	name, err := r.resolveName(options)
	if err != nil {
		return err
	}
	store, err := r.deps.OpenStoreFunc(name)
	if err != nil {
		if options.Mode == ModeResumeNamed {
			return fmt.Errorf("resume %s: %w", name, err)
		}
		return err
	}
	entries, err := store.Load()
	if err != nil {
		if options.Mode == ModeResumeNamed {
			return fmt.Errorf("resume %s: %w", name, err)
		}
		return err
	}
	state := graph.NewState(entries)
	chatModel, err := r.deps.NewModelFunc(r.deps.Config)
	if err != nil {
		return err
	}
	return r.deps.StartREPLFunc(StartREPLRequest{
		Name:   name,
		State:  state,
		Store:  store,
		Model:  chatModel,
		Config: r.deps.Config,
			NoTUI:  options.NoTUI,
	})
}

func (r SessionRunner) resolveName(options Options) (string, error) {
	switch options.Mode {
	case ModeFresh:
		return generatedSessionName(), nil
	case ModeResumeNamed:
		return options.Name, nil
	case ModeResumePicker:
		summaries, err := r.deps.ListSessionsFunc()
		if err != nil {
			return "", err
		}
		return r.deps.PickSessionFunc(summaries)
	default:
		return "", fmt.Errorf("unsupported mode %q", options.Mode)
	}
}

func (d RunnerDeps) withDefaults() RunnerDeps {
	if d.Stdout == nil {
		d.Stdout = os.Stdout
	}
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
	if d.Stdin == nil {
		d.Stdin = os.Stdin
	}
	if d.OpenStoreFunc == nil {
		d.OpenStoreFunc = func(name string) (SessionStore, error) {
			store, err := sessionstore.Open(sessionstore.DefaultRoot(), name)
			if err != nil {
				return nil, err
			}
			return &store, nil
		}
	}
	if d.NewModelFunc == nil {
		d.NewModelFunc = func(cfg config.AppConfig) (einomodel.BaseChatModel, error) {
			return providers.NewFactory(cfg).New(context.Background(), false)
		}
	}
	if d.PromptNameFunc == nil {
		d.PromptNameFunc = func() (string, error) {
			fmt.Fprint(d.Stdout, "session name: ")
			scanner := bufio.NewScanner(d.Stdin)
			if !scanner.Scan() {
				return "", scanner.Err()
			}
			return strings.TrimSpace(scanner.Text()), nil
		}
	}
	if d.ListSessionsFunc == nil {
		d.ListSessionsFunc = defaultListSessions
	}
	if d.PickSessionFunc == nil {
		d.PickSessionFunc = func(summaries []SessionSummary) (string, error) {
			if len(summaries) == 0 {
				return "", fmt.Errorf("no sessions found")
			}
			for i, summary := range summaries {
				label := summary.Title
				if label == "" {
					label = summary.Name
				}
				fmt.Fprintf(d.Stdout, "%d. %s (%s)\n", i+1, label, summary.Name)
			}
			return summaries[0].Name, nil
		}
	}
	if d.StartREPLFunc == nil {
		d.StartREPLFunc = func(req StartREPLRequest) error {
			useTUI := !req.NoTUI
			if f, ok := d.Stdout.(*os.File); ok {
				useTUI = useTUI && term.IsTerminal(int(f.Fd()))
			} else {
				useTUI = false
			}
			if useTUI {
				return runTUISession(req)
			}
			return runREPLSession(req, d)
		}
	}
	return d
}

func pickStatusReporter(out io.Writer) sessionruntime.StatusReporter {
	if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return sessionrepl.NewTTYReporter(out)
	}
	return &sessionrepl.PlainReporter{Out: out}
}

func generatedSessionName() string {
	return fmt.Sprintf("session-%d-%d", time.Now().UTC().UnixNano(), os.Getpid())
}

func defaultListSessions() ([]SessionSummary, error) {
	root := sessionstore.DefaultRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionSummary{}, nil
		}
		return nil, err
	}
	summaries := make([]SessionSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		summaries = append(summaries, SessionSummary{Name: name})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	return summaries, nil
}

type chatRunnerFunc func(string) error

func (fn chatRunnerFunc) RunChat(input string) error {
	return fn(input)
}

type lineReader struct {
	scanner *bufio.Scanner
}

func newLineReader(in io.Reader) *lineReader {
	return &lineReader{scanner: bufio.NewScanner(in)}
}

func (r *lineReader) ReadLine(prompt string) (string, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return r.scanner.Text(), nil
}

func (r *lineReader) ReadEvent(prompt string) sessionrepl.InputEvent {
	line, err := r.ReadLine(prompt)
	if err != nil {
		return sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Err: err}
	}
	if strings.Contains(line, "\x14") {
		return sessionrepl.InputEvent{Kind: sessionrepl.EventTranscript}
	}
	return sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Line: line}
}

func runTUISession(req StartREPLRequest) error {
	m := sessiontui.NewSessionModel(sessiontui.SessionConfig{
		State:       req.State,
		Store:       req.Store,
		Model:       req.Model,
		TitleModel:  req.Model,
		ModelName:   req.Config.DefaultModel,
		SessionName: req.Name,
	})
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout)).Run()
	return err
}

func runREPLSession(req StartREPLRequest, d RunnerDeps) error {
	sessionName := req.Name
	runner := sessionrepl.ChatRunner(chatRunnerFunc(func(input string) error {
		return sessionruntime.RunChatTurn(context.Background(), sessionruntime.ChatTurnRequest{
			Input:       input,
			State:       req.State,
			Store:       req.Store,
			Writer:      d.Stdout,
			Model:       req.Model,
			TitleModel:  req.Model,
			SessionName: sessionName,
			ModelName:   req.Config.DefaultModel,
			OnTitle: func(result sessionruntime.TitleResult) error {
				nextName := sessionstore.NameFromTitle(result.Title)
				if concrete, ok := req.Store.(interface {
					AvailableName(string) string
				}); ok {
					nextName = concrete.AvailableName(result.Title)
				}
				if err := req.Store.Rename(nextName); err != nil {
					return err
				}
				sessionName = nextName
				return nil
			},
		})
	}))
	code := sessionrepl.Loop(sessionrepl.LoopOptions{
		Reader:  newLineReader(d.Stdin),
		Chat:    runner,
		State:   req.State,
		Stdout:  d.Stdout,
		Stderr:  d.Stderr,
		Store:   req.Store,
		Overlay: sessiontui.NewOverlay(),
	})
	if code != 0 {
		return fmt.Errorf("repl exited with code %d", code)
	}
	return nil
}
