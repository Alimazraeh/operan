package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublish_EmptyBroker(t *testing.T) {
	pub := NewPublisher("")
	// Should not panic
	pub.Publish(context.Background(), EventListingCreated, map[string]interface{}{
		"listing_id": "test-1",
	})
}

func TestPublishListingCreated(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishListingCreated(context.Background(), "tenant-1", "listing-1", "vendor-1", "agent", "vetted")
}

func TestPublishListingApproved(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishListingApproved(context.Background(), "tenant-1", "listing-1", "vendor-1")
}

func TestPublishSubscriptionCreated(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishSubscriptionCreated(context.Background(), "tenant-1", "listing-1", "basic")
}

func TestPublishSubscriptionExpired(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishSubscriptionExpired(context.Background(), "tenant-1", "listing-1", "basic")
}

func TestPublishDeploy(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishDeploy(context.Background(), "tenant-1", "listing-1", 2, 1, "deploy-1")
}

func TestPublishDeployFailed(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishDeployFailed(context.Background(), "tenant-1", "listing-1", "M04 error", "deploy-1")
}

func TestPublishReviewCreated(t *testing.T) {
	pub := NewPublisher("")
	pub.PublishReviewCreated(context.Background(), "tenant-1", "listing-1", 5)
}

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "operan.marketplace.listing_created", EventListingCreated)
	assert.Equal(t, "operan.marketplace.listing_approved", EventListingApproved)
	assert.Equal(t, "operan.marketplace.subscription_created", EventSubscriptionCreated)
	assert.Equal(t, "operan.marketplace.subscription_expired", EventSubscriptionExpired)
	assert.Equal(t, "operan.marketplace.deployed", EventDeployed)
	assert.Equal(t, "operan.marketplace.deploy_failed", EventDeployFailed)
	assert.Equal(t, "operan.marketplace.review_created", EventReviewCreated)
}