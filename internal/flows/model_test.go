package flows

import (
	"errors"
	"strings"
	"testing"
)

func TestKeyFromLabel(t *testing.T) {
	cases := []struct {
		label string
		taken []string
		want  string
	}{
		{"PDF erstellen", nil, "pdf_erstellen"},
		{"KI-Schritt", nil, "ki_schritt"},
		{"Wenn/Dann", nil, "wenn_dann"},
		{"Grüße & Öl", nil, "gruesse_oel"},
		{"   ", nil, "node"},
		{"1. Schritt", nil, "n_1_schritt"},
		{"Trigger", nil, "trigger_node"},
		{"Websuche", []string{"websuche"}, "websuche_2"},
		{"Websuche", []string{"websuche", "websuche_2"}, "websuche_3"},
		{strings.Repeat("a", 60), nil, strings.Repeat("a", 36)},
	}
	for _, tc := range cases {
		taken := map[string]bool{}
		for _, k := range tc.taken {
			taken[k] = true
		}
		got := KeyFromLabel(tc.label, taken)
		if got != tc.want {
			t.Errorf("KeyFromLabel(%q) = %q, want %q", tc.label, got, tc.want)
		}
		if !ValidKey(got) {
			t.Errorf("KeyFromLabel(%q) produced invalid key %q", tc.label, got)
		}
	}
}

func TestValidKey(t *testing.T) {
	for _, key := range []string{"websuche", "a", "ki_schritt2", strings.Repeat("a", 40)} {
		if !ValidKey(key) {
			t.Errorf("ValidKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"", "1abc", "Web", "trigger", "run", "item", "a-b", strings.Repeat("a", 41)} {
		if ValidKey(key) {
			t.Errorf("ValidKey(%q) = true, want false", key)
		}
	}
	if !IsReservedKey("secrets") || IsReservedKey("websuche") {
		t.Fatal("IsReservedKey mismatch")
	}
}

func TestNewIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewNodeID()
		if !ValidNodeID(id) {
			t.Fatalf("NewNodeID() = %q is not valid", id)
		}
		if seen[id] {
			t.Fatalf("duplicate node id %q", id)
		}
		seen[id] = true
	}
	if id := NewFlowID(); !strings.HasPrefix(id, "flow_") || len(id) != 15 {
		t.Fatalf("NewFlowID() = %q", id)
	}
	if id := NewRunID(); !strings.HasPrefix(id, "run_") || len(id) != 16 {
		t.Fatalf("NewRunID() = %q", id)
	}
	if id := NewEdgeID(); !strings.HasPrefix(id, "e_") || len(id) != 10 {
		t.Fatalf("NewEdgeID() = %q", id)
	}
}

func TestParseFlowAppliesDefaults(t *testing.T) {
	data := []byte(`{"schema":1,"id":"flow_x","name":"A","nodes":[{"id":"n_aaaaaaaa","key":"a","type":"test.echo","label":"A","position":{"x":1,"y":2},"settings":{"retry":{"count":9,"delay_seconds":-3}}}]}`)
	f, err := ParseFlow(data)
	if err != nil {
		t.Fatalf("ParseFlow: %v", err)
	}
	if f.Kind != KindFlow || f.Settings.Concurrency != ConcurrencyQueue ||
		f.Settings.MaxRunSeconds != DefaultMaxRunSeconds || f.Settings.NotifyOnError != DefaultNotifyOnError {
		t.Fatalf("flow defaults not applied: %+v", f)
	}
	if f.Edges == nil {
		t.Fatal("edges must be a non-nil empty slice")
	}
	n := f.Nodes[0]
	if n.TypeVersion != 1 || n.Params == nil || n.Settings.OnError != ErrorStop {
		t.Fatalf("node defaults not applied: %+v", n)
	}
	if n.Settings.Retry.Count != MaxRetryCount || n.Settings.Retry.DelaySeconds != 0 {
		t.Fatalf("retry not clamped: %+v", n.Settings.Retry)
	}
	if n.Position.X != 1 || n.Position.Y != 2 {
		t.Fatalf("position = %+v", n.Position)
	}
}

func TestParseFlowRejectsBadInput(t *testing.T) {
	if _, err := ParseFlow([]byte(`{"schema":2,"id":"f","name":"x"}`)); !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("schema 2: err = %v, want ErrUnsupportedSchema", err)
	}
	big := make([]byte, MaxDocumentBytes+1)
	if _, err := ParseFlow(big); !errors.Is(err, ErrDocumentTooLarge) {
		t.Fatalf("big document: err = %v, want ErrDocumentTooLarge", err)
	}
	if _, err := ParseFlow([]byte(`{`)); err == nil {
		t.Fatal("broken JSON must fail")
	}
}

func TestFlowLookupAndClone(t *testing.T) {
	f := &Flow{Schema: SchemaVersion, ID: "flow_x", Name: "x", Nodes: []Node{
		{ID: "n_aaaaaaaa", Key: "a", Type: "test.echo", Params: map[string]any{"value": "1"}},
		{ID: "n_aaaaaaab", Key: "b", Type: "test.echo"},
	}}
	if n := f.NodeByID("n_aaaaaaab"); n == nil || n.Key != "b" {
		t.Fatalf("NodeByID = %+v", n)
	}
	if n := f.NodeByKey("a"); n == nil || n.ID != "n_aaaaaaaa" {
		t.Fatalf("NodeByKey = %+v", n)
	}
	if f.NodeByID("missing") != nil || f.NodeByKey("missing") != nil {
		t.Fatal("lookups of missing nodes must return nil")
	}
	clone, err := f.Clone()
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	clone.Nodes[0].Params["value"] = "changed"
	if f.Nodes[0].Params["value"] != "1" {
		t.Fatal("Clone must deep-copy params")
	}
}
