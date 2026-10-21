package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
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
)

type SessionStore interface {
	Load() (sessionstore.LoadResult, error)
	Exists() (bool, error)
	Rename(name string) error
	sessionruntime.AppendStore
}

type SessionSummary struct {
	Name     string
	Title    string
	Modified time.Time
}

type StartREPLRequest struct {
	Name          string
	State         *graph.State
	Store         SessionStore
	Model         einomodel.BaseChatModel
	Config        config.AppConfig
	NoTUI         bool
	ReplayHistory bool
	// ReplayOnResume controls whether /resume replays the switched-to session's
	// history into the main buffer, following the same rules as startup replay.
	ReplayOnResume bool
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
	chatConfig := r.deps.Config
	var state *graph.State
	if options.Mode == ModeFresh {
		result, loadErr := store.Load()
		if loadErr != nil {
			return loadErr
		}
		if len(result.SkippedLines) > 0 {
			fmt.Fprintf(r.deps.Stderr, "warning: session %s: skipped unreadable line(s) %v\n", name, result.SkippedLines)
		}
		state = graph.NewState(result.Entries)
	} else {
		state, chatConfig, err = loadSessionState(store, chatConfig, name, r.deps.Stderr)
		if err != nil {
			if options.Mode == ModeResumeNamed {
				return fmt.Errorf("resume %s: %w", name, err)
			}
			return err
		}
	}
	chatModel, err := r.deps.NewModelFunc(chatConfig)
	if err != nil {
		return err
	}
	if options.Mode != ModeFresh {
		fmt.Fprintf(r.deps.Stderr, "resumed %s (%d entries, model %s)\n", name, len(state.Entries), chatConfig.DefaultModel)
	}
	return r.deps.StartREPLFunc(StartREPLRequest{
		Name:           name,
		State:          state,
		Store:          store,
		Model:          chatModel,
		Config:         chatConfig,
		NoTUI:          options.NoTUI,
		ReplayHistory:  r.shouldReplayHistory(options, state),
		ReplayOnResume: r.shouldReplayOnResume(options),
	})
}

// shouldReplayHistory decides whether a resumed conversation is written straight
// into the terminal main buffer. Replay is on by default for terminals so a
// resumed session shows its prior turns immediately; redirected or piped stdout
// stays free of conversation output unless --replay asks for it.
func (r SessionRunner) shouldReplayHistory(options Options, state *graph.State) bool {
	if options.Mode == ModeFresh || state == nil || state.HeadID == "" {
		return false
	}
	switch options.Replay {
	case ReplayOn:
		return true
	case ReplayOff:
		return false
	}
	return isTerminalWriter(r.deps.Stdout)
}

// shouldReplayOnResume applies the same replay preference to sessions switched
// to with /resume. Unlike startup replay, a fresh session can still replay a
// session it resumes into.
func (r SessionRunner) shouldReplayOnResume(options Options) bool {
	switch options.Replay {
	case ReplayOn:
		return true
	case ReplayOff:
		return false
	}
	return isTerminalWriter(r.deps.Stdout)
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
	case ModeContinue:
		summaries, err := r.deps.ListSessionsFunc()
		if err != nil {
			return "", err
		}
		if len(summaries) == 0 {
			return "", fmt.Errorf("no sessions found")
		}
		return summaries[0].Name, nil
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
			// The picker is interface chrome, not conversation output, so it goes
			// to stderr and leaves stdout for the session transcript.
			return pickSession(d.Stderr, d.Stdin, summaries)
		}
	}
	if d.StartREPLFunc == nil {
		d.StartREPLFunc = func(req StartREPLRequest) error {
			if !req.NoTUI && isTerminalReader(d.Stdin) && isTerminalWriter(d.Stdout) {
				return runInteractiveSession(req, d)
			}
			return runREPLSession(req, d)
		}
	}
	return d
}

