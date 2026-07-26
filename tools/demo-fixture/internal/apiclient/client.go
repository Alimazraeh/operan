// Package apiclient wraps the HTTP surfaces of the Operan platform modules
// this tool talks to (Module 01 tenant-control-plane, 02 identity-access,
// 04 agent-registry, 05 department-template-engine, 08 tool-execution, 09
// human-supervision). Every request goes over the same public HTTP APIs the
// portal and other modules use — nothing here imports a module's internal
// packages or touches a database directly.
//
// Route prefixes are deliberately inconsistent across these modules (some
// bare, some /v1, some /api/v1/iam) — that inconsistency is a fact about the
// platform, verified against each module's router source, not a guess. Each
// per-module file below documents the exact prefix it uses and where that
// was confirmed.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPDoer is the subset of *http.Client this package needs. Tests supply a
// client pointed at an httptest.Server; production code uses http.Client
// with a timeout.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Doer performs authenticated JSON HTTP calls against the platform and
// turns non-2xx responses into a typed *APIError carrying enough detail
// (status, body, method, URL) to explain a restore failure without a
// cluster shell to go dig further.
type Doer struct {
	HTTP HTTPDoer
}

// NewDoer returns a Doer with a sane default timeout. Individual calls that
// need longer (template deploy fan-out, LLM drafting during replay) pass a
// context with its own deadline.
func NewDoer() *Doer {
	return &Doer{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// APIError is returned for any non-2xx HTTP response.
type APIError struct {
	Method string
	URL    string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.URL, e.Status, truncateForError(e.Body, 500))
}

// IsConflict reports whether the API refused the call because the thing
// being created already exists (HTTP 409, or the specific 400s a couple of
// these handlers use for a duplicate name). Restore logic uses this to fall
// back to a find-by-natural-key lookup instead of treating the error as
// fatal — see internal/restorecmd for why almost none of these create
// endpoints are idempotent on their own.
func IsConflict(err error) bool {
	ae, ok := err.(*APIError)
	return ok && ae.Status == http.StatusConflict
}

// IsNotFound reports whether the API answered 404.
func IsNotFound(err error) bool {
	ae, ok := err.(*APIError)
	return ok && ae.Status == http.StatusNotFound
}

func truncateForError(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Call is one authenticated JSON HTTP round trip. token may be empty for
// the two unauthenticated login endpoints. body, when non-nil, is
// JSON-encoded as the request payload. out, when non-nil, receives the
// JSON-decoded response body on 2xx. The raw response body is always
// returned as well, so callers that need a field this client's typed
// structs do not model (rare, but export's historical-invocation capture
// wants to stay tolerant of upstream additions) can parse it themselves.
func (d *Doer) Call(ctx context.Context, method, url, token, tenantID string, body, out interface{}) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body for %s %s: %w", method, url, err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build request %s %s: %w", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}

	resp, err := d.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body for %s %s: %w", method, url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, &APIError{Method: method, URL: url, Status: resp.StatusCode, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return respBody, fmt.Errorf("decode response body for %s %s: %w\nbody: %s", method, url, err, truncateForError(string(respBody), 500))
		}
	}
	return respBody, nil
}
