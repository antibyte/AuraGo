package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func contactSyncTestGraph(t *testing.T) *KnowledgeGraph {
	t.Helper()
	kg, err := NewKnowledgeGraph(":memory:", "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	return kg
}

func TestContactSyncClearsOwnedFieldsAndPreservesForeignProperties(t *testing.T) {
	for _, key := range []string{"email", "phone", "mobile", "relationship", "birthday"} {
		t.Run(key, func(t *testing.T) {
			kg := contactSyncTestGraph(t)
			fields := map[string]string{"email": "synthetic@example.invalid", "phone": "fixture-phone", "mobile": "fixture-mobile", "relationship": "Old Org", "birthday": "2000-01-01"}
			if err := kg.SyncContact(t.Context(), "contact_1", "Synthetic Contact", fields); err != nil {
				t.Fatal(err)
			}
			if err := kg.AddNode("contact_1", "", map[string]string{"foreign_note": "preserve me"}); err != nil {
				t.Fatal(err)
			}
			fields[key] = ""
			for range 2 {
				if err := kg.SyncContact(t.Context(), "contact_1", "Synthetic Contact", fields); err != nil {
					t.Fatal(err)
				}
			}
			node, err := kg.GetNode("contact_1")
			if err != nil || node.Properties[key] != "" || node.Properties["foreign_note"] != "preserve me" {
				t.Fatalf("reconciled node=%+v err=%v", node, err)
			}
			var claims int
			if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_claims WHERE subject_id='contact_1'`).Scan(&claims); err != nil {
				t.Fatal(err)
			}
			want := 1
			if key == "relationship" {
				want = 0
			}
			if claims != want {
				t.Fatalf("idempotent claims=%d want=%d", claims, want)
			}
		})
	}
}

func TestContactSyncRetainsProtectedAndUnownedRelationships(t *testing.T) {
	for _, scenario := range []string{"contact protected", "organization protected", "edge protected", "foreign claim", "legacy ownership"} {
		t.Run(scenario, func(t *testing.T) {
			kg := contactSyncTestGraph(t)
			if err := kg.SyncContact(t.Context(), "contact_1", "Synthetic Contact", map[string]string{"relationship": "Old Org"}); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "contact protected":
				if _, err := kg.SetNodeProtected("contact_1", true); err != nil {
					t.Fatal(err)
				}
			case "organization protected":
				if _, err := kg.SetNodeProtected("org_old_org", true); err != nil {
					t.Fatal(err)
				}
			case "edge protected":
				if _, err := kg.db.Exec(`UPDATE kg_edges SET properties=json_set(properties,'$.protected','true')`); err != nil {
					t.Fatal(err)
				}
			case "foreign claim":
				if err := kg.AddEdge("contact_1", "org_old_org", "belongs_to", nil); err != nil {
					t.Fatal(err)
				}
			case "legacy ownership":
				if _, err := kg.db.Exec(`UPDATE kg_edges SET properties=json_remove(properties,'$.contact_sync_id')`); err != nil {
					t.Fatal(err)
				}
			}
			if err := kg.SyncContact(t.Context(), "contact_1", "Synthetic Contact", nil); err == nil {
				t.Fatal("retained relationship must require review")
			}
			var edges int
			if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_edges WHERE source='contact_1'`).Scan(&edges); err != nil || edges != 1 {
				t.Fatalf("protected/foreign edge removed: count=%d err=%v", edges, err)
			}
			if scenario == "contact protected" {
				node, err := kg.GetNode("contact_1")
				if err != nil || node.Properties["relationship"] != "Old Org" {
					t.Fatalf("protected node changed: %+v err=%v", node, err)
				}
			}
		})
	}
}

func TestContactSyncRollsBackSQLFailureAndCancellation(t *testing.T) {
	kg := contactSyncTestGraph(t)
	if err := kg.SyncContact(t.Context(), "contact_1", "Synthetic Contact", map[string]string{"relationship": "Old Org", "email": "old@example.invalid"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`CREATE TRIGGER fail_contact_claim BEFORE INSERT ON kg_claims BEGIN SELECT RAISE(ABORT,'controlled failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := kg.SyncContact(t.Context(), "contact_1", "Synthetic Contact", map[string]string{"relationship": "New Org"}); err == nil {
		t.Fatal("fixture must fail the new claim")
	}
	node, err := kg.GetNode("contact_1")
	if err != nil || node.Properties["relationship"] != "Old Org" || node.Properties["email"] != "old@example.invalid" {
		t.Fatalf("partial SQL update committed: %+v err=%v", node, err)
	}
	var edges int
	if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_edges WHERE source='contact_1' AND target='org_old_org'`).Scan(&edges); err != nil || edges != 1 {
		t.Fatalf("rollback lost edge: %d %v", edges, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := kg.SyncContact(ctx, "contact_1", "Synthetic Contact", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
