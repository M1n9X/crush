package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
)

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrEmptyPrompt      = errors.New("prompt is empty")
	ErrSessionMissing   = errors.New("session id is missing")
)

func isCancelledErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, ErrRequestCancelled)
}

// ErrorStats tracks error statistics for monitoring and debugging.
type ErrorStats struct {
	errorCounts map[string]int64
	lastErrors  map[string]time.Time
	mu          sync.RWMutex
}

// Global error statistics with lazy initialization
var (
	globalErrorStats *ErrorStats
	errorStatsOnce   sync.Once
)

// initErrorStats initializes the global error statistics (thread-safe).
func initErrorStats() {
	errorStatsOnce.Do(func() {
		globalErrorStats = &ErrorStats{
			errorCounts: make(map[string]int64),
			lastErrors:  make(map[string]time.Time),
		}
	})
}

// getErrorStats returns the initialized global error stats.
func getErrorStats() *ErrorStats {
	initErrorStats()
	return globalErrorStats
}

// UpdateErrorStats records an error occurrence.
func UpdateErrorStats(err error) {
	category := ClassifyError(err)
	key := errorCategoryToString(category)

	stats := getErrorStats()
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.errorCounts[key]++
	stats.lastErrors[key] = time.Now()
}

// GetErrorFrequency returns the count of how many times an error category occurred.
func GetErrorFrequency(category ErrorCategory) int64 {
	key := errorCategoryToString(category)

	stats := getErrorStats()
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	return stats.errorCounts[key]
}

// GetLastErrorTime returns when an error category last occurred.
func GetLastErrorTime(category ErrorCategory) *time.Time {
	key := errorCategoryToString(category)

	stats := getErrorStats()
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	if t, ok := stats.lastErrors[key]; ok {
		return &t
	}
	return nil
}

// GetAllErrorStats returns statistics for all error categories.
func GetAllErrorStats() []ErrorStat {
	stats := getErrorStats()
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	var result []ErrorStat
	for key, count := range stats.errorCounts {
		lastOccurred := stats.lastErrors[key]
		result = append(result, ErrorStat{
			Category:     key,
			Count:        count,
			LastOccurred: lastOccurred,
		})
	}
	return result
}

// ErrorStat represents statistics for a single error category.
type ErrorStat struct {
	Category     string
	Count        int64
	LastOccurred time.Time
}

// ClearErrorStats resets all error statistics.
func ClearErrorStats() {
	stats := getErrorStats()
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.errorCounts = make(map[string]int64)
	stats.lastErrors = make(map[string]time.Time)
}

// ResetErrorStatsForTest resets error statistics for testing.
// This function is intended for use in unit tests only.
func ResetErrorStatsForTest() {
	errorStatsOnce = sync.Once{}
	globalErrorStats = nil
}

func errorCategoryToString(category ErrorCategory) string {
	switch category {
	case ErrorCategoryNetwork:
		return "network"
	case ErrorCategoryRateLimit:
		return "rate_limit"
	case ErrorCategoryModel:
		return "model"
	case ErrorCategoryAuthentication:
		return "authentication"
	case ErrorCategoryPermission:
		return "permission"
	default:
		return "unknown"
	}
}

// ErrorCategory represents the category of an error.
type ErrorCategory int

const (
	// ErrorCategoryUnknown represents an unknown error category.
	ErrorCategoryUnknown ErrorCategory = iota
	// ErrorCategoryNetwork represents network-related errors.
	ErrorCategoryNetwork
	// ErrorCategoryRateLimit represents rate limiting errors.
	ErrorCategoryRateLimit
	// ErrorCategoryModel represents model-related errors.
	ErrorCategoryModel
	// ErrorCategoryAuthentication represents authentication errors.
	ErrorCategoryAuthentication
	// ErrorCategoryPermission represents permission errors.
	ErrorCategoryPermission
)

