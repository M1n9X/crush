package agent

import (
	"testing"
	"time"
)

func TestLoadSubAgentDefinitionsWithCache_FirstLoad(t *testing.T) {
	// Clear cache before test
	clearSubAgentDefsCache()

	// First load should call actual load function
	workingDir := "/tmp/test-crush-agents"
	defs, err := loadSubAgentDefinitionsWithCache(workingDir)

	// Should have at least the builtin agent
	if err != nil && len(defs) == 0 {
		t.Errorf("Expected at least builtin agent, got error: %v", err)
	}
}

func TestLoadSubAgentDefinitionsWithCache_CacheHit(t *testing.T) {
	// Clear cache before test
	clearSubAgentDefsCache()

	workingDir := "/tmp/test-crush-agents"

	// First load
	defs1, _ := loadSubAgentDefinitionsWithCache(workingDir)
	time1 := subAgentDefsCacheTime

	// Immediate second load should use cache
	defs2, _ := loadSubAgentDefinitionsWithCache(workingDir)
	time2 := subAgentDefsCacheTime

	// Cache time should not change
	if !time1.Equal(time2) {
		t.Error("Expected cache to be used, but cache time changed")
	}

	// Should return same number of definitions
	if len(defs1) != len(defs2) {
		t.Errorf("Expected same number of definitions, got %d and %d", len(defs1), len(defs2))
	}
}

func TestLoadSubAgentDefinitionsWithCache_CacheExpired(t *testing.T) {
	// Clear cache before test
	clearSubAgentDefsCache()

	workingDir := "/tmp/test-crush-agents"

	// First load
	loadSubAgentDefinitionsWithCache(workingDir)

	// Manually expire cache by setting old time
	subAgentDefsCacheMu.Lock()
	subAgentDefsCacheTime = time.Now().Add(-10 * time.Minute)
	subAgentDefsCacheMu.Unlock()

	oldTime := subAgentDefsCacheTime

	// Load again should refresh cache
	loadSubAgentDefinitionsWithCache(workingDir)

	// Cache time should be updated
	if !subAgentDefsCacheTime.After(oldTime) {
		t.Error("Expected cache to be refreshed, but time not updated")
	}
}

func TestClearSubAgentDefsCache(t *testing.T) {
	// Load some definitions to populate cache
	workingDir := "/tmp/test-crush-agents"
	loadSubAgentDefinitionsWithCache(workingDir)

	// Verify cache is populated
	subAgentDefsCacheMu.RLock()
	hasCacheBefore := subAgentDefsCache != nil
	subAgentDefsCacheMu.RUnlock()

	if !hasCacheBefore {
		t.Error("Cache should be populated after load")
	}

	// Clear cache
	clearSubAgentDefsCache()

	// Verify cache is cleared
	subAgentDefsCacheMu.RLock()
	hasCacheAfter := subAgentDefsCache != nil
	subAgentDefsCacheMu.RUnlock()

	if hasCacheAfter {
		t.Error("Cache should be nil after clear")
	}
}

func TestCacheConcurrency(t *testing.T) {
	// Clear cache before test
	clearSubAgentDefsCache()

	workingDir := "/tmp/test-crush-agents"

	// Run multiple goroutines accessing cache concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = loadSubAgentDefinitionsWithCache(workingDir)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic - test passes if we get here
}
