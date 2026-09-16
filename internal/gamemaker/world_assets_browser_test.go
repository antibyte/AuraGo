package gamemaker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestWorldPackExportBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_WORLD_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_WORLD_BROWSER=1 for Chrome export review")
	}
	enableWorldReviewPacks(t)
	s := newTestService(t)
	p := createTestProject(t, s, "3d")
	dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
	if err := WriteScaffold(dir, p); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dimension := range []string{"2d", "3d"} {
		for _, r := range bundledRuntimeAssets(dimension) {
			data, err := runtimeFS.ReadFile(r.embeddedPath)
			if err != nil {
				t.Fatal(err)
			}
			write(r.projectPath, data)
		}
	}
	for _, name := range []string{"browser-slice.html", "browser-slice.js"} {
		data, err := os.ReadFile("../../assets/game-maker-worlds/" + name)
		if err != nil {
			t.Fatal(err)
		}
		target := "index.html"
		if name == "browser-slice.js" {
			target = "dist/game.js"
		}
		write(target, data)
	}
	manifest, err := readModelManifest("aurago-pirates-3d")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ships-sloop", "people-diver-brass", "animals-reef-shark"} {
		selected, m, err := validateModelSelection([]string{id}, manifest.ID)
		if err != nil {
			t.Fatal(err)
		}
		m.Assets = selected
		data, _ := json.Marshal(m)
		base := "assets/builtin/" + m.ID + "/" + m.Version + "/"
		write(base+"assets/"+id+".json", data)
		for _, f := range modelFiles(selected[0]) {
			data, err := bundledModelFile(f.File, m.ID)
			if err != nil {
				t.Fatal(err)
			}
			write(base+f.File, data)
		}
	}
	atlas, err := readAtlasManifest("aurago-pirates-side")
	if err != nil {
		t.Fatal(err)
	}
	atlas, err = selectAtlasAssets(atlas, []string{"people-diver-brass", "harbor-beach-hut"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(atlas)
	base := "assets/builtin/" + atlas.ID + "/" + atlas.Version + "/"
	write(base+"assets/people-diver-brass.json", data)
	write(base+"assets/harbor-beach-hut.json", data)
	for _, page := range atlas.Atlases {
		data, err := bundledAtlasFile(atlas.ID, page.File)
		if err != nil {
			t.Fatal(err)
		}
		write(base+page.File, data)
	}
	publishExportFixture(t, s, p, dir)
	files := readExportFixture(t, s, p)
	extracted := t.TempDir()
	for name, data := range files {
		path, _, err := secureJoin(extracted, name, true)
		if err != nil {
			t.Fatal(err)
		}
		os.MkdirAll(filepath.Dir(path), 0750)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.StripPrefix("/games/review/", http.FileServer(http.Dir(extracted))))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().Context(ctx).ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(server.URL + "/games/review/").MustWaitLoad()
	page.MustSetViewport(1600, 980, 1, false)
	page.MustWait(`()=>window.worldReview?.ready || window.worldReview?.errors.length>0`)
	observed := page.MustEval(`()=>window.worldReview`).JSON("", "  ")
	t.Log(observed)
	if !page.MustEval(`()=>worldReview.ready && worldReview.independent && worldReview.sockets && worldReview.pixels.seen_frames>2 && worldReview.pixels.invalid_assets===0 && worldReview.errors.length===0`).Bool() {
		t.Fatal("exported runtime review failed")
	}
	if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
		os.MkdirAll(root, 0750)
		page.MustScreenshot(filepath.Join(root, "world-slice.png"))
		os.WriteFile(filepath.Join(root, "world-slice.json"), []byte(observed), 0644)
	}
	if !page.MustEval(`()=>reviewRoof().hidden===1`).Bool() {
		t.Fatal("roof did not hide independently")
	}
	if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
		page.MustScreenshot(filepath.Join(root, "world-slice-open-roof.png"))
	}
	if !page.MustEval(`()=>{const r=reviewLayerCleanup();return r.removed===3&&r.listeners===0}`).Bool() {
		t.Fatal("sprite layers leaked objects or listeners")
	}
	page.MustEval(`()=>disposeWorldReview()`)
	if !page.MustEval(`()=>worldReview.disposed && worldReview.cache===0`).Bool() {
		t.Fatal("resource disposal failed")
	}
}
