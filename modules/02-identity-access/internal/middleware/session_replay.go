package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SessionReplayCaptureConfig holds configurable limits for session replay capture.
type SessionReplayCaptureConfig struct {
	MaxSessions   int           // Max number of sessions to keep (0 = unlimited, default: 10000)
	MaxRequests   int           // Max requests per session (0 = unlimited, default: 5000)
	CleanupInterval time.Duration // How often background cleanup runs (0 = disabled, default: 5m)
	MaxSessionAge time.Duration // Sessions older than this are evicted during cleanup (0 = disabled, default: 24h)
}

// SessionReplayCapture stores captured HTTP request/response details
// for replaying user sessions.
type SessionReplayCapture struct {
	sessions map[string]*ReplaySession
	order    []string // Tracks insertion order for LRU eviction
	mu       sync.RWMutex
	config   SessionReplayCaptureConfig
	done     chan struct{} // Signals cleanup goroutine to stop
}

// NewSessionReplayCapture creates a new capture store with sensible defaults.
func NewSessionReplayCapture() *SessionReplayCapture {
	return NewSessionReplayCaptureWithConfig(SessionReplayCaptureConfig{})
}

// NewSessionReplayCaptureWithConfig creates a new capture store with custom limits.
func NewSessionReplayCaptureWithConfig(cfg SessionReplayCaptureConfig) *SessionReplayCapture {
	// Apply defaults
	if cfg.MaxSessions == 0 {
		cfg.MaxSessions = 10000
	}
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 5000
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}
	if cfg.MaxSessionAge == 0 {
		cfg.MaxSessionAge = 24 * time.Hour
	}

	c := &SessionReplayCapture{
		sessions: make(map[string]*ReplaySession),
		order:    make([]string, 0),
		config:   cfg,
		done:     make(chan struct{}),
	}

	// Start background cleanup if interval is positive
	if cfg.CleanupInterval > 0 {
		go c.cleanupLoop(cfg.CleanupInterval)
	}

	return c
}

// cleanupLoop runs periodic cleanup of old and evicted sessions.
func (c *SessionReplayCapture) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.CleanupOldSessions(c.config.MaxSessionAge)
		case <-c.done:
			return
		}
	}
}

// Stop shuts down the background cleanup goroutine.
func (c *SessionReplayCapture) Stop() {
	select {
	case <-c.done:
		// Already closed
	default:
		close(c.done)
	}
}

// evictOldest removes the oldest session from the capture store to make room.
func (c *SessionReplayCapture) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldestID := c.order[0]
	c.order = c.order[1:]
	delete(c.sessions, oldestID)
}

// evictOldRequests removes oldest requests from a session when it exceeds maxRequests.
func (c *SessionReplayCapture) evictOldRequests(session *ReplaySession, maxRequests int) {
	if len(session.Requests) > maxRequests {
		// Remove oldest requests
		removed := len(session.Requests) - maxRequests
		session.Requests = session.Requests[removed:]
	}
}

// ReplaySession represents a single user session's captured requests.
type ReplaySession struct {
	SessionID  string
	UserID     string
	TenantID   string
	Requests   []ReplayRequest
	StartedAt  time.Time
}

// ReplayRequest represents a single HTTP request/response pair for replay.
type ReplayRequest struct {
	Timestamp  time.Time
	Method     string
	Path       string
	Query      string
	Headers    map[string]string
	Body       []byte
	StatusCode int
	Duration   time.Duration
	Response   []byte
}

// Capture creates and returns a ReplayRequest from the incoming HTTP request.
// The caller populates StatusCode, Duration, and Response after the handler runs.
func (c *SessionReplayCapture) Capture(r *http.Request) *ReplayRequest {
	req := &ReplayRequest{
		Timestamp: time.Now().UTC(),
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Headers:   make(map[string]string),
		Body:      nil, // body not read here — client code should inject if needed
	}

	// Copy relevant headers for replay context.
	headerFields := []string{
		"Content-Type",
		"User-Agent",
		"X-Trace-ID",
		"X-Request-ID",
	}
	for _, h := range headerFields {
		val := r.Header.Get(h)
		if val != "" {
			req.Headers[h] = val
		}
	}

	return req
}

// CaptureResponse records the response details for a captured request.
func (c *SessionReplayCapture) CaptureResponse(req *ReplayRequest, w http.ResponseWriter, statusCode int, duration time.Duration, body []byte) {
	req.StatusCode = statusCode
	req.Duration = duration
	if body != nil {
		req.Response = make([]byte, len(body))
		copy(req.Response, body)
	}
}

