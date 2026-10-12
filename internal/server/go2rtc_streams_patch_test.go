package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

var go2RTCPatchTestSources = map[string]string{
	"front-door": "rtsp://camera.local/front",
	"garage":     "rtsp://camera.local/garage",
	"backyard":   "rtsp://camera.local/backyard",
}

func newGo2RTCStreamsPatchTestServer(t *testing.T, enabled bool) (*Server, string, *security.Vault) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configYAML := fmt.Sprintf(`docker:
  enabled: true
go2rtc:
  enabled: %t
  url: http://127.0.0.1:1984
  api_host_port: 1984
  streams:
    - id: front-door
      name: Front door
      enabled: true
    - id: garage
      name: Garage
      enabled: true
    - id: backyard
      name: Backyard
      enabled: true
`, enabled)
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault(strings.Repeat("66", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	for id, source := range go2RTCPatchTestSources {
		if err := vault.WriteSecret(config.Go2RTCStreamSourceVaultKey(id), source); err != nil {
			t.Fatalf("seed source %s: %v", id, err)
		}
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded.ConfigPath = configPath
	loaded.ApplyVaultSecrets(vault)
	return &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}, configPath, vault
}

func assertGo2RTCPatchTestSourcesUnchanged(t *testing.T, vault *security.Vault) {
	t.Helper()
	for id, want := range go2RTCPatchTestSources {
		got, err := vault.ReadSecret(config.Go2RTCStreamSourceVaultKey(id))
		if err != nil || got != want {
			t.Fatalf("vault source for %s = %q, %v; want %q", id, got, err, want)
		}
	}
}

// The config UI once sent only the edited fields of a stream. The server
// replaces go2rtc.streams wholesale, so such a list deleted every other camera.
func TestHandleUpdateConfigRejectsIncompleteGo2RTCStreamLists(t *testing.T) {
	payloads := []struct {
		name    string
		payload string
	}{
		{"field edit as sparse list", `{"go2rtc":{"streams":[null,{"name":"Garage side"}]}}`},
		{"added stream as sparse list", `{"go2rtc":{"streams":[null,null,null,{"id":"camera-4","name":"Camera 4","enabled":true,"source":"","source_configured":false}]}}`},
		{"sparse list with new source", `{"go2rtc":{"streams":[null,{"id":"garage","source":"rtsp://camera.local/replaced"}]}}`},
		{"entry without id", `{"go2rtc":{"streams":[{"id":"front-door","name":"Front door","enabled":true},{"name":"Garage side","enabled":true}]}}`},
		{"entry with blank id", `{"go2rtc":{"streams":[{"id":"  ","name":"Front door","enabled":true}]}}`},
		{"entry that is not an object", `{"go2rtc":{"streams":["front-door"]}}`},
		{"null list", `{"go2rtc":{"streams":null}}`},
		{"numeric map with a gap", `{"go2rtc":{"streams":{"1":{"name":"Garage side"}}}}`},
	}
	for _, enabled := range []bool{false, true} {
		for _, tc := range payloads {
			t.Run(fmt.Sprintf("enabled=%t/%s", enabled, tc.name), func(t *testing.T) {
				server, configPath, vault := newGo2RTCStreamsPatchTestServer(t, enabled)
				before, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("read config: %v", err)
				}
				recorder := httptest.NewRecorder()
				handleUpdateConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(tc.payload)))

				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
				}
				if !strings.Contains(recorder.Body.String(), "go2rtc.streams") {
					t.Fatalf("rejection does not name go2rtc.streams: %s", recorder.Body.String())
				}
				after, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("read config after save: %v", err)
				}
				if !bytes.Equal(before, after) {
					t.Fatalf("rejected save changed config.yaml:\n%s", after)
				}
				assertGo2RTCPatchTestSourcesUnchanged(t, vault)
			})
		}
	}
}

// A complete list, as the config UI sends it, keeps masked sources in the vault
// and stores edits and additions.
func TestHandleUpdateConfigSavesCompleteGo2RTCStreamList(t *testing.T) {
	payload := `{"go2rtc":{"streams":[` +
		`{"id":"front-door","name":"Front door","enabled":true,"source":"••••••••","source_configured":true},` +
		`{"id":"garage","name":"Garage side","enabled":true,"source":"••••••••","source_configured":true},` +
		`{"id":"backyard","name":"Backyard","enabled":true,"source":"••••••••","source_configured":true},` +
		`{"id":"camera-4","name":"Camera 4","enabled":false,"source":"","source_configured":false}]}}`
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			server, configPath, vault := newGo2RTCStreamsPatchTestServer(t, enabled)
			recorder := httptest.NewRecorder()
			handleUpdateConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			saved, err := config.Load(configPath)
			if err != nil {
				t.Fatalf("Load after save: %v", err)
			}
			var got []string
			for _, stream := range saved.Go2RTC.Streams {
				got = append(got, stream.ID+"="+stream.Name)
			}
			want := []string{"front-door=Front door", "garage=Garage side", "backyard=Backyard", "camera-4=Camera 4"}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("saved streams = %v, want %v", got, want)
			}
			assertGo2RTCPatchTestSourcesUnchanged(t, vault)
		})
	}
}
