package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func newMQTTConfigTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("mqtt:\n  enabled: false\n  broker: tcp://127.0.0.1:1883\n  topics: [home/old]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("31", 32), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	s.initConfigSnapshot()
	return s
}

func TestMQTTConfigTopicSaveReloadPreservesSnapshots(t *testing.T) {
	s := newMQTTConfigTestServer(t)
	for _, topics := range [][]string{{"home/#", "sensors/+"}, {"sensors/+"}, {}} {
		before := s.ConfigSnapshot()
		oldTopics := append([]string(nil), before.MQTT.Topics...)
		body, _ := json.Marshal(map[string]interface{}{"mqtt": map[string]interface{}{
			"topics": topics, "qos": 0, "availability": map[string]interface{}{"qos": 0},
		}})
		rec := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
		if rec.Code != http.StatusOK {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
		loaded, err := config.Load(before.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(loaded.MQTT.Topics, topics) || loaded.MQTT.QoS != 0 || loaded.MQTT.Availability.QoS != 0 {
			t.Fatalf("round trip: topics=%v qos=%d availability=%d", loaded.MQTT.Topics, loaded.MQTT.QoS, loaded.MQTT.Availability.QoS)
		}
		if !reflect.DeepEqual(before.MQTT.Topics, oldTopics) && len(oldTopics) != 0 {
			t.Fatal("save changed the previously published topic list")
		}
	}
}

func TestMQTTConfigRejectsBeforeWritingCredentialsOrFile(t *testing.T) {
	for _, patch := range []string{
		`{"mqtt":{"topics":"home/#"}}`,
		`{"mqtt":{"qos":"1"}}`,
		`{"mqtt":{"enabled":true,"tls":{"enabled":true},"password":"new-fixture-credential"}}`,
		`{"mqtt":{"enabled":true,"broker":"mqtts://localhost:8883","tls":{"ca_file":"nonexistent-audit-ca.pem"},"password":"new-fixture-credential"}}`,
	} {
		t.Run(patch, func(t *testing.T) {
			s := newMQTTConfigTestServer(t)
			before, _ := os.ReadFile(s.Cfg.ConfigPath)
			if err := s.Vault.WriteSecret("mqtt_password", "original-fixture-credential"); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
			after, _ := os.ReadFile(s.Cfg.ConfigPath)
			password, _ := s.Vault.ReadSecret("mqtt_password")
			if rec.Code != http.StatusBadRequest || string(before) != string(after) || password != "original-fixture-credential" {
				t.Fatalf("rejected update changed persistent state: status=%d response=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(strings.ToLower(rec.Body.String()), "mqtt") {
				t.Fatalf("missing field context: %s", rec.Body.String())
			}
		})
	}
}

func TestMQTTVaultChangesPublishImmutableCredentialSnapshots(t *testing.T) {
	t.Setenv("MQTT_PASSWORD", " env-fixture-password ")
	s := newMQTTConfigTestServer(t)
	initial := s.ConfigSnapshot()
	initialPassword := initial.MQTT.Password
	body, _ := json.Marshal(map[string]string{"key": "mqtt.password", "value": " vault-fixture-password "})
	rec := httptest.NewRecorder()
	handleSetVaultSecret(s, rec, httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	written := s.ConfigSnapshot()
	if written == initial || written.MQTT.Password != " vault-fixture-password " || initial.MQTT.Password != initialPassword {
		t.Fatal("Vault set lost whitespace or changed the published snapshot")
	}
	source, err := config.ResolveMQTTPasswordSource(s.Vault)
	if err != nil || source != config.MQTTPasswordSourceVault {
		t.Fatalf("source = %s, err = %v", source, err)
	}
	rec = httptest.NewRecorder()
	handleDeleteVaultSecret(s, rec, httptest.NewRequest(http.MethodDelete, "/api/vault/secrets?key=mqtt.password", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	deleted := s.ConfigSnapshot()
	if deleted == written || deleted.MQTT.Password != " env-fixture-password " || written.MQTT.Password != " vault-fixture-password " {
		t.Fatal("Vault delete retained old credentials or changed an old snapshot")
	}
	t.Setenv("MQTT_PASSWORD", "")
	if err := s.refreshMQTTPassword(); err != nil {
		t.Fatal(err)
	}
	if s.ConfigSnapshot().MQTT.Password != "" {
		t.Fatal("missing Vault and environment credentials retained the previous password")
	}
}
