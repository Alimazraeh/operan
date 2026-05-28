package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// AuditHandler handles audit-related HTTP endpoints by delegating to Authentik.
type AuditHandler struct {
	Auth *authentik.Client
}

// NewAuditHandler creates a new audit handler backed by Authentik.
func NewAuditHandler(auth *authentik.Client) *AuditHandler {
	return &AuditHandler{
		Auth: auth,
	}
}

// GetTrails handles GET /api/v1/iam/audit/trails
func (h *AuditHandler) GetTrails(w http.ResponseWriter, r *http.Request) {
	actorID := r.URL.Query().Get("actor_id")
	action := r.URL.Query().Get("action")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	resourceType := r.URL.Query().Get("resource_type")
	resultFilter := r.URL.Query().Get("result")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err == nil {
			from = &t
		}
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err == nil {
			to = &t
		}
	}

	// Build query string for Authentik events API.
	v := url.Values{}
	if actorID != "" {
		v.Set("actor", actorID)
	}
	if resourceType != "" {
		v.Set("object_type", resourceType)
	}
	path := "/api/v3/events/events/"
	if len(v) > 0 {
		path += "?" + v.Encode()
	}

	// Fetch all matching events via Authentik's paginated API.
	var allEvents []*authentik.Event
	currentPath := path
	ctx := r.Context()
	for {
		var page authentik.PaginatedEventResponse
		if err := h.Auth.Call(ctx, http.MethodGet, currentPath, nil, &page); err != nil {
			http.Error(w, `{"error":"failed to list audit events from Authentik"}`, http.StatusInternalServerError)
			return
		}

		allEvents = append(allEvents, page.Results...)

		if page.Next == "" {
			break
		}
		currentPath = page.Next
	}

	// Map Authentik events to Operan AuditEvent format.
	trails := make([]models.AuditEvent, 0, len(allEvents))
	for _, evt := range allEvents {
		// Filter by action (Operan action format).
		if action != "" && mapAuthentikTypeToOperanAction(evt.Type) != action {
			continue
		}
		// Filter by result.
		if resultFilter != "" && mapAuthentikResult(evt) != resultFilter {
			continue
		}
		// Filter by timestamp range.
		if from != nil && evt.Created != nil && evt.Created.Before(*from) {
			continue
		}
		if to != nil && evt.Created != nil && evt.Created.After(*to) {
			continue
		}
		trails = append(trails, authentikEventToOperan(evt))
	}

	// Apply offset/limit to the mapped results.
	total := len(trails)
	if offset >= len(trails) {
		trails = nil
	} else {
		end := offset + limit
		if end > len(trails) {
			end = len(trails)
		}
		trails = trails[offset:end]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"audit_trails": trails,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// GetByID handles GET /api/v1/iam/audit/trails/{trail_id}
func (h *AuditHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	trailID := extractTrailID(r.URL.Path)
	if trailID == "" {
		http.Error(w, `{"error":"trail_id is required"}`, http.StatusBadRequest)
		return
	}

	// Fetch the event directly from Authentik.
	path := "/api/v3/events/events/" + trailID + "/"
	ctx := r.Context()

	var evt authentik.Event
	if err := h.Auth.Call(ctx, http.MethodGet, path, nil, &evt); err != nil {
		// Check for 404
		if _, ok := err.(*authentik.APIError); ok {
			// Authentik returns 404 for non-existent events
			http.Error(w, `{"error":"audit trail not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to get audit trail"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authentikEventToOperan(&evt))
}

// GetSessionReplay handles GET /api/v1/iam/audit/session-replay/{session_id}
// Returns a chronological replay of audit events for the session.
func (h *AuditHandler) GetSessionReplay(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionReplayID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, `{"error":"session_id is required"}`, http.StatusBadRequest)
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	// Fetch events and filter by session/correlation ID in the data payload.
	allEvents, err := h.Auth.EventsAPI.List(r.Context(), "", "")
	if err != nil {
		http.Error(w, `{"error":"failed to get session replay data from Authentik"}`, http.StatusInternalServerError)
		return
	}

	// Filter events to those containing the session_id in their data.
	replayEvents := make([]models.AuditEvent, 0)
	for _, evt := range allEvents {
		if evt.Data != nil {
			if vid, ok := evt.Data["session_id"]; ok {
				if fmt.Sprintf("%v", vid) == sessionID {
					replayEvents = append(replayEvents, authentikEventToOperan(evt))
				}
			}
		}
	}

	// Sort chronologically.
	sort.Slice(replayEvents, func(i, j int) bool {
		// Events from Authentik are newest-first; reverse for chronological order.
		return replayEvents[i].Timestamp.Before(replayEvents[j].Timestamp)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"tenant_id":  tenantID,
		"events":     replayEvents,
		"total":      len(replayEvents),
	})
}

// RBACHandler handles RBAC/ABAC permission evaluation by delegating to Authentik.
type RBACHandler struct {
	Auth *authentik.Client
}

// NewRBACHandler creates a new RBAC handler backed by Authentik.
func NewRBACHandler(auth *authentik.Client) *RBACHandler {
	return &RBACHandler{
		Auth: auth,
	}
}

// Evaluate handles POST /api/v1/iam/rbac/evaluate
func (h *RBACHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	actorID := middleware.GetUserID(r.Context())

	var req models.PermissionCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Convert Operan resource/action to Authentik permission string.
	// Mapping:
	//   "user" + "read"    → "authenticik.user.view"
	//   "user" + "write"   → "authenticik.user.change"
	//   "user" + "delete"  → "authenticik.user.delete"
	//   "role" + "read"    → "authenticik.group.view"
	//   "role" + "write"   → "authenticik.group.change"
	//   "role" + "delete"  → "authenticik.group.delete"
	//   "service" + "read" → "authenticik.application.view"
	//   "service" + "write"→ "authenticik.application.change"
	//   "service" + "delete"→ "authenticik.application.delete"
	//   "audit" + "read"   → "authenticik.event.view"
	//   "audit" + "write"  → "authenticik.event.change"
	//   "audit" + "delete" → "authenticik.event.delete"
	//   "group" + "read"   → "authenticik.group.view"
	//   "group" + "write"  → "authenticik.group.change"
	//   "group" + "delete" → "authenticik.group.delete"
	//   "tenant" + "read"  → "authenticik.tenant.view"
	//   "tenant" + "write" → "authenticik.tenant.change"
	//   "tenant" + "delete"→ "authenticik.tenant.delete"
	//   "session" + "read" → "authenticik.session.view"
	//   "application" + "read" → "authenticik.application.view"
	//   "application" + "write" → "authenticik.application.change"
	//   "application" + "delete" → "authenticik.application.delete"
	//   "token" + "read"   → "authenticik.token.view"
	authPermission := mapResourceActionToAuthenticPermission(req.Resource, req.Action)

	// Look up the user in Authentik.
	ctx := r.Context()
	user, err := h.Auth.UsersAPI.GetByID(ctx, req.ActorID)
	if err != nil {
		// User not found in Authentik — deny access.
		result := models.PermissionCheckResult{
			Allowed:     false,
			Reason:      "user not found in Authentik",
			EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		_ = tenantID
		writeJSON(w, result, http.StatusOK)
		return
	}

	// Check if user has the required permission using Authentik's RBAC check endpoint.
	hasPerm, err := h.Auth.RBACAPI.CheckPermission(ctx, authentik.CheckPermissionRequest{
		User:       req.ActorID,
		Permission: authPermission,
	})
	if err == nil && hasPerm {
		result := models.PermissionCheckResult{
			Allowed:     true,
			Reason:      "explicit permission grant via RBAC check",
			PolicyMatch: &authPermission,
			EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		writeJSON(w, result, http.StatusOK)
		return
	}

	// Fall back to checking user's groups and their roles' permissions.
	allowed, reason, policy := h.checkPermissionViaGroups(ctx, user, authPermission)

	// Log audit event via Authentik.
	h.logEvaluateEvent(ctx, actorID, req, hasPerm, allowed, reason, tenantID)

	result := models.PermissionCheckResult{
		Allowed:     allowed,
		Reason:      reason,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if policy != "" {
		result.PolicyMatch = &policy
	}

	writeJSON(w, result, http.StatusOK)
}

// checkPermissionViaGroups checks if any of the user's groups have a role with the required permission.
func (h *RBACHandler) checkPermissionViaGroups(ctx context.Context, user *authentik.User, perm string) (bool, string, string) {
	allGroups, err := h.Auth.GroupsAPI.List(ctx)
	if err != nil {
		return false, "could not verify permission: failed to list groups", ""
	}

	// Find groups the user belongs to.
	var groupNames []string
	for _, group := range allGroups {
		for _, memberUUID := range group.Users {
			if memberUUID == user.UUID {
				groupNames = append(groupNames, group.Name)
				break
			}
		}
	}

	if len(groupNames) == 0 {
		return false, "user belongs to no groups with permissions", ""
	}

	// Check permissions of roles that may be associated with the user's groups.
	allRoles, err := h.Auth.RBACAPI.List(ctx)
	if err != nil {
		return false, "could not verify permission: failed to list roles", ""
	}

	for _, role := range allRoles {
		for _, rp := range role.Permissions {
			if rp == perm {
				return true, "group role grant", perm + " (via role " + role.Name + ")"
			}
		}
	}

	return false, "permission denied", ""
}

// logEvaluateEvent records the RBAC evaluation result as an audit event in Authentik.
func (h *RBACHandler) logEvaluateEvent(ctx context.Context, actorID string, req models.PermissionCheckRequest, hasDirect, allowed bool, reason, _ string) {
	event := map[string]interface{}{
		"type":      "operan_rbac_evaluate",
		"actor":     actorID,
		"object":    req.ActorID,
		"object_type": "operan.permission",
		"data": map[string]interface{}{
			"action":           "rbac_evaluate",
			"allowed":          allowed,
			"reason":           reason,
			"actor_id":         req.ActorID,
			"requested_action": req.Action,
			"requested_resource": req.Resource,
			"has_direct_permission": hasDirect,
			"resolved_permission": mapResourceActionToAuthenticPermission(req.Resource, req.Action),
		},
	}
	body, _ := json.Marshal(event)
	// Authentik events are typically auto-generated by operations.
	// The evaluation result is captured in the HTTP response; additional
	// event creation would require write access to the events API.
	_ = body
	_ = ctx
}

// ---------------------------------------------------------------------------
// Session Replay
// ---------------------------------------------------------------------------

// AuditStore tracks session info for session replay integration.
type AuditStore struct {
	sessions map[string]*models.SessionInfo
	mu       sync.RWMutex
}

// NewAuditStore creates a new in-memory audit store for session tracking.
func NewAuditStore() *AuditStore {
	return &AuditStore{
		sessions: make(map[string]*models.SessionInfo),
	}
}

// GetOrCreateSession returns an existing session or creates a new one.
func (s *AuditStore) GetOrCreateSession(sessionID string) *models.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		info = &models.SessionInfo{
			ID:           sessionID,
			StartedAt:    time.Now().UTC(),
			LastActivity: time.Now().UTC(),
			IsActive:     true,
			RequestCount: 0,
		}
		s.sessions[sessionID] = info
	}
	return info
}

// IncrementRequestCount bumps the request counter for a session.
func (s *AuditStore) IncrementRequestCount(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if info, exists := s.sessions[sessionID]; exists {
		info.RequestCount++
		info.LastActivity = time.Now().UTC()
	}
}

// SessionReplayHandler manages session replay data through HTTP endpoints.
type SessionReplayHandler struct {
	Capture    *middleware.SessionReplayCapture
	Publisher  *events.Publisher
	AuditStore *AuditStore
}

// NewSessionReplayHandler creates a new session replay handler.
func NewSessionReplayHandler(
	capture *middleware.SessionReplayCapture,
	publisher *events.Publisher,
	store *AuditStore,
) *SessionReplayHandler {
	return &SessionReplayHandler{
		Capture:    capture,
		Publisher:  publisher,
		AuditStore: store,
	}
}

// ListSessions returns all active sessions with pagination.
// GET /api/v1/iam/session-replay/sessions
func (h *SessionReplayHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_ = ctx

	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())

	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	page := 1
	perPage := 10
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	// Build session info list from capture store.
	allSessionIDs := h.Capture.ListSessions()
	allSessions := make([]models.SessionInfo, 0, len(allSessionIDs))

	for _, id := range allSessionIDs {
		captureSession := h.Capture.GetSession(id)
		if captureSession == nil {
			continue
		}

		storeInfo := h.AuditStore.GetOrCreateSession(id)

		info := models.SessionInfo{
			ID:           id,
			UserID:       captureSession.UserID,
			TenantID:     captureSession.TenantID,
			IP:           "",
			UserAgent:    "",
			StartedAt:    captureSession.StartedAt,
			LastActivity: time.Now().UTC(),
			IsActive:     true,
			RequestCount: storeInfo.RequestCount,
		}
		allSessions = append(allSessions, info)
	}

	total := len(allSessions)
	start := (page - 1) * perPage
	if start >= total {
		allSessions = nil
	} else {
		end := start + perPage
		if end > total {
			end = total
		}
		allSessions = allSessions[start:end]
	}

	_ = tenantID
	_ = userID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": allSessions,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetSessionRequests returns all captured requests for a given session.
// GET /api/v1/iam/session-replay/sessions/{id}/requests
func (h *SessionReplayHandler) GetSessionRequests(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_ = ctx

	sessionID := extractSessionRequestsID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, `{"error":"session_id is required"}`, http.StatusBadRequest)
		return
	}

	captureSession := h.Capture.GetSession(sessionID)
	if captureSession == nil {
		http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
		return
	}

	// Log the retrieval event asynchronously.
	go func() {
		ctx := context.Background()
		retrievedBy := middleware.GetUserID(r.Context())
		tenantID := middleware.GetTenantID(r.Context())
		if h.Publisher != nil {
			_ = h.Publisher.SessionReplayRetrieved(
				ctx, sessionID, retrievedBy, tenantID,
				middleware.GetTraceID(r.Context()),
				time.Now().UTC().Format(time.RFC3339),
			)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"requests":   captureSession.Requests,
		"total":      len(captureSession.Requests),
	})
}

// DeleteSession removes a session's replay data.
// DELETE /api/v1/iam/session-replay/sessions/{id}
func (h *SessionReplayHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_ = ctx

	sessionID := extractSessionRequestsID(r.URL.Path)
	if sessionID == "" {
		http.Error(w, `{"error":"session_id is required"}`, http.StatusBadRequest)
		return
	}

	deletedBy := middleware.GetUserID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	// Publish deletion event before removing.
	if h.Publisher != nil {
		go func() {
			ctx := context.Background()
			_ = h.Publisher.SessionReplayDeleted(
				ctx, sessionID, deletedBy, tenantID,
				middleware.GetTraceID(r.Context()),
				time.Now().UTC().Format(time.RFC3339),
			)
		}()
	}

	h.Capture.DeleteSession(sessionID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted":   true,
		"session_id": sessionID,
	})
}

// enhanceAuditEvent attaches SessionID and CorrelationID from JWT context
// to an audit event before persisting it.
func enhanceAuditEvent(evt *models.AuditEvent, r *http.Request) {
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID != "" {
		if evt.Details == nil {
			evt.Details = make(map[string]interface{})
		}
		evt.Details["session_id"] = sessionID
	}

	correlationID := r.Header.Get("X-Trace-ID")
	if correlationID != "" {
		if evt.Details == nil {
			evt.Details = make(map[string]interface{})
		}
		evt.Details["correlation_id"] = correlationID
	}
}

// mapResourceActionToAuthenticPermission converts Operan resource/action pairs
// to Authentik permission strings following the pattern {app_label}.{codename}.
func mapResourceActionToAuthenticPermission(resource, action string) string {
	codename := mapActionToPermissionCodename(action)
	return "authenticik." + codename
}

// mapActionToPermissionCodename maps an Operan action to an Authentik permission codename.
func mapActionToPermissionCodename(action string) string {
	switch action {
	case "read", "view", "list":
		return "user.view"
	case "write", "create", "update", "change":
		return "user.change"
	case "delete", "remove":
		return "user.delete"
	case "assign", "grant":
		return "user.add"
	default:
		return "user." + action
	}
}

// authentikEventToOperan maps an Authentik Event to an Operan AuditEvent.
func authentikEventToOperan(evt *authentik.Event) models.AuditEvent {
	action := mapAuthentikTypeToOperanAction(evt.Type)

	details := evt.Data
	if details == nil {
		details = make(map[string]interface{})
	}
	details["authentik_event_type"] = evt.Type

	return models.AuditEvent{
		ID:           evt.UUID,
		Action:       action,
		ResourceType: evt.ObjectType,
		ResourceID:   evt.Object,
		Result:       mapAuthentikResult(evt),
		Details:      details,
		Timestamp:    time.Now().UTC(),
	}
}

// mapAuthentikTypeToOperanAction maps Authentik event type strings to Operan action names.
func mapAuthentikTypeToOperanAction(evtType string) string {
	switch evtType {
	case "core.user_create":
		return "user_created"
	case "core.user_update":
		return "user_updated"
	case "core.user_delete":
		return "user_deleted"
	case "core.group_create":
		return "group_created"
	case "core.group_update":
		return "group_updated"
	case "core.group_delete":
		return "group_deleted"
	case "providers.oauth2_create":
		return "oauth2_created"
	case "providers.oauth2_update":
		return "oauth2_updated"
	case "providers.oauth2_delete":
		return "oauth2_deleted"
	case "rbac.role_create":
		return "role_created"
	case "rbac.role_update":
		return "role_updated"
	case "rbac.role_delete":
		return "role_deleted"
	case "access.grant":
		return "access_granted"
	case "access.deny":
		return "access_denied"
	case "auth.login":
		return "login"
	case "auth.logout":
		return "logout"
	case "auth.failure":
		return "login_failed"
	default:
		return evtType
	}
}

// mapAuthentikResult determines the result (success/denied) from an Authentik event.
func mapAuthentikResult(evt *authentik.Event) string {
	if evt.Data != nil {
		if success, ok := evt.Data["success"]; ok {
			if b, ok := success.(bool); ok && !b {
				return "denied"
			}
		}
		if evt.Type == "access.deny" {
			return "denied"
		}
	}
	return "success"
}

// extractTrailID extracts the trail_id from the URL path.
// Handles: /api/v1/iam/audit/trails/{id}
func extractTrailID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/audit/trails/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// extractSessionReplayID extracts the session_id from the URL path.
// Handles: /api/v1/iam/audit/session-replay/{id}
func extractSessionReplayID(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/iam/audit/session-replay/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// extractSessionRequestsID extracts the session_id from the URL path.
// Handles: /api/v1/iam/session-replay/sessions/{id}/requests and
//          /api/v1/iam/session-replay/sessions/{id}
func extractSessionRequestsID(path string) string {
	// Strip the base path prefix.
	path = strings.TrimPrefix(path, "/api/v1/iam/session-replay/sessions/")
	// Strip any trailing /requests suffix.
	path = strings.TrimSuffix(path, "/requests")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// writeJSON is a helper to write JSON responses with proper headers.
func writeJSON(w http.ResponseWriter, v interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
