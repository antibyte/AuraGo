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
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Two real touch pointers exercise the exported Voxel runtime.
func TestVoxelTouchMineSurvivesCanvasLookRelease(t *testing.T) {
	if os.Getenv("GAMEMAKER_VOXEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_VOXEL_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	s := newTestService(t)
	p, dir, v := voxelFixture(t, s)
	v.Terrain = "flat"
	v.Enemies = nil
	raw, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, "src", "voxel.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if build := buildDirectory(context.Background(), dir, 100, 32<<20); !build.OK {
		t.Fatal(build.Diagnostics)
	}
	publishExportFixture(t, s, p, dir)
	files := readExportFixture(t, s, p)
	extracted := t.TempDir()
	for name, data := range files {
		dest, _, err := secureJoin(extracted, name, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.StripPrefix("/game/", http.FileServer(http.Dir(extracted))))
	defer server.Close()
	page := browser.MustPage("about:blank").Timeout(45 * time.Second)
	defer page.Close()
	page.MustSetViewport(390, 844, 1, false)
	maxTouch := 3
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &maxTouch}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEvalOnNewDocument(`window.voxel=()=>window.__AURAGO_GAME_TEST__?.observeVoxel()`)
	page.MustNavigate(server.URL + "/game/").MustWaitLoad()
	page.MustWait(`()=>voxel()?.ready`)
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>!voxel().paused`)
	page.Touch.MustTap(190, 260)
	page.MustWait(`()=>!document.querySelector('[data-voxel-actions]').hidden`)
	point := page.MustEval(`()=>{const m=document.querySelector('[data-voxel-action="primary"]').getBoundingClientRect(),c=document.querySelector('canvas').getBoundingClientRect();return {mx:m.x+m.width/2,my:m.y+m.height/2,lx:c.x+c.width/2,ly:c.y+c.height/2,target:voxel().target,metrics:voxel().metrics}}`)
	page.MustEval(`()=>{window.auditInputTrace=[];for(const n of ['pointerdown','pointerup','pointercancel','lostpointercapture'])document.addEventListener(n,e=>window.auditInputTrace.push({type:n,id:e.pointerId,target:e.target.dataset.voxelAction||e.target.tagName}),true)}`)
	base := point.Get("metrics").Get("mined").Int()
	one, two := float64(1), float64(2)
	mine := &proto.InputTouchPoint{X: point.Get("mx").Num(), Y: point.Get("my").Num(), ID: &one}
	look := &proto.InputTouchPoint{X: point.Get("lx").Num(), Y: point.Get("ly").Num(), ID: &two}
	page.Touch.MustStart(mine, look)
	page.Touch.MustMove(mine, &proto.InputTouchPoint{X: look.X + 1, Y: look.Y, ID: &two})
	if err := (proto.InputDispatchTouchEvent{Type: proto.InputDispatchTouchEventTypeTouchEnd, TouchPoints: []*proto.InputTouchPoint{look}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>new Promise(r=>setTimeout(r,1000))`)
	if page.MustEval(`()=>voxel().metrics.mined`).Int() <= base {
		t.Fatal("releasing canvas look cancelled held Mine:", page.MustEval(`()=>({state:voxel(),trace:auditInputTrace})`).String())
	}
	if !page.MustEval(`()=>auditInputTrace.some(e=>e.type==='pointerup'&&e.target==='CANVAS')&&!auditInputTrace.some(e=>e.type==='pointerup'&&e.target==='primary')`).Bool() {
		t.Fatal("fixture did not retain Mine while releasing only the camera pointer")
	}
	page.Touch.MustCancel()
}
