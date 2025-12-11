package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/charmbracelet/crush/internal/message"
)

// Capability represents an ability or role a subagent can fulfil.
type Capability string

// SandboxMode controls the execution sandbox granted to a subagent.
type SandboxMode string

const (
	SandboxReadOnly         SandboxMode = "read-only"
	SandboxWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxDangerFullAccess SandboxMode = "danger-full-access"
)

// Profile is the runtime representation of a subagent definition.
type Profile struct {
	Name         string
	Description  string
	Tools        []string
	ToolsEnabled map[string]bool
	Wildcard     bool
	ModelName    string
	SystemPrompt string
	Source       string
	Color        string
	Mode         string
	MaxSteps     int
	Permissions  PermissionMatrix
	Builtin      bool
}

// PermissionMatrix mirrors the permissive/ask/deny structure used by opencode.
type PermissionMatrix struct {
	Edit              string            `json:"edit,omitempty"`
	Bash              map[string]string `json:"bash,omitempty"`
	Webfetch          string            `json:"webfetch,omitempty"`
	DoomLoop          string            `json:"doom_loop,omitempty"`
	ExternalDirectory string            `json:"external_directory,omitempty"`
}

// Request describes a call into a subagent adapter.
type Request struct {
	Task        string          // User or upstream instruction.
	ContextJSON json.RawMessage // Compact context payload (diffs, plan, history).
	Files       []message.Attachment
	Sandbox     SandboxMode
	Timeout     time.Duration
	Metadata    map[string]string // Workflow/step/session metadata.
	Profile     *Profile          // The resolved subagent profile.
	SessionID   string            // Session the subagent should write to.
	ParentID    string            // Parent session (for cost propagation).
	ModelName   string            // Optional override for the model to run.
	Description string            // Optional short description for logging.
}

// Result captures the structured output of a subagent invocation.
type Result struct {
	Text         string
	Artifacts    []Artifact
	ResumeToken  string
	Usage        Usage
	Observations []string
	IsRetryable  bool
	ErrorMessage string
}

// Usage records resource consumption for audit and cost tracking.
type Usage struct {
	PromptTokens     int64         `json:"prompt_tokens"`
	CompletionTokens int64         `json:"completion_tokens"`
	TotalTokens      int64         `json:"total_tokens"`
	CostUSD          float64       `json:"cost_usd"`
	Duration         time.Duration `json:"duration"`
	ModelName        string        `json:"model_name,omitempty"`
}

// Artifact represents structured content emitted by a subagent.
type Artifact struct {
	Type    string          `json:"type"` // "patch", "diff", "summary", "plan", "code"
	Name    string          `json:"name"`
	Content json.RawMessage `json:"content"`
}

// Subagent is the unified execution interface for Claude Code, Codex, and future adapters.
type Subagent interface {
	Name() string
	Capabilities() []Capability
	SupportsResume() bool

	Execute(ctx context.Context, req Request) (*Result, error)
	Resume(ctx context.Context, resumeToken string, req Request) (*Result, error)
	ExecuteStreamed(ctx context.Context, req Request, handler func(Event)) (*Result, error)
}

// Event captures streaming updates emitted by a subagent implementation.
type Event struct {
	Type     string
	Payload  any
	Provider string
}

var (
	// ErrResumeNotSupported is returned when Resume is called on a subagent
	// that does not support it.
	ErrResumeNotSupported = errors.New("subagent does not support resume")
	// ErrMissingProfile is returned when a request is missing the resolved profile.
	ErrMissingProfile = errors.New("subagent profile is required")
	// ErrStreamNotSupported is returned when streaming is unsupported.
	ErrStreamNotSupported = errors.New("subagent does not support streaming")
)
