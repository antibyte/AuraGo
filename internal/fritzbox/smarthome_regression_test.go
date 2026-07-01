package fritzbox

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetSmartHomeTemplatesParsesIdentifierAndName(t *testing.T) {
	client, _ := newTestAHAClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("switchcmd") != "gettemplatelistinfos" {
			t.Fatalf("switchcmd = %q, want gettemplatelistinfos", r.URL.Query().Get("switchcmd"))
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<templatelist version="1">
			<template identifier="tpl-1" id="17" functionbitmask="2048" autocreate="0">
				<name>Evening</name>
			</template>
		</templatelist>`))
	})

	templates, err := client.GetSmartHomeTemplates()
	if err != nil {
		t.Fatalf("GetSmartHomeTemplates: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("len(templates) = %d, want 1", len(templates))
	}
	want := SmartHomeTemplate{ID: "tpl-1", Name: "Evening", FunctionBitmask: "2048", AutoCreate: false}
	if templates[0] != want {
		t.Fatalf("template = %+v, want %+v", templates[0], want)
	}
}

func TestApplySmartHomeTemplateUsesTemplateIdentifierAsAIN(t *testing.T) {
	client, seenAIN := newTestAHAClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("switchcmd") != "applytemplate" {
			t.Fatalf("switchcmd = %q, want applytemplate", r.URL.Query().Get("switchcmd"))
		}
		if r.URL.Query().Get("ain") != "tpl-1" {
			t.Fatalf("ain = %q, want tpl-1", r.URL.Query().Get("ain"))
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := client.ApplySmartHomeTemplate("tpl-1"); err != nil {
		t.Fatalf("ApplySmartHomeTemplate: %v", err)
	}
	if *seenAIN != "tpl-1" {
		t.Fatalf("seen ain = %q, want tpl-1", *seenAIN)
	}
}

func TestSetLampBrightnessUsesLevelParameter(t *testing.T) {
	client, _ := newTestAHAClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("switchcmd") != "setlevelpercentage" {
			t.Fatalf("switchcmd = %q, want setlevelpercentage", r.URL.Query().Get("switchcmd"))
		}
		if got := r.URL.Query().Get("level"); got != "75" {
			t.Fatalf("level = %q, want 75", got)
		}
		if got := r.URL.Query().Get("param"); got != "" {
			t.Fatalf("param = %q, want empty", got)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := client.SetLampBrightness("lamp-1", 75); err != nil {
		t.Fatalf("SetLampBrightness: %v", err)
	}
}

func newTestAHAClient(t *testing.T, handler http.HandlerFunc) (*Client, *string) {
	t.Helper()
	seenAIN := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAIN = r.URL.Query().Get("ain")
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	sid := &SIDAuth{
		baseURL:   srv.URL,
		client:    srv.Client(),
		sid:       "0123456789abcdef",
		expiresAt: time.Now().Add(time.Hour),
	}
	aha := &AHAClient{
		baseURL:    srv.URL,
		sid:        sid,
		httpClient: srv.Client(),
	}
	return &Client{aha: aha}, &seenAIN
}
