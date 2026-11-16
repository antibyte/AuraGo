package manus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientAPIRejectsCrossOriginRedirect(t *testing.T) {
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client, err := NewClient("fixture-manus-token", ClientConfig{BaseURL: source.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListProjects(context.Background()); err == nil {
		t.Error("redirect accepted")
	}
	if hits != 0 {
		t.Fatalf("foreign requests=%d", hits)
	}
}
