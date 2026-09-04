package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"gopkg.in/yaml.v3"
)

// Serve the real HTML, translations, assets and lazy modules, with local-only API fixtures.
func configRefreshOrigin(t *testing.T) string {
	return configRefreshFixtureOrigin(t, "en", false)
}

func TestConfigRefreshReferenceScrollBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		t.Skip("set AURAGO_BROWSER_ARTIFACT_DIR to capture the visual references")
	}
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)
	for _, key := range []string{"server", "adguard", "space_agent", "local_llm", "sip", "telephone_agent", "three_d_printers"} {
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`async (key,theme) => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; AuraPrecisionWorkspace.setDensity('comfortable'); await selectSection(key,{scrollBehavior:'auto'}); document.querySelectorAll('#content details').forEach(el=>el.open=true); resetDirtySnapshot();}`, key, theme)
			for frame := 0; frame < 20; frame++ {
				more := page.MustEval(`frame => {const el=document.getElementById('content'); el.scrollTop=frame*(el.clientHeight-48); return el.scrollTop+el.clientHeight<el.scrollHeight;}`, frame).Bool()
				page.MustEval(`() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("scroll-%s-%s-%02d.png", key, theme, frame)), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
				if !more {
					break
				}
			}
		}
	}
}

func TestConfigRefreshPopulatedBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => ['server','providers','sip'].forEach(recordRecentSection)`)
	sections := page.MustEval(`() => ['overview', ...SECTIONS.flatMap(group => group.items.map(item => item.key))]`).Arr()
	for _, section := range sections {
		key := section.Str()
		t.Run(key, func(t *testing.T) {
			page.MustSetViewport(1440, 1000, 1, false)
			page.MustEval(`async key => { document.documentElement.dataset.theme='dark'; document.body.dataset.theme='dark'; AuraPrecisionWorkspace.setDensity('comfortable'); await selectSection(key, {scrollBehavior:'auto'}); resetDirtySnapshot(); }`, key)
			page.MustEval(`() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`)
			if page.MustEval(`() => !!document.querySelector('.cfg-error-state')`).Bool() {
				t.Fatal("populated renderer failed")
			}
			if key == "overview" && !page.MustEval(`() => document.querySelector('.save-bar').hidden && !document.querySelector('.pw-overview-stat')`).Bool() {
				t.Error("overview retains form actions or the dominant section counter")
			}
			if strings.Contains("|server|adguard|space_agent|local_llm|", "|"+key+"|") {
				if !page.MustEval(`() => {
				 const cards=[...document.querySelectorAll('#content .cfg-topic')].filter(el=>!el.parentElement.closest('.cfg-topic'));
				 return cards.length>=3 && cards.every(el=>getComputedStyle(el).borderRadius==='16px' && !!el.querySelector('.pw-panel-heading, .cfg-topic-heading')) &&
				 [...document.querySelectorAll('#content .field-group > .field-label')].every(el=>getComputedStyle(el).fontSize==='16px');
				}`).Bool() {
					t.Error("reference does not share the named topic and field contract")
				}
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "populated-"+key+".png"), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, width := range []int{390, 768, 1024, 1440, 1920} {
				page.MustSetViewport(width, 900, 1, width == 390)
				for _, theme := range []string{"dark", "light"} {
					for _, density := range []string{"comfortable", "compact"} {
						page.MustEval(`(theme,density) => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; AuraPrecisionWorkspace.setDensity(density);}`, theme, density)
						page.MustEval(`() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`)
						if page.MustEval(`() => {const content=document.getElementById('content'), dock=document.querySelector('.save-bar'); return content.scrollWidth>content.clientWidth+1 || (!dock.hidden && content.getBoundingClientRect().bottom>dock.getBoundingClientRect().top+1);}`).Bool() {
							t.Errorf("populated form overlaps or overflows: %d %s %s; %s", width, theme, density, page.MustEval(`() => [...document.querySelectorAll('#content, #content *')].filter(el=>el.clientWidth>0 && el.scrollWidth>el.clientWidth+1).map(el=>({cls:el.className,sw:el.scrollWidth,cw:el.clientWidth})).slice(0,12)`).String())
						}
						if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && strings.Contains("|overview|server|providers|adguard|space_agent|sip|local_llm|", "|"+key+"|") {
							if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("populated-%s-%s-%s-%d.png", key, theme, density, width)), page.MustScreenshot(), 0o644); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			}
		})
	}
	// A 1440px display at 125% zoom has a 1152px CSS viewport.
	page.MustSetViewport(1152, 720, 1.25, false)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"overview", "server", "adguard", "space_agent", "local_llm", "sip", "providers"} {
		page.MustEval(`async key => { await selectSection(key); resetDirtySnapshot(); }`, key)
		if !page.MustEval(`() => matchMedia('(prefers-reduced-motion: reduce)').matches && [...document.querySelectorAll('button, .toggle, summary')].every(el => getComputedStyle(el).animationName==='none') && document.getElementById('content').scrollWidth<=document.getElementById('content').clientWidth+1`).Bool() {
			t.Errorf("reduced motion or zoom contract failed: %s", key)
		}
	}
}