// SaveSession stores a captured request into the session map.
// Enforces MaxRequests per session and MaxSessions total, evicting oldest on overflow.
func (c *SessionReplayCapture) SaveSession(req *ReplayRequest, sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		// Enforce max sessions — evict oldest if at capacity
		if len(c.sessions) >= c.config.MaxSessions {
			c.evictOldest()
		}
		session = &ReplaySession{
			SessionID: sessionID,
			Requests:  make([]ReplayRequest, 0),
			StartedAt: req.Timestamp,
		}
		c.sessions[sessionID] = session
		c.order = append(c.order, sessionID)
	}

	session.Requests = append(session.Requests, *req)

	// Enforce max requests per session
	if c.config.MaxRequests > 0 {
		c.evictOldRequests(session, c.config.MaxRequests)
	}
}

// SetSessionUser sets the user context on an existing or new session.
// Creates a new session if one doesn't exist (subject to limits).
func (c *SessionReplayCapture) SetSessionUser(sessionID, userID, tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		// Enforce max sessions — evict oldest if at capacity
		if len(c.sessions) >= c.config.MaxSessions {
			c.evictOldest()
		}
		c.sessions[sessionID] = &ReplaySession{
			SessionID: sessionID,
			UserID:    userID,
			TenantID:  tenantID,
			Requests:  make([]ReplayRequest, 0),
			StartedAt: time.Now().UTC(),
		}
		c.order = append(c.order, sessionID)
		return
	}

	if userID != "" {
		session.UserID = userID
	}
	if tenantID != "" {
		session.TenantID = tenantID
	}
}

// GetSession retrieves a replay session by its ID.
func (c *SessionReplayCapture) GetSession(sessionID string) *ReplaySession {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		return nil
	}

	// Return a shallow copy so the caller can't mutate internal state.
	return &ReplaySession{
		SessionID: session.SessionID,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Requests:  append([]ReplayRequest(nil), session.Requests...),
		StartedAt: session.StartedAt,
	}
}

// ListSessions returns all active session IDs for listing endpoints.
func (c *SessionReplayCapture) ListSessions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		ids = append(ids, id)
	}
	return ids
}

// DeleteSession removes a session from the capture store.
func (c *SessionReplayCapture) DeleteSession(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.sessions, sessionID)
	// Maintain order slice
	for i, id := range c.order {
		if id == sessionID {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// CleanupOldSessions removes sessions that have not received activity
// within the provided duration. Returns the number of sessions removed.
func (c *SessionReplayCapture) CleanupOldSessions(maxAge time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var removed int
	cutoff := time.Now().UTC().Add(-maxAge)
	for id, session := range c.sessions {
		if len(session.Requests) > 0 {
			last := session.Requests[len(session.Requests)-1].Timestamp
			if last.Before(cutoff) {
				delete(c.sessions, id)
				removed++
			}
		} else if session.StartedAt.Before(cutoff) {
			delete(c.sessions, id)
			removed++
		}
	}
	// Rebuild order slice to remove evicted entries
	if removed > 0 {
		newOrder := make([]string, 0, len(c.order)-removed)
		for _, id := range c.order {
			if _, exists := c.sessions[id]; exists {
				newOrder = append(newOrder, id)
			}
		}
		c.order = newOrder
	}
	return removed
}

// responseWrapper wraps http.ResponseWriter to capture the status code.
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SessionReplayMiddleware intercepts requests and captures HTTP details
// for session replay. Requests must include an X-Session-ID header.
func SessionReplayMiddleware(capture *SessionReplayCapture) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := r.Header.Get("X-Session-ID")
			if sessionID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Extract user context from JWT for session enrichment.
			userID := GetUserID(r.Context())
			tenantID := GetTenantID(r.Context())
			if userID != "" || tenantID != "" {
				capture.SetSessionUser(sessionID, userID, tenantID)
			}

			// Wrap response writer to capture status code.
			wrapped := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}

			startTime := time.Now()
			next.ServeHTTP(wrapped, r)
			duration := time.Since(startTime)

			// Capture request details.
			reqCapture := capture.Capture(r)
			reqCapture.StatusCode = wrapped.statusCode
			reqCapture.Duration = duration

			// Save the captured request into the session.
			capture.SaveSession(reqCapture, sessionID)
		})
	}
}

// SanitizeQuery redacts common sensitive query parameters from replay data.
func SanitizeQuery(raw string) string {
	parsed, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	sensitive := []string{"password", "token", "secret", "key", "api_key", "authorization"}
	for k := range parsed {
		kl := strings.ToLower(k)
		for _, s := range sensitive {
			if strings.Contains(kl, s) {
				parsed.Set(k, "REDACTED")
				break
			}
		}
	}
	return parsed.Encode()
}
