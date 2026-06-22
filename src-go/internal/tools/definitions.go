package tools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

type Definition struct {
	Name             string
	Label            string
	Description      string
	Parameters       map[string]any
	PromptSnippet    string
	PromptGuidelines []string
}

var DefaultToolNames = []string{"read", "grep", "find", "ls"}

var presetTools = map[string]Definition{
	"read": {
		Name:             "read",
		Label:            "read",
		Description:      "Read the contents of a text file. Output is truncated for large files.",
		PromptSnippet:    "Read file contents",
		PromptGuidelines: []string{"Use read to examine files instead of cat or sed."},
		Parameters: objectSchema(map[string]any{
			"path":   map[string]any{"type": "string", "description": "Path to the file to read."},
			"offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based line offset."},
			"limit":  map[string]any{"type": "integer", "minimum": 1, "description": "Maximum number of lines to return."},
		}, []string{"path"}),
	},
	"grep": {
		Name:             "grep",
		Label:            "grep",
		Description:      "Search file contents for a pattern. Returns matching paths and lines.",
		PromptSnippet:    "Search file contents for patterns",
		PromptGuidelines: []string{"Use grep for content search."},
		Parameters: objectSchema(map[string]any{
			"pattern":     map[string]any{"type": "string", "description": "Text or regex pattern to search for."},
			"path":        map[string]any{"type": "string", "description": "Directory or file to search."},
			"glob":        map[string]any{"type": "string", "description": "Optional glob filter."},
			"ignore_case": map[string]any{"type": "boolean", "description": "Search case-insensitively."},
		}, []string{"pattern"}),
	},
	"find": {
		Name:             "find",
		Label:            "find",
		Description:      "Find files by glob pattern. Returns paths relative to the working directory.",
		PromptSnippet:    "Find files by glob pattern",
		PromptGuidelines: []string{"Use find for filename or path discovery."},
		Parameters: objectSchema(map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Glob pattern to match."},
			"path":    map[string]any{"type": "string", "description": "Directory to search from."},
			"limit":   map[string]any{"type": "integer", "minimum": 1, "description": "Maximum number of paths to return."},
		}, []string{"pattern"}),
	},
	"ls": {
		Name:          "ls",
		Label:         "ls",
		Description:   "List directory contents.",
		PromptSnippet: "List directory contents",
		Parameters: objectSchema(map[string]any{
			"path":  map[string]any{"type": "string", "description": "Directory to list."},
			"limit": map[string]any{"type": "integer", "minimum": 1, "description": "Maximum number of entries to return."},
		}, []string{}),
	},
	"bash": {
		Name:             "bash",
		Label:            "bash",
		Description:      "Execute a bash command in the current working directory. Returns sanitized stdout.",
		PromptSnippet:    "Execute bash commands",
		PromptGuidelines: []string{"Use bash only when read, grep, find, or ls cannot answer the request."},
		Parameters: objectSchema(map[string]any{
			"command": map[string]any{"type": "string", "description": "Bash command to execute."},
			"timeout": map[string]any{"type": "integer", "minimum": 1, "description": "Timeout in seconds."},
		}, []string{"command"}),
	},
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func GetDefinitions(names []string) ([]Definition, error) {
	selected := names
	if selected == nil {
		selected = DefaultToolNames
	}

	definitions := make([]Definition, 0, len(selected))
	for _, name := range selected {
		definition, ok := presetTools[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool(s): %s", name)
		}
		definitions = append(definitions, definition)
	}

	return definitions, nil
}

func ToToolInfos(definitions []Definition) ([]*schema.ToolInfo, error) {
	infos := make([]*schema.ToolInfo, 0, len(definitions))
	for _, definition := range definitions {
		jsonSchema, err := toJSONSchema(definition.Parameters)
		if err != nil {
			return nil, err
		}

		infos = append(infos, &schema.ToolInfo{
			Name:        definition.Name,
			Desc:        definition.Description,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(jsonSchema),
		})
	}

	return infos, nil
}

func BuildSystemPrompt(definitions []Definition, workingDir string, currentDate string) string {
	visibleTools := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if definition.PromptSnippet != "" {
			visibleTools = append(visibleTools, "- "+definition.Name+": "+definition.PromptSnippet)
		}
	}
	availableTools := "(none)"
	if len(visibleTools) > 0 {
		availableTools = strings.Join(visibleTools, "\n")
	}

	guidelines := make([]string, 0, 8)
	seen := map[string]bool{}
	addGuideline := func(text string) {
		normalized := strings.TrimSpace(text)
		if normalized == "" || seen[normalized] {
			return
		}
		seen[normalized] = true
		guidelines = append(guidelines, normalized)
	}

	for _, definition := range definitions {
		for _, line := range definition.PromptGuidelines {
			addGuideline(line)
		}
	}
	addGuideline("Be concise.")
	addGuideline("Show file paths clearly when working with files.")

	promptDate := currentDate
	if promptDate == "" {
		promptDate = time.Now().Format("2006-01-02")
	}
	promptCWD := filepath.ToSlash(workingDir)

	formattedGuidelines := make([]string, 0, len(guidelines))
	for _, guideline := range guidelines {
		formattedGuidelines = append(formattedGuidelines, "- "+guideline)
	}

	return fmt.Sprintf(`You are an expert CLI assistant operating inside cli-llm, a lightweight tool-call harness.
Use the available tools when they are the safest and most direct way to answer the user.

Available tools:
%s

Tool call rules:
- Call at most one tool.
- Use only the provided tools.
- Do not invent tool names or arguments.
- Prefer read/grep/find/ls over bash for file inspection.
- If no tool is needed, answer normally without a tool call.
- Do not explain the tool call in prose when calling a tool.

Guidelines:
%s

Current date: %s
Current working directory: %s`, availableTools, strings.Join(formattedGuidelines, "\n"), promptDate, promptCWD)
}

func toJSONSchema(parameters map[string]any) (*jsonschema.Schema, error) {
	data, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}

	var schemaValue jsonschema.Schema
	if err := json.Unmarshal(data, &schemaValue); err != nil {
		return nil, err
	}

	return &schemaValue, nil
}
