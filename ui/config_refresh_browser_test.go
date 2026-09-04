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

	"gopkg.in/yaml.v3"
)

// Serve the real HTML, translations, assets and lazy modules, with local-only API fixtures.
func configRefreshOrigin(t *testing.T) string {
	t.Helper()
	translations := map[string]string{}
	err := filepath.WalkDir("lang", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "en.json" {
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
	metadata, _ := json.Marshal(map[string]any{"systemLang": "en", "buildVersion": "config-refresh-test", "i18n": translations})
	html := strings.NewReplacer("{{.Lang}}", "en", "{{.BuildVersion}}", "config-refresh-test", "{{.TemplateDataJSON}}", string(metadata)).Replace(string(mustReadUIFile(t, "config.html")))
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
						result := page.MustEval(`() => {const c=document.getElementById('content'), save=document.querySelector('.save-bar'); return {overflow:document.documentElement.scrollWidth>innerWidth+1 || c.scrollWidth>c.clientWidth+1, covered:c.getBoundingClientRect().bottom>save.getBoundingClientRect().top+1};}`).Map()
						if result["overflow"].Bool() || result["covered"].Bool() {
							t.Errorf("%s %s %s %d: %v; overflow elements: %s", key, theme, density, width, result, page.MustEval(`() => [...document.querySelectorAll('#content *')].filter(el=>el.getBoundingClientRect().right>innerWidth).map(el=>({tag:el.tagName,cls:el.className,text:el.innerText?.slice(0,90)})).slice(0,12)`).String())
						}
						if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && density == "comfortable" && (width == 1440 || width == 390) && strings.Contains("|overview|server|providers|adguard|sip|local_llm|", "|"+key+"|") {
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
