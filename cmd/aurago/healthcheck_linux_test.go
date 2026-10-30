package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/webassets"

	"golang.org/x/sys/unix"
)

func TestCLIHealthcheckSkipsLocalAssetVerification(t *testing.T) {
	const expectedSet = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if os.Getenv("AURAGO_TEST_CLI_HEALTHCHECK") == "1" {
		flag.CommandLine = flag.NewFlagSet("aurago", flag.ExitOnError)
		os.Args = append(os.Args[:1], os.Args[3:]...)
		webassets.SetID = expectedSet
		main()
		os.Exit(0)
	}

	root := t.TempDir()
	assetRoot := filepath.Join(root, "assets")
	setDir := filepath.Join(assetRoot, expectedSet)
	if err := os.MkdirAll(setDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Opening this manifest blocks indefinitely, modeling slow local verification.
	if err := unix.Mkfifo(filepath.Join(setDir, webassets.ManifestName), 0600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		set   string
		ready string
		ok    bool
	}{
		{"ready", expectedSet, "true", true},
		{"different_set", strings.Repeat("b", 64), "true", false},
		{"unverified_set", expectedSet, "false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-AuraGo-Asset-Set", tc.set)
				w.Header().Set("X-AuraGo-Assets-Ready", tc.ready)
				_, _ = w.Write([]byte(`{"status":"ready"}`))
			}))
			defer server.Close()
			endpoint, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			configFile := filepath.Join(root, "config.yaml")
			configYAML := fmt.Sprintf("server:\n  host: 127.0.0.1\n  port: %s\n  https:\n    enabled: false\n", endpoint.Port())
			if err := os.WriteFile(configFile, []byte(configYAML), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCLIHealthcheckSkipsLocalAssetVerification$", "--",
				"--config", configFile, "--assets-dir", assetRoot, "--healthcheck", "--healthcheck-timeout", "500ms")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "AURAGO_TEST_CLI_HEALTHCHECK=1", "AURAGO_MASTER_KEY_FILE=")
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("healthcheck blocked on local assets: %v\n%s", ctx.Err(), output)
			}
			if tc.ok {
				if err != nil || !strings.Contains(string(output), "ready\n") {
					t.Fatalf("ready server rejected: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(string(output), "running server does not have this binary's verified asset set") {
				t.Fatalf("unverified running assets accepted: %v\n%s", err, output)
			}
		})
	}
}
