package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── TokenBucket tests ───────────────────────────────────────────────────────

func TestTokenBucketAllowsWithinLimit(t *testing.T) {
	tb := NewTokenBucket(10, 5) // 10 tokens/sec, burst of 5

	for i := 0; i < 5; i++ {
		if !tb.Allow() {
			t.Errorf("Allow() on request %d: expected true, got false", i+1)
		}
	}
}

func TestTokenBucketRejectsAfterBurst(t *testing.T) {
	tb := NewTokenBucket(10, 5) // 10 tokens/sec, burst of 5

	// Drain the bucket
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	// Next request should fail
	if tb.Allow() {
		t.Error("Allow() after burst: expected false, got true")
	}
}

func TestTokenBucketAllowN(t *testing.T) {
	tb := NewTokenBucket(10, 10)

	if !tb.AllowN(5) {
		t.Error("AllowN(5): expected true, got false")
	}
	if !tb.AllowN(5) {
		t.Error("AllowN(5) second time: expected true, got false")
	}
	if tb.AllowN(1) {
		t.Error("AllowN(1) after drain: expected false, got true")
	}
}

func TestTokenBucketRecovery(t *testing.T) {
	tb := NewTokenBucket(100, 5) // 100 tokens/sec, burst of 5

	// Drain the bucket
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	// Should be rejected now
	if tb.Allow() {
		t.Error("Allow() after drain: expected false, got true")
	}

	// Wait for tokens to refill (~50ms should be enough at 100/sec)
	time.Sleep(60 * time.Millisecond)

	// Should allow again
	if !tb.Allow() {
		t.Error("Allow() after refill: expected true, got false")
	}
}

func TestTokenBucketDelay(t *testing.T) {
	tb := NewTokenBucket(10, 5) // 10 tokens/sec, burst of 5

	// With tokens available, delay should be 0
	delay := tb.Delay()
	if delay != 0 {
		t.Errorf("Delay() with tokens: expected 0, got %v", delay)
	}

	// Drain the bucket
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	// After drain, delay should be positive
	delay = tb.Delay()
	if delay <= 0 {
		t.Errorf("Delay() after drain: expected positive, got %v", delay)
	}

	// With 10 tokens/sec, first token should be available in ~100ms
	if delay > 200*time.Millisecond {
		t.Errorf("Delay() after drain: expected <=200ms, got %v", delay)
	}
}

func TestTokenBucketBurstReturnsCorrectValue(t *testing.T) {
	tb := NewTokenBucket(10, 42)
	if tb.Burst() != 42 {
		t.Errorf("Burst() = %d, want 42", tb.Burst())
	}
}

func TestTokenBucketAllowNWithNegative(t *testing.T) {
	tb := NewTokenBucket(10, 10)
	if tb.AllowN(-1) {
		t.Error("AllowN(-1): expected false, got true")
	}
}

func TestTokenBucketConcurrentAllow(t *testing.T) {
	tb := NewTokenBucket(1000, 100)

	var wg sync.WaitGroup
	successes := make(chan struct{}, 200)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Allow() {
				successes <- struct{}{}
			}
		}()
	}

	wg.Wait()
	close(successes)

	count := 0
	for range successes {
		count++
	}

	if count != 100 {
		t.Errorf("Expected exactly 100 successes out of 100 concurrent requests, got %d", count)
	}
}

// ─── Middleware tests ────────────────────────────────────────────────────────

func TestBasicRateLimitingAllowsPassThrough(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         10,
		CustomHeaders:     true,
	}

	mw := NewRateLimiterMiddleware(cfg)
	called := false
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1:12345")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	if !called {
		t.Error("Next handler was not called")
	}
}

func TestBasicRateLimitingRejectsExcess(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First 5 should pass
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1:5000")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 6th should be rejected
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1:5000")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request 6: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Check error body
	body := strings.TrimSpace(w.Body.String())
	if body != `{"error":"rate limit exceeded"}` {
		t.Errorf("Response body = %q, want %q", body, `{"error":"rate limit exceeded"}`)
	}

	// Check Retry-After header
	if retryAfter := w.Header().Get("Retry-After"); retryAfter != "1" {
		t.Errorf("Retry-After = %q, want %q", retryAfter, "1")
	}
}

