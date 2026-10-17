package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
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

// TestNightlyExportBrowser accepts one unmodified 2D starter and one guided 3D
// starter through the real publish/export path. It skips unless
// GAMEMAKER_NIGHTLY_BROWSER=1, so ordinary package tests do not need Chrome.
// The larger GAMEMAKER_*_BROWSER suites stay behind their own flags.
func TestNightlyExportBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_NIGHTLY_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_NIGHTLY_BROWSER=1 to accept one 2D and one guided 3D export")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("Chrome not found at %s: %v", bin, err)
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).
		Set("disable-dev-shm-usage", "enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	t.Cleanup(func() {
		browser.Close()
		launch.Cleanup()
	})
	for _, tc := range []struct {
		name, dimension, template string
	}{
		{"platformer", "2d", "platformer"},
		{"exploration", "3d", "exploration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, project, root := installNightlyStarter(t, tc.dimension, tc.template)
			// Production ceilings. newTestService uses smaller fixture caps that
			// a new guided job, with its catalog models, does not build under.
			if build := buildDirectory(context.Background(), root, 250, 100<<20); !build.OK {
				t.Fatal(build.Diagnostics)
			}
			publishExportFixture(t, service, project, root)
			acceptNightlyExport(t, browser, service, project, tc.dimension)
		})
	}
}

