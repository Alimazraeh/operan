package deploy

import (
	"context"
	"fmt"

	"github.com/operan/agent-marketplace/internal/clients"
	"github.com/operan/agent-marketplace/internal/store"
)

// CheckCompatibility validates that the tenant's M04/M03 versions meet the listing's minimum requirements.
func CheckCompatibility(m04Client *clients.M04Client, m03Client *clients.M03Client, listing *store.Listing) error {
	// Check M04 compatibility
	if listing.CompatibilityVersions.Valid && listing.CompatibilityVersions.String != "" {
		if err := m04Client.HealthCheck(context.Background()); err != nil {
			return fmt.Errorf("M04 health check failed: %w", err)
		}
	}

	// Check M03 compatibility
	if listing.CompatibilityVersions.Valid && listing.CompatibilityVersions.String != "" {
		if err := m03Client.HealthCheck(context.Background()); err != nil {
			return fmt.Errorf("M03 health check failed: %w", err)
		}
	}

	return nil
}