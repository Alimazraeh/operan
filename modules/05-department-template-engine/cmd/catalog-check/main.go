// Command catalog-check parses and validates the built-in template catalog
// exactly as the service does at startup. Run it from the module root after
// editing any template JSON:
//
//	go run ./cmd/catalog-check
package main

import (
	"fmt"
	"os"

	"github.com/operan/modules/05-department-template-engine/internal/seed"
)

func main() {
	if err := seed.LoadCatalog(os.DirFS("."), "templates"); err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
	cat := seed.Catalog()
	fmt.Printf("OK: %d templates parsed + validated\n", len(cat))
	for _, t := range cat {
		fmt.Printf("  %-38v agents=%2d org=%2d services=%2d streams=%d risks=%d quality=%d compliance=%d\n",
			t.Metadata["catalog_id"], len(t.Agents), len(t.OrgChart), len(t.Services),
			len(t.ValueStreams), len(t.Risks), len(t.QualityStandards), len(t.ComplianceControls))
	}
}
