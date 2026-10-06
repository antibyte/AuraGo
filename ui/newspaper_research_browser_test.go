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
			budgetKeys := []string{"budget_mode", "budget_auto", "budget_fixed", "effective_budget_summary", "monetary_budget_enabled", "monetary_budget_disabled", "fixed_budget_note"}
			if area == "config/newspaper" {
				budgetKeys = append(budgetKeys, "overview_sources", "overview_sources_help", "source_google_news", "source_hacker_news", "source_techmeme", "preset_recommended", "preset_none", "budget_preview", "budget_preview_loading", "budget_preview_failed", "budget_mode_help", "fixed_limits_only")
			} else {
				budgetKeys = append(budgetKeys, "budget_title", "budget_intro", "budget_preview_loading", "budget_preview_failed", "research_progress", "research_activity")
			}
			for _, key := range budgetKeys {
				if labels[prefix+key] == "" {
					t.Errorf("%s/%s missing %s", area, locale, key)
				}
			}
			if !strings.Contains(labels[prefix+"effective_budget_summary"], "{stories}") || !strings.Contains(labels[prefix+"effective_budget_summary"], "{topics}") {
				t.Errorf("%s/%s missing effective budget placeholders", area, locale)
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
	mux.HandleFunc("/api/desktop/newspaper/budget-preview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			BudgetMode      string   `json:"budget_mode"`
			OverviewSources []string `json:"overview_sources"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode preview request: %v", err)
		}
		mode := req.BudgetMode
		if mode != "auto" {
			mode = "fixed"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"effective_budget": map[string]any{"mode": mode, "topics": 31, "pages": 396, "searches": 128, "overviews": 64, "minutes": 60, "candidates": 800, "stories": 31, "editor_calls": 124, "shared_pages": mode == "fixed"}, "monetary_budget_enabled": false, "overview_sources": req.OverviewSources})
	})
	mux.HandleFunc("/api/desktop/newspaper/capabilities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"enabled":true,"research_ready":false,"research_reason":"network_disabled","research_tools":[{"id":"brave_search","state":"needs_setup","reason":"key_missing","last_error":"access_denied"},{"id":"ddg_search","state":"blocked","reason":"network_disabled"}]}`)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="de"><meta charset="utf-8"><div id="content"></div><script>var configData={newspaper:{enabled:true,budget_mode:'auto',overview_sources:['google_news'],max_pages:60,max_searches:32,max_minutes:60}},labels={};window.previewRequests=[];const originalFetch=window.fetch.bind(window);window.fetch=(url,opts={})=>{if(String(url).includes('/budget-preview'))window.previewRequests.push(JSON.parse(opts.body||'{}'));return originalFetch(url,opts)};function t(k){return labels[k]||k}function escapeHtml(s){return String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;')}function attachChangeListeners(){}function toggleBool(){}</script><script src="/cfg/newspaper.js"></script><script>(async()=>{labels=await(await fetch('/lang/config/newspaper/de.json')).json();renderNewspaperSection({label:'Newspaper',desc:'Recherche'});})();</script></html>`)
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
	if got := page.MustEval(`()=>document.querySelector('[data-path="newspaper.budget_mode"]').value`).Str(); got != "auto" {
		t.Fatalf("auto budget mode was not retained: %q", got)
	}
	if !strings.Contains(page.MustElement("#content").MustText(), "Diese Limits gelten nur im festen Modus.") {
		t.Fatal("legacy limit fields were not identified as fixed-mode only")
	}
	page.MustWait(`() => document.querySelector('#newspaper-budget-preview')?.innerText.includes('Themen 31') && document.querySelector('#newspaper-budget-preview')?.innerText.includes('Artikel 31')`)
	if !strings.Contains(page.MustElement("#newspaper-budget-preview").MustText(), "Kein Kostenlimit konfiguriert; die Recherche-Limits gelten trotzdem.") {
		t.Fatal("research allowance did not distinguish the monetary budget")
	}
	page.MustEval(`()=>{const mode=document.querySelector('[data-path="newspaper.budget_mode"]');mode.value='fixed';mode.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustWait(`() => document.querySelector('#newspaper-budget-preview')?.innerText.includes('Feste Limits teilen 396 Seiten')`)
	page.MustEval(`()=>{const mode=document.querySelector('[data-path="newspaper.budget_mode"]');mode.value='auto';mode.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustWait(`() => document.querySelector('#newspaper-budget-preview')?.innerText.includes('Themen 31')`)
	page.MustElement(`[data-overview-preset="none"]`).MustClick()
	page.MustWait(`() => window.previewRequests.at(-1)?.overview_sources?.length === 0`)
	page.MustElement(`[data-overview-preset="recommended"]`).MustClick()
	page.MustWait(`() => JSON.stringify(window.previewRequests.at(-1)?.overview_sources) === JSON.stringify(['google_news','hacker_news','techmeme'])`)
	if !page.MustEval(`()=>document.querySelector('[data-path="newspaper.overview_sources"]').value==='google_news, hacker_news, techmeme'`).Bool() {
		t.Fatal("recommended overview preset did not update the saved config value")
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
