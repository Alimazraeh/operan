package store

import (
	"context"
	"encoding/json"
	"log"

	"github.com/operan/modules/01-tenant-control-plane/internal/database"
)

// Durability is attached to the in-memory stores rather than wrapping them.
//
// The alternative — a PersistentXStore implementing an interface the handlers
// accept — would mean introducing interfaces for ten stores and touching every
// handler signature to fix a bug about restarts. A write-through sink is the
// smaller change and leaves reads on the memory fast path, which is what the
// stores were built for.
type sink struct{ db *database.ControlPlaneStore }

// unmarshalJSON is shared with persist.go; it exists so this package does not
// reach for encoding/json at every call site.
func unmarshalJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

// orEmptySlice keeps a nil slice out of the API response, where the contract
// says these fields are arrays.
func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ─── Tenants ─────────────────────────────────────────────────────────────────

// Persist attaches a durable backing store. Called once at boot; when it is
// not called the store behaves exactly as before, in memory only.
func (s *TenantStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

// save writes a tenant through to the database. A failed write is loud: the
// tenant is in memory and will answer requests until the next restart, at
// which point it silently disappears — which is the whole failure this file
// exists to stop, so it must not pass unremarked.
func (s *TenantStore) save(ctx context.Context, t *Tenant) {
	if s.sink == nil {
		return
	}
	quota, err := database.MarshalJSONB(t.Quota)
	if err != nil {
		log.Printf("[TCTL] tenant %s not persisted (quota encode failed: %v) — it will be lost on restart", t.ID, err)
		return
	}
	meta, err := database.MarshalJSONB(t.CustomMetadata)
	if err != nil {
		log.Printf("[TCTL] tenant %s not persisted (metadata encode failed: %v) — it will be lost on restart", t.ID, err)
		return
	}
	row := database.TenantRow{
		ID: t.ID, TenantID: t.ID, Name: t.Name, DisplayName: t.DisplayName,
		Plan: string(t.Plan), Region: string(t.Region),
		IsolationLevel: string(t.IsolationLevel), Status: string(t.Status),
		Quota: quota, ContactEmail: t.ContactEmail, CustomMetadata: meta,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
	if err := s.sink.db.UpsertTenant(ctx, row); err != nil {
		log.Printf("[TCTL] tenant %s not persisted (%v) — it will be lost on restart", t.ID, err)
	}
}

// HydrateTenants loads persisted tenants into memory. Without this the rows
// are invisible after a restart and persistence is write-only.
func (s *TenantStore) HydrateTenants(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadTenants(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var quota QuotaConfig
		if len(r.Quota) > 0 {
			if err := unmarshalJSON(r.Quota, &quota); err != nil {
				log.Printf("[TCTL] tenant %s has unreadable quota (%v) — loaded without it", r.ID, err)
			}
		}
		var meta map[string]interface{}
		if len(r.CustomMetadata) > 0 {
			if err := unmarshalJSON(r.CustomMetadata, &meta); err != nil {
				log.Printf("[TCTL] tenant %s has unreadable custom_metadata (%v) — loaded without it", r.ID, err)
			}
		}
		t := &Tenant{
			ID: r.ID, Name: r.Name, DisplayName: r.DisplayName,
			Plan: Plan(r.Plan), Region: Region(r.Region),
			IsolationLevel: IsolationLevel(r.IsolationLevel),
			Status: TenantStatus(r.Status), Quota: quota,
			ContactEmail:   r.ContactEmail,
			CustomMetadata: meta,
			CreatedAt:      r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.tenants[t.ID] = t
		if t.Name != "" {
			s.byName[normalizeName(t.Name)] = t.ID
		}
	}
	return len(rows), nil
}

// ─── Subscriptions ───────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *SubscriptionStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *SubscriptionStore) save(ctx context.Context, sub *Subscription) {
	if s.sink == nil {
		return
	}
	quotas, err := database.MarshalJSONB(sub.CustomQuotas)
	if err != nil {
		log.Printf("[TCTL] subscription %s not persisted (custom_quotas encode failed: %v) — it will be lost on restart", sub.ID, err)
		return
	}
	row := database.SubscriptionRow{
		ID: sub.ID, TenantID: sub.TenantID, Plan: string(sub.Plan),
		PlanName: sub.PlanName, Status: string(sub.Status),
		BillingCycle: string(sub.BillingCycle), SeatCount: sub.SeatCount,
		UnitPrice: sub.UnitPrice, TotalAmount: sub.TotalAmount,
		Currency: sub.Currency, CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd: sub.CurrentPeriodEnd, NextBillingDate: sub.NextBillingDate,
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd, CancelledAt: sub.CancelledAt,
		CustomQuotas: quotas, CreatedAt: sub.CreatedAt, UpdatedAt: sub.UpdatedAt,
	}
	if err := s.sink.db.UpsertSubscription(ctx, row); err != nil {
		log.Printf("[TCTL] subscription %s not persisted (%v) — it will be lost on restart", sub.ID, err)
	}
}

// HydrateSubscriptions loads persisted subscriptions into memory.
func (s *SubscriptionStore) HydrateSubscriptions(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadSubscriptions(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var quotas *QuotaConfig
		if len(r.CustomQuotas) > 0 {
			var q QuotaConfig
			if err := unmarshalJSON(r.CustomQuotas, &q); err != nil {
				log.Printf("[TCTL] subscription %s has unreadable custom_quotas (%v) — loaded without it", r.ID, err)
			} else {
				quotas = &q
			}
		}
		sub := &Subscription{
			ID: r.ID, TenantID: r.TenantID, Plan: Plan(r.Plan), PlanName: r.PlanName,
			Status: SubscriptionStatus(r.Status), BillingCycle: BillingCycle(r.BillingCycle),
			SeatCount: r.SeatCount, UnitPrice: r.UnitPrice, TotalAmount: r.TotalAmount,
			Currency: r.Currency, CurrentPeriodStart: r.CurrentPeriodStart,
			CurrentPeriodEnd: r.CurrentPeriodEnd, NextBillingDate: r.NextBillingDate,
			CancelAtPeriodEnd: r.CancelAtPeriodEnd, CancelledAt: r.CancelledAt,
			CustomQuotas: quotas, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.subscriptions[sub.ID] = sub
		s.byTenant[sub.TenantID] = sub.ID
	}
	return len(rows), nil
}

// ─── Secrets ─────────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *SecretStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

// save writes a secret through to the database. Only the encrypted value is
// stored — the plaintext is process-lifetime state, and a restarted process
// keeps the ciphertext usable until the secret is rotated.
func (s *SecretStore) save(ctx context.Context, sec *Secret) {
	if s.sink == nil {
		return
	}
	tags, err := database.MarshalJSONBArray(sec.Tags)
	if err != nil {
		log.Printf("[TCTL] secret %s not persisted (tags encode failed: %v) — it will be lost on restart", sec.ID, err)
		return
	}
	row := database.SecretRow{
		ID: sec.ID, TenantID: sec.TenantID, Key: sec.Key,
		EncryptedValue: sec.EncryptedValue, Description: sec.Description,
		Tags: tags, Version: sec.Version,
		CreatedAt: sec.CreatedAt, UpdatedAt: sec.UpdatedAt,
	}
	if err := s.sink.db.UpsertSecret(ctx, row); err != nil {
		log.Printf("[TCTL] secret %s not persisted (%v) — it will be lost on restart", sec.ID, err)
	}
}

// HydrateSecrets loads persisted secrets into memory.
func (s *SecretStore) HydrateSecrets(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadSecrets(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var tags []string
		if len(r.Tags) > 0 {
			if err := unmarshalJSON(r.Tags, &tags); err != nil {
				log.Printf("[TCTL] secret %s has unreadable tags (%v) — loaded without them", r.ID, err)
			}
		}
		sec := &Secret{
			ID: r.ID, TenantID: r.TenantID, Key: r.Key,
			EncryptedValue: r.EncryptedValue, Description: r.Description,
			Tags: orEmptySlice(tags), Version: r.Version,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.secrets[sec.ID] = sec
		s.byKey[sec.TenantID+":"+sec.Key] = sec.ID
	}
	return len(rows), nil
}

// ─── Deployments ─────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *DeploymentStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *DeploymentStore) save(ctx context.Context, d *Deployment) {
	if s.sink == nil {
		return
	}
	manifest, err := database.MarshalJSONB(d.Manifest)
	if err != nil {
		log.Printf("[TCTL] deployment %s not persisted (manifest encode failed: %v) — it will be lost on restart", d.ID, err)
		return
	}
	desired, err := database.MarshalJSONB(d.DesiredState)
	if err != nil {
		log.Printf("[TCTL] deployment %s not persisted (desired_state encode failed: %v) — it will be lost on restart", d.ID, err)
		return
	}
	current, err := database.MarshalJSONB(d.CurrentState)
	if err != nil {
		log.Printf("[TCTL] deployment %s not persisted (current_state encode failed: %v) — it will be lost on restart", d.ID, err)
		return
	}
	refs, err := database.MarshalJSONBArray(d.ResourceRefs)
	if err != nil {
		log.Printf("[TCTL] deployment %s not persisted (resource_refs encode failed: %v) — it will be lost on restart", d.ID, err)
		return
	}
	row := database.DeploymentRow{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, Version: d.Version,
		Status: string(d.Status), Strategy: string(d.Strategy),
		Manifest: manifest, DesiredState: desired, CurrentState: current,
		Error: d.Error, ResourceRefs: refs, NamespaceID: d.NamespaceID,
		PreviousID: d.PreviousID, CreatedBy: d.CreatedBy, Notes: d.Notes,
		DeployedAt: d.DeployedAt, DeprecatedAt: d.DeprecatedAt,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if err := s.sink.db.UpsertDeployment(ctx, row); err != nil {
		log.Printf("[TCTL] deployment %s not persisted (%v) — it will be lost on restart", d.ID, err)
	}
}

// HydrateDeployments loads persisted deployments into memory.
func (s *DeploymentStore) HydrateDeployments(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadDeployments(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var manifest []byte
		if len(r.Manifest) > 0 {
			if err := json.Unmarshal(r.Manifest, &manifest); err != nil {
				log.Printf("[TCTL] deployment %s has unreadable manifest (%v) — loaded without it", r.ID, err)
			}
		}
		var desired, current map[string]interface{}
		if len(r.DesiredState) > 0 {
			if err := unmarshalJSON(r.DesiredState, &desired); err != nil {
				log.Printf("[TCTL] deployment %s has unreadable desired_state (%v) — loaded without it", r.ID, err)
			}
		}
		if len(r.CurrentState) > 0 {
			if err := unmarshalJSON(r.CurrentState, &current); err != nil {
				log.Printf("[TCTL] deployment %s has unreadable current_state (%v) — loaded without it", r.ID, err)
			}
		}
		var refs []string
		if len(r.ResourceRefs) > 0 {
			if err := unmarshalJSON(r.ResourceRefs, &refs); err != nil {
				log.Printf("[TCTL] deployment %s has unreadable resource_refs (%v) — loaded without them", r.ID, err)
			}
		}
		d := &Deployment{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Version: r.Version,
			Status: DeploymentStatus(r.Status), Strategy: DeploymentStrategy(r.Strategy),
			Manifest: manifest, DesiredState: desired, CurrentState: current,
			Error: r.Error, ResourceRefs: refs, NamespaceID: r.NamespaceID,
			PreviousID: r.PreviousID, CreatedBy: r.CreatedBy, Notes: r.Notes,
			DeployedAt: r.DeployedAt, DeprecatedAt: r.DeprecatedAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.deployments[d.ID] = d
		s.byTenant[d.TenantID] = append(s.byTenant[d.TenantID], d.ID)
	}
	return len(rows), nil
}

// ─── Environments ────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *EnvironmentStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *EnvironmentStore) save(ctx context.Context, e *Environment) {
	if s.sink == nil {
		return
	}
	iso, err := database.MarshalJSONB(e.IsolationConfig)
	if err != nil {
		log.Printf("[TCTL] environment %s not persisted (isolation_config encode failed: %v) — it will be lost on restart", e.ID, err)
		return
	}
	resources, err := database.MarshalJSONBArray(e.Resources)
	if err != nil {
		log.Printf("[TCTL] environment %s not persisted (resources encode failed: %v) — it will be lost on restart", e.ID, err)
		return
	}
	network, err := database.MarshalJSONB(e.NetworkConfig)
	if err != nil {
		log.Printf("[TCTL] environment %s not persisted (network_config encode failed: %v) — it will be lost on restart", e.ID, err)
		return
	}
	row := database.EnvironmentRow{
		ID: e.ID, TenantID: e.TenantID, Name: e.Name, Type: string(e.Type),
		State: string(e.State), IsolationLevel: string(e.IsolationLevel),
		IsolationConfig: iso, Resources: resources, NetworkConfig: network,
		CreatedBy: e.CreatedBy, Notes: e.Notes,
		ActivatedAt: e.ActivatedAt, DeactivatedAt: e.DeactivatedAt,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
	if err := s.sink.db.UpsertEnvironment(ctx, row); err != nil {
		log.Printf("[TCTL] environment %s not persisted (%v) — it will be lost on restart", e.ID, err)
	}
}

// HydrateEnvironments loads persisted environments into memory.
func (s *EnvironmentStore) HydrateEnvironments(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadEnvironments(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var iso EnvironmentIsolationConfig
		if len(r.IsolationConfig) > 0 {
			if err := unmarshalJSON(r.IsolationConfig, &iso); err != nil {
				log.Printf("[TCTL] environment %s has unreadable isolation_config (%v) — loaded without it", r.ID, err)
			}
		}
		var resources []string
		if len(r.Resources) > 0 {
			if err := unmarshalJSON(r.Resources, &resources); err != nil {
				log.Printf("[TCTL] environment %s has unreadable resources (%v) — loaded without them", r.ID, err)
			}
		}
		var network map[string]interface{}
		if len(r.NetworkConfig) > 0 {
			if err := unmarshalJSON(r.NetworkConfig, &network); err != nil {
				log.Printf("[TCTL] environment %s has unreadable network_config (%v) — loaded without it", r.ID, err)
			}
		}
		e := &Environment{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Type: EnvironmentType(r.Type),
			State: EnvironmentState(r.State),
			IsolationLevel:  EnvironmentIsolationLevel(r.IsolationLevel),
			IsolationConfig: iso, Resources: resources, NetworkConfig: network,
			CreatedBy: r.CreatedBy, Notes: r.Notes,
			ActivatedAt: r.ActivatedAt, DeactivatedAt: r.DeactivatedAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.environments[e.ID] = e
		s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID], e.ID)
		s.byType[string(e.Type)] = append(s.byType[string(e.Type)], e.ID)
	}
	return len(rows), nil
}

// ─── Namespaces ──────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *NamespaceStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *NamespaceStore) save(ctx context.Context, ns *Namespace) {
	if s.sink == nil {
		return
	}
	cfg, err := database.MarshalJSONB(ns.Config)
	if err != nil {
		log.Printf("[TCTL] namespace %s not persisted (config encode failed: %v) — it will be lost on restart", ns.ID, err)
		return
	}
	quota, err := database.MarshalJSONB(ns.ResourceQuota)
	if err != nil {
		log.Printf("[TCTL] namespace %s not persisted (resource_quota encode failed: %v) — it will be lost on restart", ns.ID, err)
		return
	}
	row := database.NamespaceRow{
		ID: ns.ID, TenantID: ns.TenantID, Name: ns.Name,
		Description: ns.Description, Status: string(ns.Status),
		Config: cfg, ResourceQuota: quota,
		CreatedAt: ns.CreatedAt, UpdatedAt: ns.UpdatedAt,
	}
	if err := s.sink.db.UpsertNamespace(ctx, row); err != nil {
		log.Printf("[TCTL] namespace %s not persisted (%v) — it will be lost on restart", ns.ID, err)
	}
}

// HydrateNamespaces loads persisted namespaces into memory.
func (s *NamespaceStore) HydrateNamespaces(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadNamespaces(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var cfg NamespaceConfig
		if len(r.Config) > 0 {
			if err := unmarshalJSON(r.Config, &cfg); err != nil {
				log.Printf("[TCTL] namespace %s has unreadable config (%v) — loaded without it", r.ID, err)
			}
		}
		var quota *NamespaceQuota
		if len(r.ResourceQuota) > 0 {
			var q NamespaceQuota
			if err := unmarshalJSON(r.ResourceQuota, &q); err != nil {
				log.Printf("[TCTL] namespace %s has unreadable resource_quota (%v) — loaded without it", r.ID, err)
			} else {
				quota = &q
			}
		}
		ns := &Namespace{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name,
			Description: r.Description, Status: NamespaceStatus(r.Status),
			Config: cfg, ResourceQuota: quota,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.namespaces[ns.ID] = ns
		s.byTenant[ns.TenantID] = append(s.byTenant[ns.TenantID], ns.ID)
		s.byName[ns.TenantID+"::"+ns.Name] = ns.ID
	}
	return len(rows), nil
}

// ─── Resources ───────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *ResourceStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *ResourceStore) save(ctx context.Context, res *Resource) {
	if s.sink == nil {
		return
	}
	spec, err := database.MarshalJSONB(res.Spec)
	if err != nil {
		log.Printf("[TCTL] resource %s not persisted (spec encode failed: %v) — it will be lost on restart", res.ID, err)
		return
	}
	row := database.ResourceRow{
		ID: res.ID, TenantID: res.TenantID, Name: res.Name,
		Type: string(res.Type), Region: string(res.Region),
		Spec: spec, Status: string(res.Status), Endpoint: res.Endpoint,
		CreatedAt: res.CreatedAt, UpdatedAt: res.UpdatedAt,
	}
	if err := s.sink.db.UpsertResource(ctx, row); err != nil {
		log.Printf("[TCTL] resource %s not persisted (%v) — it will be lost on restart", res.ID, err)
	}
}

// HydrateResources loads persisted resources into memory.
func (s *ResourceStore) HydrateResources(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadResources(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var spec ResourceSpec
		if len(r.Spec) > 0 {
			if err := unmarshalJSON(r.Spec, &spec); err != nil {
				log.Printf("[TCTL] resource %s has unreadable spec (%v) — loaded without it", r.ID, err)
			}
		}
		res := &Resource{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name,
			Type: ResourceType(r.Type), Region: Region(r.Region),
			Spec: spec, Status: ResourceStatus(r.Status), Endpoint: r.Endpoint,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.resources[res.ID] = res
		s.byTenant[res.TenantID] = append(s.byTenant[res.TenantID], res.ID)
	}
	return len(rows), nil
}

// ─── Agents (tenant-scoped agent configs) ───────────────────────────────────

// Persist attaches a durable backing store.
func (s *AgentStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *AgentStore) save(ctx context.Context, agent *Agent) {
	if s.sink == nil {
		return
	}
	toolAccess, err := database.MarshalJSONB(agent.ToolAccessJSON)
	if err != nil {
		log.Printf("[TCTL] agent %s not persisted (tool_access encode failed: %v) — it will be lost on restart", agent.ID, err)
		return
	}
	row := database.AgentRow{
		ID: agent.ID, TenantID: agent.TenantID, Name: agent.Name,
		Model: agent.Model, Role: agent.Role, SystemPrompt: agent.SystemPrompt,
		Status: string(agent.Status), CurrentWorkflow: agent.CurrentWorkflow,
		CurrentTask: agent.CurrentTask, ToolAccess: toolAccess,
		LastRunAt: agent.LastRunAt, SuccessCount: agent.SuccessCount,
		FailureCount: agent.FailureCount,
		CreatedAt: agent.CreatedAt, UpdatedAt: agent.UpdatedAt,
	}
	if err := s.sink.db.UpsertAgent(ctx, row); err != nil {
		log.Printf("[TCTL] agent %s not persisted (%v) — it will be lost on restart", agent.ID, err)
	}
}

// HydrateAgents loads persisted agent configs into memory.
func (s *AgentStore) HydrateAgents(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadAgents(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		agent := &Agent{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name,
			Model: r.Model, Role: r.Role, SystemPrompt: r.SystemPrompt,
			Status: AgentStatus(r.Status), CurrentWorkflow: r.CurrentWorkflow,
			CurrentTask: r.CurrentTask, LastRunAt: r.LastRunAt,
			SuccessCount: r.SuccessCount, FailureCount: r.FailureCount,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if len(r.ToolAccess) > 0 {
			agent.ToolAccessJSON = r.ToolAccess
		}
		s.agents[agent.ID] = agent
		s.byTenant[agent.TenantID] = append(s.byTenant[agent.TenantID], agent.ID)
	}
	return len(rows), nil
}

// ─── Invoices ────────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *BillingStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *BillingStore) save(ctx context.Context, inv *Invoice) {
	if s.sink == nil {
		return
	}
	lineItems, err := database.MarshalJSONBArray(inv.LineItems)
	if err != nil {
		log.Printf("[TCTL] invoice %s not persisted (line_items encode failed: %v) — it will be lost on restart", inv.ID, err)
		return
	}
	row := database.InvoiceRow{
		ID: inv.ID, TenantID: inv.TenantID, SubscriptionID: inv.SubscriptionID,
		IssueDate: inv.IssueDate, DueDate: inv.DueDate, DueDateRaw: inv.DueDateRaw,
		Amount: inv.Amount, Currency: inv.Currency, Status: string(inv.Status),
		LineItems: lineItems, PaidAt: inv.PaidAt,
		CreatedAt: inv.CreatedAt, UpdatedAt: inv.UpdatedAt,
	}
	if err := s.sink.db.UpsertInvoice(ctx, row); err != nil {
		log.Printf("[TCTL] invoice %s not persisted (%v) — it will be lost on restart", inv.ID, err)
	}
}

// HydrateInvoices loads persisted invoices into memory.
func (s *BillingStore) HydrateInvoices(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadInvoices(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		var lineItems []InvoiceLineItem
		if len(r.LineItems) > 0 {
			if err := unmarshalJSON(r.LineItems, &lineItems); err != nil {
				log.Printf("[TCTL] invoice %s has unreadable line_items (%v) — loaded without them", r.ID, err)
			}
		}
		inv := &Invoice{
			ID: r.ID, TenantID: r.TenantID, SubscriptionID: r.SubscriptionID,
			IssueDate: r.IssueDate, DueDate: r.DueDate, DueDateRaw: r.DueDateRaw,
			Amount: r.Amount, Currency: r.Currency,
			Status: BillingStatus(r.Status), LineItems: lineItems, PaidAt: r.PaidAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.invoices[inv.ID] = inv
		s.byTenant[inv.TenantID] = append(s.byTenant[inv.TenantID], inv.ID)
		if inv.SubscriptionID != "" {
			s.bySubscription[inv.SubscriptionID] = inv.ID
		}
	}
	return len(rows), nil
}

// ─── Payment methods ─────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *PaymentMethodStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *PaymentMethodStore) save(ctx context.Context, pm *PaymentMethod) {
	if s.sink == nil {
		return
	}
	row := database.PaymentMethodRow{
		ID: pm.ID, TenantID: pm.TenantID, Type: string(pm.Type),
		LastFour: pm.LastFour, ExpiryMonth: pm.ExpiryMonth, ExpiryYear: pm.ExpiryYear,
		BillingAddress: pm.BillingAddress, IsDefault: pm.IsDefault,
		CreatedAt: pm.CreatedAt, UpdatedAt: pm.UpdatedAt,
	}
	if err := s.sink.db.UpsertPaymentMethod(ctx, row); err != nil {
		log.Printf("[TCTL] payment method %s not persisted (%v) — it will be lost on restart", pm.ID, err)
	}
}

// HydratePaymentMethods loads persisted payment methods into memory.
func (s *PaymentMethodStore) HydratePaymentMethods(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadPaymentMethods(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		pm := &PaymentMethod{
			ID: r.ID, TenantID: r.TenantID, Type: PaymentMethodType(r.Type),
			LastFour: r.LastFour, ExpiryMonth: r.ExpiryMonth, ExpiryYear: r.ExpiryYear,
			BillingAddress: r.BillingAddress, IsDefault: r.IsDefault,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.methods[pm.ID] = pm
		s.byTenant[pm.TenantID] = append(s.byTenant[pm.TenantID], pm.ID)
	}
	return len(rows), nil
}

// ─── Policies ────────────────────────────────────────────────────────────────

// Persist attaches a durable backing store.
func (s *PolicyStore) Persist(db *database.ControlPlaneStore) { s.sink = &sink{db: db} }

func (s *PolicyStore) save(ctx context.Context, p *Policy) {
	if s.sink == nil {
		return
	}
	rules, err := database.MarshalJSONB(p.Rules)
	if err != nil {
		log.Printf("[TCTL] policy %s not persisted (rules encode failed: %v) — it will be lost on restart", p.ID, err)
		return
	}
	row := database.PolicyRow{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, Description: p.Description,
		Scope: string(p.Scope), Action: string(p.Action), Rules: rules,
		Priority: string(p.Priority), Enabled: p.Enabled, Effect: p.Effect,
		LastEvalAt: p.LastEvalAt, CreatedBy: p.CreatedBy,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if err := s.sink.db.UpsertPolicy(ctx, row); err != nil {
		log.Printf("[TCTL] policy %s not persisted (%v) — it will be lost on restart", p.ID, err)
	}
}

// HydratePolicies loads persisted policies into memory.
func (s *PolicyStore) HydratePolicies(ctx context.Context, db *database.ControlPlaneStore) (int, error) {
	rows, err := db.LoadPolicies(ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		p := &Policy{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Description: r.Description,
			Scope: PolicyScope(r.Scope), Action: PolicyAction(r.Action),
			Rules: r.Rules, Priority: PolicyPriority(r.Priority), Enabled: r.Enabled,
			Effect: r.Effect, LastEvalAt: r.LastEvalAt, CreatedBy: r.CreatedBy,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		s.policies[p.ID] = p
		s.byTenant[p.TenantID] = append(s.byTenant[p.TenantID], p.ID)
	}
	return len(rows), nil
}
