package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newGrafanaTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer gf_secret" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		switch r.URL.Path {
		case "/api/health":
			fmt.Fprint(w, `{"database":"ok","version":"10.4.0"}`)
		case "/api/search":
			if r.URL.Query().Get("type") != "dash-db" {
				t.Fatalf("type query = %q, want dash-db", r.URL.Query().Get("type"))
			}
			fmt.Fprint(w, `[{"id":1,"uid":"sys","title":"System"}]`)
		case "/api/dashboards/uid/sys":
			fmt.Fprint(w, `{"dashboard":{"uid":"sys","title":"System"}}`)
		case "/api/datasources":
			fmt.Fprint(w, `[{"id":2,"uid":"prom","name":"Prometheus","type":"prometheus"}]`)
		case "/api/ds/query":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode query body: %v", err)
			}
			if queries, ok := body["queries"].([]interface{}); !ok || len(queries) != 1 {
				t.Fatalf("queries = %#v, want one query", body["queries"])
			}
			fmt.Fprint(w, `{"results":{"A":{"status":200}}}`)
		case "/api/alerts":
			fmt.Fprint(w, `[{"id":3,"name":"CPU","state":"alerting"}]`)
		case "/api/org":
			fmt.Fprint(w, `{"id":1,"name":"Main Org."}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func grafanaTestConfig(serverURL string) GrafanaConfig {
	return GrafanaConfig{BaseURL: serverURL, APIKey: "gf_secret", RequestTimeout: 5}
}

func TestGrafanaFetchFunctions(t *testing.T) {
	srv := newGrafanaTestServer(t)
	defer srv.Close()
	cfg := grafanaTestConfig(srv.URL)
	ctx := context.Background()

	health, err := FetchGrafanaHealth(ctx, cfg)
	if err != nil || health.Database != "ok" {
		t.Fatalf("FetchGrafanaHealth() = %#v, %v", health, err)
	}
	dashboards, err := ListGrafanaDashboards(ctx, cfg, "sys")
	if err != nil || len(dashboards) != 1 || dashboards[0].UID != "sys" {
		t.Fatalf("ListGrafanaDashboards() = %#v, %v", dashboards, err)
	}
	dashboard, err := GetGrafanaDashboard(ctx, cfg, "sys")
	if err != nil || dashboard["dashboard"] == nil {
		t.Fatalf("GetGrafanaDashboard() = %#v, %v", dashboard, err)
	}
	datasources, err := ListGrafanaDatasources(ctx, cfg)
	if err != nil || len(datasources) != 1 || datasources[0].Name != "Prometheus" {
		t.Fatalf("ListGrafanaDatasources() = %#v, %v", datasources, err)
	}
	query, err := QueryGrafanaDatasource(ctx, cfg, 2, "up")
	if err != nil || query["results"] == nil {
		t.Fatalf("QueryGrafanaDatasource() = %#v, %v", query, err)
	}
	alerts, err := ListGrafanaAlerts(ctx, cfg)
	if err != nil || len(alerts) != 1 || alerts[0].State != "alerting" {
		t.Fatalf("ListGrafanaAlerts() = %#v, %v", alerts, err)
	}
	org, err := GetGrafanaOrg(ctx, cfg)
	if err != nil || org.Name != "Main Org." {
		t.Fatalf("GetGrafanaOrg() = %#v, %v", org, err)
	}
}

func TestGrafanaErrorHandling(t *testing.T) {
	if _, err := FetchGrafanaHealth(context.Background(), GrafanaConfig{}); err == nil {
		t.Fatal("expected missing URL error")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := FetchGrafanaHealth(context.Background(), grafanaTestConfig(srv.URL))
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("FetchGrafanaHealth() error = %v, want unauthorized", err)
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{bad`)
	}))
	defer badJSON.Close()
	_, err = FetchGrafanaHealth(context.Background(), grafanaTestConfig(badJSON.URL))
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("FetchGrafanaHealth() error = %v, want decode error", err)
	}
}
