package flows

import (
	"bytes"
	"crypto/sha256"
	"reflect"
	"strings"
	"testing"
)

// Keep this literal in pre-group wire order: published hashes depend on the bytes.
const groupsLegacyDocument = `{"schema":1,"id":"flow_legacy_groups","kind":"flow","name":"Legacy","nodes":[{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{"sample_data":{}},"settings":{"on_error":"stop","retry":{}}}],"edges":[],"settings":{"concurrency":"queue","max_run_seconds":1800,"notify_on_error":"desktop"}}`

func TestVisualGroupsLegacyDocumentsKeepCanonicalBytes(t *testing.T) {
	for _, kind := range []Kind{KindFlow, KindBlock} {
		t.Run(string(kind), func(t *testing.T) {
			original := []byte(strings.Replace(groupsLegacyDocument, `"kind":"flow"`, `"kind":"`+string(kind)+`"`, 1))
			doc, err := ParseFlow(original)
			if err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 3; pass++ {
				if doc.Groups != nil {
					t.Fatal("old document acquired groups")
				}
				doc.Normalize()
				encoded, err := doc.Marshal()
				if err != nil || !bytes.Equal(encoded, original) || sha256.Sum256(encoded) != sha256.Sum256(original) {
					t.Fatalf("legacy bytes changed on pass %d: %s, %v", pass, encoded, err)
				}
				doc, err = doc.Clone()
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestVisualGroupsDoNotAlterScenarioExecutionFingerprint(t *testing.T) {
	doc, err := ParseFlow([]byte(groupsLegacyDocument))
	if err != nil {
		t.Fatal(err)
	}
	scenario := Scenario{TriggerNode: "n_aaaaaaaa"}
	want, err := scenarioFingerprints(doc, scenario)
	if err != nil {
		t.Fatal(err)
	}
	groups := []VisualGroup{{ID: "g_aaaaaaaa", Title: "Preparation", Color: "blue", Note: "A display annotation", Members: []string{"n_aaaaaaaa"}}}
	doc.Groups = &groups
	doc.Nodes[0].Position = Point{X: 400, Y: 600}
	got, err := scenarioFingerprints(doc, scenario)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("layout changed execution fingerprints: %v", err)
	}
	clone, err := doc.Clone()
	if err != nil || clone.Groups == nil {
		t.Fatalf("clone lost groups: %v", err)
	}
	(*clone.Groups)[0].Members[0] = "n_bbbbbbbb"
	if (*doc.Groups)[0].Members[0] != "n_aaaaaaaa" {
		t.Fatal("group clone aliases the original membership")
	}
	// Parent already has execution significance and must not become a visual group.
	doc.Nodes[0].Parent = "legacy_parent"
	changed, err := scenarioFingerprints(doc, scenario)
	if err != nil || reflect.DeepEqual(changed, want) {
		t.Fatalf("existing Parent semantics changed: %v", err)
	}
}

func TestVisualGroupsExplicitClearSurvivesCloneAndWire(t *testing.T) {
	doc, err := ParseFlow([]byte(strings.TrimSuffix(groupsLegacyDocument, "}") + `,"groups":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	clone, err := doc.Clone()
	if err != nil || clone.Groups == nil || len(*clone.Groups) != 0 {
		t.Fatalf("explicit clear became omission: %v", err)
	}
	encoded, err := clone.Marshal()
	if err != nil || !bytes.Contains(encoded, []byte(`"groups":[]`)) {
		t.Fatalf("explicit clear disappeared from wire: %s, %v", encoded, err)
	}
}