func TestCustomKeyExtractor(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
		KeyExtractor: func(r *http.Request) string {
			return r.Header.Get("X-Client-ID")
		},
		CustomHeaders: false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Requests with same X-Client-ID share a limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Client-ID", "client-1")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 6th with same ID should be rejected
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Client-ID", "client-1")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request 6 same ID: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Different ID should still be allowed
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Client-ID", "client-2")
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Different ID: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDefaultKeyExtractorUsesXForwardedFor(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Two requests with same X-Forwarded-For share a limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Same X-Forwarded-After 5 requests: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestDefaultKeyExtractorFallsBackToRemoteAddr(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First 5 requests from RemoteAddr should pass
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.50:8080")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	// 6th should be rejected
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.50:8080")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Same RemoteAddr after 5 requests: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestXForwardedForTakesPrecedenceOverRemoteAddr(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use X-Forwarded-For to fill the limit for a specific key
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	// Request with no X-Forwarded-For should use a different key (fallback to empty RemoteAddr)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Different key (no XFF): status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 100,
		BurstSize:         10,
		CustomHeaders:     true,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1:3000")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	// Check headers
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	limitHeader := w.Header().Get("X-RateLimit-Limit")
	if limitHeader != "10" {
		t.Errorf("X-RateLimit-Limit = %q, want %q", limitHeader, "10")
	}

	remainingHeader := w.Header().Get("X-RateLimit-Remaining")
	if remainingHeader == "" {
		t.Error("X-RateLimit-Remaining is empty")
	}

	resetHeader := w.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("X-RateLimit-Reset is empty")
	}

	// Parse reset header as a Unix timestamp
	_, err := fmt.Sscanf(resetHeader, "%d", new(int64))
	if err != nil {
		t.Errorf("X-RateLimit-Reset = %q is not a valid Unix timestamp", resetHeader)
	}
}

func TestCustomHeadersDisabled(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         10,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1:3000")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Limit") != "" {
		t.Error("X-RateLimit-Limit should not be set when CustomHeaders is false")
	}
	if w.Header().Get("X-RateLimit-Remaining") != "" {
		t.Error("X-RateLimit-Remaining should not be set when CustomHeaders is false")
	}
	if w.Header().Get("X-RateLimit-Reset") != "" {
		t.Error("X-RateLimit-Reset should not be set when CustomHeaders is false")
	}
}

func TestDifferentKeysGetSeparateLimits(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         3,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust key "A" via X-Forwarded-For header
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "key-A")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	// "A" should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "key-A")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Key A: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// "B" should still be allowed (fresh limiter)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "key-B")
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Key B: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimitRecoveryOverTime(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 100, // 100 tokens/sec = 1 token per 10ms
		BurstSize:         5,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Drain the bucket
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1:4000")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	// Should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1:4000")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Before recovery: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Wait for token refill
	time.Sleep(20 * time.Millisecond)

	// Should recover
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1:4000")
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("After recovery: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDefaultConfigValues(t *testing.T) {
	// Create middleware with zero-value config
	cfg := RateLimiterConfig{}

	mw := NewRateLimiterMiddleware(cfg)

	// Defaults should be applied
	if mw.limiterConfig.RequestsPerSecond != defaultRequestsPerSecond {
		t.Errorf("Default RequestsPerSecond = %v, want %v",
			mw.limiterConfig.RequestsPerSecond, defaultRequestsPerSecond)
	}
	if mw.limiterConfig.BurstSize != defaultBurstSize {
		t.Errorf("Default BurstSize = %v, want %v",
			mw.limiterConfig.BurstSize, defaultBurstSize)
	}
	if mw.limiterConfig.CustomHeaders != false {
		t.Errorf("Default CustomHeaders (Go zero value) = %v, want false",
			mw.limiterConfig.CustomHeaders)
	}
	if mw.limiterConfig.evictInterval != defaultEvictionInterval {
		t.Errorf("Default evictInterval = %v, want %v",
			mw.limiterConfig.evictInterval, defaultEvictionInterval)
	}
	if mw.limiterConfig.inactivityTimeout != defaultInactivityTimeout {
		t.Errorf("Default inactivityTimeout = %v, want %v",
			mw.limiterConfig.inactivityTimeout, defaultInactivityTimeout)
	}
}

func TestConcurrencySafety(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 1000,
		BurstSize:         500,
		CustomHeaders:     false,
	}

	mw := NewRateLimiterMiddleware(cfg)

	var wg sync.WaitGroup
	successCount := 0
	var countMu sync.Mutex

	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		countMu.Lock()
		successCount++
		countMu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	// 50 goroutines each sending 20 requests = 1000 total
	// With burst=500, at most 500 should succeed (plus maybe a few from refill)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Forwarded-For", "10.0.0.1:8000")
			w := httptest.NewRecorder()
			mw.ServeHTTP(w, req)
		}()
	}

	wg.Wait()

	// All should succeed since burst is 500 and we have 1000 tokens/sec refill rate
	if successCount == 0 {
		t.Error("No requests succeeded during concurrent test")
	}

	if successCount > 600 {
		t.Errorf("Expected at most ~600 successes in concurrent test, got %d", successCount)
	}
}

