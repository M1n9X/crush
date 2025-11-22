package agent

import (
	"sync"
	"time"
)

// Cache for subagent definitions to reduce repeated file I/O
var (
	subAgentDefsCache     []SubAgentDefinition
	subAgentDefsCacheMu   sync.RWMutex
	subAgentDefsCacheTime time.Time
	subAgentDefsCacheTTL  = 5 * time.Minute
)

// loadSubAgentDefinitionsWithCache loads agent definitions with caching support.
// It uses a simple TTL-based cache to avoid repeated file I/O operations.
func loadSubAgentDefinitionsWithCache(workingDir string) ([]SubAgentDefinition, error) {
	subAgentDefsCacheMu.RLock()
	if time.Since(subAgentDefsCacheTime) < subAgentDefsCacheTTL && subAgentDefsCache != nil {
		// Cache hit - return cached definitions
		cached := make([]SubAgentDefinition, len(subAgentDefsCache))
		copy(cached, subAgentDefsCache)
		subAgentDefsCacheMu.RUnlock()
		return cached, nil
	}
	subAgentDefsCacheMu.RUnlock()

	// Cache miss or expired - load from disk
	defs, err := loadSubAgentDefinitions(workingDir)
	if err != nil {
		return nil, err
	}

	// Update cache
	subAgentDefsCacheMu.Lock()
	subAgentDefsCache = defs
	subAgentDefsCacheTime = time.Now()
	subAgentDefsCacheMu.Unlock()

	return defs, nil
}

// clearSubAgentDefsCache clears the cached subagent definitions.
// This should be called when agent configuration files are modified.
func clearSubAgentDefsCache() {
	subAgentDefsCacheMu.Lock()
	subAgentDefsCache = nil
	subAgentDefsCacheTime = time.Time{}
	subAgentDefsCacheMu.Unlock()
}

// ClearSubAgentCache clears the cached subagent definitions so that subsequent
// calls will reload from disk.
func ClearSubAgentCache() {
	clearSubAgentDefsCache()
}
