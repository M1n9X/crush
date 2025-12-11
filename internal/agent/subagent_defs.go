package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// SubAgentPermissions models the permission matrix attached to a subagent definition.
type SubAgentPermissions struct {
	Edit              string            `yaml:"edit" json:"edit"`
	Bash              map[string]string `yaml:"bash" json:"bash"`
	Webfetch          string            `yaml:"webfetch" json:"webfetch"`
	DoomLoop          string            `yaml:"doom_loop" json:"doom_loop"`
	ExternalDirectory string            `yaml:"external_directory" json:"external_directory"`
}

// SubAgentDefinition models a user-defined subagent profile.
type SubAgentDefinition struct {
	Name        string
	Description string
	Tools       []string
	// ToolsEnabled captures explicit tool toggles (true/false) from frontmatter maps.
	ToolsEnabled map[string]bool
	Wildcard     bool
	ModelName    string
	SystemPrompt string
	Source       string
	Color        string
	// Mode controls where the agent can be used: subagent, primary, or all.
	Mode string
	// MaxSteps is an advisory limit for how many steps the agent should perform.
	MaxSteps int
	// Permissions captures the permission matrix from frontmatter.
	Permissions SubAgentPermissions
}

// loadSubAgentDefinitions loads agent definitions from priority-ordered locations
// and returns the merged set (later sources override earlier ones).
func loadSubAgentDefinitions(workingDir string) ([]SubAgentDefinition, error) {
	home, _ := os.UserHomeDir()

	type src struct {
		dir    string
		source string
	}

	paths := []src{
		{dir: filepath.Join(home, ".claude", "agents"), source: "home-.claude"},
		{dir: filepath.Join(home, ".codebreeze", "agents"), source: "home-.codebreeze"},
		{dir: filepath.Join(home, ".crush", "agents"), source: "home-.crush"},
		{dir: filepath.Join(workingDir, ".claude", "agents"), source: "project-.claude"},
		{dir: filepath.Join(workingDir, ".codebreeze", "agents"), source: "project-.codebreeze"},
		{dir: filepath.Join(workingDir, ".crush", "agents"), source: "project-.crush"},
	}

	// builtin fallback
	merged := builtinSubAgentDefinitions()

	for _, p := range paths {
		files, err := os.ReadDir(p.dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			path := filepath.Join(p.dir, f.Name())
			def, err := parseSubAgentDefinition(path, p.source)
			if err != nil {
				continue
			}
			merged[def.Name] = def
		}
	}

	var result []SubAgentDefinition
	for _, def := range merged {
		result = append(result, def)
	}
	slices.SortFunc(result, func(a, b SubAgentDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result, nil
}

type frontmatter struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Tools       interface{}         `yaml:"tools"`
	ModelName   string              `yaml:"model_name"`
	Model       string              `yaml:"model"` // deprecated alias for model_name
	Color       string              `yaml:"color"`
	Mode        string              `yaml:"mode"`
	MaxSteps    int                 `yaml:"max_steps"`
	Permission  SubAgentPermissions `yaml:"permission"`
	Permissions SubAgentPermissions `yaml:"permissions"`
}

func parseSubAgentDefinition(path, source string) (SubAgentDefinition, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return SubAgentDefinition{}, err
	}
	content := strings.TrimLeft(string(body), "\ufeff\r\n\t ")
	if !strings.HasPrefix(content, "---") {
		return SubAgentDefinition{}, fmt.Errorf("missing frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return SubAgentDefinition{}, fmt.Errorf("invalid frontmatter")
	}

	meta := parts[1]
	promptBody := strings.TrimSpace(parts[2])

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(meta), &fm); err != nil {
		return SubAgentDefinition{}, err
	}

	if fm.Name == "" || fm.Description == "" {
		return SubAgentDefinition{}, fmt.Errorf("%s: missing required fields (name or description)", path)
	}

	tools, wildcard, toggles := parseToolsField(fm.Tools)

	// Handle deprecated 'model' field as fallback for 'model_name'
	modelName := strings.TrimSpace(fm.ModelName)
	if modelName == "" && fm.Model != "" {
		modelName = strings.TrimSpace(fm.Model)
	}

	mode := strings.TrimSpace(fm.Mode)
	if mode == "" {
		mode = "subagent"
	}

	permissions := fm.Permission
	if isZeroPermissions(permissions) {
		permissions = fm.Permissions
	}

	return SubAgentDefinition{
		Name:         strings.TrimSpace(fm.Name),
		Description:  strings.TrimSpace(fm.Description),
		Tools:        tools,
		ToolsEnabled: toggles,
		Wildcard:     wildcard,
		ModelName:    modelName,
		SystemPrompt: promptBody,
		Source:       source,
		Color:        strings.TrimSpace(fm.Color),
		Mode:         mode,
		MaxSteps:     fm.MaxSteps,
		Permissions:  permissions,
	}, nil
}

