package truenas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateDatasetReadonlySendsReadonlyProperty(t *testing.T) {
	var seenMethod, seenPath string
	var seenBody map[string]string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer api.Close()

	client := &Client{baseURL: api.URL + "/api/v2.0", apiKey: "token", httpClient: api.Client()}

	if err := client.UpdateDatasetReadonly(context.Background(), "tank/data", true); err != nil {
		t.Fatalf("UpdateDatasetReadonly: %v", err)
	}
	if seenMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", seenMethod)
	}
	if seenPath != "/api/v2.0/pool/dataset/id/tank%2Fdata" {
		t.Fatalf("path = %q", seenPath)
	}
	if seenBody["readonly"] != "on" {
		t.Fatalf("readonly body = %#v, want on", seenBody)
	}
}

func TestUpdateDatasetReadonlyReturnsUpdateError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "readonly failed", http.StatusInternalServerError)
	}))
	defer api.Close()

	client := &Client{baseURL: api.URL + "/api/v2.0", apiKey: "token", httpClient: api.Client()}

	err := client.UpdateDatasetReadonly(context.Background(), "tank/data", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "update dataset tank/data") {
		t.Fatalf("error = %q", err.Error())
	}
}
