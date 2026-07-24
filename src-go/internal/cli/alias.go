package cli

// commandAliases maps short aliases to full subcommand names.
// Resolution happens at the root dispatch level in PrepareExecution,
// before any builtin or plugin lookup. Adding or removing entries here
// is the only step needed to register a new alias — no per-command
// changes required.
var commandAliases = map[string]string{
	"s": "session",
	"c": "chat",
	"i": "inspect",
	"p": "provider",
	"t": "toolcall",
}