func isTerminalReader(in io.Reader) bool {
	file, ok := in.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func isTerminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func pickStatusReporter(out io.Writer) sessionruntime.StatusReporter {
	if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return sessionrepl.NewTTYReporter(out)
	}
	return &sessionrepl.PlainReporter{Out: out}
}

func runChatTurnWithInterrupt(parent context.Context, stdin io.Reader, stderr io.Writer, run func(context.Context) error) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt)
	defer stop()
	monitor, err := startTurnKeyMonitor(stdin, stop)
	if err != nil {
		return err
	}
	runErr := run(ctx)
	if monitorErr := monitor.stop(); monitorErr != nil {
		return monitorErr
	}
	if runErr != nil {
		if errors.Is(runErr, context.Canceled) {
			fmt.Fprintln(stderr, "cancelled")
			return nil
		}
		return runErr
	}
	return nil
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
		summary := SessionSummary{Name: strings.TrimSuffix(entry.Name(), ".jsonl")}
		if info, statErr := entry.Info(); statErr == nil {
			summary.Modified = info.ModTime()
		}
		summary.Title = readSessionTitle(filepath.Join(root, entry.Name()))
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if !summaries[i].Modified.Equal(summaries[j].Modified) {
			return summaries[i].Modified.After(summaries[j].Modified)
		}
		return summaries[i].Name < summaries[j].Name
	})
	return summaries, nil
}

// readSessionTitle returns the persisted session title, if the file starts with
// one. Unreadable files yield no title; the picker still lists them by name.
func readSessionTitle(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 16*1024)
	for line := 0; line < 20; line++ {
		text, readErr := reader.ReadString('\n')
		if strings.TrimSpace(text) != "" {
			var entry model.Entry
			if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &entry); err == nil && entry.Type == model.EntryTypeSessionInfo {
				if data, dataErr := entry.SessionInfoData(); dataErr == nil {
					return data.Title
				}
				return ""
			}
		}
		if readErr != nil {
			return ""
		}
	}
	return ""
}

// loadSessionState reads an existing session, reporting skipped lines, and
// restores the model recorded in its session_info entry. The caller decides how
// to present errors.
func loadSessionState(store SessionStore, cfg config.AppConfig, name string, stderr io.Writer) (*graph.State, config.AppConfig, error) {
	exists, err := store.Exists()
	if err != nil {
		return nil, cfg, err
	}
	if !exists {
		return nil, cfg, ErrSessionNotFound
	}
	result, err := store.Load()
	if err != nil {
		return nil, cfg, err
	}
	if len(result.SkippedLines) > 0 {
		fmt.Fprintf(stderr, "warning: session %s: skipped unreadable line(s) %v\n", name, result.SkippedLines)
	}
	state := graph.NewState(result.Entries)
	if info, ok := sessionruntime.SessionInfoOf(state); ok && info.Model != "" && info.Model != cfg.DefaultModel {
		if resolved, restorable := resolveSessionModel(cfg, info.Model); restorable {
			cfg = resolved
		} else {
			fmt.Fprintf(stderr, "warning: could not restore model %s; using %s\n", info.Model, cfg.DefaultModel)
		}
	}
	return state, cfg, nil
}