func parseToolsField(v interface{}) ([]string, bool, map[string]bool) {
	switch t := v.(type) {
	case nil:
		return nil, true, nil
	case string:
		if strings.TrimSpace(t) == "*" {
			return nil, true, nil
		}
		if strings.TrimSpace(t) == "" {
			return nil, true, nil
		}
		return []string{strings.TrimSpace(t)}, false, nil
	case []interface{}:
		var res []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				if strings.TrimSpace(s) == "*" {
					return nil, true, nil
				}
				res = append(res, strings.TrimSpace(s))
			}
		}
		if len(res) == 0 {
			return nil, true, nil
		}
		return res, false, nil
	case map[string]interface{}:
		toggles := make(map[string]bool)
		for key, raw := range t {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			enabled := true
			switch val := raw.(type) {
			case bool:
				enabled = val
			case string:
				s := strings.TrimSpace(val)
				if s == "" {
					enabled = true
				} else {
					enabled = strings.EqualFold(s, "true") || strings.EqualFold(s, "yes") || s == "1"
				}
			default:
				enabled = true
			}
			toggles[name] = enabled
		}
		var res []string
		for name, enabled := range toggles {
			if enabled {
				res = append(res, name)
			}
		}
		wildcard := len(toggles) == 0
		if len(res) == 0 && !wildcard {
			wildcard = true
		}
		return res, wildcard, toggles
	default:
		return nil, true, nil
	}
}

func isZeroPermissions(p SubAgentPermissions) bool {
	return p.Edit == "" && len(p.Bash) == 0 && p.Webfetch == "" && p.DoomLoop == "" && p.ExternalDirectory == ""
}

func builtinSubAgentDefinitions() map[string]SubAgentDefinition {
	generalPrompt := "You are a general-purpose subagent. Stay concise and use only the tools provided to complete the task."
	explorePrompt := strings.TrimSpace(`
You are a file search specialist. You excel at rapidly navigating and exploring codebases.

Guidelines:
- Use glob for broad file pattern matching.
- Use grep/rg for searching file contents with regex.
- Use read/view when you know the specific file path you need to read.
- Do not create files or run commands that modify the user's system state.
- Return file paths as absolute paths when reporting findings.
`)
	planPrompt := strings.TrimSpace(`
You are a planning-focused subagent. Create concise, safe, and verifiable plans.
- Produce numbered steps.
- Avoid making filesystem changes or running risky shell commands.
- Keep the plan minimal but sufficient to unblock the parent agent.
`)

	return map[string]SubAgentDefinition{
		"general-purpose": {
			Name:         "general-purpose",
			Description:  "General-purpose subagent for quick reconnaissance and lookups.",
			Wildcard:     true,
			SystemPrompt: generalPrompt,
			Source:       "builtin",
			Mode:         "subagent",
		},
		"general": {
			Name:         "general",
			Description:  "General-purpose subagent for multi-step tasks and quick research.",
			Wildcard:     true,
			SystemPrompt: generalPrompt,
			Source:       "builtin",
			Mode:         "subagent",
		},
		"explore": {
			Name:         "explore",
			Description:  "Fast, read-only subagent for searching and reading code.",
			Wildcard:     true,
			ToolsEnabled: map[string]bool{"write": false, "edit": false},
			SystemPrompt: explorePrompt,
			Source:       "builtin",
			Mode:         "subagent",
		},
		"plan": {
			Name:         "plan",
			Description:  "Planning-first subagent that drafts concise, reviewable plans.",
			Wildcard:     true,
			ToolsEnabled: map[string]bool{"write": false, "edit": false},
			SystemPrompt: planPrompt,
			Source:       "builtin",
			Mode:         "all",
			MaxSteps:     0,
			Permissions: SubAgentPermissions{
				Edit: "deny",
			},
		},
	}
}

func formatSubAgentDefinitions(defs []SubAgentDefinition) string {
	lines := make([]string, 0, len(defs))
	for _, def := range defs {
		tools := "*"
		if !def.Wildcard && len(def.Tools) > 0 {
			tools = strings.Join(def.Tools, ", ")
		}
		description := strings.TrimSpace(def.Description)
		if description == "" {
			description = "No description provided"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (Tools: %s)", def.Name, description, tools))
	}
	return strings.Join(lines, "\n")
}
