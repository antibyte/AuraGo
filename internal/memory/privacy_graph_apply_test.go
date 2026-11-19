package memory

import (
	"context"
	"testing"
)

func TestSessionGraphPrivacyClaimRetainsProtectedEndpoint(t *testing.T) {
	kg := contactSyncTestGraph(t)
	ctx := context.Background()
	marker := "deleted:session-fingerprint"
	if err := kg.AddNode("protected_person", "Protected person", map[string]string{"protected": "true"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("ordinary_fact", "Ordinary fact", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.ExecContext(ctx, `INSERT INTO kg_claims(id,subject_id,predicate,object_id,source_kind,session_id,source_message_id)
		VALUES('privacy-claim','protected_person','knows','ordinary_fact','chat',?,'')`, marker); err != nil {
		t.Fatal(err)
	}
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	values, _, err := readPrivacyClaimFingerprintValues(ctx, tx, "privacy-claim")
	_ = tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	target := privacyDeletionTarget{
		Store: "kg_claim", TargetID: "privacy-claim", SourceMarker: marker,
		ExpectedFingerprint: privacyFingerprint(values[1:]...), State: "pending",
	}
	deleted, err := kg.applySessionGraphPrivacyTarget(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("claim on a protected endpoint was deleted")
	}
	var count int
	if err := kg.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_claims WHERE id='privacy-claim'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("protected endpoint claim count = %d, want 1", count)
	}
}

func TestSessionGraphPrivacyEdgeRetainsProtectedOrKeepForeverState(t *testing.T) {
	for _, scenario := range []string{"edge_keep_forever", "endpoint_keep_forever", "endpoint_protected"} {
		t.Run(scenario, func(t *testing.T) {
			kg := contactSyncTestGraph(t)
			ctx := context.Background()
			marker := "deleted:edge-protection"
			sourceProps := map[string]string{}
			if scenario == "endpoint_keep_forever" {
				sourceProps["keep_forever"] = "true"
			}
			if scenario == "endpoint_protected" {
				sourceProps["protected"] = "true"
			}
			if err := kg.AddNode("privacy_source", "Source", sourceProps); err != nil {
				t.Fatal(err)
			}
			if err := kg.AddNode("privacy_destination", "Destination", nil); err != nil {
				t.Fatal(err)
			}
			properties := `{"source":"chat","session_id":"` + marker + `"}`
			if scenario == "edge_keep_forever" {
				properties = `{"source":"chat","keep_forever":"true","session_id":"` + marker + `"}`
			}
			if _, err := kg.db.ExecContext(ctx, `INSERT INTO kg_edges(source,target,relation,properties) VALUES('privacy_source','privacy_destination','privacy_relation',?)`, properties); err != nil {
				t.Fatal(err)
			}
			target := privacyDeletionTarget{
				Store: "kg_edge", TargetID: graphPrivacyEdgeID("privacy_source", "privacy_destination", "privacy_relation"),
				ExpectedFingerprint: privacyFingerprint("privacy_source", "privacy_destination", "privacy_relation", properties),
				SourceMarker:        marker, State: "pending",
			}
			deleted, err := kg.applySessionGraphPrivacyTarget(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			if deleted {
				t.Fatal("privacy cleanup deleted an edge marked protected or keep_forever")
			}
			var count int
			if err := kg.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_edges WHERE source='privacy_source' AND target='privacy_destination'`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("protected edge count=%d, want 1 (%s)", count, scenario)
			}
		})
	}
}

func TestPrivacyEdgesRequireExclusiveProperties(t *testing.T) {
	const marker = "deleted:example"
	for _, tc := range []struct {
		name, store, properties string
		wantDeleted             bool
	}{
		{"chat exclusively owned", "kg_edge", `{"session_id":"deleted:example","source":"chat","confidence":"0.80"}`, true},
		{"chat same source listed", "kg_edge", `{"session_id":"deleted:example","source":"chat","source_sessions":"[\"deleted:example\"]"}`, true},
		{"chat mixed sessions", "kg_edge", `{"session_id":"deleted:example","source":"chat","source_sessions":"[\"deleted:example\",\"other\"]"}`, false},
		{"chat unknown sessions", "kg_edge", `{"session_id":"deleted:example","source":"chat","source_sessions":"uncertain"}`, false},
		{"chat manual", "kg_edge", `{"session_id":"deleted:example","source":"manual"}`, false},
		{"chat merged", "kg_edge", `{"session_id":"deleted:example","source":"chat","external_note":"curated"}`, false},
		{"contact exclusively owned", "kg_contact_edge", `{"contact_sync_id":"deleted:example","source":"contacts_sync","confidence":"0.50","extracted_at":"2026-01-01"}`, true},
		{"contact merged", "kg_contact_edge", `{"contact_sync_id":"deleted:example","source":"contacts_sync","external_note":"curated"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kg := contactSyncTestGraph(t)
			for _, id := range []string{"person", "club"} {
				if err := kg.AddNode(id, id, nil); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := kg.db.Exec(`INSERT INTO kg_edges(source,target,relation,properties) VALUES('person','club','belongs_to',?)`, tc.properties); err != nil {
				t.Fatal(err)
			}
			target := privacyDeletionTarget{
				Store: tc.store, TargetID: graphPrivacyEdgeID("person", "club", "belongs_to"), SourceMarker: marker,
				ExpectedFingerprint: privacyFingerprint("person", "club", "belongs_to", tc.properties),
			}
			apply := kg.applySessionGraphPrivacyTarget
			if tc.store == "kg_contact_edge" {
				apply = kg.applyContactPrivacyTarget
			}
			deleted, err := apply(t.Context(), target)
			if err != nil || deleted != tc.wantDeleted {
				t.Fatalf("deleted=%v want=%v err=%v", deleted, tc.wantDeleted, err)
			}
			var remaining int
			if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_edges`).Scan(&remaining); err != nil || (remaining == 0) != tc.wantDeleted {
				t.Fatalf("remaining=%d err=%v", remaining, err)
			}
		})
	}
}
