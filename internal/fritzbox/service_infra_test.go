package fritzbox

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aurago/internal/config"
)

func newInfraTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{}
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = u.Hostname()
	cfg.FritzBox.Port = mustAtoi(t, u.Port())
	cfg.FritzBox.Timeout = 2
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.SubFeatures.Hosts = true
	cfg.FritzBox.Network.SubFeatures.WakeOnLAN = true
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func infraSOAPResponse(w http.ResponseWriter, action string, values map[string]string) {
	var fields strings.Builder
	for key, value := range values {
		fields.WriteString("<" + key + ">" + value + "</" + key + ">")
	}
	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:%sResponse xmlns:u="urn:dslforum-org:service:Hosts:1">%s</u:%sResponse></s:Body></s:Envelope>`, action, fields.String(), action)
}

func TestInfraHostsUsesOpaqueKeysAndFriendlyNameActionReadback(t *testing.T) {
	friendly := "old"
	var setBody string
	client := newInfraTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(strings.TrimSpace(r.Header.Get("SoapAction")), `"`)
		if cut := strings.LastIndex(action, "#"); cut >= 0 {
			action = action[cut+1:]
		}
		body, _ := io.ReadAll(r.Body)
		switch action {
		case "GetHostNumberOfEntries":
			infraSOAPResponse(w, action, map[string]string{"NewHostNumberOfEntries": "1"})
		case "GetGenericHostEntry":
			infraSOAPResponse(w, action, map[string]string{"NewMACAddress": "AA:BB:CC:DD:EE:FF", "NewIPAddress": "192.0.2.10", "NewHostName": "printer", "NewActive": "1", "NewInterfaceType": "Ethernet"})
		case "X_AVM-DE_GetSpecificHostEntryByIP":
			infraSOAPResponse(w, action, map[string]string{"NewX_AVM-DE_FriendlyName": friendly, "NewX_AVM-DE_FriendlyNameIsWriteable": "1"})
		case "X_AVM-DE_SetFriendlyNameByMAC":
			setBody = string(body)
			if !strings.Contains(setBody, "<NewMACAddress>AA:BB:CC:DD:EE:FF</NewMACAddress>") {
				t.Errorf("write did not use the internally resolved MAC: %s", setBody)
			}
			friendly = "Office printer"
			infraSOAPResponse(w, action, nil)
		default:
			t.Errorf("unexpected SOAP action %s", action)
			http.Error(w, "unexpected action", http.StatusBadRequest)
		}
	})

	hosts, err := client.InfraHosts()
	if err != nil || len(hosts) != 1 {
		t.Fatalf("InfraHosts() = %+v, %v", hosts, err)
	}
	encoded, _ := json.Marshal(hosts)
	if strings.Contains(string(encoded), "AA:BB:CC:DD:EE:FF") || !strings.HasPrefix(hosts[0].Key, "fbh1_") {
		t.Fatalf("host projection leaked its MAC or missing opaque key: %s", encoded)
	}
	status, err := client.InfraSetHostFriendlyName(hosts[0].Key, "Office printer")
	if err != nil || status != "completed" || !strings.Contains(setBody, "Office printer") {
		t.Fatalf("friendly name update = %q, %v; body=%s", status, err, setBody)
	}
}

func TestInfraHostsRejectPartialEnumerationAndReadonlyWrites(t *testing.T) {
	var calls int
	client := newInfraTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(strings.TrimSpace(r.Header.Get("SoapAction")), `"`)
		if cut := strings.LastIndex(action, "#"); cut >= 0 {
			action = action[cut+1:]
		}
		calls++
		switch action {
		case "GetHostNumberOfEntries":
			infraSOAPResponse(w, action, map[string]string{"NewHostNumberOfEntries": "2"})
		case "GetGenericHostEntry":
			if calls == 2 {
				infraSOAPResponse(w, action, map[string]string{"NewMACAddress": "AA:BB:CC:DD:EE:FF", "NewIPAddress": "192.0.2.10"})
				return
			}
			http.Error(w, "partial", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected SOAP action %s", action)
		}
	})
	if hosts, err := client.InfraHosts(); err == nil || hosts != nil {
		t.Fatalf("partial enumeration returned data: %+v, %v", hosts, err)
	}
	client.Cfg.FritzBox.Network.ReadOnly = true
	status, err := client.InfraSetHostFriendlyName("fbh1_"+strings.Repeat("0", 64), "name")
	if status != "failed" || err == nil || calls != 3 {
		t.Fatalf("readonly write = %q, %v, provider calls=%d", status, err, calls)
	}
}

func TestInfraWakeResolvesOpaqueKeyBeforeSending(t *testing.T) {
	var wokeMAC string
	client := newInfraTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(strings.TrimSpace(r.Header.Get("SoapAction")), `"`)
		if cut := strings.LastIndex(action, "#"); cut >= 0 {
			action = action[cut+1:]
		}
		body, _ := io.ReadAll(r.Body)
		switch action {
		case "GetHostNumberOfEntries":
			infraSOAPResponse(w, action, map[string]string{"NewHostNumberOfEntries": "1"})
		case "GetGenericHostEntry":
			infraSOAPResponse(w, action, map[string]string{"NewMACAddress": "AA:BB:CC:DD:EE:FF", "NewIPAddress": "192.0.2.10", "NewHostName": "printer"})
		case "X_AVM-DE_WakeOnLANByMACAddress":
			wokeMAC = "AA:BB:CC:DD:EE:FF"
			if !strings.Contains(string(body), "<NewMACAddress>"+wokeMAC+"</NewMACAddress>") {
				t.Errorf("wake did not resolve the host key internally: %s", body)
			}
			infraSOAPResponse(w, action, nil)
		default:
			t.Errorf("unexpected SOAP action %s", action)
		}
	})
	hosts, err := client.InfraHosts()
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.InfraWakeHost(hosts[0].Key)
	if err != nil || status != "completed" || wokeMAC == "" {
		t.Fatalf("InfraWakeHost = %q, %v, MAC=%q", status, err, wokeMAC)
	}
}
