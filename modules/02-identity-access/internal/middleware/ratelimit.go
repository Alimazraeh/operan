// Package middleware provides HTTP middleware for the Operan IAM module.
// This file implements token bucket rate limiting using the standard library.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	defaultRequestsPerSecond = 100
	defaultBurstSize         = 200
	defaultEvictionInterval  = time.Minute
	defaultInactivityTimeout = 10 * time.Minute
	defaultCustomHeaders     = true
)

// TokenBucket implements a thread-safe token bucket algorithm using only
// standard library primitives. It provides the same API surface as
// golang.org/x/time/rate.Limiter (Allow, AllowN, Wait, Delay, Burst).
//
// This was implemented as a drop-in replacement when the external
// golang.org/x/time/rate dependency could not be fetched.
type TokenBucket struct {
	mu       sync.Mutex
	rate     float64    // tokens per second
	burst    int        // max tokens
	tokens   float64    // current tokens
	lastTime time.Time  // last refill time
}

// NewTokenBucket creates a new token bucket rate limiter with the given
// rate (tokens per second) and burst size.
func NewTokenBucket(rate float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

// refill adds tokens based on elapsed time since the last refill.
// Must be called while holding tb.mu.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
}

// Allow reports whether one request is allowed right now.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN reports whether n requests are allowed right now.
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if n < 0 {
		return false
	}

	tb.refill()
	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

// Burst returns the burst size of this limiter.
func (tb *TokenBucket) Burst() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.burst
}

// Wait blocks until one token is available or the context is cancelled.
// It returns nil if a token was acquired, or the context error otherwise.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	tb.mu.Lock()
	delay := tb.maxDelayLocked()
	tb.mu.Unlock()

	if delay <= 0 {
		return nil
	}

	t := time.NewTimer(delay)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// maxDelayLocked returns the maximum delay before the next request can be made.
// Must be called while holding tb.mu.
func (tb *TokenBucket) maxDelayLocked() time.Duration {
	tb.refill()
	if tb.tokens >= 1 {
		return 0
	}
	tokensNeeded := 1 - tb.tokens
	return time.Duration(tokensNeeded / tb.rate * float64(time.Second))
}

// Delay returns the maximum delay before the next request can be made.
func (tb *TokenBucket) Delay() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.maxDelayLocked()
}

// ─── RateLimiterConfig ───────────────────────────────────────────────────────

// RateLimiterConfig holds configuration for the rate limiting middleware.
type RateLimiterConfig struct {
	// RequestsPerSecond is the sustained rate of allowed requests.
	// Default: 100
	RequestsPerSecond float64

	// BurstSize is the maximum number of requests allowed in a burst.
	// Default: 200
	BurstSize int

	// KeyExtractor extracts a client identifier from the request.
	// If nil, defaults to extracting from X-Forwarded-For header,
	// falling back to RemoteAddr.
	KeyExtractor func(r *http.Request) string

	// CustomHeaders controls whether rate limit headers are set on responses.
	// Default: true
	CustomHeaders bool

	// evictInterval controls the cleanup goroutine frequency.
	// Used internally for testing.
	evictInterval time.Duration

	// inactivityTimeout controls how long an unused entry lives.
	// Used internally for testing.
	inactivityTimeout time.Duration
}

// RateLimitEntry wraps a token bucket with its last-access timestamp.
type RateLimitEntry struct {
	Limiter    *TokenBucket
	LastAccess time.Time
}

// RateLimiterMiddleware provides per-client token bucket rate limiting.
type RateLimiterMiddleware struct {
	limiterConfig RateLimiterConfig
	entries       sync.Map // key -> *RateLimitEntry
	next          http.Handler
}

// NewRateLimiterMiddleware creates a new rate limiting middleware.
func NewRateLimiterMiddleware(cfg RateLimiterConfig) *RateLimiterMiddleware {
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = defaultRequestsPerSecond
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = defaultBurstSize
	}
	if cfg.KeyExtractor == nil {
		cfg.KeyExtractor = defaultKeyExtractor
	}
	if cfg.evictInterval <= 0 {
		cfg.evictInterval = defaultEvictionInterval
	}
	if cfg.inactivityTimeout <= 0 {
		cfg.inactivityTimeout = defaultInactivityTimeout
	}

	mw := &RateLimiterMiddleware{
		limiterConfig: cfg,
	}

	// Start background eviction goroutine
	go mw.runEvictionLoop()

	return mw
}

