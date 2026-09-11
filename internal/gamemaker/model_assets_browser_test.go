package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModelPackBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_MODEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_MODEL_BROWSER=1 for real WebGL checks")
	}
	bin := ""
	for _, candidate := range []string{os.Getenv("CHROME_BIN"), "C:/Program Files/Google/Chrome/Application/chrome.exe", "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe", "/usr/bin/chromium", "/usr/bin/google-chrome"} {
		if candidate != "" {
			if _, err := os.Stat(candidate); err == nil {
				bin = candidate
				break
			}
		}
	}
	if bin == "" {
		t.Fatal("Chrome/Chromium executable required")
	}
	pageData, err := os.ReadFile("../../assets/game-maker-low-poly/browser-check.html")
	if err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/studio" {
			data, _ := os.ReadFile("../../assets/game-maker-low-poly/studio-check.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ui/") {
			http.StripPrefix("/ui/", http.FileServer(http.Dir("../../ui"))).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/game-maker/asset-packs" {
			packs, err := service.ListAssetPacks()
			if err != nil {
				t.Error(err)
			}
			json.NewEncoder(w).Encode(map[string]any{"packs": packs})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/game-maker/asset-packs/") {
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/game-maker/asset-packs/"), "/", 2)
			if len(parts) == 2 {
				data, err := bundledAssetPackFile(parts[0], parts[1])
				if err == nil {
					if strings.HasSuffix(parts[1], ".js") {
						w.Header().Set("Content-Type", "text/javascript")
					} else if strings.HasSuffix(parts[1], ".glb") {
						w.Header().Set("Content-Type", "model/gltf-binary")
					}
					w.Write(data)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(pageData)
			return
		}
		var data []byte
		var err error
		if strings.HasPrefix(r.URL.Path, "/runtime/") {
			data, _, err = bundledRuntimeFile("3d", "vendor/"+strings.TrimPrefix(r.URL.Path, "/runtime/"))
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		} else if strings.HasPrefix(r.URL.Path, "/pack/") {
			name := strings.TrimPrefix(r.URL.Path, "/pack/")
			data, err = bundledModelFile(name)
			if strings.HasSuffix(name, ".glb") {
				w.Header().Set("Content-Type", "model/gltf-binary")
			}
		}
		if err != nil || len(data) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url := launch.MustLaunch()
	defer launch.Cleanup()
	browser := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer browser.Close()
	page := browser.MustPage("about:blank")
	page.MustSetViewport(1920, 1080, 1, false)
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustWait(`() => window.__ready === true`)
	result := page.MustEval(`async () => await inspectModels.smoke()`)
	t.Log(result.String())
	if result.Get("models").Int() != 220 || len(result.Get("errors").Arr()) != 0 {
		t.Fatalf("model runtime: %s", result.String())
	}
	page.MustEval(`async () => await inspectModels.hero()`)
	page.MustEval(`() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	reports, err := filepath.Abs("../../reports/low-poly")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(reports, 0750); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot(filepath.Join(reports, "three-hero.png"))
	var motion []any
	for _, id := range []string{"humans-civilian-a", "animals-dog", "animals-bird"} {
		clip := "walk"
		if id == "animals-bird" {
			clip = "fly"
		}
		for _, phase := range []float64{.1, .35, .6, .85} {
			pose := page.MustEval(`async(id,clip,t)=>await inspectModels.pose(id,clip,t)`, id, clip, phase)
			motion = append(motion, pose.Val())
			if id == "humans-civilian-a" {
				feet := pose.Get("feet").Arr()
				low := min(feet[0].Get("1").Num(), feet[1].Get("1").Num())
				if low < .105 || low > .135 {
					t.Errorf("walking foot lost ground contact: %s", pose.String())
				}
			}
			page.MustScreenshot(filepath.Join(reports, fmt.Sprintf("motion-%s-%s-%.2f.png", id, clip, phase)))
		}
	}
	for _, clip := range []string{"idle", "fire", "reload", "reload_empty"} {
		for _, phase := range []float64{.1, .5, .85} {
			pose := page.MustEval(`async(clip,t)=>await inspectModels.fpsPose(clip,t)`, clip, phase)
			motion = append(motion, pose.Val())
			if pose.Get("rightGripError").Num() > .025 {
				t.Errorf("FPS grip drift: %s", pose.String())
			}
			if strings.HasPrefix(clip, "reload") && phase == .5 && pose.Get("magazineGripError").Num() > .025 {
				t.Errorf("FPS magazine drift: %s", pose.String())
			}
			page.MustScreenshot(filepath.Join(reports, fmt.Sprintf("motion-fps-%s-%.2f.png", clip, phase)))
		}
	}
	motionJSON, _ := json.MarshalIndent(motion, "", "  ")
	os.WriteFile(filepath.Join(reports, "motion-contracts.json"), motionJSON, 0640)
	if err := os.WriteFile(filepath.Join(reports, "browser-contracts.json"), []byte(result.JSON("", "  ")), 0640); err != nil {
		t.Fatal(err)
	}
	page.MustNavigate(server.URL + "/studio").MustWaitLoad()
	page.MustWait(`()=>window.__uiReady===true`)
	page.MustElement("[data-gm-action=assets]").MustClick()
	page.MustWait(`()=>document.querySelectorAll('[data-model]').length===220`)
	page.MustEval(`()=>{const s=document.querySelector('[data-asset-search]');s.value='explorer';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustElement("[data-model='humans-explorer-a']").MustClick()
	page.MustWait(`()=>!!document.querySelector('[data-model-stage] canvas')`)
	page.MustEval(`()=>document.querySelector('[data-select-model="humans-explorer-a"]').click()`)
	if page.MustEval(`()=>__uiState.selectedModelAssetIDs[0]`).Str() != "humans-explorer-a" {
		t.Fatal("model selection lost")
	}
	for _, theme := range []string{"standard", "light", "dark"} {
		for _, density := range []string{"normal", "compact"} {
			for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {390, 844}} {
				page.MustSetViewport(size[0], size[1], 1, size[0] < 500)
				page.MustEval(`(theme,density)=>{document.body.dataset.theme=theme==='standard'?'standard':'fruity';document.body.dataset.fruityMode=theme;document.body.dataset.density=density}`, theme, density)
				page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
				if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
					t.Fatalf("horizontal overflow: %s %s %v", theme, density, size)
				}
				if page.MustEval(`()=>{const m=document.querySelector('.gm-assets-modal'),r=m.getBoundingClientRect();return r.left<0||r.right>innerWidth+1||m.scrollWidth>m.clientWidth+1||document.querySelector('.gm-asset-detail').scrollWidth>document.querySelector('.gm-asset-detail').clientWidth+1}`).Bool() {
					t.Fatalf("clipped model controls: %s %s %v", theme, density, size)
				}
				page.MustScreenshot(filepath.Join(reports, fmt.Sprintf("studio-%s-%s-%d.png", theme, density, size[0])))
			}
		}
	}
	page.MustEval(`()=>{const s=document.querySelector('[data-model-animation]');s.value='walk';s.dispatchEvent(new Event('change'))}`)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,400))`)
	page.MustElement("[data-model-pause]").MustClick()
	page.MustEval(`()=>{const s=document.querySelector('[data-model-lod]');s.value='2';s.dispatchEvent(new Event('change'))}`)
	page.MustEval(`()=>GameMakerStudioModals.closeModal(__uiState)`)
	if page.MustEval(`async()=>{const A=await import('/api/game-maker/asset-packs/runtime/aurago-three-assets-1.js');return A.assetCacheSize()}`).Int() != 0 {
		t.Fatal("Studio viewer retained assets")
	}
	if errors := page.MustEval(`()=>__uiErrors`); len(errors.Arr()) != 0 {
		t.Fatal(errors.String())
	}
	// Model selections must never disappear silently when submitting a 2D job.
	if !page.MustEval(`()=>{
		__uiState.project={id:'two-dimensional',dimension:'2d'};
		const form=document.querySelector('[data-gm-change-form]');
		form.querySelector('textarea').value='Keep this request';
		form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
		const ok=__uiErrors.pop()===__uiState.context.t('game_maker.model_requires_3d')
			&&form.querySelector('textarea').value==='Keep this request'&&!__uiState.jobActive;
		__uiState.project=null;return ok;
	}`).Bool() {
		t.Fatal("2D request silently discarded its 3D selection")
	}
	page.MustEval(`()=>GameMakerStudioApp.dispose('fixture')`)
}
