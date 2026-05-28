package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// Publisher abstracts the async event broker for IAM events.
type Publisher struct {
	brokerURL string
	logger    *log.Logger
}

// NewPublisher creates a new event publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{
		brokerURL: brokerURL,
		logger:    log.Default(),
	}
}

// Event represents a common IAM event envelope.
type Event struct {
	EventType   string                 `json:"event_type"`
	CorrelationID string                `json:"correlationId"`
	TenantID    string                 `json:"tenantId"`
	Timestamp   string                 `json:"timestamp"`
	Payload     map[string]interface{} `json:"payload"`
}

// Publish sends an event to the broker.
func (p *Publisher) Publish(ctx context.Context, eventType string, tenantID, correlationID, timestamp string, payload map[string]interface{}) error {
	event := Event{
		EventType:     eventType,
		CorrelationID: correlationID,
		TenantID:      tenantID,
		Timestamp:     timestamp,
		Payload:       payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// TODO: Implement actual AMQP broker connection
	// For now, log the event as a stub
	p.logger.Printf("[IAM Events] Publishing to %s: %s -> %s", p.brokerURL, eventType, string(data))
	return nil
}

// UserCreated publishes the user.created event.
func (p *Publisher) UserCreated(ctx context.Context, userID, tenantID, email, role, createdBy, authMethod, correlationID, timestamp string) error {
	return p.Publish(ctx, "user.created", tenantID, correlationID, timestamp, map[string]interface{}{
		"user_id":             userID,
		"tenant_id":           tenantID,
		"email":               email,
		"role":                role,
		"created_by":          createdBy,
		"authentication_method": authMethod,
	})
}

// UserUpdated publishes the user.updated event.
func (p *Publisher) UserUpdated(ctx context.Context, userID, tenantID, email, updatedBy, correlationID, timestamp string) error {
	return p.Publish(ctx, "user.updated", tenantID, correlationID, timestamp, map[string]interface{}{
		"user_id":    userID,
		"tenant_id":  tenantID,
		"email":      email,
		"updated_by": updatedBy,
	})
}

// UserSuspended publishes the user.suspended event.
func (p *Publisher) UserSuspended(ctx context.Context, userID, tenantID, reason, suspendedBy, effectiveAt, correlationID, timestamp string) error {
	return p.Publish(ctx, "user.suspended", tenantID, correlationID, timestamp, map[string]interface{}{
		"user_id":      userID,
		"tenant_id":    tenantID,
		"reason":       reason,
		"suspended_by": suspendedBy,
		"effective_at": effectiveAt,
	})
}

// IdentityRotated publishes the identity.rotated event.
func (p *Publisher) IdentityRotated(ctx context.Context, identityID, tenantID, identityType, keyID, rotatedBy, correlationID, timestamp string) error {
	return p.Publish(ctx, "identity.rotated", tenantID, correlationID, timestamp, map[string]interface{}{
		"identity_id":    identityID,
		"tenant_id":      tenantID,
		"identity_type":  identityType,
		"key_id":         keyID,
		"rotated_by":     rotatedBy,
	})
}

// PermissionGranted publishes the permission.granted event.
func (p *Publisher) PermissionGranted(ctx context.Context, tenantID, principalID, principalType, permissionID, scope, grantedBy, correlationID, timestamp string) error {
	return p.Publish(ctx, "permission.granted", tenantID, correlationID, timestamp, map[string]interface{}{
		"tenant_id":      tenantID,
		"principal_id":   principalID,
		"principal_type": principalType,
		"permission_id":  permissionID,
		"scope":          scope,
		"granted_by":     grantedBy,
	})
}

// PermissionRevoked publishes the permission.revoked event.
func (p *Publisher) PermissionRevoked(ctx context.Context, tenantID, principalID, principalType, permissionID, scope, revokedBy, reason, correlationID, timestamp string) error {
	return p.Publish(ctx, "permission.revoked", tenantID, correlationID, timestamp, map[string]interface{}{
		"tenant_id":      tenantID,
		"principal_id":   principalID,
		"principal_type": principalType,
		"permission_id":  permissionID,
		"scope":          scope,
		"revoked_by":     revokedBy,
		"reason":         reason,
	})
}

// SessionCreated publishes the session.created event.
func (p *Publisher) SessionCreated(ctx context.Context, sessionID, userID, tenantID, authMethod, ipAddress, userAgent, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.created", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id":  sessionID,
		"user_id":     userID,
		"tenant_id":   tenantID,
		"auth_method": authMethod,
		"ip_address":  ipAddress,
		"user_agent":  userAgent,
	})
}

