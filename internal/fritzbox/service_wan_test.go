package fritzbox

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aurago/internal/config"
)

// fakeTR064 answers SOAP actions from a static map keyed by "controlPath#Action".
// Unknown actions return a UPnP 401 fault like a real Fritz!Box does for
// services that do not exist on the current uplink type.
type fakeTR064 struct {
	answers map[string]map[string]string
	calls   []string
}

func (f *fakeTR064) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		action := strings.TrimSpace(strings.Trim(r.Header.Get("SoapAction"), `"`))
		if idx := strings.Index(action, "#"); idx >= 0 {
			action = action[idx+1:]
		}
		key := r.URL.Path + "#" + action
		f.calls = append(f.calls, key)
		if r.Method != http.MethodPost || !strings.Contains(string(body), "<u:"+action) {
			t.Errorf("unexpected SOAP request %s: %s", key, body)
		}
		values, ok := f.answers[key]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:dslforum-org:control-1-0"><errorCode>401</errorCode><errorDescription>Invalid Action</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
			return
		}
		var fields strings.Builder
		for k, v := range values {
			fields.WriteString("<" + k + ">" + v + "</" + k + ">")
		}
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:%sResponse xmlns:u="urn:dslforum-org:service:Test:1">%s</u:%sResponse></s:Body></s:Envelope>`, action, fields.String(), action)
	}
}

func (f *fakeTR064) count(key string) int {
	n := 0
	for _, call := range f.calls {
		if call == key {
			n++
		}
	}
	return n
}

func newWANTestClient(t *testing.T, fake *fakeTR064) *Client {
	t.Helper()
	srv := httptest.NewServer(fake.handler(t))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	cfg := config.Config{}
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = u.Hostname()
	cfg.FritzBox.Port = mustAtoi(t, u.Port())
	cfg.FritzBox.Timeout = 2
	cfg.FritzBox.Network.Enabled = true
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestGetWANStatusPrefersPPPAndCollectsAddresses(t *testing.T) {
	fake := &fakeTR064{answers: map[string]map[string]string{
		ctlWANPPPConn + "#GetStatusInfo":                   {"NewConnectionStatus": "Connected", "NewUptime": "86461", "NewLastConnectionError": "ERROR_NONE"},
		ctlWANPPPConn + "#GetExternalIPAddress":            {"NewExternalIPAddress": "203.0.113.7"},
		ctlWANPPPConn + "#X_AVM_DE_GetExternalIPv6Address": {"NewExternalIPv6Address": "2001:db8::1", "NewPrefixLength": "64"},
		ctlWANPPPConn + "#X_AVM_DE_GetIPv6Prefix":          {"NewIPv6Prefix": "2001:db8:1::", "NewPrefixLength": "56"},
	}}
	client := newWANTestClient(t, fake)
	status, err := client.GetWANStatus()
	if err != nil {
		t.Fatalf("GetWANStatus: %v", err)
	}
	if !status.Connected || status.Status != "Connected" || status.Uptime != 86461 || status.Service != "ppp" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.ExternalIPv4 != "203.0.113.7" || status.ExternalIPv6 != "2001:db8::1" || status.IPv6Prefix != "2001:db8:1::/56" {
		t.Fatalf("unexpected addresses: %+v", status)
	}
	if fake.count(ctlWANIPConn+"#GetStatusInfo") != 0 {
		t.Fatal("WANIPConnection must not be queried when WANPPPConnection answers")
	}
}

func TestGetWANStatusFallsBackToWANIPConnection(t *testing.T) {
	fake := &fakeTR064{answers: map[string]map[string]string{
		ctlWANIPConn + "#GetStatusInfo":        {"NewConnectionStatus": "Disconnected", "NewUptime": "0", "NewLastConnectionError": "ERROR_ISP_TIME_OUT"},
		ctlWANIPConn + "#GetExternalIPAddress": {"NewExternalIPAddress": "0.0.0.0"},
	}}
	client := newWANTestClient(t, fake)
	status, err := client.GetWANStatus()
	if err != nil {
		t.Fatalf("GetWANStatus: %v", err)
	}
	if status.Connected || status.Service != "ip" || status.LastError != "ERROR_ISP_TIME_OUT" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.ExternalIPv4 != "" || status.ExternalIPv6 != "" {
		t.Fatalf("placeholder addresses must be normalized away: %+v", status)
	}
	if fake.count(ctlWANPPPConn+"#GetStatusInfo") != 1 {
		t.Fatal("WANPPPConnection must be tried first")
	}
}

func TestGetWANStatusFailsWhenNoServiceAnswers(t *testing.T) {
	client := newWANTestClient(t, &fakeTR064{answers: map[string]map[string]string{}})
	if _, err := client.GetWANStatus(); err == nil {
		t.Fatal("expected error when neither WAN service answers")
	}
}

func TestGetWANLinkInfoParsesRatesAndCounters(t *testing.T) {
	fake := &fakeTR064{answers: map[string]map[string]string{
		ctlWANCommonIfConfig + "#GetCommonLinkProperties": {"NewWANAccessType": "DSL", "NewPhysicalLinkStatus": "Up", "NewLayer1DownstreamMaxBitRate": "112640000", "NewLayer1UpstreamMaxBitRate": "42560000"},
		ctlWANCommonIfConfig + "#GetAddonInfos":           {"NewByteSendRate": "1200", "NewByteReceiveRate": "540000", "NewTotalBytesSent": "123", "NewTotalBytesReceived": "456", "NewX_AVM_DE_TotalBytesSent64": "5000000000", "NewX_AVM_DE_TotalBytesReceived64": "9000000000"},
	}}
	client := newWANTestClient(t, fake)
	link, err := client.GetWANLinkInfo()
	if err != nil {
		t.Fatalf("GetWANLinkInfo: %v", err)
	}
	if link.AccessType != "DSL" || !link.LinkUp || link.MaxDownstreamBps != 112640000 || link.MaxUpstreamBps != 42560000 {
		t.Fatalf("unexpected link: %+v", link)
	}
	if link.ByteSendRate != 1200 || link.ByteReceiveRate != 540000 {
		t.Fatalf("unexpected rates: %+v", link)
	}
	if link.TotalBytesSent != 5000000000 || link.TotalBytesReceived != 9000000000 {
		t.Fatalf("64-bit counters must win over 32-bit counters: %+v", link)
	}
}

func TestGetWANLinkInfoToleratesMissingAddonInfos(t *testing.T) {
	fake := &fakeTR064{answers: map[string]map[string]string{
		ctlWANCommonIfConfig + "#GetCommonLinkProperties": {"NewWANAccessType": "X_AVM-DE_Cable", "NewPhysicalLinkStatus": "Down", "NewLayer1DownstreamMaxBitRate": "1000000000", "NewLayer1UpstreamMaxBitRate": "50000000"},
	}}
	client := newWANTestClient(t, fake)
	link, err := client.GetWANLinkInfo()
	if err != nil {
		t.Fatalf("GetWANLinkInfo: %v", err)
	}
	if link.LinkUp || link.TotalBytesSent != 0 || link.ByteReceiveRate != 0 {
		t.Fatalf("missing addon infos must leave counters at zero: %+v", link)
	}
}

func TestGetOnlineMonitorReordersOldestFirst(t *testing.T) {
	fake := &fakeTR064{answers: map[string]map[string]string{
		ctlWANCommonIfConfig + "#X_AVM-DE_GetOnlineMonitor": {
			"NewSyncGroupName": "sync_dsl", "NewSyncGroupMode": "DSL",
			"Newmax_ds": "14080000", "Newmax_us": "5320000",
			"Newds_current_bps": "300,200,100", "Newus_current_bps": "30,20,10", "Newmc_current_bps": "0,,x",
		},
	}}
	client := newWANTestClient(t, fake)
	monitor, err := client.GetOnlineMonitor()
	if err != nil {
		t.Fatalf("GetOnlineMonitor: %v", err)
	}
	if monitor.IntervalSeconds != 5 || monitor.GroupMode != "DSL" || monitor.MaxDownstreamBytesPerS != 14080000 {
		t.Fatalf("unexpected monitor header: %+v", monitor)
	}
	if got := fmt.Sprint(monitor.DownstreamBytesPerS); got != "[100 200 300]" {
		t.Fatalf("downstream must be oldest first, got %s", got)
	}
	if got := fmt.Sprint(monitor.UpstreamBytesPerS); got != "[10 20 30]" {
		t.Fatalf("upstream must be oldest first, got %s", got)
	}
	if got := fmt.Sprint(monitor.MulticastBytesPerS); got != "[0 0 0]" {
		t.Fatalf("malformed samples must become zero and keep alignment, got %s", got)
	}
}

func TestWANCallsRequireNetworkFeatureGroup(t *testing.T) {
	fake := &fakeTR064{answers: map[string]map[string]string{}}
	client := newWANTestClient(t, fake)
	client.Cfg.FritzBox.Network.Enabled = false
	if _, err := client.GetWANStatus(); err == nil {
		t.Fatal("GetWANStatus must fail when the network group is disabled")
	}
	if _, err := client.GetWANLinkInfo(); err == nil {
		t.Fatal("GetWANLinkInfo must fail when the network group is disabled")
	}
	if _, err := client.GetOnlineMonitor(); err == nil {
		t.Fatal("GetOnlineMonitor must fail when the network group is disabled")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("disabled network group must not contact the box, got %v", fake.calls)
	}
}
