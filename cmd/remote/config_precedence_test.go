package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/remote"
)

func TestPersonalizedClientConfigPrecedence(t *testing.T) {
	const original = "wss://original-supervisor.example/api/remote/ws"
	const migrated = "wss://new-supervisor.example/api/remote/ws"
	const trailerToken = "synthetic-download-token"
	if os.Getenv("AURAGO_CONFIG_PRECEDENCE_CHILD") == "1" {
		previous := configFileOverride
		t.Cleanup(func() { configFileOverride = previous })
		identity := clientConfig{SupervisorURL: migrated, DeviceID: "synthetic-device", SharedKey: strings.Repeat("bc", 32)}
		pending := identity
		pending.EnrollToken, pending.EnrollmentURL = "synthetic-replacement-token", original
		for _, tc := range []struct {
			name       string
			stored     *clientConfig
			url, token string
			want       clientConfig
		}{
			{name: "fresh download", want: clientConfig{SupervisorURL: original, EnrollToken: trailerToken}},
			{name: "migrated identity", stored: &identity, want: identity},
			{name: "pending replacement", stored: &pending, want: clientConfig{SupervisorURL: original, EnrollToken: pending.EnrollToken}},
			{name: "explicit replacement", stored: &identity, url: original, token: "synthetic-explicit-token", want: clientConfig{SupervisorURL: original, EnrollToken: "synthetic-explicit-token"}},
			{name: "installed endpoint", stored: &clientConfig{SupervisorURL: migrated, EnrollToken: "synthetic-installed-token"}, want: clientConfig{SupervisorURL: migrated, EnrollToken: "synthetic-installed-token"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				configFileOverride = filepath.Join(t.TempDir(), "config.json")
				if tc.stored != nil {
					if err := saveConfig(*tc.stored); err != nil {
						t.Fatal(err)
					}
				}
				cfg, err := loadConfig(tc.url, tc.token, "")
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cfg, tc.want) {
					t.Fatalf("unexpected endpoint/identity/token precedence: endpoint=%q device=%q", cfg.SupervisorURL, cfg.DeviceID)
				}
				stored, err := readStoredConfig(configFileOverride)
				if err != nil || !reflect.DeepEqual(stored, tc.stored) {
					t.Fatal("loading configuration changed the persisted identity")
				}
			})
		}
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	personalized, err := remote.BuildPersonalizedBinary(data, remote.BinaryConfig{SupervisorURL: original, EnrollToken: trailerToken})
	if err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(t.TempDir(), "personalized-test")
	if runtime.GOOS == "windows" {
		childPath += ".exe"
	}
	if err := os.WriteFile(childPath, personalized, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, childPath, "-test.run=^TestPersonalizedClientConfigPrecedence$", "-test.v")
	child.Env = append(os.Environ(), "AURAGO_CONFIG_PRECEDENCE_CHILD=1")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("personalized restart failed: %v\n%s", err, output)
	}
}
