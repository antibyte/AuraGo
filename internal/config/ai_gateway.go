package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ValidAIGatewaySegment accepts one unambiguous URL path component, never a URL.
func ValidAIGatewaySegment(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if char != '-' && char != '_' && !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

func ValidateGatewayProvider(p ProviderEntry) error {
	if p.Type != "cloudflare-rest" {
		return nil
	}
	if !ValidAIGatewaySegment(p.AccountID) || !ValidAIGatewaySegment(p.GatewayID) {
		return fmt.Errorf("Cloudflare REST provider %q requires valid account_id and gateway_id", p.ID)
	}
	if p.AuthType != "" && p.AuthType != "api_key" {
		return fmt.Errorf("Cloudflare REST requires a Cloudflare API token")
	}
	if p.BaseURL != "" {
		u, err := url.Parse(p.BaseURL)
		if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "api.cloudflare.com") || (u.Port() != "" && u.Port() != "443") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.TrimRight(u.Path, "/") != "/client/v4/accounts/"+p.AccountID+"/ai/v1" {
			return fmt.Errorf("Cloudflare REST requires its account-specific HTTPS endpoint")
		}
	}
	author, model, ok := strings.Cut(p.Model, "/")
	if !ok || model == "" || (!ValidAIGatewaySegment(author) && author != "@cf") || strings.ContainsAny(p.Model, " \t\r\n?#\\") || strings.Contains(p.Model, "..") || (author == "@cf" && !strings.Contains(model, "/")) {
		return fmt.Errorf("Cloudflare REST requires a canonical author/model or @cf/author/model ID")
	}
	return nil
}

func ValidateAIGatewayConfig(c *Config) error {
	if c == nil {
		return nil
	}
	g := c.AIGateway
	if g.Enabled && (!ValidAIGatewaySegment(strings.TrimSpace(g.AccountID)) || !ValidAIGatewaySegment(strings.TrimSpace(g.GatewayID))) {
		return fmt.Errorf("AI Gateway requires valid account_id and gateway_id")
	}
	if g.HuggingFaceProviderSlug != "" && !ValidAIGatewaySegment(g.HuggingFaceProviderSlug) {
		return fmt.Errorf("AI Gateway Hugging Face custom provider slug is invalid")
	}
	if len(g.Metadata) > 5 {
		return fmt.Errorf("AI Gateway metadata supports at most five entries")
	}
	seen := map[string]bool{}
	for key, value := range g.Metadata {
		key = strings.TrimSpace(key)
		if key == "" || strings.TrimSpace(value) == "" || seen[key] {
			return fmt.Errorf("AI Gateway metadata requires unique nonempty keys and values")
		}
		seen[key] = true
	}
	encoded, err := json.Marshal(g.Metadata)
	if err != nil || len(encoded) > 4096 {
		return fmt.Errorf("AI Gateway metadata must fit in a 4 KiB JSON header")
	}
	if g.RequestTimeoutMS < 0 || g.RequestTimeoutMS > 300000 {
		return fmt.Errorf("AI Gateway request_timeout_ms must be between 0 and 300000")
	}
	return nil
}
