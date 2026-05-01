package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

// GrafanaConfig contains connection settings for the Grafana HTTP API.
type GrafanaConfig struct {
	BaseURL        string
	APIKey         string
	InsecureSSL    bool
	RequestTimeout int
}

type GrafanaHealthResponse struct {
	Database string `json:"database,omitempty"`
	Version  string `json:"version,omitempty"`
	Commit   string `json:"commit,omitempty"`
}

type GrafanaDashboard struct {
	ID          int64    `json:"id,omitempty"`
	UID         string   `json:"uid,omitempty"`
	Title       string   `json:"title,omitempty"`
	URI         string   `json:"uri,omitempty"`
	URL         string   `json:"url,omitempty"`
	Type        string   `json:"type,omitempty"`
	FolderID    int64    `json:"folderId,omitempty"`
	FolderUID   string   `json:"folderUid,omitempty"`
	FolderTitle string   `json:"folderTitle,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type GrafanaDatasource struct {
	ID        int64  `json:"id,omitempty"`
	UID       string `json:"uid,omitempty"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
	URL       string `json:"url,omitempty"`
	Access    string `json:"access,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

type GrafanaAlert struct {
	ID             int64  `json:"id,omitempty"`
	DashboardID    int64  `json:"dashboardId,omitempty"`
	DashboardUID   string `json:"dashboardUid,omitempty"`
	DashboardSlug  string `json:"dashboardSlug,omitempty"`
	PanelID        int64  `json:"panelId,omitempty"`
	Name           string `json:"name,omitempty"`
	State          string `json:"state,omitempty"`
	NewStateDate   string `json:"newStateDate,omitempty"`
	ExecutionError string `json:"executionError,omitempty"`
}

type GrafanaOrg struct {
	ID      int64  `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Address struct {
		Address1 string `json:"address1,omitempty"`
		Address2 string `json:"address2,omitempty"`
		City     string `json:"city,omitempty"`
		ZipCode  string `json:"zipCode,omitempty"`
		State    string `json:"state,omitempty"`
		Country  string `json:"country,omitempty"`
	} `json:"address,omitempty"`
}

func normalizeGrafanaBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("grafana base_url is not configured")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid grafana base_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("grafana base_url must include scheme and host")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func grafanaHTTPClient(cfg GrafanaConfig) *http.Client {
	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: cfg.InsecureSSL, //nolint:gosec // user-controlled for self-signed homelab instances
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

func grafanaDoRequest(ctx context.Context, cfg GrafanaConfig, method, path string, body io.Reader, out interface{}) error {
	baseURL, err := normalizeGrafanaBaseURL(cfg.BaseURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("grafana api key is not configured")
	}
	security.RegisterSensitive(cfg.APIKey)

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create grafana request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := grafanaHTTPClient(cfg).Do(req)
	if err != nil {
		return fmt.Errorf("request grafana api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("grafana api request failed: %s", msg)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode grafana response: %w", err)
	}
	return nil
}

func FetchGrafanaHealth(ctx context.Context, cfg GrafanaConfig) (GrafanaHealthResponse, error) {
	var out GrafanaHealthResponse
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/health", nil, &out); err != nil {
		return GrafanaHealthResponse{}, err
	}
	return out, nil
}

func ListGrafanaDashboards(ctx context.Context, cfg GrafanaConfig, query string) ([]GrafanaDashboard, error) {
	path := "/api/search?type=dash-db"
	if strings.TrimSpace(query) != "" {
		path += "&query=" + url.QueryEscape(strings.TrimSpace(query))
	}
	var out []GrafanaDashboard
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func GetGrafanaDashboard(ctx context.Context, cfg GrafanaConfig, uid string) (map[string]interface{}, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, fmt.Errorf("dashboard uid is required")
	}
	var out map[string]interface{}
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/dashboards/uid/"+url.PathEscape(uid), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func ListGrafanaDatasources(ctx context.Context, cfg GrafanaConfig) ([]GrafanaDatasource, error) {
	var out []GrafanaDatasource
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/datasources", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func QueryGrafanaDatasource(ctx context.Context, cfg GrafanaConfig, dsID int64, query string) (map[string]interface{}, error) {
	if dsID <= 0 {
		return nil, fmt.Errorf("datasource id is required")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	body := map[string]interface{}{
		"queries": []map[string]interface{}{{
			"refId":        "A",
			"datasourceId": dsID,
			"expr":         query,
		}},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal grafana query: %w", err)
	}
	var out map[string]interface{}
	if err := grafanaDoRequest(ctx, cfg, http.MethodPost, "/api/ds/query", bytes.NewReader(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func ListGrafanaAlerts(ctx context.Context, cfg GrafanaConfig) ([]GrafanaAlert, error) {
	var out []GrafanaAlert
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/alerts", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func GetGrafanaOrg(ctx context.Context, cfg GrafanaConfig) (GrafanaOrg, error) {
	var out GrafanaOrg
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/org", nil, &out); err != nil {
		return GrafanaOrg{}, err
	}
	return out, nil
}

func GrafanaJSONResponse(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":%q}`, err.Error())
	}
	return string(raw)
}

func grafanaErrorJSON(err error) string {
	return GrafanaJSONResponse(map[string]interface{}{"status": "error", "message": err.Error()})
}

func GrafanaHealthJSON(ctx context.Context, cfg GrafanaConfig) string {
	data, err := FetchGrafanaHealth(ctx, cfg)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "data": data})
}

func GrafanaListDashboardsJSON(ctx context.Context, cfg GrafanaConfig, query string) string {
	data, err := ListGrafanaDashboards(ctx, cfg, query)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "count": len(data), "data": data})
}

func GrafanaGetDashboardJSON(ctx context.Context, cfg GrafanaConfig, uid string) string {
	data, err := GetGrafanaDashboard(ctx, cfg, uid)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "data": data})
}

func GrafanaListDatasourcesJSON(ctx context.Context, cfg GrafanaConfig) string {
	data, err := ListGrafanaDatasources(ctx, cfg)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "count": len(data), "data": data})
}

func GrafanaQueryDatasourceJSON(ctx context.Context, cfg GrafanaConfig, dsID int64, query string) string {
	data, err := QueryGrafanaDatasource(ctx, cfg, dsID, query)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "data": data})
}

func GrafanaListAlertsJSON(ctx context.Context, cfg GrafanaConfig) string {
	data, err := ListGrafanaAlerts(ctx, cfg)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "count": len(data), "data": data})
}

func GrafanaGetOrgJSON(ctx context.Context, cfg GrafanaConfig) string {
	data, err := GetGrafanaOrg(ctx, cfg)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "data": data})
}
