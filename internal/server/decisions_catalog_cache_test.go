package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"aurago/internal/decisions"
	"aurago/internal/flows"
)

func TestDecisionsCatalogTracksRuntimeAvailability(t *testing.T) {
	s, token := newFlowsTestServer(t)
	fake := &fakeDecisions{}
	s.Decisions = fake
	config := s.ConfigSnapshot()
	generation := s.Flows.Registry().Generation()
	tags := map[string]string{}
	for _, tc := range []struct {
		name   string
		status decisions.Status
		state  string
		code   string
	}{
		{"not installed", decisions.Status{Enabled: true}, flows.NeedsSetupState, "DECISIONS_NOT_INSTALLED"},
		{"installed", decisions.Status{Enabled: true, Installed: true}, flows.NeedsSetupState, "DECISIONS_RUNTIME_UNAVAILABLE"},
		{"started", decisions.Status{Enabled: true, Installed: true, Ready: true}, flows.AvailableState, ""},
		{"stopped", decisions.Status{Enabled: true, Installed: true}, flows.NeedsSetupState, "DECISIONS_RUNTIME_UNAVAILABLE"},
		{"CPU permission required", decisions.Status{Enabled: true, Installed: true, Code: decisions.CodeCPUConsentRequired}, flows.NeedsSetupState, "DECISIONS_CPU_FALLBACK_DENIED"},
		{"attestation failed", decisions.Status{Enabled: true, Installed: true, Code: decisions.CodeRuntimeAttestationFailed}, flows.NeedsSetupState, "DECISIONS_DEVICE_UNVERIFIED"},
		{"disabled", decisions.Status{Installed: true}, flows.NeedsSetupState, "DECISIONS_DISABLED"},
		{"provider missing key", decisions.Status{Enabled: true, Backend: "provider"}, flows.NeedsSetupState, "DECISIONS_NOT_CONFIGURED"},
		{"provider configured", decisions.Status{Enabled: true, Backend: "provider", Configured: true}, flows.AvailableState, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake.status = tc.status
			for _, lang := range []string{"en", "de"} {
				before := c19Builds(s)
				w := c19NodeTypes(t, s, token, lang, tags[lang])
				if w.Code != http.StatusOK {
					t.Fatalf("%s retained stale availability: HTTP %d", lang, w.Code)
				}
				var body struct {
					NodeTypes []flows.NodeTypeInfo `json:"node_types"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, node := range body.NodeTypes {
					if node.Type == flows.TypeAIDecision {
						found = true
						if node.Availability.State != tc.state || node.Availability.Code != tc.code {
							t.Fatalf("%s availability = %+v", lang, node.Availability)
						}
					}
				}
				if !found || c19Builds(s) != before+1 {
					t.Fatal("decision availability was not rebuilt exactly once")
				}
				tags[lang] = w.Header().Get("ETag")
				// Progress/device diagnostics do not change the node's availability.
				fake.status.Progress.Percent++
				if cached := c19NodeTypes(t, s, token, lang, tags[lang]); cached.Code != http.StatusNotModified || c19Builds(s) != before+1 {
					t.Fatalf("%s unchanged availability did not retain its cache", lang)
				}
			}
		})
	}
	if s.ConfigSnapshot() != config || s.Flows.Registry().Generation() != generation {
		t.Fatal("runtime transitions must not require config or registry replacement")
	}
}

func TestDecisionsCatalogRejectsLateStaleCacheEntry(t *testing.T) {
	s, token := newFlowsTestServer(t)
	fake := &fakeDecisions{status: decisions.Status{Enabled: true, Installed: true}}
	s.Decisions = fake
	reg, cfg := s.Flows.Registry(), s.ConfigSnapshot()
	stopped, err := flowNodeTypesBody(reg, "en")
	if err != nil {
		t.Fatal(err)
	}
	fake.status.Ready = true
	ready := c19NodeTypes(t, s, token, "en", "")
	// A slow request can finish encoding the old state after a newer cache put.
	s.flowNodeTypesCache.put(reg.Generation(), cfg, cfg, "en", stopped)
	w := c19NodeTypes(t, s, token, "en", stopped.etag)
	if w.Code != http.StatusOK || w.Header().Get("ETag") != ready.Header().Get("ETag") {
		t.Fatalf("late stale entry survived revalidation: HTTP %d", w.Code)
	}
}
