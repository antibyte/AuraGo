package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestNewspaperResearchTranslations(t *testing.T) {
	for _, area := range []string{"desktop", "config/newspaper"} {
		prefix := "newspaper."
		if area != "desktop" {
			prefix = "config.newspaper."
		}
		for _, locale := range []string{"en", "de", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
			body, err := os.ReadFile(filepath.Join("lang", area, locale+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var labels map[string]string
			if err := json.Unmarshal(body, &labels); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"tool_brave_search", "tool_ddg_search", "tool_rss", "tool_web_scraper", "reason_ready", "reason_disabled", "reason_key_missing", "reason_network_disabled", "reason_feeds_missing", "reason_scraper_disabled", "reason_read_only", "reason_model_missing", "reason_skill_missing", "reason_rate_limited", "reason_quota_exhausted", "reason_access_denied", "reason_failed", "last_attempt"} {
				if labels[prefix+key] == "" {
					t.Errorf("%s/%s missing %s", area, locale, key)
				}
			}
			if !strings.Contains(labels[prefix+"last_attempt"], "{result}") {
				t.Errorf("%s/%s missing result placeholder", area, locale)
			}
			if area == "desktop" {
				for _, placeholder := range []string{"{candidates}", "{pages}", "{articles}"} {
					if !strings.Contains(labels[prefix+"research_counts"], placeholder) {
						t.Errorf("%s missing %s", locale, placeholder)
					}
				}
				if !strings.Contains(labels[prefix+"research_gaps"], "{topics}") {
					t.Errorf("%s missing topics", locale)
				}
			} else if labels[prefix+"max_searches"] == "" {
				t.Errorf("%s missing search budget label", locale)
			}
		}
	}
}

func TestNewspaperConfigResearchBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/newspaper/capabilities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"enabled":true,"research_ready":false,"research_reason":"network_disabled","research_tools":[{"id":"brave_search","state":"needs_setup","reason":"key_missing","last_error":"access_denied"},{"id":"ddg_search","state":"blocked","reason":"network_disabled"}]}`)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="de"><meta charset="utf-8"><div id="content"></div><script>var configData={newspaper:{enabled:true}},labels={};function t(k){return labels[k]||k}function escapeHtml(s){return String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;')}function attachChangeListeners(){}function toggleBool(){}</script><script src="/cfg/newspaper.js"></script><script>(async()=>{labels=await(await fetch('/lang/config/newspaper/de.json')).json();renderNewspaperSection({label:'Newspaper',desc:'Recherche'});})();</script></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustElement(`[data-path="newspaper.max_searches"]`)
	if !page.MustEval(`()=>{const e=document.querySelector('[data-path="newspaper.max_searches"]');return e.value==='32'&&e.min==='1'&&e.max==='64'}`).Bool() {
		t.Fatal("search budget default or range missing")
	}
	page.MustElement("button.dc-test-btn").MustClick()
	page.MustElement("#newspaper-config-status li")
	text := page.MustElement("#newspaper-config-status").MustText()
	for _, want := range []string{"Netzwerkanfragen", "Brave-API-Schlüssel", "Letzter Versuch", "API-Schlüssel"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
}
