package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

func TestMergePreservesLegacyHTTPS(t *testing.T) {
	_, template := repositoryTemplateMap(t)
	for _, source := range []string{
		"server:\n  port: 8088\n", "server: null\n", "server:\n  https: null\n",
		"server:\n  https:\n    enabled: false\n",
		"server:\n  https:\n    enabled: true\n    domain: aurago.example.com\n    http_port: 80\n",
		"server:\n  https:\n    enabled: true\n    domain: aurago.example.com\n",
		"server:\n  https:\n    enabled: true\n    domain: aurago.example.com\n    http_port: 0\n",
		"server:\n  https:\n    enabled: true\n    cert_mode: custom\n    cert_file: data/cert.pem\n    key_file: data/key.pem\n    https_port: 9443\n",
		"server:\n  https:\n    behind_proxy: true\n    trusted_proxy_cidrs: [127.0.0.1/32]\n",
	} {
		t.Run(source, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			user, err := parseYAMLMap(source)
			if err != nil {
				t.Fatal(err)
			}
			merged := mergeUserConfig(template, user)
			data, err := yaml.Marshal(merged.merged)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			after, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			b, a := before.Server.HTTPS, after.Server.HTTPS
			// An empty disabled mode and auto have identical runtime semantics.
			if b.Enabled != a.Enabled || b.HTTPSPort != a.HTTPSPort || b.HTTPPort != a.HTTPPort || b.BehindProxy != a.BehindProxy || b.CertFile != a.CertFile || (b.Enabled && b.CertMode != a.CertMode) {
				t.Fatalf("HTTPS changed: before=%+v after=%+v", b, a)
			}
		})
	}
}