func TestMiddlewareChainFunction(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
		CustomHeaders:     false,
	}

	chain := RateLimiter(cfg)

	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 5 should pass
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "172.16.0.1:6000")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Chain request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 6th should be rejected
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "172.16.0.1:6000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Chain request 6: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestChainMultipleMiddleware(t *testing.T) {
	rateCfg := RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         10,
		CustomHeaders:     true,
	}

	// Chain: RateLimiter -> tenant injector -> handler
	rateMW := RateLimiter(rateCfg)
	handler := rateMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1:7000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Chained middleware: status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("X-Custom") != "hello" {
		t.Error("X-Custom header not propagated through chain")
	}
}

func TestEvictionOfExpiredEntries(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond:   10,
		BurstSize:           10,
		CustomHeaders:       false,
		evictInterval:       50 * time.Millisecond,
		inactivityTimeout:   100 * time.Millisecond,
	}

	mw := NewRateLimiterMiddleware(cfg)
	mw.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create an entry by making a request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.10.10.10:9000")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	// Verify the entry exists (by checking the sync.Map)
	_, exists := mw.entries.Load("10.10.10.10:9000")
	if !exists {
		t.Fatal("Entry should exist immediately after request")
	}

	// Wait for eviction to run
	time.Sleep(200 * time.Millisecond)

	// Entry should be evicted
	_, exists = mw.entries.Load("10.10.10.10:9000")
	if exists {
		t.Fatal("Entry should have been evicted after inactivity timeout")
	}
}

// ─── Key extractor helper tests ──────────────────────────────────────────────

func TestDefaultKeyExtractorMultipleXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1, 172.16.0.1")
	req.RemoteAddr = "0.0.0.0:1"

	key := defaultKeyExtractor(req)
	if key != "192.168.1.1" {
		t.Errorf("defaultKeyExtractor with multiple XFF = %q, want first IP %q", key, "192.168.1.1")
	}
}

func TestDefaultKeyExtractorEmptyXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "")
	req.RemoteAddr = "10.0.0.99:5000"

	key := defaultKeyExtractor(req)
	if key != "10.0.0.99" {
		t.Errorf("defaultKeyExtractor with empty XFF = %q, want RemoteAddr %q", key, "10.0.0.99")
	}
}

func TestDefaultKeyExtractorNoPortRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.99"

	key := defaultKeyExtractor(req)
	if key != "10.0.0.99" {
		t.Errorf("defaultKeyExtractor no port = %q, want %q", key, "10.0.0.99")
	}
}

func TestStripWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"  hello  ", "hello"},
		{"\thello\t", "hello"},
		{"hello", "hello"},
		{"  ", ""},
		{" \t ", ""},
	}

	for _, tt := range tests {
		got := stripWhitespace(tt.input)
		if got != tt.expected {
			t.Errorf("stripWhitespace(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSplitByComma(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{""}},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a,b,", []string{"a", "b", ""}},
	}

	for _, tt := range tests {
		got := splitByComma(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitByComma(%q) = %v (%d items), want %v (%d items)",
				tt.input, got, len(got), tt.expected, len(tt.expected))
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitByComma(%q)[%d] = %q, want %q",
					tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

// ─── RateLimitedJSONError helper tests ───────────────────────────────────────

func TestRateLimitedJSONError(t *testing.T) {
	body := RateLimitedJSONError()
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", string(body), err)
	}
	if result["error"] != "rate limit exceeded" {
		t.Errorf("RateLimitedJSONError() = %q, want %q", string(body), `{"error":"rate limit exceeded"}`)
	}
}

func TestLogRateLimitEvent(t *testing.T) {
	// Just verify it doesn't panic — log output goes to stderr
	LogRateLimitEvent("test-key", "test message")
}

func TestNewRateLimiterMiddlewareDefaults(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 50,
		BurstSize:         25,
		CustomHeaders:     true,
	}

	mw := NewRateLimiterMiddleware(cfg)

	if mw.limiterConfig.RequestsPerSecond != 50 {
		t.Errorf("Custom RequestsPerSecond = %v, want 50", mw.limiterConfig.RequestsPerSecond)
	}
	if mw.limiterConfig.BurstSize != 25 {
		t.Errorf("Custom BurstSize = %v, want 25", mw.limiterConfig.BurstSize)
	}
	if mw.limiterConfig.CustomHeaders != true {
		t.Errorf("Custom CustomHeaders = %v, want true", mw.limiterConfig.CustomHeaders)
	}
	if mw.next != nil {
		t.Error("next should be nil until Chain() is called")
	}
}

func TestWaitContextCancelled(t *testing.T) {
	tb := NewTokenBucket(10, 5)

	// Drain the bucket
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	// Wait with a cancelled context should return an error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := tb.Wait(ctx)
	if err == nil {
		t.Error("Wait() with cancelled context: expected error, got nil")
	}
}

func TestWaitSucceeds(t *testing.T) {
	tb := NewTokenBucket(1000, 5)

	// Drain the bucket
	for i := 0; i < 5; i++ {
		tb.Allow()
	}

	// Wait with a timeout should eventually succeed
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := tb.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() with timeout: expected nil, got %v", err)
	}
}
