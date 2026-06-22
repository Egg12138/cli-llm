package plugins

type ResultKind string

const (
	Builtin ResultKind = "builtin"
	Plugin  ResultKind = "plugin"
	Missing ResultKind = "missing"
)

type LookPathFunc func(string) (string, error)

type Dispatcher struct {
	builtins map[string]struct{}
	lookPath LookPathFunc
}

type Result struct {
	Kind     ResultKind
	Name     string
	Path     string
	ExecArgs []string
}

func NewDispatcher(builtins []string, lookPath LookPathFunc) Dispatcher {
	builtinSet := make(map[string]struct{}, len(builtins))
	for _, name := range builtins {
		builtinSet[name] = struct{}{}
	}
	return Dispatcher{builtins: builtinSet, lookPath: lookPath}
}

func (dispatcher Dispatcher) Resolve(args []string) Result {
	if len(args) == 0 {
		return Result{Kind: Missing}
	}

	name := args[0]
	if _, ok := dispatcher.builtins[name]; ok {
		return Result{Kind: Builtin, Name: name}
	}

	path, err := dispatcher.lookPath("llm-" + name)
	if err != nil {
		return Result{Kind: Missing, Name: name}
	}

	execArgs := make([]string, 0, len(args))
	execArgs = append(execArgs, path)
	execArgs = append(execArgs, args[1:]...)
	return Result{
		Kind:     Plugin,
		Name:     name,
		Path:     path,
		ExecArgs: execArgs,
	}
}
