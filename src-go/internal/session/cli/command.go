package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/config"
)

type Mode string

const (
	ModeFresh        Mode = "fresh"
	ModeHelp         Mode = "help"
	ModeVersion      Mode = "version"
	ModeResumePicker Mode = "resume-picker"
	ModeResumeNamed  Mode = "resume-named"
	ModeContinue     Mode = "continue"
)

// ReplayMode controls whether a resumed session replays its history into the
// terminal main buffer before the prompt appears.
type ReplayMode string

const (
	// ReplayAuto replays history when the session renders to a terminal; piped
	// or redirected stdout stays free of conversation output.
	ReplayAuto ReplayMode = ""
	ReplayOn   ReplayMode = "on"
	ReplayOff  ReplayMode = "off"
)

type Options struct {
	Mode   Mode
	Name   string
	NoTUI  bool
	Replay ReplayMode
}

type Runner interface {
	Run(options Options) error
}

var (
	commandStdout io.Writer = os.Stdout
	commandStderr io.Writer = os.Stderr
)

func Parse(args []string) (Options, error) {
	flags := flag.NewFlagSet("llm-session", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	resume := flags.Bool("resume", false, "resume an existing session")
	resumeShort := flags.Bool("r", false, "resume an existing session (shorthand)")
	continueLatest := flags.Bool("continue", false, "continue the most recent session")
	continueShort := flags.Bool("c", false, "continue the most recent session (shorthand)")
	showVersion := flags.Bool("version", false, "print version and exit")
	noTUI := flags.Bool("no-tui", false, "disable TUI mode (line-based REPL)")
	replay := flags.Bool("replay", false, "always replay resumed history")
	noReplay := flags.Bool("no-replay", false, "never replay resumed history")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return Options{Mode: ModeHelp}, nil
		}
		return Options{}, err
	}
	if *showVersion {
		return Options{Mode: ModeVersion}, nil
	}

	resumeMode := *resume || *resumeShort
	continueMode := *continueLatest || *continueShort
	if resumeMode && continueMode {
		return Options{}, fmt.Errorf("--resume and --continue are mutually exclusive")
	}
	if *replay && *noReplay {
		return Options{}, fmt.Errorf("--replay and --no-replay are mutually exclusive")
	}
	replayMode := ReplayAuto
	if *replay {
		replayMode = ReplayOn
	}
	if *noReplay {
		replayMode = ReplayOff
	}

	rest := flags.Args()
	if len(rest) > 1 {
		return Options{}, fmt.Errorf("expected at most one session name, got %d", len(rest))
	}

	if continueMode {
		if len(rest) > 0 {
			return Options{}, fmt.Errorf("session name requires --resume")
		}
		return Options{Mode: ModeContinue, NoTUI: *noTUI, Replay: replayMode}, nil
	}
	if !resumeMode {
		if len(rest) > 0 {
			return Options{}, fmt.Errorf("session name requires --resume")
		}
		return Options{Mode: ModeFresh, NoTUI: *noTUI, Replay: replayMode}, nil
	}
	if len(rest) == 0 {
		return Options{Mode: ModeResumePicker, NoTUI: *noTUI, Replay: replayMode}, nil
	}
	return Options{Mode: ModeResumeNamed, Name: rest[0], NoTUI: *noTUI, Replay: replayMode}, nil
}

func Run(args []string, runner Runner) int {
	options, err := Parse(args)
	if err != nil {
		fmt.Fprintln(commandStderr, err)
		return 2
	}
	if options.Mode == ModeHelp {
		printUsage(commandStdout)
		return 0
	}
	if options.Mode == ModeVersion {
		printVersion(commandStdout)
		return 0
	}
	if err := runner.Run(options); err != nil {
		if errors.Is(err, errResumeCancelled) {
			return 0
		}
		fmt.Fprintln(commandStderr, err)
		return 1
	}
	return 0
}

func printVersion(out io.Writer) {
	fmt.Fprintf(out, "llm-session %s\n", Version)
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: llm-session [--resume [NAME] | --continue]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Modes:")
	fmt.Fprintln(out, "  llm-session                 Start a fresh session")
	fmt.Fprintln(out, "  llm-session --resume        Pick a session to resume (-r)")
	fmt.Fprintln(out, "  llm-session --resume NAME   Resume NAME")
	fmt.Fprintln(out, "  llm-session --continue      Continue the most recent session (-c)")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Options:")
	fmt.Fprintln(out, "  --replay                    Always replay the resumed conversation history")
	fmt.Fprintln(out, "  --no-replay                 Never replay the resumed conversation history")
	fmt.Fprintln(out, "  --no-tui                    Disable TUI (line-based REPL)")
}

func Main(args []string) int {
	cfg, err := defaultAppConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return Run(args, defaultRunner{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		config: cfg,
	})
}

type defaultRunner struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	config config.AppConfig
}

func (r defaultRunner) Run(options Options) error {
	return NewRunner(RunnerDeps{
		Config: r.config,
		Stdin:  r.stdin,
		Stdout: r.stdout,
		Stderr: r.stderr,
	}).Run(options)
}

func defaultAppConfig() (config.AppConfig, error) {
	return config.NewLoader(defaultConfigPath(), nil).Load(config.LoadOptions{
		Environment: currentEnvironment(),
	})
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cli-llm", "config.toml")
}

func currentEnvironment() map[string]string {
	environment := make(map[string]string)
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		environment[parts[0]] = parts[1]
	}
	return environment
}
