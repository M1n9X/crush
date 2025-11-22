package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/util"
)

var agentNamePattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// NewAddSubAgentDialog opens a simple form to create a subagent definition.
func NewAddSubAgentDialog() dialogs.DialogModel {
	args := []Argument{
		{Name: "name", Title: "Name", Description: "kebab-case identifier", Required: true},
		{Name: "description", Title: "Description", Description: "When to use this agent", Required: true},
		{Name: "tools", Title: "Tools", Description: "Comma-separated list (optional)"},
		{Name: "model_name", Title: "Model", Description: "Optional model override"},
		{Name: "location", Title: "Location", Description: "project or user (default: project)"},
		{Name: "prompt", Title: "System Prompt", Description: "Custom prompt (optional)"},
	}

	return NewCommandArgumentsDialog(
		"add_subagent",
		"Add Subagent",
		"Add Subagent",
		"Create a subagent config in .crush/agents",
		args,
		func(vals map[string]string) tea.Cmd {
			return createSubAgent(vals)
		},
	)
}

func createSubAgent(vals map[string]string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.Get()
		if cfg == nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: "config not loaded"}
		}

		name := sanitizeAgentName(vals["name"])
		if name == "" {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: "agent name is required"}
		}

		description := strings.TrimSpace(vals["description"])
		if description == "" {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: "description is required"}
		}

		location := strings.ToLower(strings.TrimSpace(vals["location"]))
		baseDir := cfg.WorkingDir()
		if location == "user" {
			if home := home.Dir(); home != "" {
				baseDir = home
			}
		}

		targetDir := filepath.Join(baseDir, ".crush", "agents")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("create dir: %v", err)}
		}

		targetPath := filepath.Join(targetDir, fmt.Sprintf("%s.md", name))
		if _, err := os.Stat(targetPath); err == nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("subagent %q already exists", name)}
		}

		tools := parseTools(vals["tools"])
		model := strings.TrimSpace(vals["model_name"])
		prompt := strings.TrimSpace(vals["prompt"])
		if prompt == "" {
			prompt = "You are a specialized subagent. Follow your allowed tools and stay concise."
		}

		content, err := buildSubAgentFile(name, description, tools, model, prompt)
		if err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
		}

		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("write file: %v", err)}
		}

		agent.ClearSubAgentCache()

		return util.InfoMsg{Type: util.InfoTypeSuccess, Msg: fmt.Sprintf("Created subagent at %s", targetPath)}
	}
}

func parseTools(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tools []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tools = append(tools, p)
		}
	}
	return tools
}

func sanitizeAgentName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = agentNamePattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

func buildSubAgentFile(name, description string, tools []string, model, prompt string) (string, error) {
	fm := map[string]any{
		"name":        name,
		"description": description,
	}

	if len(tools) == 0 {
		fm["tools"] = "*"
	} else {
		fm["tools"] = tools
	}

	if model != "" {
		fm["model_name"] = model
	}

	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	body := prompt
	if !strings.HasPrefix(body, "\n") {
		body = "\n" + body
	}

	return fmt.Sprintf("---\n%s---%s\n", string(data), body), nil
}

// HandleSlashAgent builds a command for the /agent shortcut.
func HandleSlashAgent(args string) tea.Cmd {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: NewAddSubAgentDialog(),
		})
	}

	parts := strings.Fields(trimmed)
	name := ""
	desc := ""
	if len(parts) > 0 {
		name = parts[0]
		desc = strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))
	}

	return createSubAgent(map[string]string{
		"name":        name,
		"description": desc,
	})
}
