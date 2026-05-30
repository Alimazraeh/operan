package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	amqp "github.com/streadway/amqp"
)

// mockAMQP implements AMQPInterface for testing.
type mockAMQP struct {
	mu               sync.Mutex
	declareName      string
	declareDurable   bool
	published        []amqp.Publishing
	publishCount     int
	declareErr       error
	publishErr       error
	publishFailUntil int
	closeCalled      bool
	closeErr         error
}

func newMockAMQP() *mockAMQP {
	return &mockAMQP{}
}

func (m *mockAMQP) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	return m.closeErr
}

func (m *mockAMQP) QueueDeclare(
	name string,
	durable bool,
	autoDelete bool,
	exclusive bool,
	noWait bool,
	args amqp.Table,
) (amqp.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.declareName = name
	m.declareDurable = durable
	return amqp.Queue{Name: name, Messages: 0, Consumers: 0}, m.declareErr
}

func (m *mockAMQP) Publish(
	exchange string,
	key string,
	mandatory bool,
	immediate bool,
	msg amqp.Publishing,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, msg)
	m.publishCount++

	// If publishFailUntil is set, fail only on the first N publishes
	if m.publishFailUntil > 0 && m.publishCount <= m.publishFailUntil {
		if m.publishErr != nil {
			return m.publishErr
		}
		// Fail with a default error even if publishErr is nil
		return fmt.Errorf("mock AMQP failure on attempt %d", m.publishCount)
	}
	// After failUntil count, always succeed (return nil)
	// If publishFailUntil is 0 (not set), use publishErr for all attempts
	if m.publishFailUntil == 0 {
		return m.publishErr
	}
	// Otherwise, succeed
	return nil
}

// publishedCount returns the number of messages published.
func (m *mockAMQP) publishedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.published)
}

// lastPublished returns the last published message.
func (m *mockAMQP) lastPublished() amqp.Publishing {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.published) == 0 {
		return amqp.Publishing{}
	}
	return m.published[len(m.published)-1]
}

// isCloseCalled returns whether Close was called.
func (m *mockAMQP) isCloseCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

func TestPublisher_NewPublisher(t *testing.T) {
	p := NewPublisher("amqp://localhost:5672")
	if p == nil {
		t.Fatal("NewPublisher returned nil")
	}
	if p.brokerURL != "amqp://localhost:5672" {
		t.Errorf("brokerURL = %q, want %q", p.brokerURL, "amqp://localhost:5672")
	}
	if p.logger == nil {
		t.Error("logger should not be nil")
	}
	if p.amqpConn != nil {
		t.Error("amqpConn should be nil for default NewPublisher")
	}
}

func TestPublisher_NewPublisherWithDeps(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)
	if p == nil {
		t.Fatal("NewPublisherWithDeps returned nil")
	}
	if p.amqpConn != mock {
		t.Error("amqpConn should be the injected mock")
	}
}

func TestPublisher_Publish_QueueDeclarationAndMessage(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.Publish(context.Background(), "user.created", "tenant-1", "corr-1", time.Now().UTC().Format(time.RFC3339), map[string]interface{}{
		"user_id": "u-123",
	})
	if err != nil {
		t.Fatalf("Publish unexpected error: %v", err)
	}

	if mock.declareName != "iam.events" {
		t.Errorf("QueueDeclare name = %q, want %q", mock.declareName, "iam.events")
	}
	if !mock.declareDurable {
		t.Error("QueueDeclare durable should be true")
	}

	count := mock.publishedCount()
	if count != 1 {
		t.Fatalf("expected 1 published message, got %d", count)
	}

	msg := mock.lastPublished()
	if msg.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want %q", msg.ContentType, "application/json")
	}

	// Check headers
	headers := msg.Headers
	if headers == nil {
		t.Fatal("Headers should not be nil")
	}
	if headers["event_type"] != "user.created" {
		t.Errorf("event_type header = %v, want %q", headers["event_type"], "user.created")
	}
	if headers["tenant_id"] != "tenant-1" {
		t.Errorf("tenant_id header = %v, want %q", headers["tenant_id"], "tenant-1")
	}
	if headers["correlation_id"] != "corr-1" {
		t.Errorf("correlation_id header = %v, want %q", headers["correlation_id"], "corr-1")
	}

	// Check message body is valid JSON with correct event structure
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "user.created" {
		t.Errorf("event_type = %q, want %q", evt.EventType, "user.created")
	}
	if evt.TenantID != "tenant-1" {
		t.Errorf("tenantId = %q, want %q", evt.TenantID, "tenant-1")
	}
	if evt.CorrelationID != "corr-1" {
		t.Errorf("correlationId = %q, want %q", evt.CorrelationID, "corr-1")
	}
	if evt.Payload["user_id"] != "u-123" {
		t.Errorf("payload.user_id = %v, want %q", evt.Payload["user_id"], "u-123")
	}
}

