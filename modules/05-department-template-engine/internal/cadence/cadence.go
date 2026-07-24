// Package cadence is the department's operating rhythm: the scheduler fires
// each live department's business_logic.operating_cadence entries (daily at
// the briefing hour, weekly on Monday), gathers real numbers from the request
// ledger, has the department head DRAFT the briefing through Module 03's
// agent (real LLM, grounded in department memory), and files it in the
// briefing store for the portal.
//
// Cadence runs have no human caller, so the scheduler mints a short-lived
// service JWT with the shared platform secret (issuer
// operan-tenant-control-plane, role admin) — the same trust the modules
// already pin.
package cadence

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// ── Briefing model + store ──────────────────────────────────

type BriefStats struct {
	OpenRequests     int `json:"open_requests"`
	AwaitingApproval int `json:"awaiting_approval"`
	CompletedLast24h int `json:"completed_last_24h"`
	FailedLast24h    int `json:"failed_last_24h"`
	SLABreached      int `json:"sla_breached"`
	TokensLast24h    int `json:"tokens_last_24h"`
}

type Briefing struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	DepartmentID   string     `json:"department_id"`
	DepartmentName string     `json:"department_name"`
	CadenceName    string     `json:"cadence_name"`
	Frequency      string     `json:"frequency"`
	Stats          BriefStats `json:"stats"`
	Content        string     `json:"content"`
	Model          string     `json:"model,omitempty"`
	Tokens         int        `json:"tokens,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type BriefingStore struct {
	mu    sync.RWMutex
	items []Briefing // newest first
}

func NewBriefingStore() *BriefingStore { return &BriefingStore{} }

const keepPerDepartment = 30 // a month of dailies per department is plenty

func (s *BriefingStore) Add(b Briefing) Briefing {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	s.items = append([]Briefing{b}, s.items...)
	// Bound growth per department.
	count := 0
	kept := s.items[:0]
	for _, it := range s.items {
		if it.DepartmentID == b.DepartmentID {
			count++
			if count > keepPerDepartment {
				continue
			}
		}
		kept = append(kept, it)
	}
	s.items = kept
	return b
}

// List returns the tenant's briefings, newest first, optionally scoped to a
// department.
func (s *BriefingStore) List(tenantID, departmentID string, limit int) []Briefing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Briefing{}
	for _, b := range s.items {
		if b.TenantID != tenantID {
			continue
		}
		if departmentID != "" && b.DepartmentID != departmentID {
			continue
		}
		out = append(out, b)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Latest returns the newest briefing for a department+cadence pair — the
// scheduler's idempotence anchor across restarts.
func (s *BriefingStore) Latest(departmentID, cadenceName string) *Briefing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.items {
		if b.DepartmentID == departmentID && b.CadenceName == cadenceName {
			cp := b
			return &cp
		}
	}
	return nil
}

func (s *BriefingStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.items)
}

func (s *BriefingStore) Import(data []byte) error {
	var items []Briefing
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	s.mu.Lock()
	s.items = items
	s.mu.Unlock()
	return nil
}

// ── Scheduler ───────────────────────────────────────────────

type Scheduler struct {
	Departments *store.DepartmentStore
	Requests    *store.RequestStore
	Briefings   *BriefingStore
	Orch        *clients.OrchestrationClient
	JWTSecret   string
	BriefHour   int           // local hour dailies (and weeklies, on Monday) fire
	TestEvery   time.Duration // >0: fire every interval regardless of clock (demo/testing)
	Now         func() time.Time
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Run ticks once a minute (or the test interval) and fires whatever is due.
func (s *Scheduler) Run(ctx context.Context) {
	tick := time.Minute
	if s.TestEvery > 0 {
		tick = s.TestEvery
		log.Printf("[CADENCE] TEST MODE — firing every %s", s.TestEvery)
	}
	log.Printf("[CADENCE] scheduler started (briefing hour %02d:00, tick %s)", s.BriefHour, tick)
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.FireDue(ctx)
		}
	}
}

// FireDue runs one scheduling pass. Exported for tests.
func (s *Scheduler) FireDue(ctx context.Context) {
	for _, dept := range s.Departments.All() {
		if dept.Status != "operational" && dept.Status != "degraded" {
			continue
		}
		for _, entry := range s.cadenceEntries(&dept) {
			if !s.due(&dept, entry) {
				continue
			}
			if err := s.brief(ctx, &dept, entry); err != nil {
				log.Printf("[CADENCE] %s / %s: %v", dept.Name, entry.Name, err)
			}
		}
	}
}

// cadenceEntries returns the department's schedulable rituals; a department
// without any declared cadence still gets a daily operations digest — every
// department owes its humans a morning brief.
func (s *Scheduler) cadenceEntries(d *store.Department) []store.CadenceEntry {
	var out []store.CadenceEntry
	if d.BusinessLogic != nil {
		for _, c := range d.BusinessLogic.OperatingCadence {
			if c.Frequency == "daily" || c.Frequency == "weekly" {
				out = append(out, c)
			}
		}
	}
	if len(out) == 0 {
		out = append(out, store.CadenceEntry{Name: "Daily operations digest", Frequency: "daily"})
	}
	return out
}

func (s *Scheduler) due(d *store.Department, entry store.CadenceEntry) bool {
	last := s.Briefings.Latest(d.ID, entry.Name)
	now := s.now()
	if s.TestEvery > 0 {
		return last == nil || now.Sub(last.CreatedAt) >= s.TestEvery
	}
	var slot time.Time // the most recent scheduled occurrence
	switch entry.Frequency {
	case "weekly":
		slot = time.Date(now.Year(), now.Month(), now.Day(), s.BriefHour, 0, 0, 0, now.Location())
		for slot.Weekday() != time.Monday {
			slot = slot.AddDate(0, 0, -1)
		}
		if slot.After(now) {
			slot = slot.AddDate(0, 0, -7)
		}
	default: // daily
		slot = time.Date(now.Year(), now.Month(), now.Day(), s.BriefHour, 0, 0, 0, now.Location())
		if slot.After(now) {
			slot = slot.AddDate(0, 0, -1)
		}
	}
	return last == nil || last.CreatedAt.Before(slot)
}

// brief gathers the ledger numbers, asks the department head to write the
// digest, and files it.
func (s *Scheduler) brief(ctx context.Context, d *store.Department, entry store.CadenceEntry) error {
	stats := s.gather(d)
	agentID := headAgent(d)
	content, model, tokens := "", "", 0

	if agentID != "" && s.Orch != nil {
		caller := clients.Caller{Authorization: "Bearer " + s.serviceJWT(d.TenantID), TenantID: d.TenantID}
		out, err := s.Orch.DraftAgent(ctx, caller, clients.DraftRequest{
			AgentID:      agentID,
			Role:         "department head",
			DepartmentID: d.ID,
			MaxTokens:    2200,
			Instruction: fmt.Sprintf(
				"Write the %s for the %s department (%s). Audience: the humans supervising this department. "+
					"Use ONLY these real figures — do not invent numbers: open requests %d; awaiting human approval %d; "+
					"completed in the last 24h %d; failed in the last 24h %d; SLA breaches on open work %d; "+
					"tokens consumed in the last 24h %d. Summarise the state of the operation in under 180 words, "+
					"lead with what needs a human decision, flag SLA risk, and end with one concrete recommendation.",
				entry.Name, d.Name, d.Mission, stats.OpenRequests, stats.AwaitingApproval,
				stats.CompletedLast24h, stats.FailedLast24h, stats.SLABreached, stats.TokensLast24h),
		})
		if err != nil {
			log.Printf("[CADENCE] draft failed for %s (%s) — filing stats-only briefing: %v", d.Name, entry.Name, err)
		} else {
			content, model, tokens = out.Output, out.Model, out.Tokens
		}
	}
	if strings.TrimSpace(content) == "" {
		// The briefing still files honestly without an LLM: numbers first.
		content = fmt.Sprintf(
			"%s — %s.\nOpen requests: %d (awaiting approval: %d, SLA breached: %d). "+
				"Last 24h: %d completed, %d failed. Tokens: %d.\n(Drafting agent unavailable — figures only.)",
			entry.Name, d.Name, stats.OpenRequests, stats.AwaitingApproval, stats.SLABreached,
			stats.CompletedLast24h, stats.FailedLast24h, stats.TokensLast24h)
	}

	b := s.Briefings.Add(Briefing{
		TenantID: d.TenantID, DepartmentID: d.ID, DepartmentName: d.Name,
		CadenceName: entry.Name, Frequency: entry.Frequency,
		Stats: stats, Content: content, Model: model, Tokens: tokens,
		CreatedAt: s.now().UTC(),
	})
	log.Printf("[CADENCE] filed %q for %s (%s, %d tokens)", b.CadenceName, d.Name, b.Frequency, tokens)
	return nil
}

func (s *Scheduler) gather(d *store.Department) BriefStats {
	items, _, _ := s.Requests.ListByDepartment(d.TenantID, d.ID, nil, 1, 500)
	now := s.now().UTC()
	day := now.Add(-24 * time.Hour)
	st := BriefStats{}
	for _, r := range items {
		switch r.Status {
		case "completed":
			if r.CompletedAt != nil && r.CompletedAt.After(day) {
				st.CompletedLast24h++
			}
		case "failed", "cancelled":
			if r.UpdatedAt.After(day) {
				st.FailedLast24h++
			}
		default:
			st.OpenRequests++
			if r.Status == "awaiting_approval" {
				st.AwaitingApproval++
			}
			if r.SLAResolutionDue != nil && r.SLAResolutionDue.Before(now) {
				st.SLABreached++
			}
		}
		if r.CreatedAt.After(day) {
			st.TokensLast24h += r.TokensUsed
		}
	}
	return st
}

// headAgent picks the drafting voice: the org chart's root position holder,
// falling back to any provisioned agent.
func headAgent(d *store.Department) string {
	for _, p := range d.OrgChart {
		if p.ReportsTo == "" && p.AgentID != "" {
			return p.AgentID
		}
	}
	for _, p := range d.OrgChart {
		if p.AgentID != "" {
			return p.AgentID
		}
	}
	if len(d.AgentIDs) > 0 {
		return d.AgentIDs[0]
	}
	return ""
}

// serviceJWT mints the scheduler's short-lived credential (HS256, the shared
// platform secret). Claims mirror what the modules pin: issuer
// operan-tenant-control-plane, role admin, tenant-scoped.
func (s *Scheduler) serviceJWT(tenantID string) string {
	now := s.now().Unix()
	header := b64(`{"alg":"HS256","typ":"JWT"}`)
	claims, _ := json.Marshal(map[string]interface{}{
		"iss": "operan-tenant-control-plane", "sub": "cadence-scheduler",
		"tenant_id": tenantID, "role": "admin", "roles": []string{"admin"},
		"user_type": "service", "iat": now, "exp": now + 600,
	})
	payload := header + "." + b64(string(claims))
	mac := hmac.New(sha256.New, []byte(s.JWTSecret))
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
