package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/fritzbox"
)

// fakeFritzBackend records every call and answers from static fixtures.
type fakeFritzBackend struct {
	mu       sync.Mutex
	calls    map[string]int
	closed   atomic.Int32
	failWAN  bool
	failHost bool
	delay    time.Duration
}

func newFakeFritzBackend() *fakeFritzBackend {
	return &fakeFritzBackend{calls: map[string]int{}}
}

func (f *fakeFritzBackend) hit(name string) {
	f.mu.Lock()
	f.calls[name]++
	f.mu.Unlock()
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
}

func (f *fakeFritzBackend) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func (f *fakeFritzBackend) GetSystemInfo() (*fritzbox.SystemInfo, error) {
	f.hit("system")
	return &fritzbox.SystemInfo{ModelName: "FRITZ!Box 7590", SoftwareVersion: "154.08.00", Uptime: 3600, Serial: "SECRET-SERIAL"}, nil
}

func (f *fakeFritzBackend) GetWANStatus() (*fritzbox.WANStatus, error) {
	f.hit("wan")
	if f.failWAN {
		return nil, errors.New("tr064: HTTP 401 – auth failed")
	}
	return &fritzbox.WANStatus{Status: "Connected", Connected: true, Uptime: 120, LastError: "ERROR_NONE", ExternalIPv4: "203.0.113.7", ExternalIPv6: "2001:db8::1", Service: "ppp"}, nil
}

func (f *fakeFritzBackend) GetWANLinkInfo() (*fritzbox.WANLink, error) {
	f.hit("link")
	return &fritzbox.WANLink{AccessType: "X_AVM-DE_Cable", LinkStatus: "Up", LinkUp: true, MaxDownstreamBps: 1000000000, MaxUpstreamBps: 50000000, ByteSendRate: 100, ByteReceiveRate: 200, TotalBytesSent: 10, TotalBytesReceived: 20}, nil
}

func (f *fakeFritzBackend) GetOnlineMonitor() (*fritzbox.OnlineMonitor, error) {
	f.hit("monitor")
	return &fritzbox.OnlineMonitor{IntervalSeconds: 5, DownstreamBytesPerS: []int64{1000, 2000, 3000}, UpstreamBytesPerS: []int64{10, 20, 30}}, nil
}

func (f *fakeFritzBackend) GetHostList() ([]fritzbox.HostEntry, error) {
	f.hit("hosts")
	if f.failHost {
		return nil, errors.New("tr064: dial tcp: connection refused")
	}
	hosts := []fritzbox.HostEntry{
		{MACAddress: "AA:BB:CC:DD:EE:01", IPAddress: "192.168.178.20", Name: "zeta-laptop", Active: true, Interface: "802.11"},
		{MACAddress: "AA:BB:CC:DD:EE:02", IPAddress: "192.168.178.10", Name: "alpha-nas", Active: true, Interface: "Ethernet"},
		{MACAddress: "AA:BB:CC:DD:EE:03", IPAddress: "192.168.178.30", Name: "", Active: false, Interface: "802.11"},
	}
	for i := 0; i < 45; i++ {
		hosts = append(hosts, fritzbox.HostEntry{MACAddress: "AA:BB:CC:DD:FF:00", IPAddress: "192.168.178.100", Name: "sleeper", Active: false, Interface: "Ethernet"})
	}
	return hosts, nil
}

func (f *fakeFritzBackend) GetWLANInfo(index int) (*fritzbox.WLANInfo, error) {
	f.hit("wlan")
	switch index {
	case 1:
		return &fritzbox.WLANInfo{Index: 1, SSID: "Home", Channel: "6", Enabled: true}, nil
	case 2:
		return &fritzbox.WLANInfo{Index: 2, SSID: "Home", Channel: "100", Enabled: true}, nil
	case 3:
		return &fritzbox.WLANInfo{Index: 3, SSID: "Guests", Channel: "0", Enabled: false}, nil
	}
	return nil, errors.New("tr064: HTTP 500 – UPnP error 401: Invalid Action")
}

