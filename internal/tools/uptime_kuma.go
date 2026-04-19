package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

// UptimeKumaConfig contains the connection settings for the /metrics endpoint.
type UptimeKumaConfig struct {
	BaseURL        string
	APIKey         string
	InsecureSSL    bool
	RequestTimeout int
}

// UptimeKumaMonitorSnapshot represents the current known state of a single monitor.
type UptimeKumaMonitorSnapshot struct {
	MonitorName     string            `json:"monitor_name"`
	MonitorType     string            `json:"monitor_type,omitempty"`
	MonitorURL      string            `json:"monitor_url,omitempty"`
	MonitorHostname string            `json:"monitor_hostname,omitempty"`
	MonitorPort     string            `json:"monitor_port,omitempty"`
	Status          string            `json:"status"`
	ResponseTimeMS  int64             `json:"response_time_ms,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
}

// Target returns the most useful monitor target string for UI/prompt usage.
func (m UptimeKumaMonitorSnapshot) Target() string {
	switch {
	case strings.TrimSpace(m.MonitorURL) != "":
		return m.MonitorURL
	case strings.TrimSpace(m.MonitorHostname) != "" && strings.TrimSpace(m.MonitorPort) != "":
		return m.MonitorHostname + ":" + m.MonitorPort
	case strings.TrimSpace(m.MonitorHostname) != "":
		return m.MonitorHostname
	default:
		return ""
	}
}

// UptimeKumaSummary aggregates the current states across all monitors.
type UptimeKumaSummary struct {
	Total            int      `json:"total"`
	Up               int      `json:"up"`
	Down             int      `json:"down"`
	Unknown          int      `json:"unknown"`
	DownMonitorNames []string `json:"down_monitor_names,omitempty"`
}

// UptimeKumaSnapshot is a full scrape result from the metrics endpoint.
type UptimeKumaSnapshot struct {
	ScrapedAt time.Time                   `json:"scraped_at"`
	Summary   UptimeKumaSummary           `json:"summary"`
	Monitors  []UptimeKumaMonitorSnapshot `json:"monitors"`
}

func normalizeUptimeKumaBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("uptime kuma base_url is not configured")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid uptime kuma base_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("uptime kuma base_url must include scheme and host")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/metrics")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func uptimeKumaHTTPClient(cfg UptimeKumaConfig) *http.Client {
	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: cfg.InsecureSSL, //nolint:gosec // user-controlled for self-signed homelab instances
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// FetchUptimeKumaSnapshot scrapes the metrics endpoint and maps it into a typed snapshot.
func FetchUptimeKumaSnapshot(ctx context.Context, cfg UptimeKumaConfig, logger *slog.Logger) (UptimeKumaSnapshot, error) {
	baseURL, err := normalizeUptimeKumaBaseURL(cfg.BaseURL)
	if err != nil {
		return UptimeKumaSnapshot{}, err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return UptimeKumaSnapshot{}, fmt.Errorf("uptime kuma api key is not configured")
	}
	security.RegisterSensitive(cfg.APIKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/metrics", nil)
	if err != nil {
		return UptimeKumaSnapshot{}, fmt.Errorf("create uptime kuma request: %w", err)
	}
	req.SetBasicAuth("", cfg.APIKey)

	resp, err := uptimeKumaHTTPClient(cfg).Do(req)
	if err != nil {
		return UptimeKumaSnapshot{}, fmt.Errorf("request uptime kuma metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return UptimeKumaSnapshot{}, fmt.Errorf("uptime kuma metrics request failed: %s", msg)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return UptimeKumaSnapshot{}, fmt.Errorf("read uptime kuma metrics response: %w", err)
	}

	snapshot, err := parseUptimeKumaMetrics(raw, logger)
	if err != nil {
		return UptimeKumaSnapshot{}, err
	}
	snapshot.ScrapedAt = time.Now().UTC()
	return snapshot, nil
}

func parseUptimeKumaMetrics(raw []byte, logger *slog.Logger) (UptimeKumaSnapshot, error) {
	monitors := make(map[string]*UptimeKumaMonitorSnapshot)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		metricName, labels, value, ok := parsePrometheusMetricLine(line)
		if !ok {
			if logger != nil {
				logger.Debug("Ignoring malformed Uptime Kuma metrics line", "line", line)
			}
			continue
		}
		if metricName != "monitor_status" && metricName != "monitor_response_time" {
			continue
		}

		key := uptimeKumaMonitorKey(labels)
		monitor := monitors[key]
		if monitor == nil {
			monitor = &UptimeKumaMonitorSnapshot{
				MonitorName:     labels["monitor_name"],
				MonitorType:     labels["monitor_type"],
				MonitorURL:      labels["monitor_url"],
				MonitorHostname: labels["monitor_hostname"],
				MonitorPort:     labels["monitor_port"],
				Status:          "unknown",
				Labels:          cloneStringMap(labels),
			}
			monitors[key] = monitor
		}

		switch metricName {
		case "monitor_status":
			monitor.Status = mapUptimeKumaStatus(value)
		case "monitor_response_time":
			monitor.ResponseTimeMS = int64(math.Round(value))
		}
	}
	if err := scanner.Err(); err != nil {
		return UptimeKumaSnapshot{}, fmt.Errorf("scan uptime kuma metrics: %w", err)
	}

	result := make([]UptimeKumaMonitorSnapshot, 0, len(monitors))
	for _, monitor := range monitors {
		result = append(result, *monitor)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MonitorName != result[j].MonitorName {
			return strings.ToLower(result[i].MonitorName) < strings.ToLower(result[j].MonitorName)
		}
		return strings.ToLower(result[i].Target()) < strings.ToLower(result[j].Target())
	})

	summary := UptimeKumaSummary{Total: len(result)}
	for _, monitor := range result {
		switch monitor.Status {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
			summary.DownMonitorNames = append(summary.DownMonitorNames, monitor.MonitorName)
		default:
			summary.Unknown++
		}
	}
	sort.Strings(summary.DownMonitorNames)

	return UptimeKumaSnapshot{
		Summary:  summary,
		Monitors: result,
	}, nil
}

func parsePrometheusMetricLine(line string) (string, map[string]string, float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil, 0, false
	}

	i := 0
	for i < len(line) && line[i] != '{' && !isPrometheusSpace(line[i]) {
		i++
	}
	if i == 0 {
		return "", nil, 0, false
	}
	name := line[:i]
	labels := map[string]string{}

	if i < len(line) && line[i] == '{' {
		end := strings.IndexByte(line[i:], '}')
		if end < 0 {
			return "", nil, 0, false
		}
		end += i
		var err error
		labels, err = parsePrometheusLabels(line[i+1 : end])
		if err != nil {
			return "", nil, 0, false
		}
		i = end + 1
	}
	for i < len(line) && isPrometheusSpace(line[i]) {
		i++
	}
	if i >= len(line) {
		return "", nil, 0, false
	}
	valueField := line[i:]
	if idx := strings.IndexAny(valueField, " \t"); idx >= 0 {
		valueField = valueField[:idx]
	}
	value, err := strconv.ParseFloat(valueField, 64)
	if err != nil {
		return "", nil, 0, false
	}
	return name, labels, value, true
}

func parsePrometheusLabels(raw string) (map[string]string, error) {
	labels := make(map[string]string)
	i := 0
	for i < len(raw) {
		for i < len(raw) && (raw[i] == ' ' || raw[i] == ',') {
			i++
		}
		if i >= len(raw) {
			break
		}

		start := i
		for i < len(raw) && raw[i] != '=' {
			i++
		}
		if i >= len(raw) {
			return nil, fmt.Errorf("missing label separator")
		}
		key := strings.TrimSpace(raw[start:i])
		i++
		if i >= len(raw) || raw[i] != '"' {
			return nil, fmt.Errorf("missing label quote")
		}
		i++
		var sb strings.Builder
		for i < len(raw) {
			ch := raw[i]
			if ch == '\\' {
				i++
				if i >= len(raw) {
					return nil, fmt.Errorf("incomplete escape sequence")
				}
				switch raw[i] {
				case '\\', '"':
					sb.WriteByte(raw[i])
				case 'n':
					sb.WriteByte('\n')
				case 't':
					sb.WriteByte('\t')
				default:
					sb.WriteByte(raw[i])
				}
				i++
				continue
			}
			if ch == '"' {
				i++
				break
			}
			sb.WriteByte(ch)
			i++
		}
		labels[key] = sb.String()
		for i < len(raw) && raw[i] != ',' {
			if raw[i] != ' ' {
				break
			}
			i++
		}
		if i < len(raw) && raw[i] == ',' {
			i++
		}
	}
	return labels, nil
}

func mapUptimeKumaStatus(value float64) string {
	switch int(math.Round(value)) {
	case 1:
		return "up"
	case 0:
		return "down"
	default:
		return "unknown"
	}
}

func uptimeKumaMonitorKey(labels map[string]string) string {
	return strings.Join([]string{
		labels["monitor_name"],
		labels["monitor_type"],
		labels["monitor_url"],
		labels["monitor_hostname"],
		labels["monitor_port"],
	}, "|")
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func isPrometheusSpace(ch byte) bool {
	return ch == ' ' || ch == '\t'
}

func uptimeKumaJSONResponse(payload map[string]interface{}) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return `{"status":"error","message":"failed to encode uptime kuma response"}`
	}
	return string(data)
}

func uptimeKumaErrorJSON(err error) string {
	return uptimeKumaJSONResponse(map[string]interface{}{
		"status":  "error",
		"message": err.Error(),
	})
}

// UptimeKumaSummaryJSON returns aggregated monitor counts for agent tool usage.
func UptimeKumaSummaryJSON(ctx context.Context, cfg UptimeKumaConfig, logger *slog.Logger) string {
	snapshot, err := FetchUptimeKumaSnapshot(ctx, cfg, logger)
	if err != nil {
		return uptimeKumaErrorJSON(err)
	}
	return uptimeKumaJSONResponse(map[string]interface{}{
		"status": "ok",
		"data":   snapshot.Summary,
	})
}

// UptimeKumaListMonitorsJSON returns the monitor list for agent tool usage.
func UptimeKumaListMonitorsJSON(ctx context.Context, cfg UptimeKumaConfig, logger *slog.Logger) string {
	snapshot, err := FetchUptimeKumaSnapshot(ctx, cfg, logger)
	if err != nil {
		return uptimeKumaErrorJSON(err)
	}
	return uptimeKumaJSONResponse(map[string]interface{}{
		"status":  "ok",
		"count":   len(snapshot.Monitors),
		"summary": snapshot.Summary,
		"data":    snapshot.Monitors,
	})
}

// UptimeKumaGetMonitorJSON returns a single monitor by its friendly name.
func UptimeKumaGetMonitorJSON(ctx context.Context, cfg UptimeKumaConfig, monitorName string, logger *slog.Logger) string {
	name := strings.TrimSpace(monitorName)
	if name == "" {
		return uptimeKumaErrorJSON(fmt.Errorf("monitor_name is required"))
	}
	snapshot, err := FetchUptimeKumaSnapshot(ctx, cfg, logger)
	if err != nil {
		return uptimeKumaErrorJSON(err)
	}
	for _, monitor := range snapshot.Monitors {
		if strings.EqualFold(strings.TrimSpace(monitor.MonitorName), name) {
			return uptimeKumaJSONResponse(map[string]interface{}{
				"status": "ok",
				"data":   monitor,
			})
		}
	}
	return uptimeKumaErrorJSON(fmt.Errorf("monitor %q not found", name))
}
