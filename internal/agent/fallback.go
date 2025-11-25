// Package agent provides model fallback functionality.
package agent

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
)

// ModelFallbackManager handles fallback to alternative models when primary fails.
type ModelFallbackManager struct {
	currentModelIndex int
	fallbackModels    []string
	primaryModel      string
}

// NewModelFallbackManager creates a new fallback manager.
func NewModelFallbackManager(primaryModel string, fallbackModels []string) *ModelFallbackManager {
	return &ModelFallbackManager{
		currentModelIndex: -1, // -1 means using primary
		fallbackModels:    fallbackModels,
		primaryModel:      primaryModel,
	}
}

// ShouldFallback determines if we should try a fallback model based on the error.
func (m *ModelFallbackManager) ShouldFallback(err error) bool {
	if len(m.fallbackModels) == 0 {
		return false
	}

	// Check if we've exhausted all fallbacks
	if m.currentModelIndex >= len(m.fallbackModels)-1 {
		return false
	}

	// Check if error is model-related
	category := ClassifyError(err)
	return category == ErrorCategoryModel ||
		category == ErrorCategoryRateLimit ||
		category == ErrorCategoryNetwork
}

// GetNextModel returns the next fallback model to try.
// Returns empty string if no more fallbacks available.
func (m *ModelFallbackManager) GetNextModel() string {
	if m.currentModelIndex >= len(m.fallbackModels)-1 {
		return ""
	}

	m.currentModelIndex++
	model := m.fallbackModels[m.currentModelIndex]

	slog.Info("Falling back to alternative model",
		"primary_model", m.primaryModel,
		"fallback_model", model,
		"fallback_index", m.currentModelIndex,
	)

	return model
}

// GetCurrentModel returns the currently active model.
func (m *ModelFallbackManager) GetCurrentModel() string {
	if m.currentModelIndex == -1 {
		return m.primaryModel
	}
	if m.currentModelIndex < len(m.fallbackModels) {
		return m.fallbackModels[m.currentModelIndex]
	}
	return m.primaryModel
}

// Reset resets the fallback state to use primary model.
func (m *ModelFallbackManager) Reset() {
	m.currentModelIndex = -1
}

// HasFallbacks returns true if fallback models are configured.
func (m *ModelFallbackManager) HasFallbacks() bool {
	return len(m.fallbackModels) > 0
}

// createFallbackManager creates a fallback manager from config.
func createFallbackManager(modelCfg config.SelectedModel) *ModelFallbackManager {
	if modelCfg.Retry == nil || len(modelCfg.Retry.FallbackModels) == 0 {
		return nil
	}

	return NewModelFallbackManager(
		modelCfg.Model,
		modelCfg.Retry.FallbackModels,
	)
}

// GetFallbackModel finds a fallback model configuration.
// Returns nil if not found.
func GetFallbackModel(cfg *config.Config, modelName string) *config.SelectedModel {
	// Check both large and small models
	if cfg.Models != nil {
		for _, modelCfg := range cfg.Models {
			if modelCfg.Model == modelName {
				return &modelCfg
			}
		}
	}

	return nil
}

// ValidateFallbackModels checks if all fallback models are available.
func ValidateFallbackModels(cfg *config.Config, fallbackModels []string) error {
	for _, modelName := range fallbackModels {
		if fallbackCfg := GetFallbackModel(cfg, modelName); fallbackCfg == nil {
			return fmt.Errorf("fallback model %s not found in configuration", modelName)
		}
	}
	return nil
}
