package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/operan/agent-marketplace/internal/clients"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/store"
)

// DeploymentResult holds the outcome of a marketplace listing deployment.
type DeploymentResult struct {
	Success         bool     `json:"success"`
	DeploymentID    string   `json:"deployment_id"`
	CreatedAgents   []string `json:"created_agents"`
	CreatedWorkflows []string `json:"created_workflows"`
	Errors          []string `json:"errors,omitempty"`
	DeployedAt      time.Time `json:"deployed_at"`
}

// Deployer handles deploying marketplace listings to tenant environments.
type Deployer struct {
	m04Client *clients.M04Client
	m03Client *clients.M03Client
	listingStore *store.ListingStore
	subStore    *store.SubscriptionStore
	evtPub      *events.Publisher
	m04Token    string // bearer token for M04 calls
}

// NewDeployer creates a new Deployer.
func NewDeployer(m04Client *clients.M04Client, m03Client *clients.M03Client,
	listingStore *store.ListingStore, subStore *store.SubscriptionStore,
	evtPub *events.Publisher, m04Token string) *Deployer {
	return &Deployer{
		m04Client:    m04Client,
		m03Client:    m03Client,
		listingStore: listingStore,
		subStore:     subStore,
		evtPub:       evtPub,
		m04Token:     m04Token,
	}
}

// Deploy provisions a listing's agents and workflows into the tenant's environment.
// It validates the subscription, runs compatibility checks, registers agents in M04,
// creates workflows in M03, and publishes deployment events.
func (d *Deployer) Deploy(ctx context.Context, tenantID, listingID string) (*DeploymentResult, error) {
	deploymentID := uuid.New().String()
	result := &DeploymentResult{
		DeploymentID: deploymentID,
		DeployedAt:   time.Now(),
		CreatedAgents:   []string{},
		CreatedWorkflows: []string{},
		Errors:          []string{},
	}

	// Step 1: Look up listing
	listing, err := d.listingStore.GetByID(ctx, listingID)
	if err != nil {
		if err == store.ErrNotFound {
			result.Errors = append(result.Errors, "listing not found")
			d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
				"error": "listing not found",
			})
			return result, fmt.Errorf("listing not found")
		}
		result.Errors = append(result.Errors, fmt.Sprintf("failed to lookup listing: %v", err))
		return result, err
	}

	// Step 2: Check listing is not deactivated
	if listing.Status == "deactivated" {
		result.Errors = append(result.Errors, "listing is deactivated")
		d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
			"error": "listing is deactivated",
		})
		return result, fmt.Errorf("listing is deactivated")
	}

	// Step 3: Check subscription
	active, err := d.subStore.IsActive(ctx, tenantID, listingID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("subscription check failed: %v", err))
		return result, err
	}
	if listing.RequiresSubscription && !active {
		result.Errors = append(result.Errors, "no active subscription")
		d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
			"error": "no active subscription",
		})
		return result, fmt.Errorf("no active subscription for listing %s", listingID)
	}

	// Step 4: Run compatibility checks
	if err := CheckCompatibility(d.m04Client, d.m03Client, listing); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("compatibility check failed: %v", err))
		d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
			"error": err.Error(),
		})
		return result, err
	}

	// Step 5: Register agents in M04
	agentID, err := d.m04Client.RegisterAgent(ctx, tenantID, d.m04Token, clients.M04Agent{
		Name:         listing.Name,
		Role:         "Marketplace-" + listing.Category,
		Capabilities: listing.Capabilities.ToSlice(),
		Tools:        []string{},
		TenantID:     tenantID,
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("M04 registration failed: %v", err))
		d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
			"error": err.Error(),
		})
		return result, err
	}
	result.CreatedAgents = append(result.CreatedAgents, agentID.Name)

	// Step 6: Create workflows in M03
	wfResult, err := d.m03Client.CreateWorkflow(ctx, tenantID, d.m04Token, clients.M03Workflow{
		Name:     listing.Name + "-workflow",
		TenantID: tenantID,
		Steps: []map[string]interface{}{
			{"action": "initialize", "agent": agentID.Name},
		},
		Metadata: map[string]interface{}{
			"source":       "marketplace",
			"listing_id":   listingID,
			"listing_name": listing.Name,
		},
	})
	if err != nil {
		// Rollback M04 registration
		result.Errors = append(result.Errors, fmt.Sprintf("M03 workflow creation failed: %v", err))
		result.Success = false
		d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
			"error": fmt.Sprintf("M03 workflow creation failed: %v", err),
		})
		return result, err
	}
	result.CreatedWorkflows = append(result.CreatedWorkflows, wfResult.ID)
	result.Success = true

	// Step 7: Mark as deployed
	deployedAt := time.Now()
	if err := d.subStore.UpdateDeployed(ctx, tenantID, listingID, true, deployedAt); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("deployment record update failed: %v", err))
	}

	// Step 8: Update listing download count
	if err := d.listingStore.IncrementDownloads(ctx, listingID); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("download count update failed: %v", err))
	}

	// Step 9: Publish deployment success event
	d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployed, map[string]interface{}{
		"agent_count":   len(result.CreatedAgents),
		"workflow_count": len(result.CreatedWorkflows),
	})

	return result, nil
}

// Rollback removes agents and workflows created during a failed deployment.
func (d *Deployer) Rollback(ctx context.Context, tenantID, listingID string) error {
	// NOTE: Rollback would require M04 DELETE and M03 DELETE calls.
	// The current implementation tracks errors; full rollback requires additional M04/M03 endpoints.
	return nil
}

func (d *Deployer) publishEvent(ctx context.Context, deploymentID, tenantID, listingID string, eventType string, payload map[string]interface{}) {
	allPayload := map[string]interface{}{
		"tenant_id":      tenantID,
		"listing_id":     listingID,
		"deployment_id":  deploymentID,
		"event_type":     eventType,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range payload {
		allPayload[k] = v
	}
	d.evtPub.Publish(ctx, eventType, allPayload)
}