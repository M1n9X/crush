// Package workflow provides safety hooks for dangerous operation detection.
package workflow

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/crush/internal/subagent"
)

// SafetyConfig defines configuration for safety checks.
type SafetyConfig struct {
	// EnableCodexFullAuto when true allows Codex full-auto mode without approval.
	// Default is false (always require approval for full-auto).
	EnableCodexFullAuto bool `json:"enable_codex_full_auto"`

	// CustomPatterns allows adding custom dangerous patterns.
	CustomPatterns []DangerousPattern `json:"custom_patterns,omitempty"`
}

// DefaultSafetyConfig returns the default safety configuration.
func DefaultSafetyConfig() SafetyConfig {
	return SafetyConfig{
		EnableCodexFullAuto: false, // Default: always require approval
	}
}

// DangerousPattern defines a pattern for dangerous operations.
type DangerousPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"` // Regex pattern
	Description string `json:"description"`
	Severity    string `json:"severity"` // "high", "medium", "low"
}

// SafetyCheck represents the result of a safety evaluation.
type SafetyCheck struct {
	IsDangerous  bool              `json:"is_dangerous"`
	Violations   []SafetyViolation `json:"violations,omitempty"`
	RequiresGate bool              `json:"requires_gate"`
	GateType     string            `json:"gate_type,omitempty"` // "approval", "confirmation"
}

// SafetyViolation describes a detected dangerous operation.
type SafetyViolation struct {
	PatternName string `json:"pattern_name"`
	Match       string `json:"match"`
	Severity    string `json:"severity"`
	Location    string `json:"location,omitempty"`
}

// SafetyService detects dangerous operations and gates them through approval.
type SafetyService struct {
	config            SafetyConfig
	dangerousPatterns []DangerousPattern
	compiledPatterns  []*regexp.Regexp
}

// NewSafetyService creates a new SafetyService with default patterns.
func NewSafetyService(cfg SafetyConfig) *SafetyService {
	patterns := DefaultDangerousPatterns()
	patterns = append(patterns, cfg.CustomPatterns...)

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p.Pattern); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &SafetyService{
		config:            cfg,
		dangerousPatterns: patterns,
		compiledPatterns:  compiled,
	}
}

// DefaultDangerousPatterns returns the built-in dangerous command patterns.
func DefaultDangerousPatterns() []DangerousPattern {
	return []DangerousPattern{
		// High severity - destructive commands
		{
			Name:        "rm_rf_root",
			Pattern:     `rm\s+-rf\s+/(?:[^/]|$)`,
			Description: "Recursive force delete from root",
			Severity:    "high",
		},
		{
			Name:        "sudo_rm",
			Pattern:     `sudo\s+rm`,
			Description: "Privileged file deletion",
			Severity:    "high",
		},
		{
			Name:        "chmod_777",
			Pattern:     `chmod\s+777`,
			Description: "World-writable permissions",
			Severity:    "high",
		},
		{
			Name:        "curl_pipe_sh",
			Pattern:     `curl\s+.*\|\s*(?:ba)?sh`,
			Description: "Pipe remote content to shell",
			Severity:    "high",
		},
		{
			Name:        "wget_pipe_sh",
			Pattern:     `wget\s+.*\|\s*(?:ba)?sh`,
			Description: "Pipe remote content to shell",
			Severity:    "high",
		},
		// Medium severity - system modification
		{
			Name:        "dd_if",
			Pattern:     `dd\s+if=`,
			Description: "Raw disk operations",
			Severity:    "medium",
		},
		{
			Name:        "mkfs",
			Pattern:     `mkfs\.`,
			Description: "Filesystem creation",
			Severity:    "medium",
		},
		{
			Name:        "fdisk",
			Pattern:     `fdisk\s+`,
			Description: "Disk partitioning",
			Severity:    "medium",
		},
		{
			Name:        "format_disk",
			Pattern:     `format\s+/dev/`,
			Description: "Disk formatting",
			Severity:    "medium",
		},
		// Low severity - potentially risky
		{
			Name:        "sudo_command",
			Pattern:     `^sudo\s+`,
			Description: "Privileged command execution",
			Severity:    "low",
		},
	}
}

