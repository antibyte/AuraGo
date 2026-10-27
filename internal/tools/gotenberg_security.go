package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"aurago/internal/config"
)

const gotenbergDiagnosticLimit = 1 << 20

// Only these public flags are decoded; raw diagnostics can contain credentials.
type gotenbergSecurityDiagnostic struct {
	Version string                     `json:"version"`
	Flags   map[string]json.RawMessage `json:"flags"`
}

func gotenbergSecurityError(setting string) error {
	return fmt.Errorf("Gotenberg security verification failed: missing or unsafe setting %s", setting)
}

func validateGotenbergDiagnostic(data []byte) (string, error) {
	var diagnostic gotenbergSecurityDiagnostic
	if err := json.Unmarshal(data, &diagnostic); err != nil {
		return "", fmt.Errorf("Gotenberg security diagnostics are not valid JSON")
	}
	parts := strings.Split(diagnostic.Version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("Gotenberg security verification requires version 8.38.0 or newer within major version 8")
	}
	version := make([]int, 3)
	for index, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || strconv.Itoa(n) != part {
			return "", fmt.Errorf("Gotenberg security diagnostics contain an unsupported version")
		}
		version[index] = n
	}
	if version[0] != 8 || version[1] < 38 {
		return "", fmt.Errorf("Gotenberg security verification requires version 8.38.0 or newer within major version 8")
	}
	flag := func(name string) (string, bool) {
		var value string
		raw, exists := diagnostic.Flags[name]
		if !exists || json.Unmarshal(raw, &value) != nil {
			return "", false
		}
		return value, true
	}
	for _, name := range []string{
		"chromium-deny-private-ips", "libreoffice-deny-private-ips",
		"webhook-deny-private-ips", "api-download-from-deny-private-ips",
	} {
		if value, ok := flag(name); !ok || value != "true" {
			return diagnostic.Version, gotenbergSecurityError(name)
		}
	}
	for _, name := range []string{
		"chromium-enable-environment-proxy", "libreoffice-enable-environment-proxy",
		"webhook-enable-environment-proxy", "api-download-from-enable-environment-proxy",
		"chromium-disable-web-security", "chromium-allow-file-access-from-files",
		"chromium-shared-browser-context", "chromium-ignore-certificate-errors",
		"chromium-allow-insecure-localhost",
	} {
		if value, ok := flag(name); !ok || value != "false" {
			return diagnostic.Version, gotenbergSecurityError(name)
		}
	}
	for _, name := range []string{
		"chromium-allow-list", "libreoffice-allow-list", "webhook-allow-list", "api-download-from-allow-list",
	} {
		if value, ok := flag(name); !ok || (value != "[]" && value != "") {
			return diagnostic.Version, gotenbergSecurityError(name)
		}
	}
	for _, name := range []string{"chromium-proxy-server", "chromium-host-resolver-rules"} {
		if value, ok := flag(name); !ok || value != "" {
			return diagnostic.Version, gotenbergSecurityError(name)
		}
	}
	return diagnostic.Version, nil
}

func verifyGotenbergSecurity(ctx context.Context, cfg *config.GotenbergConfig) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if cfg == nil {
		return "", fmt.Errorf("Gotenberg URL is not configured")
	}
	endpoint, err := url.Parse(cfg.URL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", fmt.Errorf("Gotenberg URL must be an HTTP(S) URL without credentials, query or fragment")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.URL, "/")+"/debug", nil)
	if err != nil {
		return "", fmt.Errorf("cannot create Gotenberg security diagnostic request")
	}
	resp, err := getGotenbergClient(cfg.Timeout).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("Gotenberg security diagnostics are unavailable; expose /debug privately to AuraGo")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gotenberg security diagnostics returned HTTP %d; expose /debug privately to AuraGo", resp.StatusCode)
	}
	body, err := readHTTPResponseBody(resp.Body, gotenbergDiagnosticLimit)
	if err != nil {
		return "", fmt.Errorf("Gotenberg security diagnostics are unreadable or exceed the 1 MiB limit")
	}
	return validateGotenbergDiagnostic(body)
}
