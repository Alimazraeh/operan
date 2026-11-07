package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

var migSeq atomic.Int64

// ─── Recording driver ───────────────────────────────────────────────────────
//
// Migrate's entire contract is the SQL it issues, so without a live PostgreSQL
// that is the testable surface: a minimal database/sql driver captures each
// statement in order. Same approach as the reconciliation test in
// internal/repository.

type recordingDriver struct{ stmts []string }

func (d *recordingDriver) Open(string) (driver.Conn, error) { return &recordingConn{d: d}, nil }

type recordingConn struct{ d *recordingDriver }

func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not part of this test")
}
func (c *recordingConn) Close() error              { return nil }
func (c *recordingConn) Begin() (driver.Tx, error) { return nil, errors.New("no tx") }

func (c *recordingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.d.stmts = append(c.d.stmts, query)
	return driver.RowsAffected(0), nil
}

func migrationSQL(t *testing.T) []string {
	t.Helper()
	d := &recordingDriver{}
	name := fmt.Sprintf("recording-%d", migSeq.Add(1))
	sql.Register(name, d)

	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(d.stmts) == 0 {
		t.Fatal("Migrate issued no statements")
	}
	return d.stmts
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// ─── Tenant ids are slugs, not UUIDs ────────────────────────────────────────

// Tenant ids in this platform are slugs — the portal mints them, and every
// other module stores tenant_id as a string. M03 declared them UUID, so every
// insert died with 22P02 once its writes actually reached Postgres.
func TestMigrate_TenantIDIsNeverUUID(t *testing.T) {
	for _, stmt := range migrationSQL(t) {
		for _, line := range strings.Split(stmt, "\n") {
			l := strings.TrimSpace(line)
			if !strings.HasPrefix(l, "tenant_id") {
				continue
			}
			if strings.Contains(strings.ToUpper(l), "UUID") {
				t.Errorf("tenant_id declared as UUID in %q: a slug tenant id cannot be stored", l)
			}
		}
	}
}

// The foreign key pointed at a tenants table that nothing ever populates —
// ensureTenant() was a no-op stub — so even a well-formed UUID failed 23503.
func TestMigrate_NoForeignKeyIntoTenants(t *testing.T) {
	for _, stmt := range migrationSQL(t) {
		// The conversion block names the tenants table in order to drop these
		// constraints; only table declarations may not carry them.
		if strings.Contains(stmt, "DROP CONSTRAINT") {
			continue
		}
		if strings.Contains(stmt, "REFERENCES tenants(id)") {
			t.Errorf("foreign key into tenants in %q: nothing populates that table", firstLine(stmt))
		}
	}
}

// ─── The conversion for databases created by earlier versions ───────────────

func TestMigrate_ConvertsLegacyColumnsAfterCreatingTables(t *testing.T) {
	stmts := migrationSQL(t)
	last := stmts[len(stmts)-1]

	if !strings.Contains(last, "ALTER COLUMN tenant_id TYPE TEXT") {
		t.Errorf("the legacy conversion must run last, after the CREATE TABLEs; last statement was %q", firstLine(last))
	}
	if !strings.Contains(last, "DROP CONSTRAINT") {
		t.Error("the conversion must also drop the tenants foreign keys, or the type change cannot apply")
	}
}

// Migrate runs on every boot, so an unconditional ALTER would fail the second
// time. Both loops must select only what is still unconverted.
func TestMigrate_ConversionIsRerunSafe(t *testing.T) {
	stmts := migrationSQL(t)
	last := stmts[len(stmts)-1]

	for _, guard := range []string{"data_type = 'uuid'", "contype = 'f'"} {
		if !strings.Contains(last, guard) {
			t.Errorf("conversion is missing the %q guard — it would not be safe to re-run on boot", guard)
		}
	}
}

// Every table that scopes rows by tenant needs the column indexed; the type
// change must not have dropped the indexes with the constraints.
func TestMigrate_TenantColumnsStayIndexed(t *testing.T) {
	declared, indexed := 0, 0
	for _, stmt := range migrationSQL(t) {
		for _, line := range strings.Split(stmt, "\n") {
			// The DDL pads the type column for alignment, so collapse runs of
			// whitespace before matching.
			if strings.HasPrefix(strings.Join(strings.Fields(line), " "), "tenant_id TEXT NOT NULL") {
				declared++
			}
		}
		indexed += strings.Count(stmt, "(tenant_id)")
	}
	if declared == 0 {
		t.Fatal("no tenant_id columns declared at all")
	}
	if indexed < declared {
		t.Errorf("%d tenant_id columns declared but only %d indexed", declared, indexed)
	}
}
