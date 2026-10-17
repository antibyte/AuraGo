package decisions

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestModelAvailabilityAndPinnedCatalogIdentity(t *testing.T) {
	now := time.Now()
	models := []DecisionModel{{ID: "typesafe/jev-latest", CanonicalSlug: "typesafe/jev-1.13-20261001", AliasTarget: "typesafe/jev-1.13-20261001", ContextLength: 8192, SupportsDecisions: true, CheckedAt: now, InputModalities: []string{"text"}}}
	for _, id := range []string{models[0].ID, models[0].CanonicalSlug} {
		got := CheckModelCatalog(models, id, now)
		if got.State != "usable" || got.Model.ResolvedID() != models[0].CanonicalSlug {
			t.Fatalf("%s: %+v", id, got)
		}
		got.Model.InputModalities[0] = "changed"
		if models[0].InputModalities[0] != "text" {
			t.Fatal("catalog was mutated")
		}
		if got.Snapshot(now.Add(5*time.Minute)).State != "unknown" {
			t.Fatal("expired catalog remained authoritative")
		}
	}
	for _, context := range []int{0, -1} {
		models[0].ContextLength = context
		if got := CheckModelCatalog(models, models[0].ID, now); got.Code != CodeModelContextUnverified || got.Model.Usable {
			t.Fatalf("bad context: %+v", got)
		}
	}
	for _, id := range []string{"", "missing/model"} {
		if got := CheckModelCatalog(models, id, now); got.Code != CodeModelNotAvailable {
			t.Fatalf("missing model: %+v", got)
		}
	}
}

func TestOpenRouterCatalogPinsAliasAndRechecksExpiredContext(t *testing.T) {
	gets, posts := 0, 0
	catalog := `{"data":[{"id":"~fixture/latest","canonical_slug":"fixture/version-1","alias_target":{"slug":"fixture/version-1"},"expiration_date":"2027-01-01","context_length":8192,"architecture":{"input_modalities":["text"],"output_modalities":["decisions"]}}]}`
	client := openRouterTestClient(t, "fixture-only", func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			gets++
			return openRouterHTTPResponse(200, catalog, nil), nil
		}
		posts++
		var wire openRouterDecisionRequest
		if err := json.NewDecoder(request.Body).Decode(&wire); err != nil {
			t.Fatal(err)
		}
		if wire.Model != "fixture/version-1" {
			t.Fatalf("alias was not pinned: %s", wire.Model)
		}
		return openRouterHTTPResponse(200, `{"model":"fixture/version-1","answers":{"q":{"type":"noul","noul":0.8}},"usage":{"input_tokens":4,"output_tokens":0}}`, nil), nil
	})
	request := Request{State: "test", Questions: []Question{{ID: "q", Type: "noul"}}}
	for _, id := range []string{"~fixture/latest", "fixture/version-1"} {
		if _, err := client.Evaluate(context.Background(), id, request); err != nil {
			t.Fatal(err)
		}
	}
	if gets != 1 || posts != 2 || client.CatalogCheckedAt().IsZero() {
		t.Fatalf("cache/preflight: gets=%d posts=%d", gets, posts)
	}
	client.cacheMu.Lock()
	client.modelsAt = time.Now().Add(-6 * time.Minute)
	client.cacheMu.Unlock()
	catalog = `{"data":[{"id":"~fixture/latest","context_length":0,"architecture":{"output_modalities":["decisions"]}}]}`
	if _, err := client.Evaluate(context.Background(), "~fixture/latest", request); ErrorCode(err) != CodeModelContextUnverified || gets != 2 || posts != 2 {
		t.Fatalf("expired context reached provider: %v", err)
	}
}
