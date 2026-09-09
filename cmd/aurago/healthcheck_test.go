package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/webassets"
)

func TestRunCLIHealthcheckBindsRunningAssetSet(t *testing.T) {
	previous := webassets.SetID
	webassets.SetID = "expected-set"
	defer func() { webassets.SetID = previous }()
	for _, set := range []string{"", "old-set", "expected-set"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-AuraGo-Asset-Set", set)
			w.Header().Set("X-AuraGo-Assets-Ready", "true")
			w.Write([]byte(`{"status":"ready"}`))
		}))
		endpoint, _ := url.Parse(server.URL)
		port, _ := strconv.Atoi(endpoint.Port())
		cfg := &config.Config{}
		cfg.Server.Host = endpoint.Hostname()
		cfg.Server.Port = port
		err := runCLIHealthcheck(cfg, time.Second, false)
		server.Close()
		if (err == nil) != (set == "expected-set") {
			t.Fatalf("running set %q: %v", set, err)
		}
	}
}

func TestRunCLIHealthcheckRequestsTsNetReadiness(t *testing.T) {
	var gotRequire string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequire = r.URL.Query().Get("require")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}))
	defer server.Close()

	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(endpoint.Port())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Host = endpoint.Hostname()
	cfg.Server.Port = port

	if err := runCLIHealthcheck(cfg, time.Second, true); err != nil {
		t.Fatalf("runCLIHealthcheck() error = %v", err)
	}
	if gotRequire != "tsnet" {
		t.Fatalf("require query = %q, want tsnet", gotRequire)
	}
}

func TestRunCLIHealthcheckReturnsSafeReadinessCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"initializing","error_code":"TSNET_LOGIN_REQUIRED"}`))
	}))
	defer server.Close()

	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(endpoint.Port())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Host = endpoint.Hostname()
	cfg.Server.Port = port

	start := time.Now()
	err = runCLIHealthcheck(cfg, 100*time.Millisecond, true)
	if err == nil {
		t.Fatal("runCLIHealthcheck() succeeded for a non-ready server")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("healthcheck exceeded its deadline: %s", elapsed)
	}
}
