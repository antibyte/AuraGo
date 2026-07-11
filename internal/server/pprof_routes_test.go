package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterPProfRoutesExposesEndpoints(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	registerPProfRoutes(mux)

	paths := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/symbol",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound {
			t.Fatalf("pprof route %q not registered (status %d)", path, rec.Code)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("pprof route %q returned status %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}
