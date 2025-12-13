package workflow

import (
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidSpec is returned when a workflow spec is invalid.
	ErrInvalidSpec = errors.New("invalid workflow spec")
	// ErrCycleDetected is returned when a cycle is detected in the DAG.
	ErrCycleDetected = errors.New("cycle detected in workflow DAG")
	// ErrNoTerminalSuccess is returned when no terminal success node exists.
	ErrNoTerminalSuccess = errors.New("workflow spec must have a terminal success node")
	// ErrNoStartNode is returned when no start node can be determined.
	ErrNoStartNode = errors.New("cannot determine start node in workflow")
	// ErrNodeNotFound is returned when a referenced node doesn't exist.
	ErrNodeNotFound = errors.New("node not found in workflow spec")
)

// WorkflowSpec represents a parsed workflow specification (DAG or sequential).
type WorkflowSpec struct {
	Version     int                    `yaml:"version" json:"version"`
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	Vars        map[string]interface{} `yaml:"vars" json:"vars"`
	Nodes       []NodeSpec             `yaml:"nodes" json:"nodes"`
}

// NodeSpec defines a single node in the workflow DAG.
type NodeSpec struct {
	ID               string      `yaml:"id" json:"id"`
	Name             string      `yaml:"name" json:"name"`
	Capabilities     []string    `yaml:"capabilities" json:"capabilities"`
	PreferredAgents  []string    `yaml:"preferred_agents" json:"preferred_agents"`
	Input            *InputSpec  `yaml:"input,omitempty" json:"input,omitempty"`
	Outputs          *OutputSpec `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	RequiresApproval bool        `yaml:"requires_approval" json:"requires_approval"`
	Terminal         string      `yaml:"terminal,omitempty" json:"terminal,omitempty"` // "success" or "fail"

	// Transitions (edges)
	Next      []string   `yaml:"next,omitempty" json:"next,omitempty"`
	OnApprove []string   `yaml:"on_approve,omitempty" json:"on_approve,omitempty"`
	OnReject  []string   `yaml:"on_reject,omitempty" json:"on_reject,omitempty"`
	OnFail    []string   `yaml:"on_fail,omitempty" json:"on_fail,omitempty"`
	OnError   *ErrorSpec `yaml:"on_error,omitempty" json:"on_error,omitempty"`
}

// InputSpec defines input configuration for a node.
type InputSpec struct {
	Context *ContextSpec `yaml:"context,omitempty" json:"context,omitempty"`
	Budget  *BudgetSpec  `yaml:"budget,omitempty" json:"budget,omitempty"`
}

// ContextSpec defines which context keys to include.
type ContextSpec struct {
	Include []string `yaml:"include" json:"include"`
}

// BudgetSpec defines token budget for context.
type BudgetSpec struct {
	Tokens int `yaml:"tokens" json:"tokens"`
}

// OutputSpec defines outputs to save from the node.
type OutputSpec struct {
	SaveAs []string `yaml:"save_as" json:"save_as"`
}

// ErrorSpec defines error handling behavior.
type ErrorSpec struct {
	Retry int    `yaml:"retry" json:"retry"`
	Goto  string `yaml:"goto" json:"goto"`
}

// ParseSpecYAML parses a workflow spec from YAML data.
func ParseSpecYAML(data []byte) (*WorkflowSpec, error) {
	var spec WorkflowSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSpec, err)
	}
	return &spec, nil
}

// ParseSpecJSON parses a workflow spec from JSON data.
func ParseSpecJSON(data []byte) (*WorkflowSpec, error) {
	var spec WorkflowSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSpec, err)
	}
	return &spec, nil
}

// MarshalSpecJSON serializes a workflow spec to JSON.
func MarshalSpecJSON(spec *WorkflowSpec) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Validate checks the spec for structural correctness.
func (s *WorkflowSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidSpec)
	}
	if len(s.Nodes) == 0 {
		return fmt.Errorf("%w: at least one node is required", ErrInvalidSpec)
	}

	// Build node index
	nodeIndex := make(map[string]*NodeSpec)
	for i := range s.Nodes {
		node := &s.Nodes[i]
		if node.ID == "" {
			return fmt.Errorf("%w: node at index %d has no id", ErrInvalidSpec, i)
		}
		if _, exists := nodeIndex[node.ID]; exists {
			return fmt.Errorf("%w: duplicate node id %q", ErrInvalidSpec, node.ID)
		}
		nodeIndex[node.ID] = node
	}

	// Validate references
	for _, node := range s.Nodes {
		if node.Terminal == "" && len(node.Capabilities) == 0 {
			return fmt.Errorf("%w: node %q has no capabilities", ErrInvalidSpec, node.ID)
		}
		for _, target := range node.AllOutEdges() {
			if _, exists := nodeIndex[target]; !exists {
				return fmt.Errorf("%w: node %q references unknown node %q", ErrInvalidSpec, node.ID, target)
			}
		}
	}

	// Check terminal success exists
	if !s.HasTerminalSuccess() {
		return ErrNoTerminalSuccess
	}

	// Check for cycles
	if err := s.DetectCycle(); err != nil {
		return err
	}

	return nil
}

// HasTerminalSuccess returns true if a terminal success node exists.
func (s *WorkflowSpec) HasTerminalSuccess() bool {
	for _, node := range s.Nodes {
		if node.Terminal == "success" {
			return true
		}
	}
	return false
}

// HasTerminalFail returns true if a terminal fail node exists.
func (s *WorkflowSpec) HasTerminalFail() bool {
	for _, node := range s.Nodes {
		if node.Terminal == "fail" {
			return true
		}
	}
	return false
}

// GetNode returns a node by ID, or nil if not found.
func (s *WorkflowSpec) GetNode(id string) *NodeSpec {
	for i := range s.Nodes {
		if s.Nodes[i].ID == id {
			return &s.Nodes[i]
		}
	}
	return nil
}

// GetStartNode returns the first node to execute (first non-terminal node with no incoming edges).
func (s *WorkflowSpec) GetStartNode() *NodeSpec {
	// Build set of nodes that are targets of edges
	hasIncoming := make(map[string]bool)
	for _, node := range s.Nodes {
		for _, target := range node.AllOutEdges() {
			hasIncoming[target] = true
		}
	}

	// Find first node with no incoming edges and not terminal
	for i := range s.Nodes {
		node := &s.Nodes[i]
		if node.Terminal == "" && !hasIncoming[node.ID] {
			return node
		}
	}

	// Fallback: first non-terminal node
	for i := range s.Nodes {
		if s.Nodes[i].Terminal == "" {
			return &s.Nodes[i]
		}
	}

	return nil
}

// AllOutEdges returns all outgoing edge targets for a node.
func (n *NodeSpec) AllOutEdges() []string {
	var edges []string
	edges = append(edges, n.Next...)
	edges = append(edges, n.OnApprove...)
	edges = append(edges, n.OnReject...)
	edges = append(edges, n.OnFail...)
	if n.OnError != nil && n.OnError.Goto != "" {
		edges = append(edges, n.OnError.Goto)
	}
	return edges
}

// ForwardEdges returns only forward edges (next, on_approve) used for cycle detection.
// Recovery edges (on_reject, on_fail, on_error) are excluded as they are intentional
// back-edges for workflow recovery and require human action or failure to trigger.
func (n *NodeSpec) ForwardEdges() []string {
	var edges []string
	edges = append(edges, n.Next...)
	edges = append(edges, n.OnApprove...)
	return edges
}

// IsTerminal returns true if this is a terminal node.
func (n *NodeSpec) IsTerminal() bool {
	return n.Terminal == "success" || n.Terminal == "fail"
}

// DetectCycle checks for cycles in the DAG using DFS.
// Only forward edges (next, on_approve) are checked; recovery edges
// (on_reject, on_fail, on_error) are excluded as they are intentional back-edges.
func (s *WorkflowSpec) DetectCycle() error {
	// Build adjacency list using only forward edges
	adj := make(map[string][]string)
	for _, node := range s.Nodes {
		adj[node.ID] = node.ForwardEdges()
	}

	// DFS state: 0=unvisited, 1=visiting, 2=visited
	state := make(map[string]int)

	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		if state[nodeID] == 1 {
			return true // cycle found
		}
		if state[nodeID] == 2 {
			return false // already processed
		}

		state[nodeID] = 1 // visiting
		for _, neighbor := range adj[nodeID] {
			if dfs(neighbor) {
				return true
			}
		}
		state[nodeID] = 2 // visited
		return false
	}

	for _, node := range s.Nodes {
		if state[node.ID] == 0 {
			if dfs(node.ID) {
				return ErrCycleDetected
			}
		}
	}

	return nil
}

// TopologicalSort returns nodes in topological order.
func (s *WorkflowSpec) TopologicalSort() ([]string, error) {
	if err := s.DetectCycle(); err != nil {
		return nil, err
	}

	// Build adjacency list
	adj := make(map[string][]string)
	for _, node := range s.Nodes {
		adj[node.ID] = node.AllOutEdges()
	}

	visited := make(map[string]bool)
	var result []string

	var dfs func(nodeID string)
	dfs = func(nodeID string) {
		if visited[nodeID] {
			return
		}
		visited[nodeID] = true

		for _, neighbor := range adj[nodeID] {
			dfs(neighbor)
		}
		result = append([]string{nodeID}, result...)
	}

	// Start from nodes with no incoming edges
	hasIncoming := make(map[string]bool)
	for _, node := range s.Nodes {
		for _, target := range node.AllOutEdges() {
			hasIncoming[target] = true
		}
	}

	for _, node := range s.Nodes {
		if !hasIncoming[node.ID] {
			dfs(node.ID)
		}
	}

	// Process remaining nodes
	for _, node := range s.Nodes {
		dfs(node.ID)
	}

	return result, nil
}

// GetNextNodes determines the next nodes based on current step outcome.
func (s *WorkflowSpec) GetNextNodes(currentNodeID string, status StepStatus, approved bool) []string {
	node := s.GetNode(currentNodeID)
	if node == nil || node.IsTerminal() {
		return nil
	}

	switch status {
	case StepStatusCompleted:
		if node.RequiresApproval {
			if approved {
				if len(node.OnApprove) > 0 {
					return node.OnApprove
				}
			} else {
				if len(node.OnReject) > 0 {
					return node.OnReject
				}
				return nil // Rejection with no on_reject ends workflow
			}
		}
		if len(node.Next) > 0 {
			return node.Next
		}

	case StepStatusFailed:
		if len(node.OnFail) > 0 {
			return node.OnFail
		}
		if node.OnError != nil && node.OnError.Goto != "" {
			return []string{node.OnError.Goto}
		}
	}

	return node.Next
}

// DefaultSpec returns the default sequential workflow as a DAG spec.
func DefaultSpec(name, description string) *WorkflowSpec {
	return &WorkflowSpec{
		Version:     1,
		Name:        name,
		Description: description,
		Nodes: []NodeSpec{
			{ID: "plan", Name: "Planning", Capabilities: []string{"plan"}, PreferredAgents: []string{"claude-code"}, Next: []string{"plan_review"}},
			{ID: "plan_review", Name: "Plan Review", Capabilities: []string{"review"}, PreferredAgents: []string{"codex"}, RequiresApproval: true, OnApprove: []string{"coding"}, OnReject: []string{"plan"}},
			{ID: "coding", Name: "Coding", Capabilities: []string{"code"}, PreferredAgents: []string{"claude-code"}, Next: []string{"feature_review"}, OnError: &ErrorSpec{Retry: 2, Goto: "fail"}},
			{ID: "feature_review", Name: "Feature Review", Capabilities: []string{"review"}, PreferredAgents: []string{"codex"}, Next: []string{"final_review"}, OnFail: []string{"coding"}},
			{ID: "final_review", Name: "Final Review", Capabilities: []string{"review"}, PreferredAgents: []string{"codex"}, Next: []string{"docs"}, OnFail: []string{"coding"}},
			{ID: "docs", Name: "Documentation", Capabilities: []string{"docs"}, PreferredAgents: []string{"claude-code"}, Next: []string{"complete"}, OnError: &ErrorSpec{Retry: 1}},
			{ID: "complete", Terminal: "success"},
			{ID: "fail", Terminal: "fail"},
		},
	}
}
