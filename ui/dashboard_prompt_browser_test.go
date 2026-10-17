package ui

import (
	"context"
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
	"github.com/go-rod/rod/lib/proto"
)

func TestDashboardPromptLocales(t *testing.T) {
	var english map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/en.json"), &english); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var translated map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/"+lang+".json"), &translated); err != nil {
			t.Fatal(err)
		}
		for key := range english {
			if strings.HasPrefix(key, "dashboard.prompt_") && strings.TrimSpace(translated[key]) == "" {
				t.Errorf("%s missing %s", lang, key)
			}
		}
	}
	main := string(mustReadUIFile(t, "js/dashboard/main.js"))
	if !strings.Contains(main, "createPromptSavingsChart('prompt-savings-chart', promptStats.recent_fits)") {
		t.Fatal("budget chart must use request fits")
	}
}

func TestDashboardPromptStagesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("browser required for prompt dashboard acceptance")
	}
	dashboard := readDashboardMarkup(t)
	id := strings.Index(dashboard, `id="card-prompt"`)
	start := strings.LastIndex(dashboard[:id], "<div")
	end := strings.Index(dashboard[id:], "<!-- 10.") + id
	card := dashboard[start:end]
	html := `<!doctype html><html lang="de" data-theme="dark"><head><meta charset="utf-8">
<link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-utilities.css">
<link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/tokens.css">
<link rel="stylesheet" href="/css/dashboard.css"><link rel="stylesheet" href="/css/precision-workspace.css">
<link rel="stylesheet" href="/css/precision-pages.css">
</head><body class="pw-page" data-workspace-page="dashboard" data-density="comfortable">` + card + `
<script>
window.__errors=[]; addEventListener('error', e => __errors.push(e.message));
window.esc = s => String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.t = k => window.translations?.[k] || k;
window.cv = k => getComputedStyle(document.documentElement).getPropertyValue(k).trim() || '#778899';
window.dashSetHidden = (el, hidden) => el.classList.toggle('is-hidden', hidden);
window.charts = [];
</script><script src="/chart.min.js"></script>
<script src="/js/dashboard/dashboard-widgets.js"></script><script src="/js/dashboard/dashboard-charts.js"></script>
<script>
Chart.defaults.animation=false; Chart.defaults.maintainAspectRatio=false;
window.paint = data => {
 charts.forEach(c => c?.destroy()); renderPromptStats(data);
 charts=[createPromptSizeChart('prompt-size-chart',data.recent),createPromptSavingsChart('prompt-savings-chart',data.recent_fits),createPromptTierChart('prompt-tier-chart',data.tier_distribution),createPromptSectionDistChart('prompt-section-dist-chart',data.avg_section_sizes)];
};
</script></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(html))
			return
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true)
	u, err := l.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { l.Kill(); l.Cleanup() }()
	b := rod.New().Context(ctx).ControlURL(u).MustConnect()
	defer b.Close()
	p := b.MustPage(server.URL).MustWaitLoad()
	defer p.Close()
	for _, lang := range []string{"de", "en"} {
		for _, theme := range []string{"dark", "light"} {
			for _, width := range []int{390, 1360} {
				t.Run(fmt.Sprintf("%s_%s_%d", lang, theme, width), func(t *testing.T) {
					p.MustSetViewport(width, 1000, 1, false)
					p.MustEval(`async (lang, theme) => { document.documentElement.lang=lang; document.documentElement.dataset.theme=theme; window.translations=await (await fetch('/lang/dashboard/'+lang+'.json')).json(); document.querySelectorAll('[data-i18n]').forEach(el=>el.textContent=t(el.dataset.i18n)); }`, lang, theme)
					p.MustEval(`() => {
 window.fixture={total_builds:8,fit_count:2,measured_fit_count:2,avg_raw_len:15821,avg_optimized_len:15814,avg_optimization_pct:7/15821*100,total_format_savings:56,avg_modules_loaded:12,avg_modules_used:3,avg_modules_filtered:9,avg_module_filter_rate_pct:75,avg_filter_savings:24000,avg_fit_input_tokens:1000,avg_fit_output_tokens:800,avg_fit_saved_tokens:200,avg_fit_input_guides:2,avg_fit_output_guides:1,budget_shed_count:1,budget_exceeded_count:0,shed_rate_pct:50,request_count:300,cache_hit_count:250,cache_hit_rate_pct:250/3,shed_section_counts:{'# TOOL GUIDES':1},tier_distribution:{full:8},avg_section_sizes:{modules:12000,context:3821},recent:[{raw_len:15821,optimized_len:15814}],recent_fits:[{input_tokens:1200,output_tokens:800},{input_tokens:800,output_tokens:800}]}; paint(fixture);
 }`)
					result := p.MustEval(`() => {
 const expectedSmall=document.documentElement.lang==='de'?'< 0,1 %':'< 0.1 %';
 const clipped=[...document.querySelectorAll('#card-prompt canvas, #card-prompt .prompt-kpi')].some(el=>{const r=el.getBoundingClientRect();return r.right>innerWidth+1 || r.left<0;});
 return {small:document.getElementById('prompt-kpis').textContent.includes(expectedSmall),zero:formatPromptNumber(0,true)==='0 %',missing:formatPromptNumber(undefined)==='—',before:charts[1].data.datasets[0].data.join(','),after:charts[1].data.datasets[1].data.join(','),overflow:clipped||document.documentElement.scrollWidth>innerWidth+1,errors:__errors};
 }`)
					if !result.Get("small").Bool() || !result.Get("zero").Bool() || !result.Get("missing").Bool() || result.Get("before").Str() != "1200,800" || result.Get("after").Str() != "800,800" || result.Get("overflow").Bool() || len(result.Get("errors").Arr()) != 0 {
						t.Fatalf("dashboard failure: %s", result.JSON("", ""))
					}
					if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						shot, err := p.Screenshot(true, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("prompt-%s-%s-%d.png", lang, theme, width)), shot, 0644); err != nil {
							t.Fatal(err)
						}
					}
					ok := p.MustEval(`() => {
 const older={...fixture}; delete older.avg_modules_filtered; delete older.avg_fit_input_tokens; paint(older);
 const unknown=document.getElementById('prompt-module-kpis').textContent.includes('—') && document.getElementById('prompt-fit-kpis').textContent.includes('—');
 paint({...fixture,avg_section_sizes:{modules:10000,guides:1}});
 const tinySection=document.getElementById('prompt-section-legend').textContent.includes(formatPromptNumber(0.01,true)) && charts[3].options.plugins.tooltip.callbacks.label({label:'Guide',parsed:1}).includes(formatPromptNumber(0.01,true));
 paint({...fixture,total_builds:0,recent:[],tier_distribution:{},avg_section_sizes:{}});
 const fitOnly=!document.getElementById('prompt-content').classList.contains('is-hidden') && document.getElementById('prompt-kpis').textContent.includes(t('dashboard.prompt_no_measurements'));
 paint({total_builds:0,fit_count:0});
 return unknown && tinySection && fitOnly && document.getElementById('prompt-content').classList.contains('is-hidden') && !document.getElementById('prompt-no-data').classList.contains('is-hidden') && __errors.length===0;
 }`).Bool()
					if !ok {
						t.Fatal("legacy, fit-only or empty-state handling failed")
					}
				})
			}
		}
	}
}