func (f *fakeFritzBackend) GetCallList() ([]fritzbox.CallEntry, error) {
	f.hit("calls")
	today := time.Now().Format("02.01.06")
	calls := []fritzbox.CallEntry{
		{Type: "2", Date: today + " 09:15", Name: "Mum", Number: "+4930123456", Called: "SIP: 123", Duration: "0:00"},
		{Type: "1", Date: "01.02.24 10:00", Name: "", Number: "+4940999999", Called: "123", Duration: "0:05"},
		{Type: "3", Date: "01.02.24 09:00", Name: "Office", Number: "0800123", Called: "0800123", Duration: "0:12"},
	}
	for i := 0; i < 10; i++ {
		calls = append(calls, fritzbox.CallEntry{Type: "2", Date: "31.01.24 08:00", Name: "Old", Number: "1", Duration: "0:00"})
	}
	return calls, nil
}

func (f *fakeFritzBackend) GetTAMList(int) ([]fritzbox.TAMEntry, error) {
	f.hit("tam")
	return []fritzbox.TAMEntry{{Index: 0, Read: false, Path: "/download.lua?path=/data/tam/rec.0.000"}, {Index: 1, Read: true}}, nil
}

func (f *fakeFritzBackend) Close() { f.closed.Add(1) }

func fritzWidgetTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = "fritz.box"
	cfg.FritzBox.Port = 49000
	cfg.FritzBox.System.Enabled = true
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.SubFeatures.Hosts = true
	cfg.FritzBox.Network.SubFeatures.WLAN = true
	cfg.FritzBox.Telephony.Enabled = true
	cfg.FritzBox.Telephony.SubFeatures.CallLists = true
	cfg.FritzBox.Telephony.SubFeatures.TAM = true
	return cfg
}

func fritzWidgetTestHandler(t *testing.T, cfg *config.Config, backend *fakeFritzBackend) (http.HandlerFunc, *fritzWidgetCache) {
	t.Helper()
	s := &Server{Cfg: cfg}
	cache := newFritzWidgetCache(s)
	cache.newBackend = func(*config.Config) (fritzWidgetBackend, error) { return backend, nil }
	return handleDesktopFritzBoxOverviewWithCache(s, cache), cache
}

func fritzWidgetGet(handler http.Handler, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/fritzbox/overview"+query, nil))
	return rec
}

