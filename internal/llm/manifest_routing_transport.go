package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/config"
)

const manifestSpecificityHeader = "x-manifest-specificity"

var manifestBlockedRoutingHeaders = map[string]struct{}{
	"authorization":           {},
	"cookie":                  {},
	"set-cookie":              {},
	"proxy-authorization":     {},
	"x-api-key":               {},
	"host":                    {},
	"content-length":          {},
	manifestSpecificityHeader: {},
}

var manifestToolSpecificityPrefixes = map[string]string{
	"browser_":    "web_browsing",
	"playwright_": "web_browsing",
	"web_":        "web_browsing",
	"code_":       "coding",
	"editor_":     "coding",
	"image_":      "image_generation",
	"midjourney_": "image_generation",
	"firefly_":    "image_generation",
	"leonardo_":   "image_generation",
	"video_":      "video_generation",
	"runway_":     "video_generation",
	"sora_":       "video_generation",
	"social_":     "social_media",
	"hootsuite_":  "social_media",
	"buffer_":     "social_media",
	"email_":      "email_management",
	"gmail_":      "email_management",
	"outlook_":    "email_management",
	"superhuman_": "email_management",
	"calendar_":   "calendar_management",
	"gcal_":       "calendar_management",
	"calendly_":   "calendar_management",
	"reclaim_":    "calendar_management",
	"trade_":      "trading",
	"exchange_":   "trading",
	"robinhood_":  "trading",
	"kalshi_":     "trading",
	"coinbase_":   "trading",
}

type manifestRoutingTransport struct {
	routing config.ManifestRoutingConfig
	base    http.RoundTripper
}

func (t *manifestRoutingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req != nil && t.routing.Enabled && isManifestRoutingRequest(req) {
		applyManifestRoutingHeaders(req, t.routing)
		if specificity := manifestSpecificityForRequest(req, t.routing); specificity != "" {
			req.Header.Set(manifestSpecificityHeader, specificity)
		}
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func isManifestRoutingRequest(req *http.Request) bool {
	if req.Method != http.MethodPost || req.URL == nil {
		return false
	}
	path := strings.TrimRight(req.URL.EscapedPath(), "/")
	return strings.HasSuffix(path, "/chat/completions") ||
		strings.HasSuffix(path, "/responses") ||
		strings.HasSuffix(path, "/messages")
}

func applyManifestRoutingHeaders(req *http.Request, routing config.ManifestRoutingConfig) {
	for key, value := range routing.Headers {
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !isSafeManifestRoutingHeader(key, value) {
			continue
		}
		req.Header.Set(key, value)
	}
}

func isSafeManifestRoutingHeader(key, value string) bool {
	if key == "" || value == "" {
		return false
	}
	if _, blocked := manifestBlockedRoutingHeaders[key]; blocked {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func manifestSpecificityForRequest(req *http.Request, routing config.ManifestRoutingConfig) string {
	switch strings.ToLower(strings.TrimSpace(routing.SpecificityMode)) {
	case "fixed":
		specificity := strings.ToLower(strings.TrimSpace(routing.Specificity))
		if config.IsValidManifestSpecificityCategory(specificity) {
			return specificity
		}
	case "auto":
		return inferManifestSpecificityFromTools(req)
	}
	return ""
}

func inferManifestSpecificityFromTools(req *http.Request) string {
	if req.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(req.Body)
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(raw))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	if err != nil || len(raw) == 0 {
		return ""
	}

	var payload struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Tools) == 0 {
		return ""
	}

	categories := map[string]struct{}{}
	for _, tool := range payload.Tools {
		name := manifestToolName(tool)
		if name == "" {
			continue
		}
		if category := manifestCategoryForToolName(name); category != "" {
			categories[category] = struct{}{}
		}
	}
	if len(categories) != 1 {
		return ""
	}
	for category := range categories {
		return category
	}
	return ""
}

func manifestToolName(tool map[string]interface{}) string {
	if name, ok := tool["name"].(string); ok {
		return strings.TrimSpace(name)
	}
	function, ok := tool["function"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := function["name"].(string)
	return strings.TrimSpace(name)
}

func manifestCategoryForToolName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for prefix, category := range manifestToolSpecificityPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return category
		}
	}
	return ""
}