// resolveSessionModel restores a session's recorded model. A model listed by
// another provider profile also switches the endpoint; when the active profile
// enumerates its models and the recorded one is absent everywhere, the caller
// keeps the configured model and warns instead.
func resolveSessionModel(cfg config.AppConfig, modelName string) (config.AppConfig, bool) {
	resolved := cfg
	resolved.DefaultModel = modelName
	providers := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	for _, name := range providers {
		if name == cfg.Provider {
			continue
		}
		provider := cfg.Providers[name]
		if !containsString(provider.Models, modelName) {
			continue
		}
		resolved.Provider = name
		if provider.APIEndpoint != "" {
			resolved.APIEndpoint = provider.APIEndpoint
		}
		if provider.APIKey != "" {
			resolved.APIKey = provider.APIKey
		}
		return resolved, true
	}
	if active, ok := cfg.Providers[cfg.Provider]; ok && len(active.Models) > 0 && !containsString(active.Models, modelName) {
		return cfg, false
	}
	return resolved, true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ErrSessionNotFound reports a resume target that does not exist on disk.
var ErrSessionNotFound = errors.New("session not found")

// errResumeCancelled reports that the user dismissed the resume picker; callers
// treat it as a clean exit rather than a failure.
var errResumeCancelled = errors.New("resume cancelled")

func pickSession(out io.Writer, in io.Reader, summaries []SessionSummary) (string, error) {
	if len(summaries) == 0 {
		return "", fmt.Errorf("no sessions found")
	}
	for i, summary := range summaries {
		if summary.Title != "" && summary.Title != summary.Name {
			fmt.Fprintf(out, "%d. %s (%s)\n", i+1, summary.Title, summary.Name)
			continue
		}
		fmt.Fprintf(out, "%d. %s\n", i+1, summary.Name)
	}
	for {
		fmt.Fprintf(out, "Resume which session? [1-%d, name, Enter to cancel]: ", len(summaries))
		line, err := readPickerLine(in)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errResumeCancelled
			}
			return "", err
		}
		choice := strings.TrimSpace(line)
		if choice == "" {
			return "", errResumeCancelled
		}
		if index, convErr := strconv.Atoi(choice); convErr == nil {
			if index >= 1 && index <= len(summaries) {
				return summaries[index-1].Name, nil
			}
			fmt.Fprintf(out, "No session %d; choose a number between 1 and %d.\n", index, len(summaries))
			continue
		}
		for _, summary := range summaries {
			if choice == summary.Name || (summary.Title != "" && choice == summary.Title) {
				return summary.Name, nil
			}
		}
		fmt.Fprintf(out, "No session named %q; choose a number or an exact name.\n", choice)
	}
}

// readPickerLine reads one line without over-reading stdin, so an interactive
// terminal is left untouched for the session editor that starts afterwards.
func readPickerLine(in io.Reader) (string, error) {
	var line strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			switch buf[0] {
			case '\n':
				return line.String(), nil
			case '\r':
			default:
				line.WriteByte(buf[0])
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) && line.Len() > 0 {
				return line.String(), nil
			}
			return "", err
		}
	}
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

func runInteractiveSession(req StartREPLRequest, d RunnerDeps) error {
	return runSessionLoop(req, d, sessiontui.NewEditorReader(d.Stdin, d.Stdout))
}

func runREPLSession(req StartREPLRequest, d RunnerDeps) error {
	return runSessionLoop(req, d, newLineReader(d.Stdin))
}

// activeSession is the mutable session target of the REPL. /resume swaps it so
// chat turns, title renames, and command persistence follow the new session.
type activeSession struct {
	name   string
	state  *graph.State
	store  SessionStore
	model  einomodel.BaseChatModel
	config config.AppConfig
}

