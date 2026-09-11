package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/security"
)

func decodeArtifacts(t *testing.T, body string) []string {
	t.Helper()
	var payload struct {
		Artifacts []string `json:"artifacts"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("invalid artifacts payload %q: %v", body, err)
	}
	if payload.Artifacts == nil {
		t.Fatal("artifacts must be a list, never null")
	}
	return payload.Artifacts
}

func TestSystemWorldMemoryArtifactsSampling(t *testing.T) {
	s := newSystemWorldVoiceTestServer(t)
	secret := "hologram-test-sensitive-value"
	security.RegisterSensitive(secret)
	long := strings.Repeat("Übergrößenträger ", 40)
	for i := 0; i < 12; i++ {
		if _, err := s.ShortTermMem.AddCoreMemoryFact(fmt.Sprintf("Kernfakt Nummer %d bleibt im Archiv erhalten.", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ShortTermMem.AddCoreMemoryFact("Der Schlüssel lautet " + secret + "."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShortTermMem.AddCoreMemoryFact(long); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShortTermMem.InsertMessage("default", "tool", "PRIVATE INTERNAL TEXT MUST NOT BE SHOWN.", false, false); err != nil {
		t.Fatal(err)
	}
	before, _ := s.ShortTermMem.GetAllMemoryMeta(1, 0)
	for round := 0; round < 6; round++ {
		got := systemWorldSampleExcerpts(context.Background(), s, systemWorldArtifactLimit, systemWorldArtifactRunes)
		if len(got) == 0 || len(got) > systemWorldArtifactLimit {
			t.Fatalf("artifact count out of bounds: %d", len(got))
		}
		seen := map[string]bool{}
		for _, text := range got {
			if seen[text] {
				t.Fatalf("duplicate artifact %q", text)
			}
			seen[text] = true
			if !utf8.ValidString(text) || utf8.RuneCountInString(text) > systemWorldArtifactRunes || utf8.RuneCountInString(text) < 12 {
				t.Fatalf("artifact bounds violated: %q", text)
			}
			if strings.Contains(text, secret) || strings.Contains(text, "PRIVATE INTERNAL") {
				t.Fatalf("protected text leaked: %q", text)
			}
		}
	}
	after, _ := s.ShortTermMem.GetAllMemoryMeta(1, 0)
	if len(before) > 0 && (after[0].AccessCount != before[0].AccessCount || after[0].LastAccessed != before[0].LastAccessed) {
		t.Fatal("Sampling mutated memory access metadata")
	}
	if got := systemWorldSampleExcerpts(context.Background(), s, 1, 180); len(got) != 1 {
		t.Fatalf("voice wrapper limit: %d", len(got))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := systemWorldSampleExcerpts(ctx, s, systemWorldArtifactLimit, systemWorldArtifactRunes); len(got) != 0 {
		t.Fatal("cancelled sampling returned artifacts")
	}
}

func TestSystemWorldMemoryArtifactsHandler(t *testing.T) {
	s := newSystemWorldVoiceTestServer(t)
	h := handleSystemWorldMemoryArtifacts(s)
	for _, tc := range []struct {
		method, token string
		auth          bool
		want          int
	}{
		{"POST", "", false, 405}, {"GET", "bad", false, 403}, {"GET", "", true, 401},
	} {
		s.Cfg.Auth.Enabled = tc.auth
		r := httptest.NewRequest(tc.method, "/api/desktop/system-world/memory-artifacts", nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h(w, r)
		if w.Code != tc.want {
			t.Fatalf("gate %s: %d want %d", tc.method, w.Code, tc.want)
		}
	}
	// Valid read/write tokens still cannot read owner memory through the hologram.
	gated, read, write := testDesktopPermissionServer(t)
	for _, token := range []string{read, write} {
		r := httptest.NewRequest("GET", "/api/desktop/system-world/memory-artifacts", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handleSystemWorldMemoryArtifacts(gated)(w, r)
		if w.Code != 403 {
			t.Fatal("scoped token gained global memory access")
		}
	}
	s.Cfg.Auth.Enabled = false
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/api/desktop/system-world/memory-artifacts", nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("empty sources: %d %q", w.Code, w.Header())
	}
	if got := decodeArtifacts(t, w.Body.String()); len(got) != 0 {
		t.Fatalf("empty sources returned %v", got)
	}
	w = httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/api/desktop/system-world/memory-artifacts", nil))
	if w.Code != 429 {
		t.Fatalf("missing request cooldown: %d", w.Code)
	}
	const phrase = "Die Terrassen des Archivs leuchten im Abendlicht."
	if _, err := s.ShortTermMem.AddCoreMemoryFact(phrase); err != nil {
		t.Fatal(err)
	}
	h = handleSystemWorldMemoryArtifacts(s)
	w = httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/api/desktop/system-world/memory-artifacts", nil))
	if w.Code != 200 {
		t.Fatalf("artifacts: %d", w.Code)
	}
	if got := decodeArtifacts(t, w.Body.String()); len(got) != 1 || got[0] != phrase {
		t.Fatalf("unexpected artifacts %v", got)
	}
	s.Cfg.VirtualDesktop.Enabled = false
	w = httptest.NewRecorder()
	handleSystemWorldMemoryArtifacts(s)(w, httptest.NewRequest("GET", "/api/desktop/system-world/memory-artifacts", nil))
	if w.Code != 503 {
		t.Fatalf("disabled desktop: %d", w.Code)
	}
}
