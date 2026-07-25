package store

import (
	"context"
	"encoding/json"
	"log"

	"github.com/operan/modules/08-tool-execution/internal/database"
)

// durabilitySink writes through to PostgreSQL. Attached at boot; nil means
// memory only, which the service announces rather than hides.
type durabilitySink struct{ db *database.Store }

func (s *ProviderStore) Persist(db *database.Store)   { s.sink = &durabilitySink{db: db} }
func (s *BindingStore) Persist(db *database.Store)    { s.sink = &durabilitySink{db: db} }
func (s *InvocationStore) Persist(db *database.Store) { s.sink = &durabilitySink{db: db} }

func (s *ProviderStore) saveProvider(p *Provider) {
	if s.sink == nil {
		return
	}
	if err := s.sink.db.UpsertProvider(context.Background(), p.ID, p.TenantID, p); err != nil {
		log.Printf("[CAPABILITY] provider %s not persisted (%v) — it will be lost on restart", p.ID, err)
	}
}

func (s *BindingStore) saveBinding(b *CapabilityBinding) {
	if s.sink == nil {
		return
	}
	if err := s.sink.db.UpsertBinding(context.Background(), b.ID, b.TenantID, b); err != nil {
		log.Printf("[CAPABILITY] binding %s not persisted (%v) — it will be lost on restart", b.ID, err)
	}
}

func (s *InvocationStore) saveInvocation(inv *Invocation) {
	if s.sink == nil {
		return
	}
	// The invocation is the audit record; losing it silently defeats the
	// point of the layer, so a failed write is loud.
	if err := s.sink.db.InsertInvocation(context.Background(), inv.ID, inv.TenantID, inv); err != nil {
		log.Printf("[CAPABILITY] AUDIT RECORD %s NOT PERSISTED (%v) — the trail has a gap", inv.ID, err)
	}
}

// Hydrate loads every durable row back into memory at boot. Without this,
// persistence is write-only and a restart still loses everything — the exact
// failure Module 04 shipped with.
func Hydrate(ctx context.Context, db *database.Store, providers *ProviderStore, bindings *BindingStore, invocations *InvocationStore) (np, nb, ni int, err error) {
	err = db.LoadAll(ctx, "m08_providers", func(doc []byte) error {
		var p Provider
		if e := json.Unmarshal(doc, &p); e != nil {
			log.Printf("[CAPABILITY] unreadable provider row skipped: %v", e)
			return nil
		}
		providers.mu.Lock()
		providers.byID[p.ID] = &p
		providers.mu.Unlock()
		np++
		return nil
	})
	if err != nil {
		return
	}
	err = db.LoadAll(ctx, "m08_bindings", func(doc []byte) error {
		var b CapabilityBinding
		if e := json.Unmarshal(doc, &b); e != nil {
			log.Printf("[CAPABILITY] unreadable binding row skipped: %v", e)
			return nil
		}
		bindings.mu.Lock()
		bindings.byID[b.ID] = &b
		bindings.mu.Unlock()
		nb++
		return nil
	})
	if err != nil {
		return
	}
	err = db.LoadAll(ctx, "m08_invocations", func(doc []byte) error {
		var inv Invocation
		if e := json.Unmarshal(doc, &inv); e != nil {
			log.Printf("[CAPABILITY] unreadable invocation row skipped: %v", e)
			return nil
		}
		invocations.mu.Lock()
		invocations.rows = append(invocations.rows, &inv)
		invocations.mu.Unlock()
		ni++
		return nil
	})
	return
}
