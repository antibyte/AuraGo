package main

import (
	"maps"
	"reflect"
	"strings"

	"aurago/internal/config"
)

// Old files without HTTPS fields loaded as HTTP, auto certificate selection,
// port 443 and no redirect listener. Template defaults are for new installs.
// Public-domain auto TLS retains its old port-80 challenge listener when omitted.
func preserveLegacyHTTPS(merged, user map[string]interface{}) bool {
	server, ok := asStringMap(merged["server"])
	if !ok {
		server = map[string]interface{}{}
	}
	server = maps.Clone(server)
	merged["server"] = server
	https, ok := asStringMap(server["https"])
	if !ok {
		https = map[string]interface{}{}
	}
	https = maps.Clone(https)
	server["https"] = https
	userServer, _ := asStringMap(user["server"])
	userHTTPS, _ := asStringMap(userServer["https"])
	legacyHTTPPort := 0
	mode, _ := userHTTPS["cert_mode"].(string)
	domain, _ := userHTTPS["domain"].(string)
	if userHTTPS["enabled"] == true && strings.TrimSpace(domain) != "" && config.NormalizeCertMode(mode) != "custom" && config.NormalizeCertMode(mode) != "selfsigned" {
		legacyHTTPPort = 80
	}
	changed := false
	for key, fallback := range map[string]interface{}{"enabled": false, "cert_mode": "auto", "https_port": 443, "http_port": legacyHTTPPort} {
		if value, exists := userHTTPS[key]; exists && value != nil {
			continue
		}
		if !reflect.DeepEqual(https[key], fallback) {
			https[key] = fallback
			changed = true
		}
	}
	return changed
}
