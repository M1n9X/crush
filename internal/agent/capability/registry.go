package capability

import "sync"

// Descriptor captures metadata about a tool/capability so that permissions and
// telemetry can reason about it without depending on the tool implementation.
type Descriptor struct {
	ID          string
	Description string
	Category    string
	ReadOnly    bool
	Tags        []string
}

// Registry stores capability descriptors in-memory. It is intentionally small
// and can be extended later to persist to disk or emit events.
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
}

// NewRegistry creates an empty capability registry.
func NewRegistry() *Registry {
	return &Registry{
		descriptors: make(map[string]Descriptor),
	}
}

// Register adds or updates a descriptor keyed by ID.
func (r *Registry) Register(desc Descriptor) {
	if desc.ID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.descriptors[desc.ID] = desc
}

// Get returns a descriptor by ID.
func (r *Registry) Get(id string) (Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	desc, ok := r.descriptors[id]
	return desc, ok
}

// List returns all descriptors.
func (r *Registry) List() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.descriptors))
	for _, desc := range r.descriptors {
		out = append(out, desc)
	}
	return out
}

// Count returns number of registered descriptors.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.descriptors)
}
