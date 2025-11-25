// Package ratelimit provides rate limiting functionality for API requests.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter combines request-based and token-based rate limiting.
type Limiter struct {
	requestLimiter *rate.Limiter
	tokenLimiter   *rate.Limiter
	mu             sync.Mutex
}

// New creates a new rate limiter with the specified limits.
// requestsPerMinute controls the number of requests per minute (0 = unlimited).
// tokensPerMinute controls the number of tokens per minute (0 = unlimited).
func New(requestsPerMinute, tokensPerMinute int64) *Limiter {
	l := &Limiter{}

	if requestsPerMinute > 0 {
		// Convert requests per minute to requests per second
		rps := float64(requestsPerMinute) / 60.0
		// Allow burst of 1 to smooth out variations
		l.requestLimiter = rate.NewLimiter(rate.Limit(rps), 1)
	}

	if tokensPerMinute > 0 {
		// Convert tokens per minute to tokens per second
		tps := float64(tokensPerMinute) / 60.0
		// Allow burst of up to 1 second worth of tokens
		burst := int(tps) + 1
		if burst < 1 {
			burst = 1
		}
		l.tokenLimiter = rate.NewLimiter(rate.Limit(tps), burst)
	}

	return l
}

// Wait blocks until the limiter permits the request to proceed.
// It returns an error if the context is canceled.
func (l *Limiter) Wait(ctx context.Context) error {
	if l.requestLimiter != nil {
		if err := l.requestLimiter.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// WaitTokens blocks until the limiter permits n tokens to be consumed.
// It returns an error if the context is canceled.
func (l *Limiter) WaitTokens(ctx context.Context, n int64) error {
	if l.tokenLimiter != nil {
		if err := l.tokenLimiter.WaitN(ctx, int(n)); err != nil {
			return err
		}
	}
	return nil
}

// Allow reports whether a request may happen now.
func (l *Limiter) Allow() bool {
	if l.requestLimiter != nil {
		return l.requestLimiter.Allow()
	}
	return true
}

// AllowTokens reports whether n tokens may be consumed now.
func (l *Limiter) AllowTokens(n int64) bool {
	if l.tokenLimiter != nil {
		return l.tokenLimiter.AllowN(time.Now(), int(n))
	}
	return true
}

// Reserve returns a Reservation that indicates how long the caller must wait
// before the request can proceed.
func (l *Limiter) Reserve() *rate.Reservation {
	if l.requestLimiter != nil {
		return l.requestLimiter.Reserve()
	}
	return nil
}

// ReserveTokens returns a Reservation that indicates how long the caller must wait
// before n tokens can be consumed.
func (l *Limiter) ReserveTokens(n int64) *rate.Reservation {
	if l.tokenLimiter != nil {
		return l.tokenLimiter.ReserveN(time.Now(), int(n))
	}
	return nil
}

// Enabled returns true if rate limiting is enabled.
func (l *Limiter) Enabled() bool {
	return l.requestLimiter != nil || l.tokenLimiter != nil
}
