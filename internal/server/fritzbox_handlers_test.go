package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestFritzBoxStatusWithoutCheckDoesNotClaimConnected(t *testing.T) {
	cfg := &config.Config{}
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = "fritz.box"
	cfg.FritzBox.Port = 49000
	s := &Server{Cfg: cfg, Logger: testServerLogger()}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fritzbox/status", nil)
	handleFritzBoxStatus(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["connected"] == true {
		t.Fatalf("status without check must not claim live connectivity: %s", rec.Body.String())
	}
	if body["configured"] != true {
		t.Fatalf("configured = %v, want true", body["configured"])
	}
}

func TestFritzBoxStatusWithCheckPerformsLiveProbe(t *testing.T) {
	srv := newTR064InfoServer(t, false)
	host, port := splitHostPort(t, srv.URL)

	cfg := &config.Config{}
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = host
	cfg.FritzBox.Port = port
	cfg.FritzBox.Timeout = 2
	s := &Server{Cfg: cfg, Logger: testServerLogger()}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fritzbox/status?check=1", nil)
	handleFritzBoxStatus(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["connected"] != true {
		t.Fatalf("connected = %v, want true; body=%s", body["connected"], rec.Body.String())
	}
	if body["model"] != "FRITZ!Box Test" {
		t.Fatalf("model = %v, want FRITZ!Box Test", body["model"])
	}
}

func TestFritzBoxTestAcceptsTLSOverrideWithSelfSignedCertificate(t *testing.T) {
	srv := newTR064InfoServer(t, true)
	host, port := splitHostPort(t, srv.URL)

	cfg := &config.Config{}
	cfg.FritzBox.Enabled = false
	cfg.FritzBox.Timeout = 2
	s := &Server{Cfg: cfg, Logger: testServerLogger()}

	body := fmt.Sprintf(`{"host":%q,"port":%d,"https":true,"insecure_skip_verify":true}`, host, port)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/fritzbox/test", strings.NewReader(body))
	handleFritzBoxTest(s).ServeHTTP(rec, req)

	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("status = %v, want ok; body=%s", got["status"], rec.Body.String())
	}
}

func TestFritzBoxTestOverridesAllowWebPortZero(t *testing.T) {
	cfg := config.Config{}
	cfg.FritzBox.WebPort = 8443

	zero := 0
	applyFritzBoxTestOverrides(&cfg, fritzBoxTestRequest{WebPort: &zero})

	if cfg.FritzBox.WebPort != 0 {
		t.Fatalf("web_port override = %d, want 0", cfg.FritzBox.WebPort)
	}
}

func newTR064InfoServer(t *testing.T, tls bool) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/upnp/control/deviceinfo" {
			t.Fatalf("path = %q, want /upnp/control/deviceinfo", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetInfoResponse xmlns:u="urn:dslforum-org:service:DeviceInfo:1">
      <NewModelName>FRITZ!Box Test</NewModelName>
      <NewSoftwareVersion>8.00</NewSoftwareVersion>
      <NewHardwareVersion>test</NewHardwareVersion>
      <NewUpTime>42</NewUpTime>
      <NewSerialNumber>serial</NewSerialNumber>
      <NewOEM>avm</NewOEM>
    </u:GetInfoResponse>
  </s:Body>
</s:Envelope>`))
	})
	if tls {
		srv := httptest.NewTLSServer(handler)
		t.Cleanup(srv.Close)
		return srv
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func splitHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	hostPort := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	host, portString, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", hostPort, err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", portString, err)
	}
	return host, port
}

func testServerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
