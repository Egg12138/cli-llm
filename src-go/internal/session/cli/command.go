package cli

import (
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
)

type Options struct {
	Mode  Mode
	Name  string
	NoTUI bool
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
	showVersion := flags.Bool("version", false, "print version and exit")
	noTUI := flags.Bool("no-tui", false, "disable TUI mode (line-based REPL)")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return Options{Mode: ModeHelp}, nil
		}
		return Options{}, err
	}
	if *showVersion {
		return Options{Mode: ModeVersion}, nil
	}

	rest := flags.Args()
	if len(rest) > 1 {
		return Options{}, fmt.Errorf("expected at most one session name, got %d", len(rest))
	}

	if !*resume {
		if len(rest) > 0 {
			return Options{}, fmt.Errorf("session name requires --resume")
		}
		return Options{Mode: ModeFresh, NoTUI: *noTUI}, nil
	}
	if len(rest) == 0 {
		return Options{Mode: ModeResumePicker, NoTUI: *noTUI}, nil
	}
	return Options{Mode: ModeResumeNamed, Name: rest[0], NoTUI: *noTUI}, nil
}

func Run(args []string, runner Runner) int {
	options, err := Parse(args)
	if err != nil {
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
		return 1
	}
	return 0
}

func printVersion(out io.Writer) {
	fmt.Fprintf(out, "llm-session %s\n", Version)
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: llm-session [--resume [NAME]]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Modes:")
	fmt.Fprintln(out, "  llm-session                 Start a fresh session")
	fmt.Fprintln(out, "  llm-session --resume        Pick a session to resume")
	fmt.Fprintln(out, "  llm-session --resume NAME   Resume NAME")
	fmt.Fprintln(out, "  llm-session --no-tui         Disable TUI (line-based REPL)")
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