// ClassifyError determines the category of an error.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	var providerErr *fantasy.ProviderError
	if errors.As(err, &providerErr) {
		return classifyProviderError(providerErr)
	}

	// Fallback to string matching with more specific patterns
	errStr := strings.ToLower(err.Error())

	// Check rate limit first (most specific)
	if containsAny(errStr, []string{"rate limit", "too many requests", "status 429", "status code 429"}) {
		return ErrorCategoryRateLimit
	}

	// Check network errors (avoid broad matches)
	if containsAny(errStr, []string{"network error", "connection refused", "connection timeout", "dial tcp", "i/o timeout", "eof"}) {
		return ErrorCategoryNetwork
	}

	// Check authentication (more specific patterns)
	if containsAny(errStr, []string{"authentication failed", "unauthorized", "status 401", "status code 401", "invalid api key", "api key"}) {
		return ErrorCategoryAuthentication
	}

	// Check permission errors
	if containsAny(errStr, []string{"permission denied", "forbidden", "status 403", "status code 403"}) {
		return ErrorCategoryPermission
	}

	// Check model errors (more specific patterns to avoid false positives)
	if containsAny(errStr, []string{"invalid model", "model not found", "model error", "status 404", "status code 404"}) {
		return ErrorCategoryModel
	}

	return ErrorCategoryUnknown
}

// classifyProviderError classifies a ProviderError.
func classifyProviderError(err *fantasy.ProviderError) ErrorCategory {
	// Check HTTP status code if available
	if err.StatusCode != 0 {
		switch err.StatusCode {
		case 429:
			return ErrorCategoryRateLimit
		case 401:
			return ErrorCategoryAuthentication
		case 403:
			return ErrorCategoryPermission
		case 404:
			return ErrorCategoryModel
		case 500, 502, 503, 504:
			return ErrorCategoryNetwork
		}
	}

	// Fallback to message matching
	msgLower := strings.ToLower(err.Message)

	if containsAny(msgLower, []string{"rate limit", "too many requests"}) {
		return ErrorCategoryRateLimit
	}

	if containsAny(msgLower, []string{"network", "connection", "timeout"}) {
		return ErrorCategoryNetwork
	}

	if containsAny(msgLower, []string{"authentication", "unauthorized", "invalid key"}) {
		return ErrorCategoryAuthentication
	}

	if containsAny(msgLower, []string{"permission", "forbidden"}) {
		return ErrorCategoryPermission
	}

	if containsAny(msgLower, []string{"model", "not found"}) {
		return ErrorCategoryModel
	}

	return ErrorCategoryUnknown
}

// IsRetryable determines if an error should be retried.
func IsRetryable(err error) bool {
	category := ClassifyError(err)

	switch category {
	case ErrorCategoryNetwork:
		// Network errors are generally retryable
		return true
	case ErrorCategoryRateLimit:
		// Rate limit errors are retryable after appropriate delay
		return true
	case ErrorCategoryModel:
		// Model errors (e.g., overloaded) may be retryable
		return true
	case ErrorCategoryAuthentication, ErrorCategoryPermission:
		// Auth/permission errors are not retryable
		return false
	default:
		// Unknown errors - conservative approach: don't retry
		return false
	}
}

// GetRetryDelay calculates the retry delay based on error and retry attempt.
// Returns the delay using exponential backoff.
func GetRetryDelay(err error, retryCount int, baseDelay time.Duration, multiplier float64, maxDelay time.Duration) time.Duration {
	// Check if provider error contains Retry-After header
	var providerErr *fantasy.ProviderError
	if errors.As(err, &providerErr) && providerErr.ResponseHeaders != nil {
		if retryAfter, ok := providerErr.ResponseHeaders["Retry-After"]; ok {
			// Parse Retry-After header (could be seconds or HTTP date)
			if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
				if seconds > maxDelay {
					return maxDelay
				}
				return seconds
			}
		}
	}

	// Calculate exponential backoff
	delay := baseDelay
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * multiplier)
	}

	if delay > maxDelay {
		return maxDelay
	}

	return delay
}

// GetErrorMessage returns a user-friendly error message.
func GetErrorMessage(err error) string {
	category := ClassifyError(err)

	switch category {
	case ErrorCategoryRateLimit:
		return "Rate limit exceeded. The request will be retried after a delay."
	case ErrorCategoryNetwork:
		return "Network error occurred. The request will be retried."
	case ErrorCategoryAuthentication:
		return "Authentication failed. Please check your API key."
	case ErrorCategoryPermission:
		return "Permission denied. You may not have access to this resource."
	case ErrorCategoryModel:
		return "Model error. The model may be unavailable or overloaded."
	default:
		var providerErr *fantasy.ProviderError
		if errors.As(err, &providerErr) {
			return providerErr.Message
		}
		return err.Error()
	}
}

// containsAny checks if the string contains any of the substrings.
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
