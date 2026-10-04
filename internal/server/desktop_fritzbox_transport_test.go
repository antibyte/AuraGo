package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

// Exercise the production client through the Desktop endpoint, including
// Digest authentication, the advertised export URL and the readonly boundary.
func TestDesktopFritzBoxOverviewReadsAdvertisedListThroughConfiguredClient(t *testing.T) {
	var exports atomic.Int32
	router := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upnp/control/x_contact":
			if r.Header.Get("Authorization") == "" {
				w.Header().Set("WWW-Authenticate", `Digest realm="fixture", nonce="fixture-nonce", qop="auth"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetCallListResponse xmlns:u="urn:fixture"><NewCallListURL>http://192.0.2.1:49000/calllist.lua?sid=export-fixture</NewCallListURL></u:GetCallListResponse></s:Body></s:Envelope>`)
		case "/calllist.lua":
			if r.URL.Query().Get("sid") != "export-fixture" {
				t.Error("missing export query")
			}
			exports.Add(1)
			fmt.Fprint(w, `<root><Call><Type>1</Type><Name>Fixture caller</Name><Date>01.01.26 12:00</Date></Call></root>`)
		default:
			t.Errorf("unexpected router path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer router.Close()
	u, _ := url.Parse(router.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.ReadOnly = true
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = u.Hostname()
	cfg.FritzBox.Port = port
	cfg.FritzBox.WebPort = port
	cfg.FritzBox.HTTPS = true
	cfg.FritzBox.InsecureSkipVerify = true
	cfg.FritzBox.Telephony.Enabled = true
	cfg.FritzBox.Telephony.SubFeatures.CallLists = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Server{Cfg: cfg, integrationCtx: ctx}
	cache := newFritzWidgetCache(s)
	defer cache.close()
	handler := handleDesktopFritzBoxOverviewWithCache(s, cache)
	for attempt := 0; attempt < 2; attempt++ {
		requestCtx, cancelRequest := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/desktop/fritzbox/overview?sections=telephony", nil).WithContext(requestCtx)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		cancelRequest()
		var payload struct {
			Telephony fritzWidgetTelephony `json:"telephony"`
			Errors    map[string]string    `json:"errors"`
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("overview returned %d", rec.Code)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Errors) != 0 || len(payload.Telephony.Calls) != 1 || payload.Telephony.Calls[0].Name != "Fixture caller" {
			t.Fatalf("overview lost available router data: %+v", payload)
		}
	}
	if exports.Load() != 1 {
		t.Fatalf("expected a single cached export read, got %d", exports.Load())
	}
}
