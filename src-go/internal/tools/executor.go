package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

const MaxStdoutChars = 64 * 1024

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
var controlPattern = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]`)

type Call struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type ExecutionResult struct {
	Tool      string
	Arguments map[string]any
	Stdout    string
	ExitCode  int
}

type Executor struct {
	WorkingDir string
}

func ParseToolCalls(message *schema.Message) ([]Call, error) {
	if message == nil {
		return nil, fmt.Errorf("provider response did not include tool calls")
	}

	calls := make([]Call, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		arguments, err := decodeArguments(toolCall.Function.Arguments)
		if err != nil {
			return nil, fmt.Errorf("invalid tool arguments for %s: %w", toolCall.Function.Name, err)
		}
		calls = append(calls, Call{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: arguments,
		})
	}

	return calls, nil
}

func (e Executor) Execute(definition Definition, arguments map[string]any) (ExecutionResult, error) {
	validated, err := ValidateArguments(definition, arguments)
	if err != nil {
		return ExecutionResult{}, err
	}

	switch definition.Name {
	case "read":
		return e.executeRead(definition, validated)
	case "ls":
		return e.executeLS(definition, validated)
	case "find":
		return e.executeFind(definition, validated)
	case "grep":
		return e.executeGrep(definition, validated)
	case "bash":
		return e.executeBash(definition, validated)
	default:
		return ExecutionResult{}, fmt.Errorf("no executor for tool %s", definition.Name)
	}
}

func ValidateArguments(definition Definition, arguments map[string]any) (map[string]any, error) {
	required := stringSlice(definition.Parameters["required"])
	properties, _ := definition.Parameters["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
	}

	for _, key := range required {
		if _, ok := arguments[key]; !ok {
			return nil, fmt.Errorf("missing required argument '%s' for tool %s", key, definition.Name)
		}
	}

	if additional, ok := definition.Parameters["additionalProperties"].(bool); ok && !additional {
		unknown := make([]string, 0)
		for key := range arguments {
			if _, ok := properties[key]; !ok {
				unknown = append(unknown, key)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, fmt.Errorf("unknown argument(s) for tool %s: %s", definition.Name, strings.Join(unknown, ", "))
		}
	}

	validated := make(map[string]any, len(arguments))
	for key, value := range arguments {
		property, _ := properties[key].(map[string]any)
		expectedType, _ := property["type"].(string)

		normalized, err := normalizeValue(value, expectedType)
		if err != nil {
			return nil, fmt.Errorf("argument '%s' for tool %s %s", key, definition.Name, err.Error())
		}
		validated[key] = normalized
	}

	return validated, nil
}

func SafeStdout(text string, maxChars int) string {
	cleaned := ansiPattern.ReplaceAllString(text, "")
	cleaned = controlPattern.ReplaceAllString(cleaned, "")
	if len(cleaned) > maxChars {
		return cleaned[:maxChars] + "\n[truncated]\n"
	}
	return cleaned
}

func (e Executor) executeRead(definition Definition, arguments map[string]any) (ExecutionResult, error) {
	path, err := resolvePath(e.WorkingDir, stringArg(arguments, "path"))
	if err != nil {
		return ExecutionResult{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ExecutionResult{}, err
	}

	lines := splitLines(string(data))
	offset := intArg(arguments, "offset", 0)
	if offset > len(lines) {
		offset = len(lines)
	}
	limit := intArgPtr(arguments, "limit")
	selected := lines[offset:]
	if limit != nil {
		end := offset + *limit
		if end < len(lines) {
			selected = lines[offset:end]
		}
	}

	return ExecutionResult{
		Tool:      definition.Name,
		Arguments: arguments,
		Stdout:    SafeStdout(strings.Join(selected, ""), MaxStdoutChars),
		ExitCode:  0,
	}, nil
}

func (e Executor) executeLS(definition Definition, arguments map[string]any) (ExecutionResult, error) {
	path, err := resolvePath(e.WorkingDir, stringArg(arguments, "path"))
	if err != nil {
		return ExecutionResult{}, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return ExecutionResult{}, err
	}

	limit := intArg(arguments, "limit", 200)
	lines := make([]string, 0, min(limit, len(entries)))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
		if len(lines) >= limit {
			break
		}
	}

	return ExecutionResult{
		Tool:      definition.Name,
		Arguments: arguments,
		Stdout:    SafeStdout(strings.Join(lines, "\n")+"\n", MaxStdoutChars),
		ExitCode:  0,
	}, nil
}

func (e Executor) executeFind(definition Definition, arguments map[string]any) (ExecutionResult, error) {
	root, err := resolvePath(e.WorkingDir, stringArg(arguments, "path"))
	if err != nil {
		return ExecutionResult{}, err
	}

	pattern := stringArg(arguments, "pattern")
	limit := intArg(arguments, "limit", 200)
	matches := make([]string, 0, limit)

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(e.WorkingDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		nameMatched, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			return err
		}
		relMatched, err := filepath.Match(pattern, rel)
		if err != nil {
			return err
		}
		if nameMatched || relMatched {
			if entry.IsDir() {
				rel += "/"
			}
			matches = append(matches, rel)
		}
		if len(matches) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return ExecutionResult{}, err
	}
	sort.Strings(matches)

	return ExecutionResult{
		Tool:      definition.Name,
		Arguments: arguments,
		Stdout:    SafeStdout(strings.Join(matches, "\n")+"\n", MaxStdoutChars),
		ExitCode:  0,
	}, nil
}

func (e Executor) executeGrep(definition Definition, arguments map[string]any) (ExecutionResult, error) {
	root, err := resolvePath(e.WorkingDir, stringArg(arguments, "path"))
	if err != nil {
		return ExecutionResult{}, err
	}

	pattern := stringArg(arguments, "pattern")
	if boolArg(arguments, "ignore_case", false) {
		pattern = "(?i)" + pattern
	}
	matcher, err := regexp.Compile(pattern)
	if err != nil {
		return ExecutionResult{}, err
	}

	glob := stringArg(arguments, "glob")
	files := make([]string, 0)
	info, err := os.Stat(root)
	if err != nil {
		return ExecutionResult{}, err
	}
	if !info.IsDir() {
		files = append(files, root)
	} else {
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return ExecutionResult{}, err
		}
	}

	sort.Strings(files)
	lines := make([]string, 0)
	for _, file := range files {
		rel, err := filepath.Rel(e.WorkingDir, file)
		if err != nil {
			return ExecutionResult{}, err
		}
		rel = filepath.ToSlash(rel)
		if glob != "" {
			matched, matchErr := filepath.Match(glob, rel)
			if matchErr != nil {
				return ExecutionResult{}, matchErr
			}
			if !matched {
				continue
			}
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, err
		}
		fileLines := strings.Split(string(data), "\n")
		for index, line := range fileLines {
			if matcher.MatchString(line) {
				lines = append(lines, fmt.Sprintf("%s:%d:%s", rel, index+1, line))
			}
		}
	}

	return ExecutionResult{
		Tool:      definition.Name,
		Arguments: arguments,
		Stdout:    SafeStdout(strings.Join(lines, "\n")+"\n", MaxStdoutChars),
		ExitCode:  0,
	}, nil
}

func (e Executor) executeBash(definition Definition, arguments map[string]any) (ExecutionResult, error) {
	timeout := intArg(arguments, "timeout", 30)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "bash", "-lc", stringArg(arguments, "command"))
	command.Dir = e.WorkingDir
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &bytes.Buffer{}
	err := command.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return ExecutionResult{}, err
		}
	}

	return ExecutionResult{
		Tool:      definition.Name,
		Arguments: arguments,
		Stdout:    SafeStdout(stdout.String(), MaxStdoutChars),
		ExitCode:  exitCode,
	}, nil
}

func decodeArguments(raw string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	if value == nil {
		return map[string]any{}, nil
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tool arguments must be a JSON object")
	}

	return object, nil
}

func normalizeValue(value any, expectedType string) (any, error) {
	switch expectedType {
	case "string":
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		return str, nil
	case "integer":
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			if v != float64(int(v)) {
				return nil, fmt.Errorf("must be an integer")
			}
			return int(v), nil
		case json.Number:
			i, err := v.Int64()
			if err != nil {
				return nil, fmt.Errorf("must be an integer")
			}
			return int(i), nil
		default:
			return nil, fmt.Errorf("must be an integer")
		}
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("must be a boolean")
		}
		return boolean, nil
	default:
		return value, nil
	}
}

func resolvePath(workingDir string, value string) (string, error) {
	target := workingDir
	if value != "" {
		target = filepath.Join(workingDir, value)
	}

	root, err := filepath.Abs(workingDir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("tool path escapes the working directory")
	}

	return resolved, nil
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.SplitAfter(text, "\n")
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func stringArg(arguments map[string]any, key string) string {
	if value, ok := arguments[key].(string); ok {
		return value
	}
	return ""
}

func intArg(arguments map[string]any, key string, defaultValue int) int {
	if value, ok := arguments[key].(int); ok {
		return value
	}
	return defaultValue
}

func intArgPtr(arguments map[string]any, key string) *int {
	value, ok := arguments[key].(int)
	if !ok {
		return nil
	}
	return &value
}

func boolArg(arguments map[string]any, key string, defaultValue bool) bool {
	if value, ok := arguments[key].(bool); ok {
		return value
	}
	return defaultValue
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
