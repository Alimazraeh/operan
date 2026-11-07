package database

import (
	"regexp"
	"strings"
	"testing"
)

var (
	createTablePattern = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
	uuidTenantPattern  = regexp.MustCompile(`(?i)^\s*tenant_id\s+UUID\b`)
)

// Tenant identity in this platform is a slug string ("acme-corp") carried in
// the JWT tenant_id claim. M02's early migrations declared tenant_id as UUID
// with a foreign key into a local `tenants` table that M01 owns, so an insert
// failed twice over: 22P02 on the slug, and 23503 on a well-formed UUID,
// because nothing populates that local table.
//
// V007 diagnosed this and converted exactly one table — `users` — leaving ten
// behind. That is how the bug survived long enough to break the department
// work loop on 2026-11-07. This test fails for any table declared with a UUID
// tenant_id that no later migration converts, so the next table someone adds
// cannot quietly reintroduce it.
func TestEveryUUIDTenantColumnIsConverted(t *testing.T) {
	list, err := LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}

	declaredIn := map[string]string{} // table -> migration that declared it UUID
	var conversions []string

	for _, m := range list {
		if strings.Contains(strings.ToUpper(m.SQL), "ALTER COLUMN TENANT_ID TYPE") {
			conversions = append(conversions, m.SQL)
		}

		table := ""
		for _, line := range strings.Split(m.SQL, "\n") {
			if match := createTablePattern.FindStringSubmatch(line); match != nil {
				table = match[1]
			}
			if table == "" || !uuidTenantPattern.MatchString(line) {
				continue
			}
			if _, seen := declaredIn[table]; !seen {
				declaredIn[table] = m.Name
			}
		}
	}

	if len(declaredIn) == 0 {
		t.Skip("no migration declares a UUID tenant_id any more — nothing left to convert")
	}

	for table, origin := range declaredIn {
		// Word boundaries matter: "roles" is a substring of "delegation_roles",
		// and a conversion of one is not a conversion of the other.
		named := regexp.MustCompile(`\b` + regexp.QuoteMeta(table) + `\b`)

		converted := false
		for _, sql := range conversions {
			if named.MatchString(sql) {
				converted = true
				break
			}
		}
		if !converted {
			t.Errorf("%s declares %s.tenant_id as UUID and no migration converts it — a slug tenant id cannot be stored in it",
				origin, table)
		}
	}
}

// The conversion runs on a database that may already be converted, so it has
// to be guarded rather than unconditional. A bare ALTER would fail the second
// time the module boots against the same database.
func TestTenantConversionsAreGuarded(t *testing.T) {
	list, err := LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range list {
		if !strings.Contains(strings.ToUpper(m.SQL), "ALTER COLUMN TENANT_ID TYPE") {
			continue
		}
		if !strings.Contains(m.SQL, "data_type = 'uuid'") {
			t.Errorf("%s converts tenant_id without checking the column is still uuid — not safe to re-run", m.Name)
		}
		if !strings.Contains(strings.ToUpper(m.SQL), "DROP CONSTRAINT") {
			t.Errorf("%s converts tenant_id without dropping the foreign key — the type change cannot apply", m.Name)
		}
	}
}
