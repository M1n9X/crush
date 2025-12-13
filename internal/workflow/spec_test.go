package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSpecYAML(t *testing.T) {
	yaml := `
version: 1
name: "test-workflow"
description: "Test DAG"
nodes:
  - id: start
    name: "Start"
    capabilities: ["plan"]
    preferred_agents: ["claude-code"]
    next: [complete]
  - id: complete
    terminal: success
`
	spec, err := ParseSpecYAML([]byte(yaml))
	require.NoError(t, err)
	require.Equal(t, 1, spec.Version)
	require.Equal(t, "test-workflow", spec.Name)
	require.Len(t, spec.Nodes, 2)
	require.Equal(t, "start", spec.Nodes[0].ID)
	require.Equal(t, []string{"plan"}, spec.Nodes[0].Capabilities)
	require.Equal(t, []string{"complete"}, spec.Nodes[0].Next)
	require.Equal(t, "success", spec.Nodes[1].Terminal)
}

func TestParseSpecJSON(t *testing.T) {
	json := `{
		"version": 1,
		"name": "json-workflow",
		"nodes": [
			{"id": "start", "capabilities": ["code"], "next": ["end"]},
			{"id": "end", "terminal": "success"}
		]
	}`
	spec, err := ParseSpecJSON([]byte(json))
	require.NoError(t, err)
	require.Equal(t, "json-workflow", spec.Name)
	require.Len(t, spec.Nodes, 2)
}

func TestValidateSpec(t *testing.T) {
	t.Run("valid spec", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "valid",
			Nodes: []NodeSpec{
				{ID: "start", Capabilities: []string{"plan"}, Next: []string{"end"}},
				{ID: "end", Terminal: "success"},
			},
		}
		require.NoError(t, spec.Validate())
	})

	t.Run("missing name", func(t *testing.T) {
		spec := &WorkflowSpec{
			Nodes: []NodeSpec{{ID: "a", Terminal: "success"}},
		}
		err := spec.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "name is required")
	})

	t.Run("no nodes", func(t *testing.T) {
		spec := &WorkflowSpec{Name: "empty"}
		err := spec.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least one node")
	})

	t.Run("duplicate node id", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "dup",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}, Next: []string{"a"}},
				{ID: "a", Terminal: "success"},
			},
		}
		err := spec.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate node id")
	})

	t.Run("missing capabilities", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "nocap",
			Nodes: []NodeSpec{
				{ID: "a", Next: []string{"end"}},
				{ID: "end", Terminal: "success"},
			},
		}
		err := spec.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "no capabilities")
	})

	t.Run("unknown node reference", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "badref",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}, Next: []string{"nonexistent"}},
				{ID: "end", Terminal: "success"},
			},
		}
		err := spec.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown node")
	})

	t.Run("no terminal success", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "noterm",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}},
			},
		}
		err := spec.Validate()
		require.ErrorIs(t, err, ErrNoTerminalSuccess)
	})
}

func TestDetectCycle(t *testing.T) {
	t.Run("no cycle", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "linear",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}, Next: []string{"b"}},
				{ID: "b", Capabilities: []string{"code"}, Next: []string{"c"}},
				{ID: "c", Terminal: "success"},
			},
		}
		require.NoError(t, spec.DetectCycle())
	})

	t.Run("cycle detected", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "cyclic",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}, Next: []string{"b"}},
				{ID: "b", Capabilities: []string{"code"}, Next: []string{"a"}}, // cycle
				{ID: "c", Terminal: "success"},
			},
		}
		err := spec.DetectCycle()
		require.ErrorIs(t, err, ErrCycleDetected)
	})

	t.Run("self loop", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "selfloop",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}, Next: []string{"a"}},
				{ID: "c", Terminal: "success"},
			},
		}
		err := spec.DetectCycle()
		require.ErrorIs(t, err, ErrCycleDetected)
	})

	t.Run("diamond ok", func(t *testing.T) {
		spec := &WorkflowSpec{
			Name: "diamond",
			Nodes: []NodeSpec{
				{ID: "a", Capabilities: []string{"plan"}, Next: []string{"b", "c"}},
				{ID: "b", Capabilities: []string{"code"}, Next: []string{"d"}},
				{ID: "c", Capabilities: []string{"review"}, Next: []string{"d"}},
				{ID: "d", Terminal: "success"},
			},
		}
		require.NoError(t, spec.DetectCycle())
	})
}

