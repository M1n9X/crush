package plugin

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Descriptor represents a plugin entry.
type Descriptor struct {
	Name       string
	APIVersion string
	Path       string
	Enabled    bool
	Sandbox    string
}

// Host keeps a registry of plugins; execution is out of scope for now.
type Host struct {
	mu         sync.RWMutex
	plugins    map[string]Descriptor
	disabled   map[string]bool
	storePath  string
	workingDir string
}

// NewHost creates an empty host.
func NewHost(storePath, workingDir string) *Host {
	h := &Host{
		plugins:    make(map[string]Descriptor),
		disabled:   make(map[string]bool),
		storePath:  storePath,
		workingDir: workingDir,
	}
	h.loadDisabled()
	return h
}

// Register adds or updates a descriptor.
func (h *Host) Register(desc Descriptor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if desc.Name == "" {
		return
	}
	if h.disabled[desc.Name] {
		desc.Enabled = false
	}
	if desc.Enabled == false && !h.disabled[desc.Name] {
		// Respect explicit disabled flag but keep the entry.
		desc.Enabled = false
	}
	h.plugins[desc.Name] = desc
}

// Unregister removes a plugin by name.
func (h *Host) Unregister(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.plugins, name)
}

// List returns all descriptors.
func (h *Host) List() []Descriptor {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Descriptor, 0, len(h.plugins))
	for _, d := range h.plugins {
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b Descriptor) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// Get returns a descriptor by name.
func (h *Host) Get(name string) (Descriptor, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	d, ok := h.plugins[name]
	return d, ok
}

// Disable marks a plugin as disabled and persists the state.
func (h *Host) Disable(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if name == "" {
		return
	}
	h.disabled[name] = true
	if d, ok := h.plugins[name]; ok {
		d.Enabled = false
		h.plugins[name] = d
	}
	_ = h.persistDisabledLocked()
}

// Enable marks a plugin as enabled (if known) and persists the state.
func (h *Host) Enable(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.disabled, name)
	if d, ok := h.plugins[name]; ok {
		d.Enabled = true
		h.plugins[name] = d
	}
	_ = h.persistDisabledLocked()
}

func (h *Host) IsDisabled(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.disabled[name]
}

func (h *Host) persistDisabledLocked() error {
	if h.storePath == "" {
		return nil
	}
	entries := make([]string, 0, len(h.disabled))
	for name := range h.disabled {
		entries = append(entries, name)
	}
	slices.Sort(entries)
	if err := os.MkdirAll(filepath.Dir(h.storePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.storePath, data, 0o600)
}

func (h *Host) loadDisabled() {
	if h.storePath == "" {
		return
	}
	data, err := os.ReadFile(h.storePath)
	if err != nil {
		return
	}
	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	for _, name := range entries {
		h.disabled[name] = true
	}
}

// PersistentPath resolves a per-workspace plugin state file.
func PersistentPath(dataDir, workingDir string) string {
	if dataDir == "" || workingDir == "" {
		return ""
	}
	sum := sha1.Sum([]byte(workingDir))
	filename := fmt.Sprintf("%x.json", sum)
	return filepath.Join(dataDir, "plugins", filename)
}
