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

func TestListGrafanaDashboardsSendsDefaultLimitAndPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Fatalf("path = %q, want /api/search", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Fatalf("limit = %q, want 50", got)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("page = %q, want 1", got)
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	if _, err := ListGrafanaDashboards(context.Background(), grafanaTestConfig(srv.URL), ""); err != nil {
		t.Fatalf("ListGrafanaDashboards() error = %v", err)
	}
}

func TestListGrafanaDashboardsCapsCustomLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("limit = %q, want capped 200", got)
		}
		if got := r.URL.Query().Get("page"); got != "3" {
			t.Fatalf("page = %q, want 3", got)
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := ListGrafanaDashboards(context.Background(), grafanaTestConfig(srv.URL), "sys", GrafanaListOptions{Limit: 999, Page: 3})
	if err != nil {
		t.Fatalf("ListGrafanaDashboards() error = %v", err)
	}
}

func TestQueryGrafanaDatasourceSupportsUIDAndDatasourceType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ds/query" {
			t.Fatalf("path = %q, want /api/ds/query", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode query body: %v", err)
		}
		queries := body["queries"].([]interface{})
		query := queries[0].(map[string]interface{})
		datasource := query["datasource"].(map[string]interface{})
		if datasource["uid"] != "loki-main" {
			t.Fatalf("datasource.uid = %#v, want loki-main", datasource["uid"])
		}
		if query["expr"] != `{job="aurago"}` {
			t.Fatalf("expr = %#v, want Loki expression", query["expr"])
		}
		if _, ok := query["datasourceId"]; ok {
			t.Fatalf("datasourceId should be omitted when datasource_uid is used: %#v", query)
		}
		fmt.Fprint(w, `{"results":{"A":{"status":200}}}`)
	}))
	defer srv.Close()

	_, err := QueryGrafanaDatasource(context.Background(), grafanaTestConfig(srv.URL), 0, `{job="aurago"}`, GrafanaQueryOptions{
		DatasourceUID:  "loki-main",
		DatasourceType: "loki",
	})
	if err != nil {
		t.Fatalf("QueryGrafanaDatasource() error = %v", err)
	}
}

func TestQueryGrafanaDatasourceRejectsUnknownDatasourceType(t *testing.T) {
	_, err := QueryGrafanaDatasource(context.Background(), grafanaTestConfig("http://grafana.local"), 0, "select 1", GrafanaQueryOptions{
		DatasourceUID:  "mysql-main",
		DatasourceType: "mysql",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported grafana datasource_type") {
		t.Fatalf("QueryGrafanaDatasource() error = %v, want unsupported datasource type", err)
	}
}

func TestListGrafanaAlertsPrefersUnifiedPrometheusAlerts(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/prometheus/grafana/api/v1/alerts":
			fmt.Fprint(w, `{"status":"success","data":{"alerts":[{"labels":{"alertname":"HighCPU","severity":"critical"},"annotations":{"summary":"CPU high"},"state":"firing","activeAt":"2026-05-02T10:00:00Z"}]}}`)
		default:
			t.Fatalf("unexpected fallback request to %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	alerts, err := ListGrafanaAlerts(context.Background(), grafanaTestConfig(srv.URL))
	if err != nil {
		t.Fatalf("ListGrafanaAlerts() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].Name != "HighCPU" || alerts[0].State != "firing" || alerts[0].Source != "prometheus_alerts" {
		t.Fatalf("alerts = %#v, want mapped unified prometheus alert", alerts)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %#v, want only primary endpoint", paths)
	}
}

func TestListGrafanaAlertsFallsBackToAlertRules(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/prometheus/grafana/api/v1/alerts":
			http.NotFound(w, r)
		case "/api/alert-rules":
			fmt.Fprint(w, `[{"uid":"rule-1","title":"DiskFull","condition":"C","data":[]}]`)
		default:
			t.Fatalf("unexpected request to %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	alerts, err := ListGrafanaAlerts(context.Background(), grafanaTestConfig(srv.URL))
	if err != nil {
		t.Fatalf("ListGrafanaAlerts() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].RuleUID != "rule-1" || alerts[0].Name != "DiskFull" || alerts[0].Source != "alert_rules" {
		t.Fatalf("alerts = %#v, want mapped alert rule", alerts)
	}
}

func TestListGrafanaAlertsFallsBackToLegacyAlerts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/prometheus/grafana/api/v1/alerts", "/api/alert-rules":
			http.NotFound(w, r)
		case "/api/alerts":
			fmt.Fprint(w, `[{"id":3,"name":"CPU","state":"alerting"}]`)
		default:
			t.Fatalf("unexpected request to %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	alerts, err := ListGrafanaAlerts(context.Background(), grafanaTestConfig(srv.URL))
	if err != nil {
		t.Fatalf("ListGrafanaAlerts() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].Name != "CPU" || alerts[0].Source != "legacy_alerts" {
		t.Fatalf("alerts = %#v, want mapped legacy alert", alerts)
	}
}

func TestListGrafanaAlertsReturnsCombinedEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ListGrafanaAlerts(context.Background(), grafanaTestConfig(srv.URL))
	if err == nil {
		t.Fatal("expected combined alert endpoint error")
	}
	for _, endpoint := range []string{"/api/prometheus/grafana/api/v1/alerts", "/api/alert-rules", "/api/alerts"} {
		if !strings.Contains(err.Error(), endpoint) {
			t.Fatalf("error %q missing endpoint %s", err.Error(), endpoint)
		}
	}
}

func TestGrafanaHTTPClientIsCachedByConnectionSettings(t *testing.T) {
	cfg := GrafanaConfig{BaseURL: "https://grafana.local", RequestTimeout: 5}
	first := grafanaHTTPClient(cfg)
	second := grafanaHTTPClient(cfg)
	if first != second {
		t.Fatal("expected identical Grafana config to reuse http client")
	}
	changedTimeout := grafanaHTTPClient(GrafanaConfig{BaseURL: "https://grafana.local", RequestTimeout: 10})
	if changedTimeout == first {
		t.Fatal("expected timeout change to create a different http client")
	}
	changedTLS := grafanaHTTPClient(GrafanaConfig{BaseURL: "https://grafana.local", RequestTimeout: 5, InsecureSSL: true})
	if changedTLS == first {
		t.Fatal("expected TLS setting change to create a different http client")
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
