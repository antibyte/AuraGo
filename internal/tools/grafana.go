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
	"sync"
	"time"

	"aurago/internal/security"
)

const (
	defaultGrafanaListLimit = 50
	maxGrafanaListLimit     = 200
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

type GrafanaListOptions struct {
	Limit int
	Page  int
}

type GrafanaQueryOptions struct {
	DatasourceUID  string
	DatasourceType string
}

type GrafanaAlert struct {
	ID             int64             `json:"id,omitempty"`
	UID            string            `json:"uid,omitempty"`
	RuleUID        string            `json:"ruleUid,omitempty"`
	DashboardID    int64             `json:"dashboardId,omitempty"`
	DashboardUID   string            `json:"dashboardUid,omitempty"`
	DashboardSlug  string            `json:"dashboardSlug,omitempty"`
	PanelID        int64             `json:"panelId,omitempty"`
	Name           string            `json:"name,omitempty"`
	State          string            `json:"state,omitempty"`
	NewStateDate   string            `json:"newStateDate,omitempty"`
	ExecutionError string            `json:"executionError,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Source         string            `json:"source,omitempty"`
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

var grafanaHTTPClientCache sync.Map

func grafanaHTTPClient(cfg GrafanaConfig) *http.Client {
	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	baseURL, err := normalizeGrafanaBaseURL(cfg.BaseURL)
	if err != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	}
	cacheKey := fmt.Sprintf("%s|%t|%s", baseURL, cfg.InsecureSSL, timeout)
	if cached, ok := grafanaHTTPClientCache.Load(cacheKey); ok {
		return cached.(*http.Client)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: cfg.InsecureSSL, //nolint:gosec // user-controlled for self-signed homelab instances
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	actual, _ := grafanaHTTPClientCache.LoadOrStore(cacheKey, client)
	return actual.(*http.Client)
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

func normalizeGrafanaListOptions(opts []GrafanaListOptions) GrafanaListOptions {
	out := GrafanaListOptions{Limit: defaultGrafanaListLimit, Page: 1}
	if len(opts) > 0 {
		out = opts[0]
	}
	if out.Limit <= 0 {
		out.Limit = defaultGrafanaListLimit
	}
	if out.Limit > maxGrafanaListLimit {
		out.Limit = maxGrafanaListLimit
	}
	if out.Page <= 0 {
		out.Page = 1
	}
	return out
}

func ListGrafanaDashboards(ctx context.Context, cfg GrafanaConfig, query string, opts ...GrafanaListOptions) ([]GrafanaDashboard, error) {
	listOpts := normalizeGrafanaListOptions(opts)
	values := url.Values{}
	values.Set("type", "dash-db")
	values.Set("limit", fmt.Sprintf("%d", listOpts.Limit))
	values.Set("page", fmt.Sprintf("%d", listOpts.Page))
	if strings.TrimSpace(query) != "" {
		values.Set("query", strings.TrimSpace(query))
	}
	path := "/api/search?" + values.Encode()
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

func grafanaQueryField(datasourceType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(datasourceType)) {
	case "", "prometheus", "mimir", "cortex", "loki":
		return "expr", nil
	case "elasticsearch":
		return "query", nil
	default:
		return "", fmt.Errorf("unsupported grafana datasource_type %q for simple text query; use prometheus, mimir, cortex, loki, or elasticsearch", datasourceType)
	}
}

func QueryGrafanaDatasource(ctx context.Context, cfg GrafanaConfig, dsID int64, query string, opts ...GrafanaQueryOptions) (map[string]interface{}, error) {
	queryOpts := GrafanaQueryOptions{}
	if len(opts) > 0 {
		queryOpts = opts[0]
	}
	queryOpts.DatasourceUID = strings.TrimSpace(queryOpts.DatasourceUID)
	queryOpts.DatasourceType = strings.TrimSpace(queryOpts.DatasourceType)
	if dsID <= 0 && queryOpts.DatasourceUID == "" {
		return nil, fmt.Errorf("datasource id or datasource uid is required")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	queryField, err := grafanaQueryField(queryOpts.DatasourceType)
	if err != nil {
		return nil, err
	}
	grafanaQuery := map[string]interface{}{
		"refId":    "A",
		queryField: strings.TrimSpace(query),
	}
	if queryOpts.DatasourceUID != "" {
		ds := map[string]interface{}{"uid": queryOpts.DatasourceUID}
		if queryOpts.DatasourceType != "" {
			ds["type"] = queryOpts.DatasourceType
		}
		grafanaQuery["datasource"] = ds
	} else {
		grafanaQuery["datasourceId"] = dsID
	}
	body := map[string]interface{}{
		"queries": []map[string]interface{}{grafanaQuery},
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
	var endpointErrors []string
	if alerts, err := listGrafanaPrometheusAlerts(ctx, cfg); err == nil {
		return alerts, nil
	} else {
		endpointErrors = append(endpointErrors, fmt.Sprintf("/api/prometheus/grafana/api/v1/alerts: %v", err))
	}
	if alerts, err := listGrafanaAlertRules(ctx, cfg); err == nil {
		return alerts, nil
	} else {
		endpointErrors = append(endpointErrors, fmt.Sprintf("/api/alert-rules: %v", err))
	}
	var out []GrafanaAlert
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/alerts", nil, &out); err != nil {
		endpointErrors = append(endpointErrors, fmt.Sprintf("/api/alerts: %v", err))
		return nil, fmt.Errorf("grafana alert endpoints failed: %s", strings.Join(endpointErrors, "; "))
	}
	for i := range out {
		out[i].Source = "legacy_alerts"
	}
	return out, nil
}

func listGrafanaPrometheusAlerts(ctx context.Context, cfg GrafanaConfig) ([]GrafanaAlert, error) {
	var out struct {
		Status string `json:"status,omitempty"`
		Data   struct {
			Alerts []struct {
				Labels      map[string]string `json:"labels,omitempty"`
				Annotations map[string]string `json:"annotations,omitempty"`
				State       string            `json:"state,omitempty"`
				ActiveAt    string            `json:"activeAt,omitempty"`
			} `json:"alerts,omitempty"`
		} `json:"data,omitempty"`
	}
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/prometheus/grafana/api/v1/alerts", nil, &out); err != nil {
		return nil, err
	}
	alerts := make([]GrafanaAlert, 0, len(out.Data.Alerts))
	for _, alert := range out.Data.Alerts {
		name := firstGrafanaMapValue(alert.Labels, "alertname", "grafana_alertname", "rule_uid")
		if name == "" {
			name = firstGrafanaMapValue(alert.Annotations, "summary", "title")
		}
		alerts = append(alerts, GrafanaAlert{
			Name:         name,
			State:        alert.State,
			NewStateDate: alert.ActiveAt,
			Labels:       alert.Labels,
			Annotations:  alert.Annotations,
			Source:       "prometheus_alerts",
		})
	}
	return alerts, nil
}

func listGrafanaAlertRules(ctx context.Context, cfg GrafanaConfig) ([]GrafanaAlert, error) {
	var rules []struct {
		UID       string            `json:"uid,omitempty"`
		RuleUID   string            `json:"ruleUid,omitempty"`
		Title     string            `json:"title,omitempty"`
		Name      string            `json:"name,omitempty"`
		Labels    map[string]string `json:"labels,omitempty"`
		Condition string            `json:"condition,omitempty"`
		Updated   string            `json:"updated,omitempty"`
	}
	if err := grafanaDoRequest(ctx, cfg, http.MethodGet, "/api/alert-rules", nil, &rules); err != nil {
		return nil, err
	}
	alerts := make([]GrafanaAlert, 0, len(rules))
	for _, rule := range rules {
		name := firstNonEmptyGrafanaString(rule.Title, rule.Name, rule.UID, rule.RuleUID)
		ruleUID := firstNonEmptyGrafanaString(rule.RuleUID, rule.UID)
		alerts = append(alerts, GrafanaAlert{
			UID:          rule.UID,
			RuleUID:      ruleUID,
			Name:         name,
			State:        "rule",
			NewStateDate: rule.Updated,
			Labels:       rule.Labels,
			Source:       "alert_rules",
		})
	}
	return alerts, nil
}

func firstGrafanaMapValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyGrafanaString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

func GrafanaListDashboardsJSON(ctx context.Context, cfg GrafanaConfig, query string, listOpts ...GrafanaListOptions) string {
	opts := normalizeGrafanaListOptions(listOpts)
	data, err := ListGrafanaDashboards(ctx, cfg, query, opts)
	if err != nil {
		return grafanaErrorJSON(err)
	}
	return GrafanaJSONResponse(map[string]interface{}{"status": "ok", "count": len(data), "limit": opts.Limit, "page": opts.Page, "data": data})
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

func GrafanaQueryDatasourceJSON(ctx context.Context, cfg GrafanaConfig, dsID int64, query string, opts ...GrafanaQueryOptions) string {
	data, err := QueryGrafanaDatasource(ctx, cfg, dsID, query, opts...)
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