func TestTopologicalSort(t *testing.T) {
	spec := &WorkflowSpec{
		Name: "linear",
		Nodes: []NodeSpec{
			{ID: "a", Capabilities: []string{"plan"}, Next: []string{"b"}},
			{ID: "b", Capabilities: []string{"code"}, Next: []string{"c"}},
			{ID: "c", Terminal: "success"},
		},
	}

	order, err := spec.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, order, 3)

	// a should come before b, b before c
	aIdx, bIdx, cIdx := -1, -1, -1
	for i, id := range order {
		switch id {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		}
	}
	require.Less(t, aIdx, bIdx)
	require.Less(t, bIdx, cIdx)
}

func TestGetStartNode(t *testing.T) {
	spec := &WorkflowSpec{
		Name: "test",
		Nodes: []NodeSpec{
			{ID: "middle", Capabilities: []string{"code"}, Next: []string{"end"}},
			{ID: "start", Capabilities: []string{"plan"}, Next: []string{"middle"}},
			{ID: "end", Terminal: "success"},
		},
	}

	start := spec.GetStartNode()
	require.NotNil(t, start)
	require.Equal(t, "start", start.ID)
}

func TestGetNextNodes(t *testing.T) {
	spec := &WorkflowSpec{
		Name: "test",
		Nodes: []NodeSpec{
			{ID: "plan", Capabilities: []string{"plan"}, Next: []string{"review"}},
			{ID: "review", Capabilities: []string{"review"}, RequiresApproval: true, OnApprove: []string{"code"}, OnReject: []string{"plan"}},
			{ID: "code", Capabilities: []string{"code"}, Next: []string{"end"}, OnFail: []string{"review"}},
			{ID: "end", Terminal: "success"},
		},
	}

	t.Run("normal next", func(t *testing.T) {
		next := spec.GetNextNodes("plan", StepStatusCompleted, false)
		require.Equal(t, []string{"review"}, next)
	})

	t.Run("approval granted", func(t *testing.T) {
		next := spec.GetNextNodes("review", StepStatusCompleted, true)
		require.Equal(t, []string{"code"}, next)
	})

	t.Run("approval rejected", func(t *testing.T) {
		next := spec.GetNextNodes("review", StepStatusCompleted, false)
		require.Equal(t, []string{"plan"}, next)
	})

	t.Run("on fail", func(t *testing.T) {
		next := spec.GetNextNodes("code", StepStatusFailed, false)
		require.Equal(t, []string{"review"}, next)
	})

	t.Run("terminal node", func(t *testing.T) {
		next := spec.GetNextNodes("end", StepStatusCompleted, false)
		require.Nil(t, next)
	})
}

func TestDefaultSpec(t *testing.T) {
	spec := DefaultSpec("test", "Test workflow")

	require.Equal(t, 1, spec.Version)
	require.Equal(t, "test", spec.Name)
	require.Len(t, spec.Nodes, 8) // 6 steps + complete + fail

	require.NoError(t, spec.Validate())
	require.True(t, spec.HasTerminalSuccess())
	require.True(t, spec.HasTerminalFail())

	start := spec.GetStartNode()
	require.Equal(t, "plan", start.ID)
}

func TestNodeSpecAllOutEdges(t *testing.T) {
	node := NodeSpec{
		ID:        "test",
		Next:      []string{"a", "b"},
		OnApprove: []string{"c"},
		OnReject:  []string{"d"},
		OnFail:    []string{"e"},
		OnError:   &ErrorSpec{Goto: "f"},
	}

	edges := node.AllOutEdges()
	require.Len(t, edges, 6)
	require.Contains(t, edges, "a")
	require.Contains(t, edges, "b")
	require.Contains(t, edges, "c")
	require.Contains(t, edges, "d")
	require.Contains(t, edges, "e")
	require.Contains(t, edges, "f")
}

func TestIsTerminal(t *testing.T) {
	require.True(t, (&NodeSpec{Terminal: "success"}).IsTerminal())
	require.True(t, (&NodeSpec{Terminal: "fail"}).IsTerminal())
	require.False(t, (&NodeSpec{Terminal: ""}).IsTerminal())
}

func TestMarshalSpecJSON(t *testing.T) {
	spec := DefaultSpec("test", "desc")
	data, err := MarshalSpecJSON(spec)
	require.NoError(t, err)
	require.Contains(t, data, "test")
	require.Contains(t, data, "plan")
}
