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
	"github.com/go-rod/rod/lib/input"
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
				budgetKeys = append(budgetKeys, "source_tagesschau", "source_bbc_business", "source_science_daily", "source_nasa", "source_guardian_environment", "source_bbc_sport", "source_guardian_culture")
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
		fmt.Fprint(w, `<!doctype html><html lang="de" data-theme="dark"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-utilities.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/tokens.css"><link rel="stylesheet" href="/css/config.css"><link rel="stylesheet" href="/css/enhancements.css"><link rel="stylesheet" href="/css/precision-workspace.css"><link rel="stylesheet" href="/css/precision-pages.css"><link rel="stylesheet" href="/css/config-workspace.css">
</head><body class="pw-page" data-workspace-page="config" data-density="comfortable"><main class="cfg-content pw-page-frame" id="content"></main><script>var configData={newspaper:{enabled:true,budget_mode:'auto',overview_sources:['google_news'],max_pages:60,max_searches:32,max_minutes:60}},labels={};window.previewRequests=[];const originalFetch=window.fetch.bind(window);window.fetch=(url,opts={})=>{if(String(url).includes('/budget-preview'))window.previewRequests.push(JSON.parse(opts.body||'{}'));return originalFetch(url,opts)};function t(k){return labels[k]||k}function escapeHtml(s){return String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;')}function attachChangeListeners(){}function toggleBool(){}</script><script src="/cfg/newspaper.js"></script><script>(async()=>{labels=await(await fetch('/lang/config/newspaper/de.json')).json();renderNewspaperSection({label:'Newspaper',desc:'Recherche'});})();</script></body></html>`)
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
	if got := page.MustElements("[data-overview-source]"); len(got) != 10 {
		t.Fatalf("overview source choices=%d, want 10", len(got))
	}
	for _, source := range []string{"tagesschau", "bbc_business", "science_daily", "nasa", "guardian_environment", "bbc_sport", "guardian_culture"} {
		page.MustElement(`[data-overview-source="` + source + `"]`).MustClick()
	}
	page.MustWait(`() => window.previewRequests.at(-1)?.overview_sources?.length === 10`)
	if !page.MustEval(`()=>document.querySelector('[data-path="newspaper.overview_sources"]').value.split(', ').length===10`).Bool() {
		t.Fatal("new overview choices were not included in the saved config value")
	}
	page.MustElement(`[data-overview-source="nasa"]`).MustFocus()
	page.Keyboard.MustType(input.Space)
	page.MustWait(`() => window.previewRequests.at(-1)?.overview_sources?.length === 9`)
	page.Keyboard.MustType(input.Space)
	page.MustWait(`() => window.previewRequests.at(-1)?.overview_sources?.length === 10`)
	for _, width := range []int{390, 1280} {
		page.MustSetViewport(width, 1100, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`theme => {document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme}`, theme)
			page.MustElement("#newspaper-overview-sources").MustScrollIntoView()
			if !page.MustEval(`()=>{
const grid=document.querySelector('#newspaper-overview-sources'), choices=[...grid.querySelectorAll('label')], actions=grid.nextElementSibling, buttons=[...actions.querySelectorAll('button')], rect=grid.getBoundingClientRect();
return getComputedStyle(grid).display==='grid' && choices.every((choice,i)=>{const r=choice.getBoundingClientRect();return r.height>=44&&r.left>=rect.left-1&&r.right<=rect.right+1&&choices.slice(i+1).every(other=>{const b=other.getBoundingClientRect();return r.right+7<=b.left||b.right+7<=r.left||r.bottom+7<=b.top||b.bottom+7<=r.top})})
&& getComputedStyle(actions).flexWrap==='wrap' && parseFloat(getComputedStyle(actions).gap)>=8 && buttons.every(button=>button.getBoundingClientRect().right<=innerWidth+1) && document.documentElement.scrollWidth<=innerWidth+1;
}`).Bool() {
				t.Fatalf("overview sources or preset actions overlap/overflow: %s %d", theme, width)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("newspaper-sources-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
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
