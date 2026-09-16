package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
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

// Optional acceptance of real generated projects, copied read-only from their
// published revisions. The fixture directory is ignored; no provider is called.
func TestExportedProjectsBrowser(t *testing.T) {
	root := os.Getenv("GAMEMAKER_EXPORT_PROJECTS")
	if root == "" {
		t.Skip("set GAMEMAKER_EXPORT_PROJECTS to a directory of published game folders")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			source := filepath.Join(root, entry.Name())
			data, err := os.ReadFile(filepath.Join(source, "game.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest gameManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			s := newTestService(t)
			p := createTestProject(t, s, manifest.Dimension)
			dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
			if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
				t.Fatal(err)
			}
			publishExportFixture(t, s, p, dir)
			var output bytes.Buffer
			if _, err := s.WriteExport(context.Background(), p.ID, &output); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
			if err != nil {
				t.Fatal(err)
			}
			extracted := t.TempDir()
			var sounds []string
			for _, f := range archive.File {
				path, _, err := secureJoin(extracted, f.Name, true)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(f.Name, ".") || strings.Contains(f.Name, "preview-tests") {
					t.Fatalf("private file in exported game: %s", f.Name)
				}
				r, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(f.Name, ".wav") {
					sounds = append(sounds, f.Name)
				}
			}
			// Test a subdirectory on an ordinary static host: no AuraGo routes,
			// preview boot, fallback resources, CORS relaxation or test driver.
			mux := http.NewServeMux()
			mux.Handle("/games/export/", http.StripPrefix("/games/export/", http.FileServer(http.Dir(extracted))))
			server := httptest.NewServer(mux)
			defer server.Close()
			page := browser.MustPage("about:blank").Timeout(45 * time.Second)
			defer page.Close()
			page.MustSetViewport(1366, 768, 1, false)
			page.MustEvalOnNewDocument(`window.exportErrors=[];addEventListener('error',e=>exportErrors.push(e.message||('resource '+(e.target.src||e.target.href))),true);addEventListener('unhandledrejection',e=>exportErrors.push(String(e.reason)));const oldError=console.error;console.error=(...args)=>{exportErrors.push(args.map(String).join(' '));oldError(...args)}`)
			page.MustNavigate(server.URL + "/games/export/").MustWaitLoad()
			if err := page.Timeout(20 * time.Second).Wait(rod.Eval(`()=>document.querySelector('canvas')?.width>0 && (!!window.__AURAGO_GAME_TEST__ || /Punkte|Score|Lives|Leben/i.test(document.getElementById('hud')?.textContent||''))`)); err != nil {
				t.Fatalf("export startup: %v; errors=%s", err, page.MustEval(`()=>exportErrors`).String())
			}
			page.MustActivate()
			page.MustElement("canvas").MustClick()
			hold := func(key input.Key) {
				t.Helper()
				if err := page.Keyboard.Press(key); err != nil {
					t.Fatal(err)
				}
				time.Sleep(350 * time.Millisecond)
				if err := page.Keyboard.Release(key); err != nil {
					t.Fatal(err)
				}
			}
			hold(input.KeyR)
			hold(input.Space)
			hold(input.KeyD)
			// Audio decoding is tested independently of hardware output/muting.
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
			report := os.Getenv("GAMEMAKER_EXPORT_REPORTS")
			if report != "" {
				if err := os.MkdirAll(report, 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(report, entry.Name()+".zip"), output.Bytes(), 0640); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(report, entry.Name()+".png"))
			}
			t.Logf("standalone %s: %d ZIP files, %d bytes, %d decoded WAVs; no runtime errors/external resources", manifest.Dimension, len(archive.File), output.Len(), decoded)
		})
	}
}
