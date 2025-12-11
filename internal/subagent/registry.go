package subagent

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Registration ties a runtime subagent implementation to a concrete profile.
type Registration struct {
	Profile Profile
	Agent   Subagent
}

// Registry keeps track of available subagents and their profiles.
type Registry struct {
	mu            sync.RWMutex
	registrations map[string]Registration
	order         []string
	defaultAgent  Subagent
}

// NewRegistry constructs an empty registry with an optional default agent fallback.
func NewRegistry(defaultAgent Subagent) *Registry {
	return &Registry{
		registrations: make(map[string]Registration),
		defaultAgent:  defaultAgent,
	}
}

// Upsert registers or replaces a subagent profile.
func (r *Registry) Upsert(reg Registration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := strings.TrimSpace(reg.Profile.Name)
	if name == "" {
		return
	}
	if reg.Agent == nil {
		reg.Agent = r.defaultAgent
	}
	if _, ok := r.registrations[name]; !ok {
		r.order = append(r.order, name)
	}
	r.registrations[name] = reg
	slices.Sort(r.order)
}

// Resolve returns the registration for a profile name or the default when name is empty.
func (r *Registry) Resolve(name string) (Registration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target := strings.TrimSpace(name)
	if target == "" {
		if reg, ok := r.registrations["general"]; ok {
			return reg, nil
		}
		if reg, ok := r.registrations["general-purpose"]; ok {
			return reg, nil
		}
		if len(r.order) > 0 {
			return r.registrations[r.order[0]], nil
		}
		return Registration{}, fmt.Errorf("no subagents registered")
	}

	if reg, ok := r.registrations[target]; ok {
		return reg, nil
	}

	names := slices.Clone(r.order)
	return Registration{}, fmt.Errorf("unknown subagent %q; available: %s", target, strings.Join(names, ", "))
}

// List returns all registrations sorted by name for deterministic ordering.
func (r *Registry) List() []Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Registration, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.registrations[name])
	}
	return result
}

// ReplaceProfiles resets the registry while keeping the default agent.
func (r *Registry) ReplaceProfiles(regs []Registration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registrations = make(map[string]Registration, len(regs))
	r.order = r.order[:0]
	for _, reg := range regs {
		name := strings.TrimSpace(reg.Profile.Name)
		if name == "" {
			continue
		}
		if reg.Agent == nil {
			reg.Agent = r.defaultAgent
		}
		r.registrations[name] = reg
		r.order = append(r.order, name)
	}
	slices.Sort(r.order)
}
