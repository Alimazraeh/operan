package vocab

import (
	"log"
	"strings"

	"github.com/google/uuid"

	"github.com/operan/modules/08-tool-execution/internal/simulated"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// SeedCapabilities loads the vocabulary into the store. Idempotent by
// construction: the vocabulary lives in code, so a boot always carries the
// current product truth.
func SeedCapabilities(s *store.CapabilityStore) int {
	caps := Capabilities()
	for _, c := range caps {
		s.Put(c)
	}
	return len(caps)
}

// seedNamespace makes seeded ids deterministic, which is what makes seeding
// idempotent across restarts: the same tenant always produces the same
// provider and binding ids, so a re-run upserts instead of duplicating.
var seedNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("operan/m08-simulated-seed"))

// SeedSimulatedTenants gives each named tenant the simulated provider and a
// tenant-default binding for every capability. This is demo bootstrap, driven
// by an explicit env var — a real customer creates providers and bindings
// through the API, and swapping simulated for live is a binding change.
func SeedSimulatedTenants(tenantsCSV string, providers *store.ProviderStore, bindings *store.BindingStore, caps *store.CapabilityStore) {
	for _, tenant := range strings.Split(tenantsCSV, ",") {
		tenant = strings.TrimSpace(tenant)
		if tenant == "" {
			continue
		}
		pid := uuid.NewSHA1(seedNamespace, []byte(tenant+"/provider")).String()
		if _, ok := providers.Get(tenant, pid); !ok {
			providers.Put(&store.Provider{
				ID: pid, TenantID: tenant, Kind: simulated.Kind,
				Name:   "Simulated systems",
				Status: "active",
			})
		}
		n := 0
		for _, c := range caps.List() {
			bid := uuid.NewSHA1(seedNamespace, []byte(tenant+"/binding/"+c.ID)).String()
			if _, ok := bindings.Get(tenant, bid); ok {
				continue
			}
			bindings.Put(&store.CapabilityBinding{
				ID: bid, TenantID: tenant, CapabilityID: c.ID,
				ProviderID: pid, ProviderTool: c.ID,
				Enabled: true, Simulated: true,
			})
			n++
		}
		if n > 0 {
			log.Printf("[CAPABILITY] seeded simulated provider + %d binding(s) for tenant %s", n, tenant)
		}
	}
}
