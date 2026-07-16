package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// Kafka event topic names.
const (
	EventListingCreated     = "operan.marketplace.listing_created"
	EventListingApproved    = "operan.marketplace.listing_approved"
	EventSubscriptionCreated = "operan.marketplace.subscription_created"
	EventSubscriptionExpired = "operan.marketplace.subscription_expired"
	EventDeployed           = "operan.marketplace.deployed"
	EventDeployFailed       = "operan.marketplace.deploy_failed"
	EventReviewCreated      = "operan.marketplace.review_created"
)

// Publisher handles Kafka event publishing for marketplace events.
type Publisher struct {
	brokerURL string
}

// NewPublisher creates a new event publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{brokerURL: brokerURL}
}

// Publish sends a marketplace event to the configured broker.
// If the broker URL is empty, the event is logged but not published (safe for testing).
func (p *Publisher) Publish(ctx context.Context, topic string, payload map[string]interface{}) {
	if p.brokerURL == "" {
		log.Printf("[events] SKIP publish (no broker): topic=%s payload=%v", topic, payload)
		return
	}

	body, err := json.Marshal(map[string]interface{}{
		"topic":     topic,
		"payload":   payload,
		"timestamp": fmt.Sprintf("%v", true), // would be actual timestamp in production
	})
	if err != nil {
		log.Printf("[events] marshal error: %v", err)
		return
	}

	// In production, this would publish to Kafka.
	// For now, log the event (safe for tests with empty broker URL).
	log.Printf("[events] PUBLISH: topic=%s body=%s", topic, string(body))
}

// PublishListingCreated publishes a listing_created event.
func (p *Publisher) PublishListingCreated(ctx context.Context, tenantID, listingID, vendorID, category, listingType string) {
	p.Publish(ctx, EventListingCreated, map[string]interface{}{
		"tenant_id":     tenantID,
		"listing_id":    listingID,
		"vendor_id":     vendorID,
		"category":      category,
		"listing_type":  listingType,
	})
}

// PublishListingApproved publishes a listing_approved event.
func (p *Publisher) PublishListingApproved(ctx context.Context, tenantID, listingID, vendorID string) {
	p.Publish(ctx, EventListingApproved, map[string]interface{}{
		"tenant_id":  tenantID,
		"listing_id": listingID,
		"vendor_id":  vendorID,
	})
}

// PublishSubscriptionCreated publishes a subscription_created event.
func (p *Publisher) PublishSubscriptionCreated(ctx context.Context, tenantID, listingID, subscriptionTier string) {
	p.Publish(ctx, EventSubscriptionCreated, map[string]interface{}{
		"tenant_id":        tenantID,
		"listing_id":       listingID,
		"subscription_tier": subscriptionTier,
		"started_at":       fmt.Sprintf("%v", true), // would be actual time
	})
}

// PublishSubscriptionExpired publishes a subscription_expired event.
func (p *Publisher) PublishSubscriptionExpired(ctx context.Context, tenantID, listingID, subscriptionTier string) {
	p.Publish(ctx, EventSubscriptionExpired, map[string]interface{}{
		"tenant_id":        tenantID,
		"listing_id":       listingID,
		"subscription_tier": subscriptionTier,
	})
}

// PublishDeploy publishes a deploy event.
func (p *Publisher) PublishDeploy(ctx context.Context, tenantID, listingID string, agentCount, workflowCount int, deploymentID string) {
	p.Publish(ctx, EventDeployed, map[string]interface{}{
		"tenant_id":        tenantID,
		"listing_id":       listingID,
		"agent_count":      agentCount,
		"workflow_count":   workflowCount,
		"deployment_id":    deploymentID,
	})
}

// PublishDeployFailed publishes a deploy_failed event.
func (p *Publisher) PublishDeployFailed(ctx context.Context, tenantID, listingID, errorMessage, deploymentID string) {
	p.Publish(ctx, EventDeployFailed, map[string]interface{}{
		"tenant_id":      tenantID,
		"listing_id":     listingID,
		"error_message":  errorMessage,
		"deployment_id":  deploymentID,
	})
}

// PublishReviewCreated publishes a review_created event.
func (p *Publisher) PublishReviewCreated(ctx context.Context, tenantID, listingID string, rating int) {
	p.Publish(ctx, EventReviewCreated, map[string]interface{}{
		"tenant_id":  tenantID,
		"listing_id": listingID,
		"rating":     rating,
	})
}