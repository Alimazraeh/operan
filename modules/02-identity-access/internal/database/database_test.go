package database

import "testing"

// The splitter previously split on every semicolon regardless of context, so
// a DO block (whose body is full of semicolons) was shredded into fragments
// that are not valid SQL — the failure that blocked V007.
func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want int
	}{
		{"simple", "SELECT 1; SELECT 2;", 2},
		{"trailing statement without semicolon", "SELECT 1; SELECT 2", 2},
		{"blank statements collapse", "SELECT 1;;; SELECT 2;", 2},
		{"semicolon inside a line comment",
			"-- first; second; third\nSELECT 1;", 1},
		{"semicolon inside a string literal",
			"INSERT INTO t VALUES ('a;b');", 1},
		{"doubled quote escape",
			"INSERT INTO t VALUES ('it''s; fine');", 1},
		{"semicolon inside a quoted identifier",
			`SELECT "weird;name" FROM t;`, 1},
		{"dollar-quoted DO block stays whole", `
DO $$
BEGIN
    ALTER TABLE users DROP CONSTRAINT IF EXISTS c;
    ALTER TABLE users ALTER COLUMN x TYPE TEXT;
END $$;`, 1},
		{"tagged dollar quote", `
DO $mig$
BEGIN
    PERFORM 1;
    PERFORM 2;
END $mig$;
SELECT 1;`, 2},
		{"DO block alongside plain statements", `
ALTER TABLE users ADD COLUMN IF NOT EXISTS a TEXT;
DO $$
BEGIN
    IF TRUE THEN
        ALTER TABLE users ADD COLUMN b TEXT;
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS i ON users(a);`, 3},
	}
	for _, c := range cases {
		got := splitStatements(c.sql)
		if len(got) != c.want {
			t.Errorf("%s: got %d statements, want %d:\n%#v", c.name, len(got), c.want, got)
		}
	}
}

// Every embedded migration must survive splitting — this is the guard that
// would have caught V007 before it reached a cluster.
func TestEmbeddedMigrationsSplitCleanly(t *testing.T) {
	list, err := LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no migrations embedded")
	}
	for _, m := range list {
		stmts := splitStatements(m.SQL)
		if len(stmts) == 0 {
			t.Errorf("%s produced no statements", m.Name)
		}
		for _, s := range stmts {
			// A fragment starting with a procedural keyword means a DO block
			// was cut in half.
			for _, bad := range []string{"BEGIN\n", "END IF", "END $$"} {
				if len(s) > len(bad) && s[:len(bad)] == bad {
					t.Errorf("%s: statement begins mid-block with %q", m.Name, bad)
				}
			}
		}
	}
}
