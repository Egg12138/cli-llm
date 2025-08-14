# CLI LLM - Interactive Mode

This project now includes an interactive mode that provides a more user-friendly experience similar to qwen-code.

## Features

1. **Interactive Shell**: When run without arguments, the CLI enters interactive mode
2. **Slash Commands**: Special commands prefixed with `/` for controlling the session
3. **Text References**: Use `@` to reference text or files
4. **Status Display**: Shows current directory, model, and endpoint

## Usage

### Interactive Mode

Run the CLI without any prompt to enter interactive mode:

```bash
python3 llm
```

### Non-Interactive Mode

Run the CLI with a prompt to use non-interactive mode:

```bash
python3 llm "Explain quantum computing"
```

Or with piped input:

```bash
echo "Explain quantum computing" | python3 llm
```

## Interactive Mode Features

### Status Display

The interactive mode shows a status line at the top:

```
[~/current/path] model: deepseek-chat endpoint: http://localhost:8080
```

### Slash Commands

Available slash commands:

- `/help` - Show help information
- `/quit` - Exit the program
- `/role [role_name]` - Switch system role (coder, chat, creative, smart)

Examples:
```
>>> /help
>>> /role chat
>>> /role
>>> /quit
```

### Text References

Use `@` to reference text or files:

```
>>> @This is some reference text
>>> Explain the above text
```

Or reference a file:
```
>>> @/path/to/file.txt
>>> Summarize the content
```

The referenced text is combined with your next prompt when sent to the LLM.

## System Roles

Available system roles:
- `coder` - Detailed programming assistant
- `chat` - General purpose chat assistant
- `creative` - Creative AI assistant
- `smart` - Simple and efficient AI assistant for general tasks

You can switch roles using the `/role` command or by specifying the `-r` option when starting the CLI.