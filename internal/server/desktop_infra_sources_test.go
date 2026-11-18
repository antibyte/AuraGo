package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/infra"
	"aurago/internal/tools"
)

func TestDesktopInfraProviderNullListKeepsPreviousSources(t *testing.T) {
	for _, tc := range []struct {
		provider string
		kind     infra.NodeKind
		nullList string
		empty    string
	}{
		{"docker", infra.NodeContainer, "null", "[]"},
		{"proxmox", infra.NodeVM, `{"data":null}`, `{"data":[]}`},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			_, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
			ctx := context.Background()
			var response atomic.Value
			response.Store(tc.nullList)
			var instance string
			var refresh func()
			switch tc.provider {
			case "docker":
				runtime.dockerRequest = func(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error) {
					return []byte(response.Load().(string)), http.StatusOK, nil
				}
				instance = desktopInfraDockerInstance(cfg)
				refresh = func() { desktopInfraRefreshDocker(ctx, cfg, store, runtime, true) }
			case "proxmox":
				srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, response.Load().(string))
				}))
				defer srv.Close()
				cfg.Proxmox.Enabled, cfg.Proxmox.Insecure, cfg.Proxmox.URL = true, true, srv.URL
				instance = desktopInfraProxmoxInstance(cfg)
				refresh = func() { desktopInfraRefreshProxmox(ctx, cfg, store, runtime, true) }
			}
			id := infra.SourceIdentity{Kind: tc.provider, Instance: instance, Key: "worker"}
			if err := store.UpsertObservation(ctx, infra.SourceObservation{
				Identity: id, Name: "worker", Observed: infra.SourceObserved{Kind: tc.kind, Name: "worker", Icon: "server"},
			}); err != nil {
				t.Fatal(err)
			}
			refresh()
			got, err := store.GetSource(ctx, id)
			if err != nil || got.State != infra.SourceStale || got.RemovedAt != nil || runtime.sourceErrors()[tc.provider+"|"+instance] != "provider_unreachable" {
				t.Fatalf("null enumeration: source=%+v, errors=%v, err=%v", got, runtime.sourceErrors(), err)
			}
			response.Store(tc.empty)
			refresh()
			got, err = store.GetSource(ctx, id)
			if err != nil || got.State != infra.SourceRemoved || got.RemovedAt == nil || len(runtime.sourceErrors()) != 0 {
				t.Fatalf("empty array: source=%+v, errors=%v, err=%v", got, runtime.sourceErrors(), err)
			}
		})
	}
}

func TestDesktopInfraFritzHostsAllowMissingInterfaceType(t *testing.T) {
	router := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, action, _ := strings.Cut(strings.Trim(r.Header.Get("SOAPAction"), `"`), "#")
		var fields string
		switch action {
		case "GetHostNumberOfEntries":
			fields = "<NewHostNumberOfEntries>2</NewHostNumberOfEntries>"
		case "GetGenericHostEntry":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if strings.Contains(string(body), "<NewIndex>0</NewIndex>") {
				fields = "<NewHostName>printer</NewHostName><NewIPAddress>192.0.2.10</NewIPAddress><NewMACAddress>02:00:00:00:00:10</NewMACAddress>"
			} else {
				fields = "<NewHostName>nas</NewHostName><NewIPAddress>192.0.2.20</NewIPAddress><NewMACAddress>02:00:00:00:00:20</NewMACAddress><NewInterfaceType>Ethernet</NewInterfaceType>"
			}
		default:
			t.Errorf("unexpected SOAP action %q", action)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprintf(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:%sResponse xmlns:u="urn:dslforum-org:service:Hosts:1">%s</u:%sResponse></s:Body></s:Envelope>`, action, fields, action)
	}))
	defer router.Close()
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	address := router.Listener.Addr().(*net.TCPAddr)
	cfg.FritzBox.Host, cfg.FritzBox.Port = address.IP.String(), address.Port
	cfg.FritzBox.Enabled, cfg.FritzBox.Network.Enabled, cfg.FritzBox.Network.SubFeatures.Hosts = true, true, true
	ctx := context.Background()
	desktopInfraRefreshFritz(ctx, s, cfg, store, runtime, true)
	got, err := store.ListSources(ctx, "fritzbox", desktopInfraFritzInstance(cfg))
	if err != nil || len(got) != 2 || len(runtime.sourceErrors()) != 0 {
		t.Fatalf("FRITZ!Box sources=%+v, errors=%v, err=%v", got, runtime.sourceErrors(), err)
	}
	for _, source := range got {
		o := source.Observed
		switch o.Name {
		case "printer":
			if source.State != infra.SourceCurrent || o.IPAddress != "192.0.2.10" || len(o.Interfaces) != 0 {
				t.Fatalf("host without interface type = %+v", source)
			}
		case "nas":
			if source.State != infra.SourceCurrent || len(o.Interfaces) != 1 || o.Interfaces[0].Name != "Ethernet" || o.Interfaces[0].IPAddress != "192.0.2.20" {
				t.Fatalf("host with interface type = %+v", source)
			}
		default:
			t.Fatalf("unexpected host %q", o.Name)
		}
	}
}
