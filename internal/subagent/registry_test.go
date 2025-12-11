package subagent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubAgent struct {
	name           string
	supportsResume bool
}

func (s stubAgent) Name() string { return s.name }

func (s stubAgent) Capabilities() []Capability { return []Capability{"plan"} }

func (s stubAgent) SupportsResume() bool { return s.supportsResume }

func (s stubAgent) Execute(ctx context.Context, req Request) (*Result, error) {
	return &Result{Text: "ok"}, nil
}

func (s stubAgent) ExecuteStreamed(ctx context.Context, req Request, handler func(Event)) (*Result, error) {
	if handler != nil {
		handler(Event{Type: "stream", Payload: req.Task})
	}
	return &Result{Text: "ok"}, nil
}

func (s stubAgent) Resume(ctx context.Context, resumeToken string, req Request) (*Result, error) {
	if !s.supportsResume {
		return nil, ErrResumeNotSupported
	}
	return &Result{Text: "resume"}, nil
}

func TestRegistryResolveDefault(t *testing.T) {
	defaultAgent := stubAgent{name: "default"}
	reg := NewRegistry(defaultAgent)
	reg.Upsert(Registration{
		Profile: Profile{Name: "general"},
	})

	entry, err := reg.Resolve("")
	require.NoError(t, err)
	require.Equal(t, "general", entry.Profile.Name)
	require.Equal(t, "default", entry.Agent.Name())
}

func TestRegistryResolveUnknown(t *testing.T) {
	reg := NewRegistry(nil)
	reg.Upsert(Registration{Profile: Profile{Name: "alpha"}, Agent: stubAgent{name: "a"}})
	reg.Upsert(Registration{Profile: Profile{Name: "beta"}, Agent: stubAgent{name: "b"}})

	_, err := reg.Resolve("missing")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "alpha"))
	require.True(t, strings.Contains(err.Error(), "beta"))
}
