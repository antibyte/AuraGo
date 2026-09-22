package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/mqtt"
)

func TestMQTTPublicationUpdatesDisabledControllerAndStatus(t *testing.T) {
	t.Setenv("MQTT_PASSWORD", "env-runtime-fixture")
	s := newMQTTConfigTestServer(t)
	s.MQTTController = mqtt.NewMQTTController(s.Logger)
	t.Cleanup(func() { _ = s.MQTTController.Stop(context.Background()) })
	first := s.ConfigSnapshot()
	next := first.Clone()
	next.MQTT.Topics = []string{"home/new"}
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	deadline := time.Now().Add(time.Second)
	for {
		status := s.MQTTController.Status()
		if status.DesiredRevision > 0 && status.ActiveRevision == status.DesiredRevision {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("disabled config was not applied: %+v", status)
		}
		time.Sleep(time.Millisecond)
	}
	rec := httptest.NewRecorder()
	handleMQTTStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/mqtt/status", nil))
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"status", "connected", "broker", "client_id", "buffer_len", "tls_enabled", "stats", "connection_state", "config_revision", "applied_config_revision", "credential_source"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing compatible/additive status field %s", field)
		}
	}
	if body["connected"] != false || body["status"] != "disabled" || body["credential_source"] != "env" {
		t.Fatalf("unexpected status: %v", body)
	}
	if strings.Contains(rec.Body.String(), "env-runtime-fixture") || first.MQTT.Topics[0] != "home/old" {
		t.Fatal("status exposed credentials or publication modified the old config")
	}
}

func TestMQTTConnectionTestHonorsCanceledRequest(t *testing.T) {
	s := newMQTTConfigTestServer(t)
	next := s.ConfigSnapshot().Clone()
	next.MQTT.Enabled = true
	s.replaceConfigSnapshot(next)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mqtt/test", nil).WithContext(ctx)
	handleMQTTTest(s).ServeHTTP(rec, req)
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "error" || !strings.Contains(body["message"].(string), "canceled") {
		t.Fatalf("cancelled test was not reported: %v", body)
	}
}

func TestMQTTConfigChangesDoNotRequireServerRestart(t *testing.T) {
	s := newMQTTConfigTestServer(t)
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"mqtt":{"topics":["home/new"]}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// This minimal server has no Speech Lab runtime, which can independently
	// request a restart. The MQTT edit itself must never add a restart reason.
	if strings.Contains(rec.Body.String(), `"MQTT"`) {
		t.Fatalf("MQTT update still requires restart: %s", rec.Body.String())
	}
}
