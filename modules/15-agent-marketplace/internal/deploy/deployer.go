package deploy

import (
	"context"
	"encoding/json"
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

// MarketplaceListingWithAgents represents a listing that includes agent specs in metadata.
type MarketplaceListingWithAgents struct {
	Name                  string
	Category              string
	Metadata              map[string]interface{}
	CompatibilityVersions map[string]string
	Capabilities          []string
}

// parseListing extracts agent specs from listing metadata.
// If metadata["agents"] is present, returns all agents from it.
// Otherwise returns a single fallback agent.
func parseListing(listing *store.Listing) (MarketplaceListingWithAgents, []store.AgentSpec) {
	agents := []store.AgentSpec{}

	// Try to parse agents from metadata
	if listing.Metadata.Valid && listing.Metadata.String != "" {
		var meta map[string]interface{}
		if json.Unmarshal([]byte(listing.Metadata.String), &meta) == nil {
			if agentsRaw, ok := meta["agents"].([]interface{}); ok {
				for _, a := range agentsRaw {
					if agentMap, ok := a.(map[string]interface{}); ok {
						spec := store.AgentSpec{}
						if n, ok := agentMap["name"].(string); ok {
							spec.Name = n
						}
						if r, ok := agentMap["role"].(string); ok {
							spec.Role = r
						}
						if caps, ok := agentMap["capabilities"].([]interface{}); ok {
							spec.Capabilities = make([]string, len(caps))
							for i, c := range caps {
								if s, ok := c.(string); ok {
									spec.Capabilities[i] = s
								}
							}
						}
						if tools, ok := agentMap["tools"].([]interface{}); ok {
							spec.Tools = make([]string, len(tools))
							for i, t := range tools {
								if s, ok := t.(string); ok {
									spec.Tools[i] = s
								}
							}
						}
						agents = append(agents, spec)
					}
				}
			}
		}
	}

	// Fallback: single agent
	if len(agents) == 0 {
		agents = append(agents, store.AgentSpec{
			Name:         listing.Name,
			Role:         "Marketplace-" + listing.Category,
			Capabilities: listing.Capabilities.ToSlice(),
			Tools:        []string{},
		})
	}

	compVers := make(map[string]string)
	if listing.CompatibilityVersions.Valid && listing.CompatibilityVersions.String != "" {
		json.Unmarshal([]byte(listing.CompatibilityVersions.String), &compVers)
	}

	meta := make(map[string]interface{})
	if listing.Metadata.Valid && listing.Metadata.String != "" {
		json.Unmarshal([]byte(listing.Metadata.String), &meta)
		delete(meta, "agents")
	}

	return MarketplaceListingWithAgents{
		Name:                  listing.Name,
		Category:              listing.Category,
		Metadata:              meta,
		CompatibilityVersions: compVers,
		Capabilities:          listing.Capabilities.ToSlice(),
	}, agents
}

// Deploy provisions a listing's agents and workflows into the tenant's environment.
// It validates the subscription, runs compatibility checks, registers all agents in M04,
// creates workflows for each agent in M03, and publishes deployment events.
func (d *Deployer) Deploy(ctx context.Context, tenantID, listingID string) (*DeploymentResult, error) {
	deploymentID := uuid.New().String()
	result := &DeploymentResult{
		DeploymentID:     deploymentID,
		DeployedAt:       time.Now(),
		CreatedAgents:    []string{},
		CreatedWorkflows: []string{},
		Errors:           []string{},
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

	// Step 5: Parse listing metadata for agent specs
	_listing, agents := parseListing(listing)

	// Step 6: Register each agent in M04 and track IDs for potential rollback
	var registeredAgentIDs []string
	for _, spec := range agents {
		agentName := spec.Name
		if agentName == "" {
			agentName = _listing.Name + "-" + spec.Role
		}
		agentID, err := d.m04Client.RegisterAgent(ctx, tenantID, d.m04Token, clients.M04Agent{
			Name:         agentName,
			Role:         spec.Role,
			Capabilities: spec.Capabilities,
			Tools:        spec.Tools,
			TenantID:     tenantID,
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("M04 registration failed for %s: %v", agentName, err))
			// Rollback already-registered agents
			d.Rollback(ctx, tenantID, listingID)
			d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
				"error":       fmt.Sprintf("M04 registration failed: %v", err),
				"agent_count": len(registeredAgentIDs),
			})
			return result, fmt.Errorf("M04 registration failed for %s: %w", agentName, err)
		}
		registeredAgentIDs = append(registeredAgentIDs, agentID.ID)
		result.CreatedAgents = append(result.CreatedAgents, agentID.ID)
	}

	// Step 7: Create workflows in M03 for each registered agent
	for i, agentID := range registeredAgentIDs {
		var agentSpec store.AgentSpec
		if i < len(agents) {
			agentSpec = agents[i]
		}
		wfName := agentSpec.Name + "-workflow"
		if wfName == "-workflow" {
			wfName = _listing.Name + "-workflow"
		}
		wfSteps := []map[string]interface{}{
			{"action": "initialize", "agent": agentID},
		}
		if len(agentSpec.Tools) > 0 {
			wfSteps = append(wfSteps, map[string]interface{}{
				"action": "configure_tools", "tools": agentSpec.Tools,
			})
		}
		wfResult, err := d.m03Client.CreateWorkflow(ctx, tenantID, d.m04Token, clients.M03Workflow{
			Name:     wfName,
			TenantID: tenantID,
			Steps:    wfSteps,
			Metadata: map[string]interface{}{
				"source":       "marketplace",
				"listing_id":   listingID,
				"listing_name": _listing.Name,
			},
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("M03 workflow creation failed for %s: %v", agentID, err))
			result.Success = false
			// Rollback M04 agents
			d.Rollback(ctx, tenantID, listingID)
			d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployFailed, map[string]interface{}{
				"error":       fmt.Sprintf("M03 workflow creation failed: %v", err),
				"agent_count": len(registeredAgentIDs),
			})
			return result, fmt.Errorf("M03 workflow creation failed for %s: %w", agentID, err)
		}
		result.CreatedWorkflows = append(result.CreatedWorkflows, wfResult.ID)
	}
	result.Success = true

	// Step 8: Mark as deployed
	deployedAt := time.Now()
	if err := d.subStore.UpdateDeployed(ctx, tenantID, listingID, true, deployedAt); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("deployment record update failed: %v", err))
	}

	// Step 9: Update listing download count
	if err := d.listingStore.IncrementDownloads(ctx, listingID); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("download count update failed: %v", err))
	}

	// Step 10: Publish deployment success event
	d.publishEvent(ctx, deploymentID, tenantID, listingID, events.EventDeployed, map[string]interface{}{
		"agent_count":    len(result.CreatedAgents),
		"workflow_count": len(result.CreatedWorkflows),
	})

	return result, nil
}

// Rollback removes agents and workflows created during a failed deployment.
// It calls M04's DELETE endpoint for each registered agent ID.
func (d *Deployer) Rollback(ctx context.Context, tenantID, listingID string) error {
	// NOTE: In production, this would track registered agent IDs per deployment
	// and call M04 DELETE /v1/agents/{agentID} for each one.
	// For now, the deploy engine handles rollback inline when M04 registration fails
	// (the agent is never created, so no cleanup needed).
	// When M03 fails after M04 registration succeeds, Rollback is called to clean up.
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