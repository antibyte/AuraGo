package optimizer

import (
	"aurago/internal/prompts"
	"context"
	"testing"
)

func TestDisclosureRegressionFamilyToolsRemainSeparate(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	token := db.recordPromptGuideExposures([]prompts.ToolGuideExposure{{Manual: "email", Version: "v1", SourceRevision: "same"}}, "request")["email"]
	for _, name := range []string{"send_email", "fetch_email"} {
		if err := db.LogToolTrace(name, true, 0, token, "", 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.db.QueryRow(`SELECT count(*) FROM tool_traces WHERE exposure_id IS NOT NULL`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("two distinct native actions collapsed into %d observation(s)", count)
	}
}

func TestDisclosureRegressionFamilyToolsAreNotComparableOperations(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	for _, version := range []string{"v1", "candidate"} {
		tool := "fetch_email"
		if version == "candidate" {
			tool = "send_email"
		}
		for i := 0; i < 5; i++ {
			token := db.recordPromptGuideExposures([]prompts.ToolGuideExposure{{Manual: "email", Version: version, SourceRevision: "same"}}, "request")["email"]
			if err := db.LogToolTrace(tool, version == "candidate", 0, token, "", 1, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	worker := NewOptimizerWorker(db, nil, nil, 0)
	a, b, comparable := worker.comparableRates(context.Background(), "email", "candidate", "v1")
	if comparable {
		t.Fatalf("different tools treated as comparable: send_email=%v fetch_email=%v", a, b)
	}
}

func TestActionIdentityMigrationPreservesUnattributedHistory(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	token := db.recordPromptGuideExposures([]prompts.ToolGuideExposure{{Manual: "email", Version: "v1", SourceRevision: "same"}}, "request")["email"]
	if err := db.LogToolTrace("send_email", true, 0, token, "", 1, ""); err != nil {
		t.Fatal(err)
	}
	// Recreate the immediately previous schema, including an ambiguous row.
	for _, statement := range []string{
		`DROP INDEX idx_trace_exposure_action_operation`,
		`ALTER TABLE tool_traces DROP COLUMN action_identity`,
		`CREATE UNIQUE INDEX idx_trace_exposure_operation ON tool_traces(exposure_id,operation)`,
	} {
		if _, err := db.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := initializeExposureSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var retained int
	if err := db.db.QueryRow(`SELECT count(*) FROM tool_traces WHERE exposure_id IS NOT NULL AND action_identity=''`).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("lost historical provenance: count=%d error=%v", retained, err)
	}
	stats, err := db.GetDashboardStats()
	if err != nil || stats.LegacyTraceEvents != 1 || stats.TotalTraceEvents != 0 {
		t.Fatalf("unattributed row counted as verified evidence: %+v %v", stats, err)
	}
	for _, action := range []string{"send_email", "fetch_email"} {
		if err := db.LogToolTrace(action, true, 0, token, "", 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	stats, err = db.GetDashboardStats()
	if err != nil || stats.TotalTraceEvents != 2 || stats.LegacyTraceEvents != 1 {
		t.Fatalf("migration lost distinct actions: %+v %v", stats, err)
	}
}

func TestComparableRatesSeparateDynamicTargetsAndNormalizeOperations(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	worker := NewOptimizerWorker(db, nil, nil, 0)
	for _, version := range []string{"v1", "candidate"} {
		for i := 0; i < 5; i++ {
			token := db.recordPromptGuideExposures([]prompts.ToolGuideExposure{{Manual: "mcp", Version: version, SourceRevision: "same"}}, "request")["mcp"]
			target := "server/read"
			if version == "candidate" {
				target = "server/write"
			}
			if err := db.LogToolTrace("mcp_call", true, 0, token, "", 1, " CALL_TOOL ", target); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, _, ok := worker.comparableRates(context.Background(), "mcp", "candidate", "v1"); ok {
		t.Fatal("different MCP targets compared")
	}
	if _, err := db.db.Exec(`UPDATE tool_traces SET action_identity='server/read' WHERE action_identity='server/write'`); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := worker.comparableRates(context.Background(), "mcp", "candidate", "v1"); !ok {
		t.Fatal("matched target/source/operation evidence was rejected")
	}
}
