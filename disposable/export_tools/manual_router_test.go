package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestManualRouterCanonicalBindingsAndSources(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	catalog, _, err := buildRoutingCatalog(root, filepath.Join(root, "training"))
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]string{}
	absent := 0
	for _, tool := range catalog.Tools {
		bindings[tool.Name] = tool.ManualID
		if tool.ManualID == "" {
			absent++
			if tool.AbsenceReason == "" {
				t.Fatalf("missing absence reason for %s", tool.Name)
			}
		}
	}
	for tool, want := range map[string]string{"send_email": "email", "fetch_email": "email", "mqtt_publish": "mqtt", "mqtt_subscribe": "mqtt", "mcp_call": "mcp", "manage_memory": "core_memory"} {
		if bindings[tool] != want {
			t.Errorf("%s manual = %q, want %q", tool, bindings[tool], want)
		}
	}
	if absent != 5 {
		t.Errorf("documented manual absences = %d, want 5", absent)
	}
	for _, manual := range catalog.Manuals {
		if routingDigest([]byte(manual.Body)) != manual.SHA256 || strings.TrimSpace(manual.Body) == "" {
			t.Errorf("invalid complete source for %s", manual.ID)
		}
	}
}

func TestManualRouterSearchHonorsExplicitEmptyAndAllowedFamilies(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	catalog, search, err := buildRoutingCatalog(root, filepath.Join(root, "training"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input string
		empty bool
	}{
		{`{"query":"email", "allowed_manuals":[]}`, true},
		{`{"query":"email", "allowed_manuals":["email"]}`, false},
	} {
		var output bytes.Buffer
		if err := serveRoutingSearch(strings.NewReader(test.input+"\n"), &output, search, catalog); err != nil {
			t.Fatal(err)
		}
		var result struct {
			IDs []string `json:"manual_ids"`
			SHA string   `json:"catalog_sha256"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.SHA != catalog.CatalogSHA256 {
			t.Fatal("search lost catalog identity")
		}
		if test.empty && len(result.IDs) != 0 {
			t.Fatalf("empty authorization returned %v", result.IDs)
		}
		if !test.empty && (len(result.IDs) != 1 || result.IDs[0] != "email") {
			t.Fatalf("shared family not deduplicated: %v", result.IDs)
		}
	}
	var output bytes.Buffer
	if serveRoutingSearch(strings.NewReader(`{"query":"email","allowed_manuals":["not_a_manual"]}`), &output, search, catalog) == nil {
		t.Fatal("unknown authorization was accepted")
	}
}