func TestDesktopFritzBoxOverviewPayloadIsSanitized(t *testing.T) {
	backend := newFakeFritzBackend()
	handler, _ := fritzWidgetTestHandler(t, fritzWidgetTestConfig(), backend)
	rec := fritzWidgetGet(handler, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("overview must not be cached by the browser")
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"AA:BB:CC", "SECRET-SERIAL", "download.lua", "mac_address", "password"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("payload leaks %q: %s", forbidden, body)
		}
	}
	var payload struct {
		Ready        bool                    `json:"ready"`
		Capabilities fritzWidgetCapabilities `json:"capabilities"`
		System       fritzWidgetSystem       `json:"system"`
		Connection   fritzWidgetConnection   `json:"connection"`
		Devices      fritzWidgetDevices      `json:"devices"`
		Telephony    fritzWidgetTelephony    `json:"telephony"`
		Errors       map[string]string       `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Ready || !payload.Capabilities.System || !payload.Capabilities.Connection || !payload.Capabilities.Devices || !payload.Capabilities.Telephony {
		t.Fatalf("capabilities: %+v", payload.Capabilities)
	}
	if len(payload.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", payload.Errors)
	}
	if payload.System.Model != "FRITZ!Box 7590" || payload.System.UptimeSeconds != 3600 {
		t.Fatalf("system: %+v", payload.System)
	}
	c := payload.Connection
	if !c.Online || c.AccessType != "cable" || c.MaxDownBps != 1000000000 || c.ExternalIPv4 != "203.0.113.7" || c.LastError != "" {
		t.Fatalf("connection: %+v", c)
	}
	if c.DownBps != 24000 || c.UpBps != 240 || c.Monitor == nil || len(c.Monitor.DownBps) != 3 || c.Monitor.DownBps[0] != 8000 {
		t.Fatalf("monitor-derived throughput must be bit/s, newest last: %+v monitor=%+v", c, c.Monitor)
	}
	d := payload.Devices
	if d.Total != 48 || d.Active != 2 || d.LANActive != 1 || d.WLANActive != 1 || !d.HostsTruncated || len(d.Hosts) != fritzWidgetMaxHosts {
		t.Fatalf("devices summary: total=%d active=%d lan=%d wlan=%d truncated=%v hosts=%d", d.Total, d.Active, d.LANActive, d.WLANActive, d.HostsTruncated, len(d.Hosts))
	}
	if d.Hosts[0].Name != "alpha-nas" || d.Hosts[1].Name != "zeta-laptop" || d.Hosts[0].Interface != "lan" || d.Hosts[1].Interface != "wlan" {
		t.Fatalf("active hosts must come first sorted by name: %+v", d.Hosts[:2])
	}
	if len(d.WLANs) != 3 || d.WLANs[0].Band != "2.4" || d.WLANs[1].Band != "5" || !d.WLANs[2].Guest || d.WLANs[0].Guest {
		t.Fatalf("wlans: %+v", d.WLANs)
	}
	tel := payload.Telephony
	if tel.MissedToday != 1 || !tel.TAMAvailable || tel.TAMNew != 1 || len(tel.Calls) != fritzWidgetMaxCalls {
		t.Fatalf("telephony: %+v", tel)
	}
	if tel.Calls[0].Type != "missed" || tel.Calls[0].Timestamp == "" || tel.Calls[1].Type != "incoming" || tel.Calls[2].Type != "outgoing" {
		t.Fatalf("call mapping: %+v", tel.Calls[:3])
	}
}

func TestDesktopFritzBoxOverviewRespectsSectionsAndGates(t *testing.T) {
	cfg := fritzWidgetTestConfig()
	cfg.FritzBox.Telephony.Enabled = false
	backend := newFakeFritzBackend()
	handler, _ := fritzWidgetTestHandler(t, cfg, backend)

	rec := fritzWidgetGet(handler, "?sections=connection,telephony")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["connection"]; !ok {
		t.Fatal("requested connection section missing")
	}
	for _, absent := range []string{"telephony", "devices", "system"} {
		if _, ok := payload[absent]; ok {
			t.Fatalf("section %q must not be present", absent)
		}
	}
	if backend.count("calls") != 0 || backend.count("hosts") != 0 || backend.count("system") != 0 {
		t.Fatalf("gated or unrequested sections must not contact the box: %v", backend.calls)
	}
	var caps struct {
		Capabilities fritzWidgetCapabilities `json:"capabilities"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &caps)
	if caps.Capabilities.Telephony || !caps.Capabilities.Devices {
		t.Fatalf("capabilities must mirror config gates: %+v", caps.Capabilities)
	}

	cfg.FritzBox.Enabled = false
	if got := fritzWidgetGet(handler, "").Code; got != http.StatusForbidden {
		t.Fatalf("disabled integration: %d", got)
	}
	cfg.FritzBox.Enabled = true
	cfg.VirtualDesktop.Enabled = false
	if got := fritzWidgetGet(handler, "").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("disabled desktop: %d", got)
	}
	cfg.VirtualDesktop.Enabled = true
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/fritzbox/overview", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", rec.Code)
	}
}