// RunEvictionLoop runs the cleanup goroutine. Exposed for testing.
func (m *RateLimiterMiddleware) RunEvictionLoop() {
	m.runEvictionLoop()
}

func (m *RateLimiterMiddleware) runEvictionLoop() {
	ticker := time.NewTicker(m.limiterConfig.evictInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.evictExpired()
	}
}

func (m *RateLimiterMiddleware) evictExpired() {
	cutoff := time.Now().Add(-m.limiterConfig.inactivityTimeout)

	m.entries.Range(func(key, value interface{}) bool {
		entry := value.(*RateLimitEntry)
		if entry.LastAccess.Before(cutoff) {
			m.entries.Delete(key)
		}
		return true
	})
}

// defaultKeyExtractor extracts a client identifier from the request.
func defaultKeyExtractor(r *http.Request) string {
	// Try X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		parts := splitHeaderValues(xff)
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}
	// Fall back to RemoteAddr (strip port if present)
	addr := r.RemoteAddr
	if idx := len(addr); idx > 0 {
		for i := idx - 1; i >= 0; i-- {
			if addr[i] == ':' {
				return addr[:i]
			}
		}
	}
	return addr
}

// splitHeaderValues splits a comma-separated header value, trimming whitespace.
func splitHeaderValues(s string) []string {
	parts := make([]string, 0, 1)
	for _, p := range splitByComma(s) {
		if trimmed := stripWhitespace(p); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitByComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func stripWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

// ServeHTTP implements http.Handler. It is the middleware entrypoint.
func (m *RateLimiterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := m.limiterConfig.KeyExtractor(r)

	// Get or create the rate limiter for this key
	entryIface, loaded := m.entries.LoadOrStore(key, &RateLimitEntry{
		Limiter:    NewTokenBucket(m.limiterConfig.RequestsPerSecond, m.limiterConfig.BurstSize),
		LastAccess: time.Now(),
	})
	entry := entryIface.(*RateLimitEntry)
	if loaded {
		// Update last access timestamp
		entry.LastAccess = time.Now()
	}

	// Check rate limit
	if !entry.Limiter.Allow() {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limit exceeded"}`)
		return
	}

	// Add rate limit headers if enabled
	if m.limiterConfig.CustomHeaders {
		entry.Limiter.mu.Lock()
		tokens := entry.Limiter.tokens
		entry.Limiter.mu.Unlock()

		remaining := int(tokens)
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", m.limiterConfig.BurstSize))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(entry.Limiter.Delay()).Unix()))
	}

	if m.next != nil {
		m.next.ServeHTTP(w, r)
	}
}

// Chain wraps the middleware as a standard middleware function.
func (m *RateLimiterMiddleware) Chain(next http.Handler) http.Handler {
	m.next = next
	return m
}

// RateLimiter is a convenience wrapper that creates a middleware function
// compatible with the standard middleware pattern:
// func RateLimiter(cfg RateLimiterConfig) func(http.Handler) http.Handler.
//
// It is a functional wrapper around NewRateLimiterMiddleware.
func RateLimiter(cfg RateLimiterConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		mw := NewRateLimiterMiddleware(cfg)
		mw.next = next
		return mw
	}
}

// RateLimitHeadersResponse represents the rate limit headers returned by the middleware.
type RateLimitHeadersResponse struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"`
}

// LogRateLimitEvent logs a rate limit event via the Go logger.
func LogRateLimitEvent(key string, message string) {
	log.Printf("[ratelimit] %s: %s", key, message)
}

// RateLimitedJSONError marshals the rate limited error response.
func RateLimitedJSONError() []byte {
	body, _ := json.Marshal(map[string]string{"error": "rate limit exceeded"})
	return body
}