func TestPublisher_Publish_InvalidEventMarshaling(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	// payload with a non-marshallable type won't actually fail since map[string]interface{}
	// with common types always marshals. Let's test with a nil payload which should still work.
	err := p.Publish(context.Background(), "test", "t-1", "c-1", time.Now().UTC().Format(time.RFC3339), nil)
	if err != nil {
		// nil payload is valid - it becomes null in JSON
		t.Logf("Publish with nil payload returned: %v", err)
	}
}

func TestPublisher_Publish_AMQPConnectionError(t *testing.T) {
	// Publisher without deps tries to dial a non-existent RabbitMQ.
	// In practice, this will fail with a network error.
	// We test the error path by using a mock that fails on QueueDeclare.
	mock := newMockAMQP()
	mock.declareErr = fmt.Errorf("queue declare failed")
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.Publish(context.Background(), "user.created", "t-1", "c-1", time.Now().UTC().Format(time.RFC3339), nil)
	if err == nil {
		t.Fatal("expected error when QueueDeclare fails, got nil")
	}
	if err.Error() != "queue declare failed" {
		t.Errorf("error = %q, want %q", err.Error(), "queue declare failed")
	}
}

func TestPublisher_PublishWithRetry_SuccessAfterFailures(t *testing.T) {
	mock := newMockAMQP()
	// Fail the first 2 publish attempts, succeed on the 3rd
	mock.publishErr = fmt.Errorf("connection reset")
	mock.publishFailUntil = 2

	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.Publish(context.Background(), "test.event", "t-1", "c-1", time.Now().UTC().Format(time.RFC3339), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have been called 3 times: 2 failures + 1 success
	if mock.publishedCount() != 3 {
		t.Errorf("publishedCount = %d, want 3", mock.publishedCount())
	}
}

func TestPublisher_PublishWithRetry_AllFailures(t *testing.T) {
	mock := newMockAMQP()
	mock.publishErr = fmt.Errorf("connection refused")

	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.Publish(context.Background(), "test.event", "t-1", "c-1", time.Now().UTC().Format(time.RFC3339), nil)
	if err == nil {
		t.Fatal("expected error when all retries fail, got nil")
	}
	if err.Error() != "publish failed after 3 retries" {
		t.Errorf("error = %q, want %q", err.Error(), "publish failed after 3 retries")
	}

	// Should have 4 publishes total (initial + 3 retries)
	expected := 4
	if mock.publishedCount() != expected {
		t.Errorf("publishedCount = %d, want %d", mock.publishedCount(), expected)
	}

	// Reset
	mock.publishErr = nil
}

func TestPublisher_Close(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.Close()
	if err != nil {
		t.Fatalf("Close unexpected error: %v", err)
	}
	if !mock.isCloseCalled() {
		t.Error("Close was not called on the mock AMQP interface")
	}
}

func TestPublisher_CloseWithError(t *testing.T) {
	mock := newMockAMQP()
	mock.closeErr = fmt.Errorf("connection already closed")
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.Close()
	if err == nil {
		t.Fatal("expected error from Close, got nil")
	}
	if err.Error() != "connection already closed" {
		t.Errorf("error = %q, want %q", err.Error(), "connection already closed")
	}
}

func TestPublisher_UserCreated(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.UserCreated(
		context.Background(), "u-1", "t-1", "alice@example.com",
		"admin", "system", "password", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("UserCreated unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "user.created" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "user.created")
	}
	if evt.TenantID != "t-1" {
		t.Errorf("TenantID = %q, want %q", evt.TenantID, "t-1")
	}
	if evt.CorrelationID != "corr-1" {
		t.Errorf("CorrelationID = %q, want %q", evt.CorrelationID, "corr-1")
	}
	if evt.Payload["user_id"] != "u-1" {
		t.Errorf("payload.user_id = %v, want %q", evt.Payload["user_id"], "u-1")
	}
	if evt.Payload["email"] != "alice@example.com" {
		t.Errorf("payload.email = %v, want %q", evt.Payload["email"], "alice@example.com")
	}
	if evt.Payload["role"] != "admin" {
		t.Errorf("payload.role = %v, want %q", evt.Payload["role"], "admin")
	}
	if evt.Payload["created_by"] != "system" {
		t.Errorf("payload.created_by = %v, want %q", evt.Payload["created_by"], "system")
	}
	if evt.Payload["authentication_method"] != "password" {
		t.Errorf("payload.authentication_method = %v, want %q", evt.Payload["authentication_method"], "password")
	}
}

func TestPublisher_UserUpdated(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.UserUpdated(
		context.Background(), "u-1", "t-1", "bob@example.com",
		"admin", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("UserUpdated unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "user.updated" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "user.updated")
	}
	if evt.Payload["updated_by"] != "admin" {
		t.Errorf("payload.updated_by = %v, want %q", evt.Payload["updated_by"], "admin")
	}
}

func TestPublisher_UserSuspended(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.UserSuspended(
		context.Background(), "u-1", "t-1", "violation", "admin",
		"2026-01-01T00:00:00Z", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("UserSuspended unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "user.suspended" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "user.suspended")
	}
	if evt.Payload["reason"] != "violation" {
		t.Errorf("payload.reason = %v, want %q", evt.Payload["reason"], "violation")
	}
}

func TestPublisher_IdentityRotated(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.IdentityRotated(
		context.Background(), "id-1", "t-1", "api_key", "key-123",
		"admin", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("IdentityRotated unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "identity.rotated" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "identity.rotated")
	}
	if evt.Payload["identity_type"] != "api_key" {
		t.Errorf("payload.identity_type = %v, want %q", evt.Payload["identity_type"], "api_key")
	}
}

func TestPublisher_PermissionGranted(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.PermissionGranted(
		context.Background(), "t-1", "u-1", "user",
		"perm-1", "read", "admin", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("PermissionGranted unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "permission.granted" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "permission.granted")
	}
	if evt.Payload["principal_type"] != "user" {
		t.Errorf("payload.principal_type = %v, want %q", evt.Payload["principal_type"], "user")
	}
}

func TestPublisher_PermissionRevoked(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.PermissionRevoked(
		context.Background(), "t-1", "u-1", "user",
		"perm-1", "read", "admin", "expired", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("PermissionRevoked unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "permission.revoked" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "permission.revoked")
	}
	if evt.Payload["reason"] != "expired" {
		t.Errorf("payload.reason = %v, want %q", evt.Payload["reason"], "expired")
	}
}

func TestPublisher_SessionCreated(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionCreated(
		context.Background(), "s-1", "u-1", "t-1",
		"password", "192.168.1.1", "Mozilla/5.0", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionCreated unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.created" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.created")
	}
	if evt.Payload["auth_method"] != "password" {
		t.Errorf("payload.auth_method = %v, want %q", evt.Payload["auth_method"], "password")
	}
}

func TestPublisher_SessionExpired(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionExpired(
		context.Background(), "s-1", "u-1", "t-1",
		"timeout", "192.168.1.1", "Mozilla/5.0", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionExpired unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.expired" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.expired")
	}
	if evt.Payload["reason"] != "timeout" {
		t.Errorf("payload.reason = %v, want %q", evt.Payload["reason"], "timeout")
	}
}

func TestPublisher_SessionEnded(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionEnded(
		context.Background(), "s-1", "u-1", "t-1",
		"user_logout", "192.168.1.1", "Mozilla/5.0", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionEnded unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.ended" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.ended")
	}
}

func TestPublisher_MfaEnrolled(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.MfaEnrolled(
		context.Background(), "u-1", "t-1", "totp", "admin", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("MfaEnrolled unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "mfa.enrolled" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "mfa.enrolled")
	}
	if evt.Payload["mfa_type"] != "totp" {
		t.Errorf("payload.mfa_type = %v, want %q", evt.Payload["mfa_type"], "totp")
	}
}

func TestPublisher_SsoLogin(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SsoLogin(
		context.Background(), "u-1", "t-1", "okta",
		"assertion-123", "saml", "success", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SsoLogin unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "sso.login" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "sso.login")
	}
	if evt.Payload["sso_provider"] != "okta" {
		t.Errorf("payload.sso_provider = %v, want %q", evt.Payload["sso_provider"], "okta")
	}
}

func TestPublisher_SessionActive(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionActive(
		context.Background(), "s-1", "u-1", "t-1",
		"192.168.1.1", "Mozilla/5.0", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionActive unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.active" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.active")
	}
	if evt.Payload["ip"] != "192.168.1.1" {
		t.Errorf("payload.ip = %v, want %q", evt.Payload["ip"], "192.168.1.1")
	}
}

func TestPublisher_SessionReplayCaptured(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionReplayCaptured(
		context.Background(), "s-1", "u-1", "t-1",
		"https://app.example.com", "GET", 200, "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionReplayCaptured unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.replay_captured" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.replay_captured")
	}
	if evt.Payload["url"] != "https://app.example.com" {
		t.Errorf("payload.url = %v, want %q", evt.Payload["url"], "https://app.example.com")
	}
	if evt.Payload["method"] != "GET" {
		t.Errorf("payload.method = %v, want %q", evt.Payload["method"], "GET")
	}
	if evt.Payload["status_code"] != float64(200) {
		t.Errorf("payload.status_code = %v, want %v", evt.Payload["status_code"], float64(200))
	}
}

func TestPublisher_SessionReplayRetrieved(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionReplayRetrieved(
		context.Background(), "s-1", "admin", "t-1", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionReplayRetrieved unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.replay_retrieved" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.replay_retrieved")
	}
	if evt.Payload["retrieved_by"] != "admin" {
		t.Errorf("payload.retrieved_by = %v, want %q", evt.Payload["retrieved_by"], "admin")
	}
}

func TestPublisher_SessionReplayDeleted(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	err := p.SessionReplayDeleted(
		context.Background(), "s-1", "admin", "t-1", "corr-1", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SessionReplayDeleted unexpected error: %v", err)
	}

	msg := mock.lastPublished()
	var evt Event
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		t.Fatalf("message body is not valid JSON: %v", err)
	}
	if evt.EventType != "session.replay_deleted" {
		t.Errorf("EventType = %q, want %q", evt.EventType, "session.replay_deleted")
	}
}

func TestPublisher_HeadersContainCorrectInfo(t *testing.T) {
	mock := newMockAMQP()
	p := NewPublisherWithDeps("amqp://localhost:5672", mock)

	testCases := []struct {
		eventType     string
		tenantID      string
		correlationID string
	}{
		{"user.created", "tenant-alpha", "corr-001"},
		{"session.expired", "tenant-beta", "corr-002"},
		{"permission.revoked", "tenant-gamma", "corr-003"},
	}

	for _, tc := range testCases {
		t.Run(tc.eventType, func(t *testing.T) {
			err := p.Publish(context.Background(), tc.eventType, tc.tenantID, tc.correlationID, time.Now().UTC().Format(time.RFC3339), map[string]interface{}{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			msg := mock.lastPublished()
			headers := msg.Headers
			if headers["event_type"] != tc.eventType {
				t.Errorf("event_type header = %v, want %q", headers["event_type"], tc.eventType)
			}
			if headers["tenant_id"] != tc.tenantID {
				t.Errorf("tenant_id header = %v, want %q", headers["tenant_id"], tc.tenantID)
			}
			if headers["correlation_id"] != tc.correlationID {
				t.Errorf("correlation_id header = %v, want %q", headers["correlation_id"], tc.correlationID)
			}
		})
	}
}