func installNightlyStarter(t *testing.T, dimension, template string) (*Service, Project, string) {
	t.Helper()
	service := newTestService(t)
	project := createTestProject(t, service, dimension)
	root := filepath.Join(service.opts.WorkspacePath, project.ProjectKey)
	if err := WriteScaffold(root, project); err != nil {
		t.Fatal(err)
	}
	plan := ExampleGamePlan(project)
	plan.Template = template
	plan.Assets = nil
	if dimension == "3d" {
		manifest, err := readModelManifest(ModelPackID)
		if err != nil {
			t.Fatal(err)
		}
		for _, role := range defaultModelRoles(template) {
			plan.Assets = append(plan.Assets, PlanAsset{
				Role: role.Role, PackID: role.PackID, Version: manifest.Version,
				AssetID: role.AssetID, Scale: 1,
			})
		}
		installNightlyCatalogModels(t, root, plan.Assets)
	}
	if err := installGameTemplate(root, plan); err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(filepath.Join(root, "src", "main.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if dimension == "2d" {
		want, err := gameTemplates.ReadFile("templates/" + template + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(main, want) {
			t.Fatal("2D export is not the unmodified starter")
		}
	} else if !bytes.Contains(main, []byte(`"mode":"exploration"`)) || !bytes.Contains(main, []byte(`"objects":[{`)) {
		t.Fatal("guided 3D export is not the exploration starter")
	}
	return service, project, root
}

// Copy the same catalog bytes a new guided job imports after plan acceptance.
func installNightlyCatalogModels(t *testing.T, root string, assets []PlanAsset) {
	t.Helper()
	seen := map[string]bool{}
	licensed := map[string]bool{}
	for _, asset := range assets {
		if asset.PackID == "" || seen[asset.PackID+"/"+asset.AssetID] {
			continue
		}
		seen[asset.PackID+"/"+asset.AssetID] = true
		selected, manifest, err := validateModelSelection([]string{asset.AssetID}, asset.PackID)
		if err != nil {
			t.Fatal(err)
		}
		if asset.Version != manifest.Version || len(selected) != 1 {
			t.Fatalf("catalog model %s@%s", asset.AssetID, manifest.Version)
		}
		base := "assets/builtin/" + manifest.ID + "/" + manifest.Version + "/"
		if !licensed[manifest.ID] {
			license, err := bundledModelFile("LICENSE.txt", manifest.ID)
			if err != nil {
				t.Fatal(err)
			}
			writeNightlyFile(t, root, base+"LICENSE.txt", license)
			licensed[manifest.ID] = true
		}
		for _, file := range modelFiles(selected[0]) {
			data, err := bundledModelFile(file.File, manifest.ID)
			if err != nil {
				t.Fatal(err)
			}
			if int64(len(data)) != file.Bytes || sha256Bytes(data) != file.SHA256 {
				t.Fatalf("3D asset integrity mismatch: %s", file.File)
			}
			writeNightlyFile(t, root, base+file.File, data)
		}
		copy := manifest
		copy.Categories = nil
		copy.Assets = []ModelAsset{selected[0]}
		data, err := json.MarshalIndent(copy, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeNightlyFile(t, root, base+"assets/"+asset.AssetID+".json", append(data, '\n'))
	}
}

func writeNightlyFile(t *testing.T, root, name string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		t.Fatal(err)
	}
}

func acceptNightlyExport(t *testing.T, browser *rod.Browser, service *Service, project Project, dimension string) {
	t.Helper()
	exported := readExportFixture(t, service, project)
	var sounds []string
	var models int
	for name := range exported {
		if strings.HasPrefix(name, ".") || strings.Contains(name, "preview-tests") {
			t.Fatalf("private file in exported game: %s", name)
		}
		if strings.HasSuffix(name, ".wav") {
			sounds = append(sounds, name)
		}
		if strings.Contains(name, "assets/builtin/"+ModelPackID+"/") {
			models++
		}
	}
	for _, required := range []string{"game.json", "index.html", "src/main.ts", "dist/game.js"} {
		if len(exported[required]) == 0 {
			t.Fatalf("export missing %s", required)
		}
	}
	if dimension == "3d" && models == 0 {
		t.Fatal("guided 3D export has no catalog models")
	}
	if bytes.Contains(exported["index.html"], []byte("__AURAGO_PREVIEW_BOOT__")) || bytes.Contains(exported["dist/game.js"], []byte("__AURAGO_PREVIEW_BOOT__")) {
		t.Fatal("export contains the preview boot")
	}
	extracted := t.TempDir()
	for name, data := range exported {
		dest, _, err := secureJoin(extracted, name, true)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dest, data, 0o640); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/games/nightly/", http.StripPrefix("/games/nightly/", http.FileServer(http.Dir(extracted))))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	page := browser.MustPage("about:blank").Timeout(90 * time.Second)
	t.Cleanup(func() { page.Close() })
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEvalOnNewDocument(`window.exportErrors=[];addEventListener('error',e=>exportErrors.push(e.message||('resource '+(e.target.src||e.target.href))),true);addEventListener('unhandledrejection',e=>exportErrors.push(String(e.reason)));const oldError=console.error;console.error=(...args)=>{exportErrors.push(args.map(String).join(' '));oldError(...args)}`)
	page.MustNavigate(server.URL + "/games/nightly/").MustWaitLoad()
	if err := page.Timeout(60 * time.Second).Wait(rod.Eval(`()=>document.querySelector('canvas')?.width>0 && !!window.__AURAGO_GAME_TEST__ && !!document.querySelector('[data-player-start]')`)); err != nil {
		t.Fatalf("export startup: %v; errors=%s", err, page.MustEval(`()=>exportErrors`).String())
	}
	page.MustActivate()
	page.MustElement("[data-player-start]").MustClick()
	page.MustElement("canvas").MustClick()
	for _, key := range []input.Key{input.KeyR, input.Space, input.KeyD} {
		if err := page.Keyboard.Press(key); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		if err := page.Keyboard.Release(key); err != nil {
			t.Fatal(err)
		}
	}
	decoded := page.MustEval(`async paths=>{if(!paths?.length)return 0;const c=new AudioContext();try{for(const path of paths){const r=await fetch(path);if(!r.ok)throw Error(path+' '+r.status);const b=await c.decodeAudioData(await r.arrayBuffer());if(!b.length)throw Error('empty audio '+path)}return paths.length}finally{await c.close()}}`, sounds).Int()
	if decoded != len(sounds) {
		t.Fatalf("decoded %d sounds, want %d", decoded, len(sounds))
	}
	if got := page.MustEval(`()=>exportErrors`).String(); got != "[]" {
		t.Fatalf("standalone runtime errors: %s", got)
	}
	if !page.MustEval(`()=>performance.getEntriesByType('resource').every(r=>!/^https?:/.test(r.name)||new URL(r.name).origin===location.origin)`).Bool() {
		t.Fatal("export requested an external runtime resource")
	}
	if page.MustEval(`()=>!!window.__AURAGO_PREVIEW_BOOT__`).Bool() {
		t.Fatal("export relies on preview boot")
	}
}
