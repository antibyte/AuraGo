package sqlconnections

import "testing"

func TestQueryMutationPermissions(t *testing.T) {
	for _, tc := range []struct {
		query          string
		change, remove bool
	}{
		{"INSERT INTO t VALUES ('DROP TRUNCATE OR REPLACE DO UPDATE')", false, false},
		{"INSERT INTO t VALUES (1) ON CONFLICT DO NOTHING", false, false},
		{"INSERT INTO t VALUES (1) ON CONFLICT (id) DO UPDATE SET id=2", true, false},
		{"INSERT INTO t VALUES (1) ON DUPLICATE KEY UPDATE id=2", true, false},
		{"REPLACE INTO t VALUES (1)", false, true},
		{"INSERT OR REPLACE INTO t VALUES (1)", false, true},
		{"UPDATE OR REPLACE t SET id=1", true, true},
		{"CREATE TABLE t (id INTEGER)", true, false},
		{"DROP TABLE t", true, true},
		{"TRUNCATE TABLE t", true, true},
		{"ALTER TABLE t DROP COLUMN id", true, true},
		{"CREATE OR REPLACE VIEW v AS SELECT id FROM t", true, true},
	} {
		t.Run(tc.query, func(t *testing.T) {
			kind, err := detectStatementType(tc.query, "sqlite")
			if err != nil {
				t.Fatal(err)
			}
			for mask := 0; mask < 4; mask++ {
				conn := ConnectionRecord{Driver: "sqlite", AllowWrite: true, AllowChange: mask&1 != 0, AllowDelete: mask&2 != 0}
				want := (!tc.change || conn.AllowChange) && (!tc.remove || conn.AllowDelete)
				if err := checkQueryPermissions(conn, kind, tc.query); (err == nil) != want {
					t.Errorf("grants %d: error=%v, want allowed=%v", mask, err, want)
				}
			}
		})
	}
}
