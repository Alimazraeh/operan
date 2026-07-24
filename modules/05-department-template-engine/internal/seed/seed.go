// Package seed loads the embedded built-in template catalog and copies it
// into tenants' template stores on demand (lazily on first list, or via
// POST /templates/seed). Catalog identity is metadata.catalog_id (the file's
// template id, e.g. "it-medium-001"), which survives the per-tenant UUID
// assignment on create.
package seed

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/operan/modules/05-department-template-engine/internal/store"
	"github.com/operan/modules/05-department-template-engine/internal/validate"
)

var (
	mu      sync.RWMutex
	catalog []store.Template // parsed built-in templates, catalog order
)

// LoadCatalog parses every *.json under dir in fsys into the package catalog.
// Any file that fails to parse or fails validation makes this return an error —
// startup should fail loudly rather than serve a drifted catalog.
func LoadCatalog(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read catalog dir: %w", err)
	}

	var loaded []store.Template
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		data, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		var t store.Template
		if err := json.Unmarshal(data, &t); err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		if t.ID == "" {
			return fmt.Errorf("%s: missing template id", name)
		}
		if issues := validate.Template(&t); len(issues) > 0 {
			return fmt.Errorf("%s: invalid template: %s", name, issues[0].Message)
		}
		// The file id (e.g. "it-medium-001") becomes the catalog identity;
		// the per-tenant copy gets a fresh UUID on create.
		if t.Metadata == nil {
			t.Metadata = map[string]interface{}{}
		}
		t.Metadata["catalog_id"] = t.ID
		loaded = append(loaded, t)
	}

	if len(loaded) == 0 {
		return fmt.Errorf("catalog dir %s contains no templates", dir)
	}

	mu.Lock()
	catalog = loaded
	mu.Unlock()
	return nil
}

// Catalog returns a copy of the parsed built-in catalog.
func Catalog() []store.Template {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]store.Template, len(catalog))
	copy(out, catalog)
	return out
}

// EnsureTenant idempotently copies any catalog template the tenant lacks into
// its store. Returns the catalog_ids seeded on this call.
func EnsureTenant(ts *store.TemplateStore, tenantID, createdBy string) []string {
	if tenantID == "" {
		return nil
	}
	cat := Catalog()
	if len(cat) == 0 {
		return nil
	}

	// Collect the tenant's existing catalog templates: id + stored version.
	type existing struct{ id, version string }
	have := map[string]existing{}
	page := 1
	for {
		templates, _, hasMore := ts.List(tenantID, page, 200, nil)
		for _, t := range templates {
			if cid, ok := t.Metadata["catalog_id"].(string); ok {
				have[cid] = existing{id: t.ID, version: t.Version}
			}
		}
		if !hasMore {
			break
		}
		page++
	}

	var seeded []string
	for _, ct := range cat {
		cid, _ := ct.Metadata["catalog_id"].(string)
		if cid == "" {
			continue
		}
		if ex, ok := have[cid]; ok {
			// Upsert: when the built-in catalog carries a newer version,
			// refresh the tenant's copy in place (same template id).
			if ex.version != ct.Version {
				cp := ct
				if _, err := ts.RefreshFromCatalog(ex.id, tenantID, &cp); err == nil {
					seeded = append(seeded, cid+"@"+ct.Version)
				}
			}
			continue
		}
		cp := ct
		cp.ID = "" // store assigns a fresh UUID
		cp.TenantID = tenantID
		cp.Status = "published"
		cp.CreatedBy = createdBy
		// Deep-copy metadata so tenants don't share the catalog map.
		md := make(map[string]interface{}, len(ct.Metadata))
		for k, v := range ct.Metadata {
			md[k] = v
		}
		cp.Metadata = md
		if _, err := ts.Create(&cp); err == nil {
			seeded = append(seeded, cid)
		}
	}
	return seeded
}
