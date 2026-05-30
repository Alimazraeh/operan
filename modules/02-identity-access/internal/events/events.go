package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/streadway/amqp"
)

// Publisher abstracts the async event broker for IAM events.
type Publisher struct {
	brokerURL string
	logger    *log.Logger

	// amqpConn is an injectable interface for testability.
	// nil in production (uses real AMQP dial); set in tests to a mock.
	amqpConn AMQPInterface
	// mu guards conn and ch in production mode
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel

	// once ensures the real connection/channel is initialized exactly once
	once    sync.Once
	initErr error
}

// Event represents a common IAM event envelope.
type Event struct {
	EventType     string                 `json:"event_type"`
	CorrelationID string                 `json:"correlationId"`
	TenantID      string                 `json:"tenantId"`
	Timestamp     string                 `json:"timestamp"`
	Payload       map[string]interface{} `json:"payload"`
}

// NewPublisher creates a new event publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{
		brokerURL: brokerURL,
		logger:    log.Default(),
	}
}

// NewPublisherWithDeps creates a new event publisher with an injectable
// AMQP interface for testing purposes.
func NewPublisherWithDeps(brokerURL string, amqpConn AMQPInterface) *Publisher {
	return &Publisher{
		brokerURL: brokerURL,
		logger:    log.Default(),
		amqpConn:  amqpConn,
	}
}

// initConnection establishes the AMQP connection and channel lazily
// on first use, using sync.Once for thread safety.
func (p *Publisher) initConnection() error {
	// Test mode: the mock AMQPInterface handles queue declaration directly
	if p.amqpConn != nil {
		_, err := p.amqpConn.QueueDeclare("iam.events", true, false, false, false, nil)
		return err
	}

	// Production mode: dial and create channel once
	p.once.Do(func() {
		rawConn, dialErr := amqp.Dial(p.brokerURL)
		if dialErr != nil {
			p.initErr = fmt.Errorf("dial AMQP: %w", dialErr)
			return
		}
		p.conn = rawConn

		ch, chErr := rawConn.Channel()
		if chErr != nil {
			p.initErr = fmt.Errorf("create AMQP channel: %w", chErr)
			return
		}
		p.ch = ch

		_, declareErr := ch.QueueDeclare(
			"iam.events", // name
			true,         // durable
			false,        // autoDelete
			false,        // exclusive
			false,        // noWait
			nil,          // args
		)
		if declareErr != nil {
			p.initErr = fmt.Errorf("declare queue %s: %w", "iam.events", declareErr)
		}
	})
	return p.initErr
}

// Publish sends an event to the broker.
func (p *Publisher) Publish(ctx context.Context, eventType string, tenantID, correlationID, timestamp string, payload map[string]interface{}) error {
	if err := p.initConnection(); err != nil {
		return err
	}

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

	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
		Headers: amqp.Table{
			"event_type":     eventType,
			"tenant_id":      tenantID,
			"correlation_id": correlationID,
		},
	}

	return p.publishWithRetry(msg)
}

// publishWithRetry retries publishing up to 3 times with exponential backoff.
func (p *Publisher) publishWithRetry(msg amqp.Publishing) error {
	backoffs := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}

	for attempt := 0; attempt < 4; attempt++ {
		var err error
		if p.amqpConn != nil {
			// Test mode: publish through mock interface
			err = p.amqpConn.Publish("", "iam.events", true, false, msg)
		} else {
			// Production mode: publish through real channel
			err = p.ch.Publish("", "iam.events", true, false, msg)
		}

		if err == nil {
			return nil
		}

		p.logger.Printf("[IAM Events] Publish attempt %d failed: %v", attempt+1, err)

		if attempt < len(backoffs) {
			select {
			case <-time.After(backoffs[attempt]):
			case <-time.After(time.Second * 2): // safety cap
			}
		}
	}

	return fmt.Errorf("publish failed after 3 retries")
}

// Close cleanly shuts down the AMQP connection and channel.
func (p *Publisher) Close() error {
	// Test mode: use mock close
	if p.amqpConn != nil {
		return p.amqpConn.Close()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var err error
	if p.ch != nil {
		chErr := p.ch.Close()
		if chErr != nil {
			err = chErr
		}
		p.ch = nil
	}
	if p.conn != nil {
		connErr := p.conn.Close()
		if connErr != nil {
			err = connErr
		}
		p.conn = nil
	}
	return err
}

// UserCreated publishes the user.created event.
func (p *Publisher) UserCreated(ctx context.Context, userID, tenantID, email, role, createdBy, authMethod, correlationID, timestamp string) error {
	return p.Publish(ctx, "user.created", tenantID, correlationID, timestamp, map[string]interface{}{
		"user_id":               userID,
		"tenant_id":             tenantID,
		"email":                 email,
		"role":                  role,
		"created_by":            createdBy,
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
		"identity_id":   identityID,
		"tenant_id":     tenantID,
		"identity_type": identityType,
		"key_id":        keyID,
		"rotated_by":    rotatedBy,
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
		"user_id":        userID,
		"tenant_id":      tenantID,
		"sso_provider":   ssoProvider,
		"assertion_id":   assertionID,
		"assertion_type": assertionType,
		"auth_result":    authResult,
	})
}

// SessionActive publishes when a new active session is created.
func (p *Publisher) SessionActive(ctx context.Context, sessionID, userID, tenantID, ip, userAgent, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.active", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id": sessionID,
		"user_id":    userID,
		"tenant_id":  tenantID,
		"ip":         ip,
		"user_agent": userAgent,
	})
}

// SessionReplayCaptured publishes when a session replay capture occurs.
func (p *Publisher) SessionReplayCaptured(ctx context.Context, sessionID, userID, tenantID, url, method string, statusCode int, correlationID, timestamp string) error {
	return p.Publish(ctx, "session.replay_captured", tenantID, correlationID, timestamp, map[string]interface{}{
		"session_id":  sessionID,
		"user_id":     userID,
		"tenant_id":   tenantID,
		"url":         url,
		"method":      method,
		"status_code": statusCode,
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