// SessionExpired publishes the session.expired event.
func (p *Publisher) SessionExpired(ctx context.Context, sessionID, userID, tenantID, reason, ipAddress, userAgent, correlationID, timestamp string) error {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"user_id":    userID,
		"tenant_id":  tenantID,
		"reason":     reason,
	}
	if ipAddress != "" {
		payload["ip_address"] = ipAddress
	}
	if userAgent != "" {
		payload["user_agent"] = userAgent
	}
	return p.Publish(ctx, "session.expired", tenantID, correlationID, timestamp, payload)
}

// SessionEnded publishes the session.ended event (explicit logout).
func (p *Publisher) SessionEnded(ctx context.Context, sessionID, userID, tenantID, reason, ipAddress, userAgent, correlationID, timestamp string) error {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"user_id":    userID,
		"tenant_id":  tenantID,
		"reason":     reason,
	}
	if ipAddress != "" {
		payload["ip_address"] = ipAddress
	}
	if userAgent != "" {
		payload["user_agent"] = userAgent
	}
	return p.Publish(ctx, "session.ended", tenantID, correlationID, timestamp, payload)
}

// MfaEnrolled publishes the mfa.enrolled event.
func (p *Publisher) MfaEnrolled(ctx context.Context, userID, tenantID, mfaType, enrolledBy, correlationID, timestamp string) error {
	return p.Publish(ctx, "mfa.enrolled", tenantID, correlationID, timestamp, map[string]interface{}{
		"user_id":     userID,
		"tenant_id":   tenantID,
		"mfa_type":    mfaType,
		"enrolled_by": enrolledBy,
	})
}

// SsoLogin publishes the sso.login event.
func (p *Publisher) SsoLogin(ctx context.Context, userID, tenantID, ssoProvider, assertionID, assertionType, authResult, correlationID, timestamp string) error {
	return p.Publish(ctx, "sso.login", tenantID, correlationID, timestamp, map[string]interface{}{
		"user_id":          userID,
		"tenant_id":        tenantID,
		"sso_provider":     ssoProvider,
		"assertion_id":     assertionID,
		"assertion_type":   assertionType,
		"auth_result":      authResult,
	})
}

// SessionActive publishes when a new active session is created.
func (p *Publisher) SessionActive(ctx context.Context, sessionID, userID, tenantID, ip, userAgent, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.active", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id":  sessionID,
		"user_id":     userID,
		"tenant_id":   tenantID,
		"ip":          ip,
		"user_agent":  userAgent,
	})
}

// SessionReplayCaptured publishes when a session replay capture occurs.
func (p *Publisher) SessionReplayCaptured(ctx context.Context, sessionID, userID, tenantID, url, method string, statusCode int, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.replay_captured", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id":   sessionID,
		"user_id":      userID,
		"tenant_id":    tenantID,
		"url":          url,
		"method":       method,
		"status_code":  statusCode,
	})
}

// SessionReplayRetrieved publishes when a session replay is retrieved.
func (p *Publisher) SessionReplayRetrieved(ctx context.Context, sessionID, retrievedBy, tenantID, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.replay_retrieved", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id":   sessionID,
		"retrieved_by": retrievedBy,
		"tenant_id":    tenantID,
	})
}

// SessionReplayDeleted publishes when a session replay is deleted.
func (p *Publisher) SessionReplayDeleted(ctx context.Context, sessionID, deletedBy, tenantID, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.replay_deleted", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id": sessionID,
		"deleted_by": deletedBy,
		"tenant_id":  tenantID,
	})
}

