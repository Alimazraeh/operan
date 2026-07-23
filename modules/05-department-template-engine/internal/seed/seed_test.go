package seed

import (
	"os"
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/store"
	"github.com/operan/modules/05-department-template-engine/internal/validate"
)

// The regression gate for the 31-template catalog: every embedded JSON must
// parse into the canonical schema and pass validation with zero errors.
func TestCatalogLoadsAndValidates(t *testing.T) {
	if err := LoadCatalog(os.DirFS("../.."), "templates"); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	cat := Catalog()
	if len(cat) != 31 {
		t.Fatalf("expected 31 templates, got %d", len(cat))
	}

	totalAgents := 0
	for _, tmpl := range cat {
		cid, _ := tmpl.Metadata["catalog_id"].(string)
		if cid == "" {
			t.Errorf("%s: missing catalog_id", tmpl.ID)
		}
		if len(tmpl.Agents) == 0 {
			t.Errorf("%s: no agents", cid)
		}
		if len(tmpl.OrgChart) == 0 {
			t.Errorf("%s: no org chart", cid)
		}
		if errs := validate.Errors(validate.Template(&tmpl)); len(errs) > 0 {
			t.Errorf("%s: %s (%s)", cid, errs[0].Message, errs[0].Path)
		}
		totalAgents += len(tmpl.Agents)

		// Chain of command: exactly one agent without reports_to per template.
		roots := 0
		for _, a := range tmpl.Agents {
			if a.ReportsTo == "" {
				roots++
			}
		}
		if roots != 1 {
			t.Errorf("%s: %d root agents (want 1)", cid, roots)
		}
	}
	if totalAgents != 171 {
		t.Errorf("expected 171 agents across catalog, got %d", totalAgents)
	}
}

// Flagship IT/ops templates must carry the full operating model.
func TestFlagshipsCarryOperatingModel(t *testing.T) {
	if err := LoadCatalog(os.DirFS("../.."), "templates"); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	flagships := map[string]bool{
		"it-small-001": true, "it-medium-001": true, "it-large-001": true,
		"ops-small-001": true, "ops-medium-001": true, "ops-large-001": true, "ops-all-001": true,
	}
	seen := 0
	for _, tmpl := range Catalog() {
		cid, _ := tmpl.Metadata["catalog_id"].(string)
		if !flagships[cid] {
			continue
		}
		seen++
		if tmpl.BusinessLogic == nil || tmpl.BusinessLogic.Purpose == "" {
			t.Errorf("%s: no business logic", cid)
		}
		if len(tmpl.Services) < 5 {
			t.Errorf("%s: %d services (want >=5)", cid, len(tmpl.Services))
		}
		if len(tmpl.ValueStreams) != 3 {
			t.Errorf("%s: %d value streams (want 3)", cid, len(tmpl.ValueStreams))
		}
		if len(tmpl.Risks) < 5 {
			t.Errorf("%s: %d risks (want >=5)", cid, len(tmpl.Risks))
		}
		if len(tmpl.QualityStandards) < 5 {
			t.Errorf("%s: %d quality standards", cid, len(tmpl.QualityStandards))
		}
		if len(tmpl.ComplianceControls) < 5 {
			t.Errorf("%s: %d compliance controls", cid, len(tmpl.ComplianceControls))
		}
		// Every service SLA + owner position must resolve.
		posIDs := map[string]bool{}
		for _, p := range tmpl.OrgChart {
			posIDs[p.ID] = true
		}
		for _, s := range tmpl.Services {
			if s.SLA == nil {
				t.Errorf("%s: service %s has no SLA", cid, s.ID)
			}
			if s.OwnerPositionID != "" && !posIDs[s.OwnerPositionID] {
				t.Errorf("%s: service %s owner position dangling", cid, s.ID)
			}
		}
	}
	if seen != 7 {
		t.Fatalf("expected 7 flagships, saw %d", seen)
	}
}

func TestEnsureTenantIsIdempotent(t *testing.T) {
	if err := LoadCatalog(os.DirFS("../.."), "templates"); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	ts := store.NewTemplateStore()

	first := EnsureTenant(ts, "tenant-a", "user-1")
	if len(first) != 31 {
		t.Fatalf("first seed: %d (want 31)", len(first))
	}
	second := EnsureTenant(ts, "tenant-a", "user-1")
	if len(second) != 0 {
		t.Fatalf("second seed not idempotent: %d", len(second))
	}
	// Different tenant gets its own copies.
	other := EnsureTenant(ts, "tenant-b", "user-1")
	if len(other) != 31 {
		t.Fatalf("tenant-b seed: %d", len(other))
	}
	_, totalA, _ := ts.List("tenant-a", 1, 200, nil)
	if totalA != 31 {
		t.Fatalf("tenant-a templates: %d", totalA)
	}
}
