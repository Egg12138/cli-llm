package prompts

import (
	_ "embed"
	"encoding/json"
)

type SystemPrompt struct {
	Content     string  `json:"content"`
	Description string  `json:"description"`
	Temperature float32 `json:"temperature"`
}

type RoleRegistry struct {
	roles map[string]SystemPrompt
}

//go:embed system_prompts.json
var embeddedSystemPrompts []byte

func NewRoleRegistry() RoleRegistry {
	roles := defaultRoleSet()
	if loaded, err := loadRoleSet(embeddedSystemPrompts); err == nil {
		roles = loaded
	}
	backfillRoleAliases(roles)

	return RoleRegistry{
		roles: roles,
	}
}

func loadRoleSet(raw []byte) (map[string]SystemPrompt, error) {
	var roles map[string]SystemPrompt
	if err := json.Unmarshal(raw, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func backfillRoleAliases(roles map[string]SystemPrompt) {
	if _, ok := roles["normal"]; !ok {
		if chatRole, ok := roles["chat"]; ok {
			roles["normal"] = chatRole
		}
	}
	if _, ok := roles["general"]; !ok {
		if smartRole, ok := roles["smart"]; ok {
			roles["general"] = smartRole
		}
	}
	if _, ok := roles["smart"]; !ok {
		if generalRole, ok := roles["general"]; ok {
			roles["smart"] = generalRole
		}
	}
}

func defaultRoleSet() map[string]SystemPrompt {
	return map[string]SystemPrompt{
		"coder": {
			Content: `{
    "ROLE": "You are a helpful programmer assistant, well skilled in GNU/Linux development."
}`,
			Description: "Detailed programming assistant prompt with specific command handling",
			Temperature: 0.0,
		},
		"chat": {
			Content:     "You are a helpful assistant. You will answer my question in details instead of making a short summary. You have to explain the components and crucial concepts.\nYour output should consist of the answer and reference. \nEach code block should be closed to 3 empty lines",
			Description: "General purpose chat assistant with detailed explanations",
			Temperature: 1.3,
		},
		"creative": {
			Content:     "You are a creative AI.",
			Description: "Creative AI assistant for imaginative tasks",
			Temperature: 1.5,
		},
		"smart": {
			Content:     "You are a smart AI assistant",
			Description: "Simple and efficient AI assistant for general tasks",
			Temperature: 0.6,
		},
		"general": {
			Content:     "You are a smart AI assistant",
			Description: "Simple and efficient AI assistant for general tasks",
			Temperature: 0.6,
		},
		"normal": {
			Content:     "You are a helpful assistant. You will answer my question in details instead of making a short summary. You have to explain the components and crucial concepts.\nYour output should consist of the answer and reference. \nEach code block should be closed to 3 empty lines",
			Description: "General purpose chat assistant with detailed explanations",
			Temperature: 1.3,
		},
		"meta": {
			Content:     "You now act as a prompt generator using a Best-of-N methodology and must output compliant YAML only.",
			Description: "Meta-prompt generator that creates optimized prompts using Best-of-N sampling methodology",
			Temperature: 0.8,
		},
	}
}

func (r RoleRegistry) Resolve(name string, fallback string) (string, SystemPrompt, string) {
	if role, ok := r.roles[name]; ok {
		return name, role, ""
	}

	if fallbackRole, ok := r.roles[fallback]; ok {
		return fallback, fallbackRole, "role " + name + " is not predefined, using " + fallback
	}

	defaultRole := r.roles["coder"]
	return "coder", defaultRole, "role " + name + " is not predefined, using coder"
}
