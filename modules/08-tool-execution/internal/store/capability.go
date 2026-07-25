package store

import (
	"sort"
	"sync"
)

// Capability is a business verb an SOP can bind to — the stable vocabulary of
// the platform. SOPs name capabilities, never vendors: "assign this ticket",
// not "call Jira". Which system actually performs the verb is decided per
// tenant (or per department) by a CapabilityBinding, so the same SOP runs
// against ManageEngine for one customer and Jira for another without a word
// of it changing.
//
// The vocabulary is product-owned and seeded from code, deliberately narrow:
// every verb added here is a permanent contract, so verbs are added on
// evidence from the SOP catalogue, not in anticipation.
type Capability struct {
	ID          string `json:"id"`     // e.g. "itsm.ticket.assign"
	Domain      string `json:"domain"` // itsm | identity | itops | comms
	Name        string `json:"name"`
	Description string `json:"description"`
	// InputSchema and OutputSchema are JSON Schema documents. The input schema
	// is enforced — an invocation whose input does not validate never reaches
	// a provider. Contracts are canonical but narrow: only the fields the verb
	// actually needs, never a vendor's whole object model.
	InputSchema  map[string]interface{} `json:"input_schema"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	// SideEffect drives governance: read | write | destructive.
	SideEffect string `json:"side_effect"`
	// MinAutonomy is the least autonomy tier a position must hold for its
	// agent to perform this verb: recommend < draft < execute < coordinate.
	MinAutonomy string `json:"min_autonomy"`
}

// AutonomyRank orders the tiers so "at least execute" is computable. Unknown
// tiers rank 0 — below everything — so an actor whose authority cannot be
// established is denied write verbs rather than waved through.
func AutonomyRank(tier string) int {
	switch tier {
	case "recommend":
		return 1
	case "draft":
		return 2
	case "execute":
		return 3
	case "coordinate":
		return 4
	default:
		return 0
	}
}

// CapabilityStore holds the vocabulary. It is global, not tenant-scoped: the
// verbs are the product's language, and tenants differ only in bindings.
type CapabilityStore struct {
	mu   sync.RWMutex
	caps map[string]*Capability
}

func NewCapabilityStore() *CapabilityStore {
	return &CapabilityStore{caps: map[string]*Capability{}}
}

func (s *CapabilityStore) Put(c *Capability) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.caps[c.ID] = c
}

func (s *CapabilityStore) Get(id string) (*Capability, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.caps[id]
	return c, ok
}

func (s *CapabilityStore) List() []*Capability {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Capability, 0, len(s.caps))
	for _, c := range s.caps {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