func TestDesktopFritzBoxOverviewCachesAndSingleFlights(t *testing.T) {
	backend := newFakeFritzBackend()
	handler, cache := fritzWidgetTestHandler(t, fritzWidgetTestConfig(), backend)
	var clock atomic.Int64
	clock.Store(time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC).UnixNano())
	advance := func(d time.Duration) { clock.Add(int64(d)) }
	cache.now = func() time.Time { return time.Unix(0, clock.Load()) }

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := fritzWidgetGet(handler, "?sections=connection,devices"); rec.Code != http.StatusOK {
				t.Errorf("status = %d", rec.Code)
			}
		}()
	}
	wg.Wait()
	if backend.count("wan") != 1 || backend.count("hosts") != 1 {
		t.Fatalf("concurrent requests must share one fetch: wan=%d hosts=%d", backend.count("wan"), backend.count("hosts"))
	}

	// Within the TTL nothing is refetched.
	advance(3 * time.Second)
	fritzWidgetGet(handler, "?sections=connection,devices")
	if backend.count("wan") != 1 || backend.count("hosts") != 1 {
		t.Fatal("fresh cache entries must be served without contacting the box")
	}

	// Connection is refreshed synchronously after its TTL; devices serve stale data
	// and refresh in the background.
	advance(2 * time.Second)
	rec := fritzWidgetGet(handler, "?sections=connection,devices")
	if backend.count("wan") != 2 {
		t.Fatalf("connection must refresh synchronously after TTL, wan=%d", backend.count("wan"))
	}
	if backend.count("hosts") != 1 || !strings.Contains(rec.Body.String(), `"devices":{`) {
		t.Fatal("devices must still be served from cache within their TTL")
	}
	advance(fritzWidgetTTLDevices)
	rec = fritzWidgetGet(handler, "?sections=devices")
	if !strings.Contains(rec.Body.String(), `"stale":{"devices":true}`) {
		t.Fatalf("stale devices must be flagged while refreshing: %s", rec.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for backend.count("hosts") < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if backend.count("hosts") != 2 {
		t.Fatal("stale devices must trigger one background refresh")
	}
}

func TestDesktopFritzBoxOverviewReportsSectionErrorsWithoutRawText(t *testing.T) {
	backend := newFakeFritzBackend()
	backend.failWAN = true
	backend.failHost = true
	handler, _ := fritzWidgetTestHandler(t, fritzWidgetTestConfig(), backend)
	rec := fritzWidgetGet(handler, "?sections=connection,devices,system")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "connection refused") || strings.Contains(body, "tr064") {
		t.Fatalf("raw transport errors must not reach the browser: %s", body)
	}
	var payload struct {
		Errors  map[string]string   `json:"errors"`
		Devices *fritzWidgetDevices `json:"devices"`
		System  *fritzWidgetSystem  `json:"system"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Errors["connection"] != "auth_failed" {
		t.Fatalf("connection error code: %v", payload.Errors)
	}
	if _, ok := payload.Errors["devices"]; ok {
		t.Fatalf("WLAN data must keep the devices section alive when only hosts fail: %v", payload.Errors)
	}
	if payload.Devices == nil || len(payload.Devices.WLANs) != 3 || payload.Devices.Total != 0 {
		t.Fatalf("devices fallback: %+v", payload.Devices)
	}
	if payload.System == nil || payload.System.Model == "" {
		t.Fatal("healthy sections must still be delivered next to failing ones")
	}
	if backend.closed.Load() == 0 {
		t.Fatal("a failing fetch must drop the backend so the next attempt rebuilds it")
	}
}

func TestDesktopFritzBoxOverviewRequiresAdminDesktopScope(t *testing.T) {
	srv, readToken, writeToken := testDesktopPermissionServer(t)
	srv.Cfg.VirtualDesktop.Enabled = true
	srv.Cfg.FritzBox.Enabled = true
	srv.Cfg.FritzBox.Host = "fritz.box"
	backend := newFakeFritzBackend()
	cache := newFritzWidgetCache(srv)
	cache.newBackend = func(*config.Config) (fritzWidgetBackend, error) { return backend, nil }
	handler := handleDesktopFritzBoxOverviewWithCache(srv, cache)

	rec := fritzWidgetGet(handler, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request must be rejected: %d", rec.Code)
	}
	for _, token := range []string{readToken, writeToken} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/desktop/fritzbox/overview", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("non-admin desktop token must be rejected: %d", rec.Code)
		}
	}
	if len(backend.calls) != 0 {
		t.Fatalf("rejected requests must not contact the box: %v", backend.calls)
	}
}
