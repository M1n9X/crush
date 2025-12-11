package agent

import (
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/subagent"
)

func definitionToProfile(def SubAgentDefinition) subagent.Profile {
	profile := subagent.Profile{
		Name:         def.Name,
		Description:  def.Description,
		Tools:        def.Tools,
		ToolsEnabled: def.ToolsEnabled,
		Wildcard:     def.Wildcard,
		ModelName:    def.ModelName,
		SystemPrompt: def.SystemPrompt,
		Source:       def.Source,
		Color:        def.Color,
		Mode:         def.Mode,
		MaxSteps:     def.MaxSteps,
		Permissions: subagent.PermissionMatrix{
			Edit:              def.Permissions.Edit,
			Bash:              def.Permissions.Bash,
			Webfetch:          def.Permissions.Webfetch,
			DoomLoop:          def.Permissions.DoomLoop,
			ExternalDirectory: def.Permissions.ExternalDirectory,
		},
		Builtin: def.Source == "builtin",
	}
	if profile.Mode == "" {
		profile.Mode = "subagent"
	}
	return profile
}

func definitionsToProfiles(defs []SubAgentDefinition) []subagent.Profile {
	profiles := make([]subagent.Profile, 0, len(defs))
	for _, def := range defs {
		profiles = append(profiles, definitionToProfile(def))
	}
	return profiles
}

// deriveAllowedTools merges the base allowlist with profile constraints and toggles.
func deriveAllowedTools(base []string, profile *subagent.Profile) []string {
	if profile == nil {
		return base
	}
	allowed := make(map[string]struct{}, len(base))
	for _, tool := range base {
		allowed[tool] = struct{}{}
	}

	if !profile.Wildcard && len(profile.Tools) > 0 {
		keep := make(map[string]struct{}, len(profile.Tools))
		for _, tool := range profile.Tools {
			if _, ok := allowed[tool]; ok {
				keep[tool] = struct{}{}
			}
		}
		allowed = keep
	}

	for tool, enabled := range profile.ToolsEnabled {
		if enabled {
			allowed[tool] = struct{}{}
		} else {
			delete(allowed, tool)
		}
	}

	result := make([]string, 0, len(allowed))
	for tool := range allowed {
		result = append(result, tool)
	}
	slices.Sort(result)
	return result
}

func prefersCodex(profile subagent.Profile) bool {
	model := strings.ToLower(strings.TrimSpace(profile.ModelName))
	return strings.HasPrefix(model, "codex")
}

func isSubagentMode(profile subagent.Profile) bool {
	mode := strings.ToLower(strings.TrimSpace(profile.Mode))
	if mode == "" || mode == "subagent" || mode == "all" {
		return true
	}
	return false
}

// deriveSandbox picks an execution sandbox based on the profile permissions.
// Default: read-only; allow/ask edits -> workspace-write; explicit danger/full -> danger-full-access.
func deriveSandbox(profile *subagent.Profile) subagent.SandboxMode {
	if profile == nil {
		return subagent.SandboxReadOnly
	}
	switch strings.ToLower(strings.TrimSpace(profile.Permissions.Edit)) {
	case "allow", "ask":
		return subagent.SandboxWorkspaceWrite
	case "danger", "full":
		return subagent.SandboxDangerFullAccess
	default:
		return subagent.SandboxReadOnly
	}
}