func runSessionLoop(req StartREPLRequest, d RunnerDeps, reader sessionrepl.InputReader) error {
	active := &activeSession{
		name:   req.Name,
		state:  req.State,
		store:  req.Store,
		model:  req.Model,
		config: req.Config,
	}
	presentation := sessiontui.NewPresentation(d.Stdout, colorsEnabled(d.Stdout))
	if req.ReplayHistory {
		if err := replaySessionHistory(presentation, active.state); err != nil {
			return err
		}
	}
	runner := sessionrepl.ChatRunner(chatRunnerFunc(func(input string) error {
		if err := presentation.WriteUser(input); err != nil {
			return err
		}
		assistant := presentation.NewAssistantWriter()
		return runChatTurnWithInterrupt(context.Background(), d.Stdin, d.Stderr, func(ctx context.Context) error {
			err := sessionruntime.RunChatTurn(ctx, sessionruntime.ChatTurnRequest{
				Input:       input,
				State:       active.state,
				Store:       active.store,
				Writer:      assistant,
				Model:       active.model,
				TitleModel:  active.model,
				SessionName: active.name,
				ModelName:   active.config.DefaultModel,
				Status:      pickStatusReporter(d.Stderr),
				Compress:    true,
				OnTitle: func(result sessionruntime.TitleResult) error {
					nextName := sessionstore.NameFromTitle(result.Title)
					if concrete, ok := active.store.(interface {
						AvailableName(string) string
					}); ok {
						nextName = concrete.AvailableName(result.Title)
					}
					if err := active.store.Rename(nextName); err != nil {
						return err
					}
					active.name = nextName
					return nil
				},
			})
			if finishErr := assistant.Finish(); err == nil {
				err = finishErr
			}
			return err
		})
	}))
	resume := func(arg string) (sessionrepl.ResumeResult, error) {
		return resumeSessionInLoop(d, presentation, active, arg, req.ReplayOnResume)
	}
	code := sessionrepl.Loop(sessionrepl.LoopOptions{
		Reader:        reader,
		Chat:          runner,
		State:         active.state,
		Prompt:        presentation.UserLabel(),
		Stdout:        d.Stdout,
		CommandOutput: presentation.CommandWriter(),
		Stderr:        d.Stderr,
		Store:         active.store,
		Resume:        resume,
		Overlay:       sessiontui.NewOverlay(),
		ReportError:   presentation.WriteError,
	})
	if code != 0 {
		return fmt.Errorf("repl exited with code %d", code)
	}
	return nil
}

// resumeSessionInLoop switches the running REPL to another session. It uses the
// same single-line picker as startup so interactive stdin is left usable for
// the next prompt, and it only swaps the active session after the new state and
// model load successfully.
func resumeSessionInLoop(d RunnerDeps, presentation sessiontui.Presentation, active *activeSession, arg string, replay bool) (sessionrepl.ResumeResult, error) {
	if d.OpenStoreFunc == nil || d.NewModelFunc == nil {
		return sessionrepl.ResumeResult{}, errors.New("resume is not configured in this session")
	}
	name := strings.TrimSpace(arg)
	if name == "" {
		if d.ListSessionsFunc == nil || d.PickSessionFunc == nil {
			return sessionrepl.ResumeResult{}, errors.New("resume picker is not configured in this session")
		}
		summaries, err := d.ListSessionsFunc()
		if err != nil {
			return sessionrepl.ResumeResult{}, err
		}
		name, err = d.PickSessionFunc(summaries)
		if err != nil {
			if errors.Is(err, errResumeCancelled) {
				fmt.Fprintln(presentation.CommandWriter(), "resume cancelled")
				return sessionrepl.ResumeResult{}, nil
			}
			return sessionrepl.ResumeResult{}, err
		}
	}
	store, err := d.OpenStoreFunc(name)
	if err != nil {
		return sessionrepl.ResumeResult{}, fmt.Errorf("resume %s: %w", name, err)
	}
	state, cfg, err := loadSessionState(store, d.Config, name, d.Stderr)
	if err != nil {
		return sessionrepl.ResumeResult{}, fmt.Errorf("resume %s: %w", name, err)
	}
	model, err := d.NewModelFunc(cfg)
	if err != nil {
		return sessionrepl.ResumeResult{}, err
	}

	fmt.Fprintf(presentation.CommandWriter(), "resumed %s (%d entries, model %s)\n", name, len(state.Entries), cfg.DefaultModel)
	if replay {
		if err := replaySessionHistory(presentation, state); err != nil {
			return sessionrepl.ResumeResult{}, err
		}
	}
	active.name = name
	active.state = state
	active.store = store
	active.model = model
	active.config = cfg
	return sessionrepl.ResumeResult{State: state, Store: store}, nil
}

func replaySessionHistory(presentation sessiontui.Presentation, state *graph.State) error {
	history, err := state.ReachableHistory()
	if err != nil {
		return err
	}
	return presentation.WriteHistory(sessionruntime.ProjectHistory(history))
}

func colorsEnabled(out io.Writer) bool {
	return os.Getenv("NO_COLOR") == "" && isTerminalWriter(out)
}
