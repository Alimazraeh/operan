package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SessionReplayCapture stores captured HTTP request/response details
// for replaying user sessions.
type SessionReplayCapture struct {
	sessions map[string]*ReplaySession
	mu       sync.RWMutex
}

// NewSessionReplayCapture creates a new capture store with a default max size.
func NewSessionReplayCapture() *SessionReplayCapture {
	return &SessionReplayCapture{
		sessions: make(map[string]*ReplaySession),
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
// If the session does not exist, it creates a new one with the request details.
func (c *SessionReplayCapture) SaveSession(req *ReplayRequest, sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		session = &ReplaySession{
			SessionID: sessionID,
			Requests:  make([]ReplayRequest, 0),
			StartedAt: req.Timestamp,
		}
		c.sessions[sessionID] = session
	}

	session.Requests = append(session.Requests, *req)
}

// SetSessionUser sets the user context on an existing or new session.
func (c *SessionReplayCapture) SetSessionUser(sessionID, userID, tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		c.sessions[sessionID] = &ReplaySession{
			SessionID: sessionID,
			UserID:    userID,
			TenantID:  tenantID,
			Requests:  make([]ReplayRequest, 0),
			StartedAt: time.Now().UTC(),
		}
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
