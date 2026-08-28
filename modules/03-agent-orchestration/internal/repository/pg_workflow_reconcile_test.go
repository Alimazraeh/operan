package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
)

var fakeSeq atomic.Int64

// ─── Fake driver for recording SQL statements ───────────────────────────────
//
// The reconciliation logic is a single UPDATE; the testable surface without a
// live PostgreSQL is that the right statement is issued with the right
// parameter and that the affected-row count is returned. A minimal
// database/sql driver records the call and lets us control the result.

type fakeDriver struct {
	lastQuery string
	lastArgs  []any
	affected  int64
	execErr   error
}

func (d *fakeDriver) Open(string) (driver.Conn, error) { return d, nil }
func (d *fakeDriver) Prepare(query string) (driver.Stmt, error) {
	d.lastQuery = query
	return &fakeStmt{d: d}, nil
}
func (d *fakeDriver) Close() error              { return nil }
func (d *fakeDriver) Begin() (driver.Tx, error) { return nil, errors.New("no tx") }

type fakeStmt struct{ d *fakeDriver }

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }
func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.d.lastArgs = make([]any, len(args))
	for i, a := range args {
		s.d.lastArgs[i] = a
	}
	if s.d.execErr != nil {
		return nil, s.d.execErr
	}
	return driver.RowsAffected(s.d.affected), nil
}
func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, errors.New("no query in fake")
}

type fakeRows struct{ cols []string }

func (r *fakeRows) Columns() []string            { return r.cols }
func (r *fakeRows) Close() error                  { return nil }
func (r *fakeRows) Next(dest []driver.Value) error { return io.EOF }

func newFakeDB(affected int64, execErr error) (*sql.DB, *fakeDriver) {
	d := &fakeDriver{affected: affected, execErr: execErr}
	name := fmt.Sprintf("reconcile-fake-%d", fakeSeq.Add(1))
	sql.Register(name, d)
	db, err := sql.Open(name, "fake")
	if err != nil {
		panic(err)
	}
	return db, d
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestFailOrphanedActive_FailsNonTerminalAndReportsCount(t *testing.T) {
	db, drv := newFakeDB(3, nil)
	defer db.Close()
	r := &WorkflowPostgres{db: db}

	n, err := r.FailOrphanedActive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 affected rows, got %d", n)
	}

	// The statement must target only non-terminal statuses and set the
	// terminal "failed" status plus completed_at.
	q := drv.lastQuery
	for _, want := range []string{
		"UPDATE workflows",
		"SET status=",
		"completed_at=now()",
		"'pending','running','paused'",
	} {
		if !contains(q, want) {
			t.Errorf("statement missing %q:\n%s", want, q)
		}
	}
	// The parameter must be the "failed" status.
	if len(drv.lastArgs) != 1 || drv.lastArgs[0] != "failed" {
		t.Errorf("expected single param 'failed', got %v", drv.lastArgs)
	}
}

func TestFailOrphanedActive_ZeroOrphans(t *testing.T) {
	db, _ := newFakeDB(0, nil)
	defer db.Close()
	r := &WorkflowPostgres{db: db}

	n, err := r.FailOrphanedActive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 affected rows, got %d", n)
	}
}

func TestFailOrphanedActive_DriverErrorPropagates(t *testing.T) {
	db, _ := newFakeDB(0, errors.New("connection refused"))
	defer db.Close()
	r := &WorkflowPostgres{db: db}

	_, err := r.FailOrphanedActive()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// contains is a tiny substring helper (avoid strings.Contains import churn in
// a test file that otherwise only needs the driver machinery).
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var _ = context.Background