func configRefreshFixtureOrigin(t *testing.T, locale string, populated bool) string {
	t.Helper()
	translations := map[string]string{}
	err := filepath.WalkDir("lang", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != locale+".json" {
			return nil
		}
		var bundle map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, path), &bundle); err == nil {
			for key, value := range bundle {
				translations[key] = value
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../config_template.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var configuration map[string]any
	if err := yaml.Unmarshal(raw, &configuration); err != nil {
		t.Fatal(err)
	}
	// Runtime config exposes Budget at the root; the template's legacy placement
	// under agent is not the schema consumed by this page.
	configuration["budget"] = configuration["agent"].(map[string]any)["budget"]
	delete(configuration["agent"].(map[string]any), "budget")
	if populated {
		// All requests remain on the fixture server. Enabled settings exercise the
		// real conditional forms without contacting or changing an installation.
		for _, value := range configuration {
			if section, ok := value.(map[string]any); ok {
				if _, exists := section["enabled"]; exists {
					section["enabled"] = true
				}
			}
		}
		configuration["adguard"] = map[string]any{"enabled": true, "url": "http://adguard.fixture.invalid", "username": "home-lab", "password": "••••••••"}
		configuration["space_agent"] = map[string]any{"enabled": true, "public_url": "https://space.fixture.invalid", "admin_user": "home-lab", "admin_password": "••••••••"}
		configuration["heartbeat"] = map[string]any{"enabled": true, "check_tasks": true, "check_appointments": true, "check_emails": true}
		configuration["agent"].(map[string]any)["allow_mcp"] = true
		configuration["tools"].(map[string]any)["document_creator"].(map[string]any)["enabled"] = true
		configuration["three_d_printers"] = map[string]any{"enabled": true, "klipper": map[string]any{"enabled": true, "printers": []any{map[string]any{"id": "fixture-printer", "name": "Home Lab · Voron 2.4", "url": "http://printer.fixture.invalid", "api_key": "••••••••"}}}}
	}
	var schemaFor func(map[string]any, string) []map[string]any
	schemaFor = func(data map[string]any, prefix string) []map[string]any {
		result := []map[string]any{}
		keys := make([]string, 0, len(data))
		for key := range data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := data[key]
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			field := map[string]any{"key": path, "yaml_key": key, "type": "string"}
			switch typed := value.(type) {
			case map[string]any:
				field["type"] = "object"
				field["children"] = schemaFor(typed, path)
			case bool:
				field["type"] = "bool"
			case int:
				field["type"] = "int"
			case float64:
				field["type"] = "float"
			case []any:
				field["type"] = "array"
			}
			result = append(result, field)
		}
		return result
	}
	metadata, _ := json.Marshal(map[string]any{"systemLang": locale, "buildVersion": "config-refresh-test", "i18n": translations})
	html := strings.NewReplacer("{{.Lang}}", locale, "{{.BuildVersion}}", "config-refresh-test", "{{.TemplateDataJSON}}", string(metadata)).Replace(string(mustReadUIFile(t, "config.html")))
	fixtures := map[string]any{
		"/api/config": configuration, "/api/config/schema": schemaFor(configuration, ""),
		"/api/vault/status":    map[string]any{"exists": true},
		"/api/providers":       []any{map[string]any{"id": "fixture-main", "name": "Home lab", "type": "openai", "model": "fixture-model", "has_api_key": true, "references": []any{}, "effective_context_window": 32768, "effective_max_output_tokens": 4096}},
		"/api/providers/types": []any{}, "/api/providers/catalog": map[string]any{"providers": []any{}},
		"/api/personalities": map[string]any{"personalities": []any{}},
		"/api/runtime":       map[string]any{"runtime": map[string]any{}, "features": map[string]any{}},
		"/api/sip/config":    map[string]any{}, "/api/sip/providers": map[string]any{"providers": []any{}},
		"/api/sip/status": map[string]any{"registered": false}, "/api/sip/app/state": map[string]any{"blockers": []any{}},
		"/api/local-llm/status": map[string]any{"state": "disabled", "release_manifest_ready": false},
	}
	if populated {
		fixtures["/api/realtime-speech/config"] = map[string]any{"enabled": true, "default_profile": "fixture-voice", "profiles": []any{map[string]any{"id": "fixture-voice", "name": "Home Lab · Sprachassistent", "provider": "openai", "model": "fixture-realtime", "voice": "fixture-voice", "enabled": true, "api_key_set": true}}}
		fixtures["/api/realtime-speech/catalog"] = map[string]any{"providers": []any{map[string]any{"id": "openai", "label": "OpenAI", "models": []any{map[string]any{"id": "fixture-realtime", "label": "Realtime"}}, "voices": []any{map[string]any{"id": "fixture-voice", "label": "Home Lab"}}}}}
		fixtures["/api/sql-connections"] = []any{map[string]any{"id": "fixture-db", "name": "Home Lab", "driver": "postgres", "host": "database.fixture.invalid", "port": 5432, "database_name": "home_lab", "description": "Lokale Vorschau", "allow_read": true}}
		fixtures["/api/email-accounts"] = []any{map[string]any{"id": "fixture-mail", "name": "Home Lab", "enabled": true, "username": "team@example.invalid", "imap_host": "imap.fixture.invalid", "imap_port": 993, "smtp_host": "smtp.fixture.invalid", "smtp_port": 465, "password": "••••••••"}}
		fixtures["/api/mcp-servers"] = []any{map[string]any{"name": "Home Lab · Dokumentensuche", "enabled": true, "transport": "http", "url": "https://mcp.fixture.invalid", "allowed_tools": []string{"search"}}}
		fixtures["/api/mcp-secrets"] = map[string]any{"secrets": []any{}}
		fixtures["/api/mcp-preferences"] = map[string]any{}
		fixtures["/api/cheatsheets"] = []any{}
		fixtures["/api/sip/agent"] = map[string]any{"config": map[string]any{}, "blockers": []string{"asr_unavailable"}}
		fixtures["/api/sip/agent/catalog"] = map[string]any{"providers": []any{}, "tools": []any{}}
		fixtures["/api/config/rules"] = map[string]any{"rules": []any{}, "candidates": map[string]any{}}
		fixtures["/api/adguard/status"] = map[string]any{"status": "ok", "data": map[string]any{"version": "0.107.77", "running": true}}
		fixtures["/api/sip/config"] = map[string]any{"enabled": true, "preset_id": "fritzbox", "display_name": "FRITZ!Box", "registrar": "sip.fixture.invalid", "username": "home-lab", "password_set": true, "permissions": map[string]any{"originate_outbound": true}, "browser_media": map[string]any{"enabled": true}}
		fixtures["/api/sip/status"] = map[string]any{"registered": true}
		fixtures["/api/sip/providers"] = map[string]any{"providers": []any{map[string]any{"id": "fritzbox", "name": "FRITZ!Box", "category": "local", "region": "LAN", "fields": []any{
			map[string]any{"key": "registrar", "label_key": "config.sip.registrar", "required": true},
			map[string]any{"key": "username", "label_key": "config.sip.username", "required": true},
			map[string]any{"key": "password", "label_key": "config.sip.password", "secret": true, "required": true},
		}}}}
		fixtures["/api/local-llm/status"] = map[string]any{"state": "running", "release_manifest_ready": true, "model_family": "qwen", "backend": "cuda"}
	}
	var fixtureMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self' data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self'")
		if r.URL.Path == "/config" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			fixtureMu.Lock()
			defer fixtureMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/config" && r.Method == http.MethodPut {
				var patch map[string]any
				if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
					w.WriteHeader(400)
					return
				}
				var merge func(map[string]any, map[string]any)
				merge = func(target, update map[string]any) {
					for key, value := range update {
						if nested, ok := value.(map[string]any); ok {
							current, ok := target[key].(map[string]any)
							if !ok {
								current = map[string]any{}
								target[key] = current
							}
							merge(current, nested)
						} else {
							target[key] = value
						}
					}
				}
				merge(configuration, patch)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
				return
			}
			if value, found := fixtures[r.URL.Path]; found {
				_ = json.NewEncoder(w).Encode(value)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "fixture_unavailable", "message": "Service unavailable in local preview"})
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestConfigRefreshRealSectionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshOrigin(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	sections := page.MustEval(`() => ['overview', ...SECTIONS.flatMap(group => group.items.map(item => item.key))]`).Arr()
	for _, section := range sections {
		key := section.Str()
		t.Run(key, func(t *testing.T) {
			page.MustSetViewport(1440, 900, 1, false)
			page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto'}); document.querySelectorAll('#content details').forEach(details => details.open=true); resetDirtySnapshot(); }`, key)
			page.MustEval(`() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`)
			orphans := page.MustEval(`() => [...document.querySelectorAll('#content .field-group')].filter(el => !el.closest('.cfg-topic, .cfg-object, [role="dialog"], .modal-overlay, details')).map(el => ({parent:el.parentElement.className, field:el.querySelector('[data-path]')?.dataset.path, label:el.textContent.trim().slice(0,70)}))`).Arr()
			if len(orphans) != 0 {
				t.Errorf("fields outside declared topic or object: %v", orphans)
			}
			result := page.MustEval(`() => ({text: document.getElementById('content').innerText, error: !!document.querySelector('.cfg-error-state'), overflow: document.documentElement.scrollWidth > innerWidth + 1})`).Map()
			if result["error"].Bool() || result["text"].Str() == "" {
				t.Errorf("section did not render: %s", result["text"].Str())
			}
			if result["overflow"].Bool() {
				t.Error("page has horizontal overflow")
			}
		})
	}
	for _, section := range sections {
		key := section.Str()
		t.Run("matrix/"+key, func(t *testing.T) {
			page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto'}); resetDirtySnapshot(); }`, key)
			for _, width := range []int{390, 768, 1024, 1440, 1920} {
				page.MustSetViewport(width, 900, 1, width == 390)
				for _, theme := range []string{"dark", "light"} {
					for _, density := range []string{"comfortable", "compact"} {
						page.MustEval(`(theme, density) => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; AuraPrecisionWorkspace.setDensity(density);}`, theme, density)
						page.MustEval(`() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`)
						result := page.MustEval(`() => {const c=document.getElementById('content'), save=document.querySelector('.save-bar'); return {overflow:document.documentElement.scrollWidth>innerWidth+1 || c.scrollWidth>c.clientWidth+1, covered:!save.hidden && c.getBoundingClientRect().bottom>save.getBoundingClientRect().top+1};}`).Map()
						if result["overflow"].Bool() || result["covered"].Bool() {
							t.Errorf("%s %s %s %d: %v; overflow elements: %s", key, theme, density, width, result, page.MustEval(`() => [...document.querySelectorAll('#content *')].filter(el=>el.getBoundingClientRect().right>innerWidth).map(el=>({tag:el.tagName,cls:el.className,text:el.innerText?.slice(0,90)})).slice(0,12)`).String())
						}
						if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && density == "comfortable" && (width == 1440 || width == 390) && strings.Contains("|overview|server|providers|adguard|space_agent|sip|local_llm|", "|"+key+"|") {
							page.MustEval(`() => Promise.all(document.getAnimations().filter(a => a.effect?.getTiming().iterations !== Infinity).map(a => a.finished.catch(() => {})))`)
							if err := os.MkdirAll(dir, 0o755); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("refresh-%s-%s-%d.png", key, theme, width)), page.MustScreenshot(), 0o644); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			}
		})
	}
}
