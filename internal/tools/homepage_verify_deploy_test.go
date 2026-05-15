package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomepageVerifyDeploymentURLRejectsProviderErrorPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><title>404: NOT_FOUND</title><body>DEPLOYMENT_NOT_FOUND</body></html>"))
	}))
	defer server.Close()

	result := homepageVerifyDeploymentURL(server.URL)
	if result.Status != "error" {
		t.Fatalf("expected provider error page to fail verification, got %+v", result)
	}
	if !strings.Contains(result.Message, "provider error") {
		t.Fatalf("expected provider error guidance, got %+v", result)
	}
}

func TestHomepageVerifyDeploymentURLChecksScriptAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><script src="/assets/app.js"></script></head><body>ok</body></html>`))
		case "/assets/app.js":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result := homepageVerifyDeploymentURL(server.URL)
	if result.Status != "error" {
		t.Fatalf("expected missing asset to fail verification, got %+v", result)
	}
	if !strings.Contains(result.Message, "asset") {
		t.Fatalf("expected missing asset guidance, got %+v", result)
	}
}