// SensitiveFilePaths returns patterns for sensitive file paths.
func SensitiveFilePaths() []string {
	return []string{
		".env",
		".ssh/",
		".aws/",
		".gnupg/",
		"id_rsa",
		"id_ed25519",
		".netrc",
		".npmrc",
		".pypirc",
		"credentials",
		"secrets",
		"password",
		"api_key",
		"private_key",
	}
}

// EvaluateCommand checks a command for dangerous patterns.
func (ss *SafetyService) EvaluateCommand(cmd string) *SafetyCheck {
	check := &SafetyCheck{
		IsDangerous: false,
	}

	for i, pattern := range ss.compiledPatterns {
		if pattern == nil {
			continue
		}
		if matches := pattern.FindStringSubmatch(cmd); len(matches) > 0 {
			check.IsDangerous = true
			check.Violations = append(check.Violations, SafetyViolation{
				PatternName: ss.dangerousPatterns[i].Name,
				Match:       matches[0],
				Severity:    ss.dangerousPatterns[i].Severity,
				Location:    "command",
			})
		}
	}

	if check.IsDangerous {
		check.RequiresGate = true
		check.GateType = "approval"
	}

	return check
}

// EvaluateFileChange checks file changes for sensitive paths.
func (ss *SafetyService) EvaluateFileChange(path string, isDelete bool) *SafetyCheck {
	check := &SafetyCheck{
		IsDangerous: false,
	}

	pathLower := strings.ToLower(path)
	for _, sensitive := range SensitiveFilePaths() {
		if strings.Contains(pathLower, strings.ToLower(sensitive)) {
			severity := "medium"
			if isDelete {
				severity = "high"
			}

			check.IsDangerous = true
			check.Violations = append(check.Violations, SafetyViolation{
				PatternName: "sensitive_file",
				Match:       sensitive,
				Severity:    severity,
				Location:    path,
			})
		}
	}

	if check.IsDangerous {
		check.RequiresGate = true
		check.GateType = "confirmation"
	}

	return check
}

// EvaluateCodexMode checks if Codex sandbox mode requires approval.
func (ss *SafetyService) EvaluateCodexMode(sandboxMode subagent.SandboxMode) *SafetyCheck {
	check := &SafetyCheck{
		IsDangerous: false,
	}

	// Full access mode is dangerous unless explicitly enabled
	if sandboxMode == subagent.SandboxDangerFullAccess {
		if !ss.config.EnableCodexFullAuto {
			check.IsDangerous = true
			check.RequiresGate = true
			check.GateType = "approval"
			check.Violations = append(check.Violations, SafetyViolation{
				PatternName: "codex_full_auto",
				Match:       string(sandboxMode),
				Severity:    "high",
				Location:    "sandbox_mode",
			})
		}
	}

	return check
}

// EvaluateSandbox is an alias for EvaluateCodexMode for clearer API.
func (ss *SafetyService) EvaluateSandbox(sandboxMode subagent.SandboxMode) *SafetyCheck {
	return ss.EvaluateCodexMode(sandboxMode)
}

// IsSafe returns true if no dangerous patterns were detected.
func (c *SafetyCheck) IsSafe() bool {
	return !c.IsDangerous
}

// HighestSeverity returns the highest severity level among violations.
func (c *SafetyCheck) HighestSeverity() string {
	if len(c.Violations) == 0 {
		return ""
	}

	severityOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
	highest := ""
	highestScore := 0

	for _, v := range c.Violations {
		if score := severityOrder[v.Severity]; score > highestScore {
			highestScore = score
			highest = v.Severity
		}
	}

	return highest
}
