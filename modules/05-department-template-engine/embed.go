package main

import "embed"

// templatesFS embeds the built-in department template catalog. The JSONs are
// parsed at startup (fail-fast on schema drift) and lazily seeded per tenant.
//
//go:embed templates/*.json
var templatesFS embed.FS
