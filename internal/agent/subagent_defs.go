package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// SubAgentDefinition models a user-defined subagent profile.
type SubAgentDefinition struct {
	Name         string
	Description  string
	Tools        []string
	Wildcard     bool
	ModelName    string
	SystemPrompt string
	Source       string
	Color        string
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
	merged := map[string]SubAgentDefinition{
		"general-purpose": {
			Name:         "general-purpose",
			Description:  "General-purpose subagent for quick reconnaissance and lookups.",
			Wildcard:     true,
			SystemPrompt: "You are a general-purpose subagent. Stay concise and use only the tools provided to complete the task.",
			Source:       "builtin",
		},
	}

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
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tools       interface{} `yaml:"tools"`
	ModelName   string      `yaml:"model_name"`
	Model       string      `yaml:"model"` // deprecated alias for model_name
	Color       string      `yaml:"color"`
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

	tools, wildcard := parseToolsField(fm.Tools)

	// Handle deprecated 'model' field as fallback for 'model_name'
	modelName := strings.TrimSpace(fm.ModelName)
	if modelName == "" && fm.Model != "" {
		modelName = strings.TrimSpace(fm.Model)
	}

	return SubAgentDefinition{
		Name:         strings.TrimSpace(fm.Name),
		Description:  strings.TrimSpace(fm.Description),
		Tools:        tools,
		Wildcard:     wildcard,
		ModelName:    modelName,
		SystemPrompt: promptBody,
		Source:       source,
		Color:        strings.TrimSpace(fm.Color),
	}, nil
}

func parseToolsField(v interface{}) ([]string, bool) {
	switch t := v.(type) {
	case nil:
		return nil, true
	case string:
		if strings.TrimSpace(t) == "*" {
			return nil, true
		}
		if strings.TrimSpace(t) == "" {
			return nil, true
		}
		return []string{strings.TrimSpace(t)}, false
	case []interface{}:
		var res []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				if strings.TrimSpace(s) == "*" {
					return nil, true
				}
				res = append(res, strings.TrimSpace(s))
			}
		}
		if len(res) == 0 {
			return nil, true
		}
		return res, false
	default:
		return nil, true
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
